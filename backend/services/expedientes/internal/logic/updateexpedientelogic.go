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
	_, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if !authorization.CanUpdate(role) {
		return nil, status.Error(codes.PermissionDenied, "el rol no puede editar expedientes")
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
