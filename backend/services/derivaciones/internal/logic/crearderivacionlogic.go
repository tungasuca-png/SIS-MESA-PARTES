package logic

import (
	"context"
	"strings"

	"derivaciones/derivaciones"
	"derivaciones/internal/authorization"
	"derivaciones/internal/interceptor"
	"derivaciones/internal/svc"
	"derivaciones/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CrearDerivacionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCrearDerivacionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CrearDerivacionLogic {
	return &CrearDerivacionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CrearDerivacionLogic) CrearDerivacion(in *derivaciones.CrearDerivacionRequest) (*derivaciones.CrearDerivacionResponse, error) {
	userID, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if !authorization.CanCreate(role) {
		return nil, status.Error(codes.PermissionDenied, "el rol no puede registrar derivaciones")
	}
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "la solicitud es obligatoria")
	}

	expedienteID := strings.TrimSpace(in.ExpedienteId)
	if !validation.IsValidUUID(expedienteID) {
		return nil, status.Error(codes.InvalidArgument, "el expediente_id no es válido")
	}

	tipo := strings.ToUpper(strings.TrimSpace(in.Tipo))
	if !validation.IsValidTipo(tipo) {
		return nil, status.Error(codes.InvalidArgument, "el tipo de derivación no es válido")
	}

	origen := strings.TrimSpace(in.Origen)
	if origen == "" {
		return nil, status.Error(codes.InvalidArgument, "el origen es obligatorio")
	}
	if len(origen) > 100 {
		return nil, status.Error(codes.InvalidArgument, "el origen no puede superar los 100 caracteres")
	}

	destino := strings.TrimSpace(in.Destino)
	if destino == "" {
		return nil, status.Error(codes.InvalidArgument, "el destino es obligatorio")
	}
	if len(destino) > 100 {
		return nil, status.Error(codes.InvalidArgument, "el destino no puede superar los 100 caracteres")
	}

	motivo := strings.TrimSpace(in.Motivo)
	if motivo == "" {
		return nil, status.Error(codes.InvalidArgument, "el motivo es obligatorio")
	}
	if len(motivo) > 500 {
		return nil, status.Error(codes.InvalidArgument, "el motivo no puede superar los 500 caracteres")
	}

	condicion := strings.TrimSpace(in.Condicion)
	if len(condicion) > 255 {
		return nil, status.Error(codes.InvalidArgument, "la condición no puede superar los 255 caracteres")
	}

	created, err := l.svcCtx.DerivacionRepository.Create(l.ctx, expedienteID, tipo, origen, destino, motivo, condicion, userID)
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo registrar la derivación")
	}

	return &derivaciones.CrearDerivacionResponse{Derivacion: toProto(created)}, nil
}
