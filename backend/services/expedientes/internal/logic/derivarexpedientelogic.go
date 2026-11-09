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

type DerivarExpedienteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDerivarExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DerivarExpedienteLogic {
	return &DerivarExpedienteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// DerivarExpediente mueve el expediente a otra área y registra el movimiento
// (y su evento de outbox) en UNA transacción (A + B). Reglas sin cambios
// respecto de la Etapa 3/4: propiedad de área, transición confirmada para el
// tipo (estados.CanDerivar) y casos especiales F2/F4. Nuevo: el área de
// origen esperada (la que vio el usuario) debe seguir siendo la actual, el
// destino debe ser distinto del origen y la operación es idempotente.
func (l *DerivarExpedienteLogic) DerivarExpediente(in *expedientes.DerivarExpedienteRequest) (*expedientes.DerivarExpedienteResponse, error) {
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
	areaDestino := strings.ToUpper(strings.TrimSpace(in.AreaDestino))
	if !estados.IsValidArea(areaDestino) {
		return nil, status.Error(codes.InvalidArgument, "el área destino no es válida")
	}
	origenEsperado := strings.ToUpper(strings.TrimSpace(in.AreaOrigenEsperada))
	if !estados.IsValidArea(origenEsperado) {
		return nil, status.Error(codes.InvalidArgument, "el área de origen esperada es obligatoria")
	}
	clave, err := validarClave(in.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	motivo := strings.TrimSpace(in.Motivo)
	if motivo == "" {
		return nil, status.Error(codes.InvalidArgument, "el motivo es obligatorio")
	}
	if len([]rune(motivo)) > 500 {
		return nil, status.Error(codes.InvalidArgument, "el motivo no puede superar los 500 caracteres")
	}
	condicion := strings.TrimSpace(in.Condicion)
	if len([]rune(condicion)) > 255 {
		return nil, status.Error(codes.InvalidArgument, "la condición no puede superar los 255 caracteres")
	}
	if err := validarRolActor(role); err != nil {
		return nil, err
	}

	res, err := l.svcCtx.ExpedienteRepository.RegistrarMovimiento(l.ctx, repository.SolicitudMovimiento{
		IDOrCodigo:     strings.TrimSpace(in.Id),
		Tipo:           repository.MovDerivacion,
		ActorID:        userID,
		ActorRol:       role,
		IdempotencyKey: clave,
		Contenido:      contenido(areaDestino, origenEsperado, motivo, condicion),
		Motivo:         motivo,
		Condicion:      condicion,
		Decidir: func(actual *repository.Expediente) (repository.Decision, error) {
			return decidirDerivacion(role, actual, origenEsperado, areaDestino)
		},
	})
	if err != nil {
		return nil, errorMovimiento(l.ctx, "derivar", err)
	}
	if !res.Repetido {
		l.svcCtx.Publicador.Avisar() // proyección inmediata (si no, el sondeo periódico)
	}
	return &expedientes.DerivarExpedienteResponse{
		Expediente: toProto(res.Expediente),
		Movimiento: movimientoToProto(res.Movimiento),
		Repetido:   res.Repetido,
	}, nil
}

// decidirDerivacion aplica las reglas sobre el expediente BLOQUEADO.
func decidirDerivacion(role string, actual *repository.Expediente, origenEsperado, areaDestino string) (repository.Decision, error) {
	if !authorization.CanViewAll(role) && actual.AreaActual != authorization.AreaDelRol(role) {
		return repository.Decision{}, status.Error(codes.PermissionDenied, "el expediente no pertenece al área del usuario")
	}
	if actual.AreaActual != origenEsperado {
		return repository.Decision{}, status.Errorf(codes.Aborted, "el expediente ya no está en %s; actualice y vuelva a intentarlo", origenEsperado)
	}
	if areaDestino == actual.AreaActual {
		return repository.Decision{}, status.Error(codes.InvalidArgument, "el área destino debe ser distinta del área actual")
	}
	if !estados.CanDerivar(actual.Tipo, actual.AreaActual, areaDestino) {
		return repository.Decision{}, status.Error(codes.FailedPrecondition,
			"no existe una transición confirmada de "+actual.AreaActual+" a "+areaDestino+" para este tipo de trámite")
	}
	estadoNuevo := actual.Estado
	switch {
	// Secretaría -> Dirección en F2/F4: el trámite entra en proceso.
	case actual.AreaActual == estados.AreaSecretaria && areaDestino == estados.AreaDirector &&
		(estados.EsF2(actual.Tipo) || estados.EsF4(actual.Tipo)) && actual.Estado == estados.EstadoPendiente:
		estadoNuevo = estados.EstadoEnProceso
	// Subdirección -> Docente en F4: destino final, cierra el expediente. Solo
	// desde EN_PROCESO (D9: único estado alcanzable en el flujo confirmado).
	case estados.EsF4(actual.Tipo) && actual.AreaActual == estados.AreaSubdirector && areaDestino == estados.AreaDocente:
		if actual.Estado != estados.EstadoEnProceso {
			return repository.Decision{}, status.Error(codes.FailedPrecondition, "solo se puede enviar al docente un expediente en proceso")
		}
		estadoNuevo = estados.EstadoAtendido
	}
	return repository.Decision{AreaDestino: areaDestino, EstadoNuevo: estadoNuevo}, nil
}
