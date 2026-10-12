package logic

import (
	"context"

	"derivaciones/derivaciones"
	"derivaciones/internal/authorization"
	"derivaciones/internal/interceptor"
	"derivaciones/internal/svc"

	"expedientes/expedientesclient"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// visibilidad es el resultado de PERMISO (Auth.HasPermission) para consultar
// derivaciones. Es independiente del ALCANCE de datos, que sigue delegado en
// Expedientes.GetExpediente:
//
//	general  -> "derivaciones.view":     alcance general del expediente (área/global)
//	propias  -> "derivaciones.view_own": solo expedientes cuyo solicitante es el usuario
//
// Si tiene ambos, manda el general. Nunca se decide por el nombre del rol.
type visibilidad struct {
	general bool
}

// resolverVisibilidad consulta a Auth y devuelve PermissionDenied si el
// usuario no tiene ninguno de los dos permisos. Cualquier error de Auth se
// propaga (fail-closed).
func resolverVisibilidad(ctx context.Context, svcCtx *svc.ServiceContext, userID string) (visibilidad, error) {
	general, err := authorization.HasPermission(ctx, svcCtx.AuthClient, userID, "derivaciones.view")
	if err != nil {
		return visibilidad{}, err
	}
	if general {
		return visibilidad{general: true}, nil
	}
	propias, err := authorization.HasPermission(ctx, svcCtx.AuthClient, userID, "derivaciones.view_own")
	if err != nil {
		return visibilidad{}, err
	}
	if !propias {
		return visibilidad{}, status.Error(codes.PermissionDenied, "el usuario no tiene el permiso requerido")
	}
	return visibilidad{}, nil
}

// verificarExpediente exige acceso real al expediente, delegado en
// Expedientes.GetExpediente reenviando el mismo JWT (existencia, ownership y
// área: fail-closed). Con solo "view_own" además el expediente debe ser del
// usuario autenticado (SolicitanteId), tomado de Expedientes — nunca de un
// dato enviado por el cliente.
func (v visibilidad) verificarExpediente(ctx context.Context, svcCtx *svc.ServiceContext, userID, expedienteID string) error {
	outCtx := ctx
	if token, ok := interceptor.TokenFromContext(ctx); ok {
		outCtx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
	}
	resp, err := svcCtx.ExpedientesClient.GetExpediente(outCtx, &expedientesclient.GetExpedienteRequest{Id: expedienteID})
	if err != nil {
		return err
	}
	if !v.general {
		if resp == nil || resp.Expediente == nil || resp.Expediente.SolicitanteId != userID {
			return status.Error(codes.PermissionDenied, "el usuario no tiene el permiso requerido")
		}
	}
	return nil
}

// proyectar devuelve la derivación tal como debe verla este usuario. Con
// "view_own" (sin "view") se entrega una vista reducida: solo el recorrido
// (tipo, origen, destino, fecha). El motivo, la condición y quién la
// registró son información interna de gestión y no se exponen.
func (v visibilidad) proyectar(d *derivaciones.Derivacion) *derivaciones.Derivacion {
	if v.general {
		return d
	}
	return &derivaciones.Derivacion{
		Id:            d.Id,
		ExpedienteId:  d.ExpedienteId,
		Tipo:          d.Tipo,
		Origen:        d.Origen,
		Destino:       d.Destino,
		FechaRegistro: d.FechaRegistro,
	}
}
