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

// RechazarExpediente (Etapa 4): exclusivo de F4 (PERMISO/JUSTIFICACION_*)
// — la regla institucional de esta etapa no define un rechazo para F2, así
// que no se inventa uno. Lo ejecuta quien tiene actualmente el expediente
// (DIRECTOR o SUBDIRECTOR, según en qué punto del flujo esté) y lo
// devuelve a SECRETARIA con estado OBSERVADO, para que el solicitante
// corrija (ver CorregirExpediente).
func (l *RechazarExpedienteLogic) RechazarExpediente(in *expedientes.RechazarExpedienteRequest) (*expedientes.RechazarExpedienteResponse, error) {
	userID, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	// Permiso real (Paso 13B): reemplaza únicamente el gate de rol. Misma
	// condición que derivar (Etapa 3): rechazar es, en esencia, una
	// derivación de vuelta a Secretaría — mismo permiso que ya usa
	// UpdateArea/DerivarExpediente. El chequeo de área, EsF4 y el estado
	// EN_PROCESO (abajo) siguen exactamente igual.
	if err := authorization.RequirePermission(l.ctx, l.svcCtx.AuthClient, userID, "expedientes.update"); err != nil {
		return nil, err
	}
	if in == nil || strings.TrimSpace(in.Id) == "" {
		return nil, status.Error(codes.InvalidArgument, "el identificador es obligatorio")
	}

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

	if !estados.EsF4(current.Tipo) {
		return nil, status.Error(codes.FailedPrecondition, "el rechazo no aplica a este tipo de trámite")
	}
	if current.Estado != estados.EstadoEnProceso {
		return nil, status.Error(codes.FailedPrecondition, "solo se puede rechazar un expediente en proceso")
	}

	updated, err := l.svcCtx.ExpedienteRepository.RechazarYDevolver(
		l.ctx, strings.TrimSpace(in.Id), current.Estado, current.AreaActual, estados.AreaSecretaria,
	)
	if err != nil {
		if errors.Is(err, repository.ErrExpedienteNotFound) {
			return nil, status.Error(codes.Aborted, "el expediente cambió, intente nuevamente")
		}
		return nil, status.Error(codes.Internal, "no se pudo rechazar el expediente")
	}

	return &expedientes.RechazarExpedienteResponse{Expediente: toProto(updated)}, nil
}
