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

type RechazarExpedienteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRechazarExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RechazarExpedienteLogic {
	return &RechazarExpedienteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RechazarExpediente (Etapa 4, reglas sin cambios): exclusivo de F4, solo
// EN_PROCESO, lo ejecuta quien tiene el expediente y lo devuelve a SECRETARIA
// como OBSERVADO. Desde A + B: movimiento RECHAZO en la misma transacción,
// área de origen esperada e idempotencia.
func (l *RechazarExpedienteLogic) RechazarExpediente(in *expedientes.RechazarExpedienteRequest) (*expedientes.RechazarExpedienteResponse, error) {
	userID, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if err := authorization.RequirePermission(l.ctx, l.svcCtx.AuthClient, userID, "expedientes.update"); err != nil {
		return nil, err
	}
	if in == nil || strings.TrimSpace(in.Id) == "" {
		return nil, status.Error(codes.InvalidArgument, "el identificador es obligatorio")
	}
	motivo := strings.TrimSpace(in.Motivo)
	if motivo == "" {
		return nil, status.Error(codes.InvalidArgument, "el motivo es obligatorio")
	}
	if len([]rune(motivo)) > 500 {
		return nil, status.Error(codes.InvalidArgument, "el motivo no puede superar los 500 caracteres")
	}
	origenEsperado := strings.ToUpper(strings.TrimSpace(in.AreaOrigenEsperada))
	if !estados.IsValidArea(origenEsperado) {
		return nil, status.Error(codes.InvalidArgument, "el área de origen esperada es obligatoria")
	}
	clave, err := validarClave(in.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	if err := validarRolActor(role); err != nil {
		return nil, err
	}

	res, err := l.svcCtx.ExpedienteRepository.RegistrarMovimiento(l.ctx, repository.SolicitudMovimiento{
		IDOrCodigo:     strings.TrimSpace(in.Id),
		Tipo:           repository.MovRechazo,
		ActorID:        userID,
		ActorRol:       role,
		IdempotencyKey: clave,
		Contenido:      contenido(origenEsperado, motivo),
		Motivo:         motivo,
		Decidir: func(actual *repository.Expediente) (repository.Decision, error) {
			if !authorization.CanViewAll(role) && actual.AreaActual != authorization.AreaDelRol(role) {
				return repository.Decision{}, status.Error(codes.PermissionDenied, "el expediente no pertenece al área del usuario")
			}
			if actual.AreaActual != origenEsperado {
				return repository.Decision{}, status.Errorf(codes.Aborted, "el expediente ya no está en %s; actualice y vuelva a intentarlo", origenEsperado)
			}
			if !estados.EsF4(actual.Tipo) {
				return repository.Decision{}, status.Error(codes.FailedPrecondition, "el rechazo no aplica a este tipo de trámite")
			}
			if actual.Estado != estados.EstadoEnProceso {
				return repository.Decision{}, status.Error(codes.FailedPrecondition, "solo se puede rechazar un expediente en proceso")
			}
			return repository.Decision{AreaDestino: estados.AreaSecretaria, EstadoNuevo: estados.EstadoObservado}, nil
		},
	})
	if err != nil {
		return nil, errorMovimiento(l.ctx, "rechazar", err)
	}
	if !res.Repetido {
		l.svcCtx.Publicador.Avisar() // proyección inmediata (si no, el sondeo periódico)
	}
	return &expedientes.RechazarExpedienteResponse{
		Expediente: toProto(res.Expediente),
		Movimiento: movimientoToProto(res.Movimiento),
		Repetido:   res.Repetido,
	}, nil
}
