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

// DerivarExpediente (Etapa 3) es la operación de negocio real detrás de
// "derivar" — a diferencia de UpdateArea (genérico, sigue existiendo para
// otros usos), esta exige que la transición esté confirmada para el tipo
// de trámite del expediente (ver estados.CanDerivar). Reutiliza
// exactamente las mismas reglas de propiedad de área (Etapa 1) y de
// concurrencia optimista (Etapa 1.1) que ya usa UpdateArea — no se
// duplica esa parte, solo se agrega la validación de transición.
func (l *DerivarExpedienteLogic) DerivarExpediente(in *expedientes.DerivarExpedienteRequest) (*expedientes.DerivarExpedienteResponse, error) {
	_, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if !authorization.CanUpdateArea(role) {
		return nil, status.Error(codes.PermissionDenied, "el rol no puede derivar el expediente")
	}
	if in == nil || strings.TrimSpace(in.Id) == "" {
		return nil, status.Error(codes.InvalidArgument, "el identificador es obligatorio")
	}

	areaDestino := strings.ToUpper(strings.TrimSpace(in.AreaDestino))
	if !estados.IsValidArea(areaDestino) {
		return nil, status.Error(codes.InvalidArgument, "el área destino no es válida")
	}

	current, err := l.svcCtx.ExpedienteRepository.FindByIDOrCodigo(l.ctx, strings.TrimSpace(in.Id))
	if err != nil {
		if errors.Is(err, repository.ErrExpedienteNotFound) {
			return nil, status.Error(codes.NotFound, "expediente no encontrado")
		}
		return nil, status.Error(codes.Internal, "no se pudo consultar el expediente")
	}

	// Mismo criterio de propiedad que UpdateArea/ChangeEstado (Etapa 1): solo
	// quien ya tiene el expediente (o CanViewAll) puede derivarlo.
	if !authorization.CanViewAll(role) && current.AreaActual != authorization.AreaDelRol(role) {
		return nil, status.Error(codes.PermissionDenied, "el expediente no pertenece al área del usuario")
	}

	// Única regla nueva de esta etapa: la transición debe estar confirmada
	// para este tipo de trámite (o el tipo es genérico y no tiene regla
	// institucional, en cuyo caso se mantiene permisivo — ver CanDerivar).
	if !estados.CanDerivar(current.Tipo, current.AreaActual, areaDestino) {
		return nil, status.Error(
			codes.FailedPrecondition,
			"no existe una transición confirmada de "+current.AreaActual+" a "+areaDestino+" para este tipo de trámite",
		)
	}

	// Casos especiales (Etapa 4) donde derivar también cambia el estado en
	// la MISMA escritura — separarlo en dos llamadas dejaría una ventana
	// donde el actor original ya no es dueño del área nueva para completar
	// el segundo paso:
	//   - Secretaría -> Dirección para F2/F4: el trámite deja la bandeja de
	//     intake y entra en trámite real (PENDIENTE -> EN_PROCESO). Es una
	//     inferencia razonable, no una cita textual: la regla institucional
	//     de esta etapa no lo dice con esas palabras, pero sin este cambio
	//     ResolverExpediente/RechazarExpediente (que exigen EN_PROCESO)
	//     nunca podrían ejecutarse — ya estaba anticipado en
	//     docs/workflow-definitivo-mesa-de-partes.md sección 9 ("Derivado a
	//     Dirección -> EN_PROCESO").
	//   - Subdirección -> Docente en F4: "destino final" del flujo
	//     confirmado — no hay ninguna acción posterior descrita, así que
	//     llegar ahí también cierra el expediente (ATENDIDO).
	switch {
	case current.AreaActual == estados.AreaSecretaria && areaDestino == estados.AreaDirector &&
		(estados.EsF2(current.Tipo) || estados.EsF4(current.Tipo)) && current.Estado == estados.EstadoPendiente:
		updated, err := l.svcCtx.ExpedienteRepository.DerivarConCambioDeEstado(
			l.ctx, strings.TrimSpace(in.Id), current.AreaActual, areaDestino, current.Estado, estados.EstadoEnProceso,
		)
		if err != nil {
			if errors.Is(err, repository.ErrExpedienteNotFound) {
				return nil, status.Error(codes.Aborted, "el expediente cambió, intente nuevamente")
			}
			return nil, status.Error(codes.Internal, "no se pudo derivar el expediente")
		}
		return &expedientes.DerivarExpedienteResponse{Expediente: toProto(updated)}, nil

	case estados.EsF4(current.Tipo) && current.AreaActual == estados.AreaSubdirector && areaDestino == estados.AreaDocente:
		updated, err := l.svcCtx.ExpedienteRepository.DerivarConCambioDeEstado(
			l.ctx, strings.TrimSpace(in.Id), current.AreaActual, areaDestino, current.Estado, estados.EstadoAtendido,
		)
		if err != nil {
			if errors.Is(err, repository.ErrExpedienteNotFound) {
				return nil, status.Error(codes.Aborted, "el expediente cambió, intente nuevamente")
			}
			return nil, status.Error(codes.Internal, "no se pudo derivar el expediente")
		}
		return &expedientes.DerivarExpedienteResponse{Expediente: toProto(updated)}, nil
	}

	// Misma guarda de concurrencia optimista que UpdateArea (Etapa 1.1): si
	// el área cambió entre la lectura y la escritura, se rechaza.
	updated, err := l.svcCtx.ExpedienteRepository.UpdateArea(l.ctx, strings.TrimSpace(in.Id), current.AreaActual, areaDestino)
	if err != nil {
		if errors.Is(err, repository.ErrExpedienteNotFound) {
			return nil, status.Error(codes.Aborted, "el área del expediente cambió, intente nuevamente")
		}
		return nil, status.Error(codes.Internal, "no se pudo derivar el expediente")
	}

	return &expedientes.DerivarExpedienteResponse{Expediente: toProto(updated)}, nil
}
