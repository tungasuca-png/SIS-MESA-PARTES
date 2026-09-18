package logic

import (
	"context"
	"strings"

	"derivaciones/derivaciones"
	"derivaciones/internal/authorization"
	"derivaciones/internal/interceptor"
	"derivaciones/internal/svc"
	"derivaciones/internal/validation"

	"expedientes/expedientesclient"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
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
	// Permiso real (Paso 15B) + regla de negocio existente (CanCreate),
	// ambas deben cumplirse.
	if err := authorization.RequirePermission(l.ctx, l.svcCtx.AuthClient, userID, "derivaciones.create"); err != nil {
		return nil, err
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

	// Expediente real + acceso + veracidad de origen/destino (Paso 20B):
	// se delega en Expedientes.GetExpediente, reenviando el mismo JWT —
	// mismo patrón exacto ya usado en ListDerivaciones (Paso 20A) y en
	// Documentos (Pasos 19B-19E). Si el expediente no existe o el actor
	// no tiene acceso a él (ownership para no-interno, área para
	// interno), GetExpediente devuelve el error por sí solo — nunca se
	// duplica esa comparación acá, ni se importa AreaDelRol (paquete
	// "internal" de otro módulo Go, inalcanzable — mismo hallazgo ya
	// documentado en Documentos, Paso 19E).
	outCtx := l.ctx
	if token, tokenOk := interceptor.TokenFromContext(l.ctx); tokenOk {
		outCtx = metadata.AppendToOutgoingContext(l.ctx, "authorization", "Bearer "+token)
	}
	expedienteResp, err := l.svcCtx.ExpedientesClient.GetExpediente(outCtx, &expedientesclient.GetExpedienteRequest{
		Id: expedienteID,
	})
	if err != nil {
		return nil, err
	}

	// origen/destino contra la realidad (Paso 20B): CrearDerivacion se usa
	// hoy en DOS patrones reales y legítimos, distinguibles solo por CUÁL
	// de los dos coincide con el área actual real:
	//   1. Gateway/DerivarExpediente (derivarexpedientelogic.go): registra
	//      el historial DESPUÉS de que Expedientes.DerivarExpediente ya
	//      movió el área — para ese momento, AreaActual real YA ES el
	//      destino, no el origen.
	//   2. Gateway/CrearDerivacion directo (crearderivacionlogic.go del
	//      Gateway): registra SIN mover nada — ahí AreaActual real sigue
	//      siendo el origen, no el destino.
	// Exigir que origen U destino (al menos uno) coincida con el área
	// real actual cierra la fabricación total (un origen/destino sin
	// ninguna relación con la realidad del expediente, como detectó el
	// Paso 20) sin romper ninguno de los dos flujos legítimos ya
	// existentes, y sin necesitar ningún RPC nuevo ni duplicar la máquina
	// de transiciones F2/F4 de Expedientes.
	areaActual := expedienteResp.Expediente.AreaActual
	if origen != areaActual && destino != areaActual {
		return nil, status.Error(
			codes.PermissionDenied,
			"el origen o el destino deben corresponder al área real actual del expediente",
		)
	}

	// Validación de destino (Paso 20B.2): SOLO tiene sentido cuando el
	// área real actual sigue siendo el origen (patrón 2, arriba) — ahí
	// destino representa una transición que TODAVÍA no ocurrió, y
	// Expedientes.ValidateDerivacion (RPC de solo lectura, sin efectos
	// secundarios) puede confirmar con las reglas reales (IsValidArea +
	// CanDerivar, las MISMAS que usa DerivarExpediente) si esa transición
	// es institucionalmente válida para el tipo real del expediente.
	//
	// Cuando el área real actual ya es el destino (patrón 1: el
	// movimiento YA ocurrió, vía Expedientes.DerivarExpediente, ANTES de
	// llegar acá), no se vuelve a validar: ValidateDerivacion siempre
	// consulta el AreaActual ACTUAL (ya es destino, no origen), así que
	// preguntaría "¿destino -> destino es válido?" — una pregunta sin
	// sentido que rechazaría el flujo real ya validado por Expedientes en
	// su momento (con el área previa al movimiento). No se llama al RPC
	// en ese caso, evitando ese falso rechazo.
	if origen == areaActual {
		validado, err := l.svcCtx.ExpedientesClient.ValidateDerivacion(outCtx, &expedientesclient.ValidateDerivacionRequest{
			ExpedienteId: expedienteID,
			AreaDestino:  destino,
		})
		if err != nil {
			return nil, err
		}
		if !validado.Valido {
			return nil, status.Error(
				codes.PermissionDenied,
				"el destino no corresponde a una transición institucional válida para este expediente",
			)
		}
	}

	created, err := l.svcCtx.DerivacionRepository.Create(l.ctx, expedienteID, tipo, origen, destino, motivo, condicion, userID)
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo registrar la derivación")
	}

	return &derivaciones.CrearDerivacionResponse{Derivacion: toProto(created)}, nil
}
