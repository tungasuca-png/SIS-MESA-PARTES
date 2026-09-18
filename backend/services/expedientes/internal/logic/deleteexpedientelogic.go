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

type DeleteExpedienteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteExpedienteLogic {
	return &DeleteExpedienteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteExpedienteLogic) DeleteExpediente(in *expedientes.DeleteExpedienteRequest) (*expedientes.DeleteExpedienteResponse, error) {
	userID, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	// Permiso real (Paso 13B): reemplaza únicamente el gate de rol
	// (CanDelete). SOLICITANTE no tiene "expedientes.delete" en el
	// catálogo, así que queda bloqueado aquí mismo, incluso sobre su propio
	// expediente y aunque esté PENDIENTE.
	if err := authorization.RequirePermission(l.ctx, l.svcCtx.AuthClient, userID, "expedientes.delete"); err != nil {
		return nil, err
	}
	if in == nil || strings.TrimSpace(in.Id) == "" {
		return nil, status.Error(codes.InvalidArgument, "el identificador es obligatorio")
	}

	// Estado y área (Paso 18C): el permiso ya confirmó que el rol puede
	// eliminar EN GENERAL; falta confirmar que ESTE expediente puede
	// eliminarse (solo si sigue PENDIENTE, antes de iniciar trámite real) y
	// que le corresponde a este actor — misma regla de área ya usada por
	// UpdateExpediente (Paso 18B) y por UpdateArea/DerivarExpediente/
	// RechazarExpediente/ResolverExpediente: reutiliza CanViewAll/AreaDelRol,
	// sin duplicar el mapeo rol->área. FindByIDOrCodigo ya filtra
	// activo = TRUE, así que un expediente inexistente o ya eliminado
	// produce el mismo NotFound que antes de este paso.
	current, err := l.svcCtx.ExpedienteRepository.FindByIDOrCodigo(l.ctx, strings.TrimSpace(in.Id))
	if err != nil {
		if errors.Is(err, repository.ErrExpedienteNotFound) {
			return nil, status.Error(codes.NotFound, "expediente no encontrado")
		}
		return nil, status.Error(codes.Internal, "no se pudo consultar el expediente")
	}
	if current.Estado != estados.EstadoPendiente {
		return nil, status.Error(codes.PermissionDenied, "solo se puede eliminar un expediente pendiente")
	}
	if !authorization.CanViewAll(role) && current.AreaActual != authorization.AreaDelRol(role) {
		return nil, status.Error(codes.PermissionDenied, "el expediente no pertenece al área del usuario")
	}

	if err := l.svcCtx.ExpedienteRepository.Delete(l.ctx, strings.TrimSpace(in.Id)); err != nil {
		if errors.Is(err, repository.ErrExpedienteNotFound) {
			return nil, status.Error(codes.NotFound, "expediente no encontrado")
		}
		return nil, status.Error(codes.Internal, "no se pudo eliminar el expediente")
	}

	return &expedientes.DeleteExpedienteResponse{Success: true}, nil
}
