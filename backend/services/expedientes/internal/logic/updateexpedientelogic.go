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

type UpdateExpedienteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateExpedienteLogic {
	return &UpdateExpedienteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateExpedienteLogic) UpdateExpediente(in *expedientes.UpdateExpedienteRequest) (*expedientes.UpdateExpedienteResponse, error) {
	userID, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	// Permiso real (Paso 13B): reemplaza únicamente el gate de rol
	// (CanUpdate). SOLICITANTE no tiene "expedientes.update" en el catálogo,
	// así que queda bloqueado aquí mismo — su único camino de edición sigue
	// siendo CorregirExpediente (expedientes.view_own + ownership), sin
	// cambios.
	if err := authorization.RequirePermission(l.ctx, l.svcCtx.AuthClient, userID, "expedientes.update"); err != nil {
		return nil, err
	}
	if in == nil || strings.TrimSpace(in.Id) == "" {
		return nil, status.Error(codes.InvalidArgument, "el identificador es obligatorio")
	}

	asunto := strings.TrimSpace(in.Asunto)
	if asunto == "" {
		return nil, status.Error(codes.InvalidArgument, "el asunto es obligatorio")
	}
	if len(asunto) > 255 {
		return nil, status.Error(codes.InvalidArgument, "el asunto no puede superar los 255 caracteres")
	}

	prioridad := strings.ToUpper(strings.TrimSpace(in.Prioridad))
	if prioridad == "" {
		prioridad = estados.PrioridadNormal
	}
	if !estados.IsValidPrioridad(prioridad) {
		return nil, status.Error(codes.InvalidArgument, "la prioridad no es válida")
	}

	// Área (Paso 18B): el permiso ya confirmó que el rol puede editar EN
	// GENERAL, pero no que este expediente EN PARTICULAR le corresponda.
	// Se lee el registro actual (nunca el área que mande el cliente — este
	// RPC no permite tocar area_actual) y se reutiliza EXACTAMENTE la misma
	// regla ya usada por UpdateArea/DerivarExpediente/RechazarExpediente/
	// ResolverExpediente: CanViewAll (ADMIN y SECRETARIA, la mesa de partes,
	// ven/editan cualquier área) o coincidencia AreaActual == AreaDelRol.
	current, err := l.svcCtx.ExpedienteRepository.FindByIDOrCodigo(l.ctx, strings.TrimSpace(in.Id))
	if err != nil {
		if errors.Is(err, repository.ErrExpedienteNotFound) {
			return nil, status.Error(codes.NotFound, "expediente no encontrado")
		}
		return nil, status.Error(codes.Internal, "no se pudo consultar el expediente")
	}
	if !authorization.CanViewAll(role) && current.AreaActual != authorization.AreaDelRol(role) {
		return nil, status.Error(codes.PermissionDenied, "el expediente no pertenece al área del usuario")
	}

	updated, err := l.svcCtx.ExpedienteRepository.Update(
		l.ctx,
		strings.TrimSpace(in.Id),
		asunto,
		strings.TrimSpace(in.Descripcion),
		prioridad,
	)
	if err != nil {
		if errors.Is(err, repository.ErrExpedienteNotFound) {
			return nil, status.Error(codes.NotFound, "expediente no encontrado")
		}
		return nil, status.Error(codes.Internal, "no se pudo actualizar el expediente")
	}

	return &expedientes.UpdateExpedienteResponse{Expediente: toProto(updated)}, nil
}
