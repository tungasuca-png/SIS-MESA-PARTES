package logic

import (
	"context"
	"errors"
	"strings"

	"expedientes/expedientes"
	"expedientes/internal/authorization"
	"expedientes/internal/estados"
	"expedientes/internal/interceptor"
	"expedientes/internal/repository"
	"expedientes/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UpdateAreaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateAreaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAreaLogic {
	return &UpdateAreaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateAreaLogic) UpdateArea(in *expedientes.UpdateAreaRequest) (*expedientes.UpdateAreaResponse, error) {
	userID, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	// Permiso real (Paso 13B): reemplaza únicamente el gate de rol
	// (CanUpdateArea). El chequeo de área (abajo) y el CAS optimista siguen
	// exactamente igual.
	if err := authorization.RequirePermission(l.ctx, l.svcCtx.AuthClient, userID, "expedientes.update"); err != nil {
		return nil, err
	}
	if in == nil || strings.TrimSpace(in.Id) == "" {
		return nil, status.Error(codes.InvalidArgument, "el identificador es obligatorio")
	}

	area := strings.ToUpper(strings.TrimSpace(in.Area))
	if !estados.IsValidArea(area) {
		return nil, status.Error(codes.InvalidArgument, "el área indicada no es válida")
	}

	current, err := l.svcCtx.ExpedienteRepository.FindByIDOrCodigo(l.ctx, strings.TrimSpace(in.Id))
	if err != nil {
		if errors.Is(err, repository.ErrExpedienteNotFound) {
			return nil, status.Error(codes.NotFound, "expediente no encontrado")
		}
		return nil, status.Error(codes.Internal, "no se pudo consultar el expediente")
	}

	// Derivar (mover) un expediente es una acción que le corresponde a quien
	// lo tiene actualmente — no a cualquier interno (ver DerivacionesPanel.jsx
	// del frontend: siempre es el área actual la que deriva hacia otra). Es
	// la misma condición que ChangeEstado por eso: en ambos casos el actor
	// debe ser dueño actual del expediente, no por casualidad sino porque las
	// dos acciones (cambiar su estado o entregarlo a otra área) solo tienen
	// sentido para quien lo tiene en este momento. CanUpdateArea ya garantizó
	// que role es personal interno.
	if !authorization.CanViewAll(role) && current.AreaActual != authorization.AreaDelRol(role) {
		return nil, status.Error(codes.PermissionDenied, "el expediente no pertenece al área del usuario")
	}

	// Concurrencia optimista: se exige que area_actual siga siendo la misma
	// que se acaba de leer (current.AreaActual). Si otra operación la cambió
	// entretanto, la fila no matchea y el repositorio devuelve
	// ErrExpedienteNotFound — acá ya se confirmó arriba que el expediente
	// existe, así que ese error en este punto solo puede significar que el
	// área cambió mientras tanto (mismo criterio que ChangeEstadoLogic).
	updated, err := l.svcCtx.ExpedienteRepository.UpdateArea(l.ctx, strings.TrimSpace(in.Id), current.AreaActual, area)
	if err != nil {
		if errors.Is(err, repository.ErrExpedienteNotFound) {
			return nil, status.Error(codes.Aborted, "el área del expediente cambió, intente nuevamente")
		}
		return nil, status.Error(codes.Internal, "no se pudo actualizar el área del expediente")
	}

	return &expedientes.UpdateAreaResponse{Expediente: toProto(updated)}, nil
}
