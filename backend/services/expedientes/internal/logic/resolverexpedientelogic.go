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

type ResolverExpedienteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolverExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolverExpedienteLogic {
	return &ResolverExpedienteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ResolverExpediente (Etapa 4): exclusivo de F2 (CERTIFICADO/CONSTANCIA).
// Secretaría cierra el expediente una vez que Dirección lo devolvió con el
// documento final — "el cierre final ocurre cuando Secretaría deja
// disponible el documento" (regla institucional de esta etapa), no cuando
// Dirección termina de revisar. La verificación de que el documento final
// existe (Documentos Service) la hace el Gateway antes de llamar acá
// (Expedientes Service no tiene cliente hacia Documentos Service, mismo
// límite de arquitectura ya documentado en etapas anteriores) — esta
// lógica solo valida lo que sí conoce: tipo, área y estado.
func (l *ResolverExpedienteLogic) ResolverExpediente(in *expedientes.ResolverExpedienteRequest) (*expedientes.ResolverExpedienteResponse, error) {
	userID, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	// Permiso real (Paso 13B): reemplaza únicamente el gate de rol
	// (CanChangeEstado). El chequeo de área, EsF2, AreaActual==SECRETARIA
	// y el estado EN_PROCESO (abajo) siguen exactamente igual.
	if err := authorization.RequirePermission(l.ctx, l.svcCtx.AuthClient, userID, "expedientes.change_estado"); err != nil {
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

	if !estados.EsF2(current.Tipo) {
		return nil, status.Error(codes.FailedPrecondition, "resolver no aplica a este tipo de trámite")
	}
	if current.AreaActual != estados.AreaSecretaria {
		return nil, status.Error(codes.FailedPrecondition, "solo Secretaría puede cerrar el expediente, y solo cuando lo tiene de vuelta")
	}
	if current.Estado != estados.EstadoEnProceso {
		return nil, status.Error(codes.FailedPrecondition, "solo se puede resolver un expediente en proceso")
	}

	updated, err := l.svcCtx.ExpedienteRepository.UpdateEstado(l.ctx, strings.TrimSpace(in.Id), current.Estado, estados.EstadoAtendido)
	if err != nil {
		if errors.Is(err, repository.ErrExpedienteNotFound) {
			return nil, status.Error(codes.Aborted, "el expediente cambió, intente nuevamente")
		}
		return nil, status.Error(codes.Internal, "no se pudo resolver el expediente")
	}

	return &expedientes.ResolverExpedienteResponse{Expediente: toProto(updated)}, nil
}
