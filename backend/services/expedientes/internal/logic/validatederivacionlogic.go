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

type ValidateDerivacionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewValidateDerivacionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ValidateDerivacionLogic {
	return &ValidateDerivacionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ValidateDerivacion (Paso 20B.2): RPC de SOLO LECTURA, sin ningún efecto
// secundario — permite que otro microservicio (Derivaciones) valide una
// transición de área ANTES de registrarla, sin ejecutarla. Reutiliza,
// SIN duplicar, exactamente las mismas piezas ya existentes:
//   - la misma regla de acceso al expediente que ya usa GetExpediente
//     (CanViewAll/AreaDelRol para interno, SolicitanteID para no interno);
//   - estados.IsValidArea y estados.CanDerivar, las MISMAS funciones que
//     ya usa DerivarExpediente (Etapa 3) — ninguna regla F2/F4 se
//     reescribe acá.
// Nunca llama a ningún método de escritura del repositorio (UpdateArea,
// DerivarConCambioDeEstado, RechazarYDevolver): es seguro invocarlo como
// dry-run.
func (l *ValidateDerivacionLogic) ValidateDerivacion(in *expedientes.ValidateDerivacionRequest) (*expedientes.ValidateDerivacionResponse, error) {
	userID, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if in == nil || strings.TrimSpace(in.ExpedienteId) == "" {
		return nil, status.Error(codes.InvalidArgument, "el identificador es obligatorio")
	}

	// Permiso real: misma decisión que ya usa GetExpediente para decidir
	// SI puede consultar este tipo de recurso — el ownership/área de abajo
	// sigue decidiendo, sin cambios, si ESTE expediente puntual corresponde.
	getPermission := "expedientes.view"
	if !authorization.IsInternal(role) {
		getPermission = "expedientes.view_own"
	}
	if err := authorization.RequirePermission(l.ctx, l.svcCtx.AuthClient, userID, getPermission); err != nil {
		return nil, err
	}

	found, err := l.svcCtx.ExpedienteRepository.FindByIDOrCodigo(l.ctx, strings.TrimSpace(in.ExpedienteId))
	if err != nil {
		if errors.Is(err, repository.ErrExpedienteNotFound) {
			return nil, status.Error(codes.NotFound, "expediente no encontrado")
		}
		return nil, status.Error(codes.Internal, "no se pudo consultar el expediente")
	}

	// Ownership/área (idéntico a GetExpedienteLogic, sin duplicar
	// CanViewAll/AreaDelRol como lógica nueva — se reutilizan tal cual):
	// un caller sin acceso legítimo al expediente no debe poder ni
	// siquiera validar transiciones sobre él (evita revelar información
	// de expedientes ajenos, como pedía la sección 4 del Paso 20B.2).
	if !authorization.CanViewAll(role) {
		if authorization.IsInternal(role) {
			if found.AreaActual != authorization.AreaDelRol(role) {
				return nil, status.Error(codes.PermissionDenied, "no tiene acceso a este expediente")
			}
		} else if found.SolicitanteID != userID {
			return nil, status.Error(codes.PermissionDenied, "no tiene acceso a este expediente")
		}
	}

	// Tipo/AreaActual REALES (nunca enviados por el cliente) + la MISMA
	// normalización que ya usa DerivarExpedienteLogic para area_destino.
	areaDestino := strings.ToUpper(strings.TrimSpace(in.AreaDestino))
	if !estados.IsValidArea(areaDestino) {
		return &expedientes.ValidateDerivacionResponse{Valido: false}, nil
	}

	valido := estados.CanDerivar(found.Tipo, found.AreaActual, areaDestino)
	return &expedientes.ValidateDerivacionResponse{Valido: valido}, nil
}
