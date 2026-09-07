package logic

import (
	"context"
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

type CreateExpedienteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateExpedienteLogic {
	return &CreateExpedienteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateExpedienteLogic) CreateExpediente(in *expedientes.CreateExpedienteRequest) (*expedientes.CreateExpedienteResponse, error) {
	userID, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if !authorization.CanCreate(role) {
		return nil, status.Error(codes.PermissionDenied, "el rol no puede registrar expedientes")
	}
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "la solicitud es obligatoria")
	}

	asunto := strings.TrimSpace(in.Asunto)
	if asunto == "" {
		return nil, status.Error(codes.InvalidArgument, "el asunto es obligatorio")
	}
	if len(asunto) > 255 {
		return nil, status.Error(codes.InvalidArgument, "el asunto no puede superar los 255 caracteres")
	}

	tipo := strings.ToUpper(strings.TrimSpace(in.Tipo))
	if tipo == "" {
		tipo = estados.TipoSolicitud
	}
	if !estados.IsValidTipo(tipo) {
		return nil, status.Error(codes.InvalidArgument, "el tipo de expediente no es válido")
	}

	prioridad := strings.ToUpper(strings.TrimSpace(in.Prioridad))
	if prioridad == "" {
		prioridad = estados.PrioridadNormal
	}
	if !estados.IsValidPrioridad(prioridad) {
		return nil, status.Error(codes.InvalidArgument, "la prioridad no es válida")
	}

	created, err := l.svcCtx.ExpedienteRepository.Create(l.ctx, &repository.Expediente{
		Tipo:          tipo,
		Asunto:        asunto,
		Descripcion:   strings.TrimSpace(in.Descripcion),
		SolicitanteID: userID,
		Prioridad:     prioridad,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo registrar el expediente")
	}

	return &expedientes.CreateExpedienteResponse{Expediente: toProto(created)}, nil
}
