package logic

import (
	"context"

	"expedientes/expedientesclient"
	"gateway/internal/types"

	"google.golang.org/grpc/metadata"
)

func toExpedienteDTO(e *expedientesclient.Expediente) types.ExpedienteDTO {
	if e == nil {
		return types.ExpedienteDTO{}
	}

	return types.ExpedienteDTO{
		Id:                 e.Id,
		Codigo:             e.Codigo,
		Tipo:               e.Tipo,
		Asunto:             e.Asunto,
		Descripcion:        e.Descripcion,
		SolicitanteId:      e.SolicitanteId,
		Estado:             e.Estado,
		Prioridad:          e.Prioridad,
		FechaRegistro:      e.FechaRegistro,
		FechaActualizacion: e.FechaActualizacion,
		Activo:             e.Activo,
		AreaActual:         e.AreaActual,
	}
}

// withAuthorization reenvía el header Authorization del cliente HTTP como
// metadata gRPC saliente: Expedientes Service exige autenticación en todas
// sus operaciones y valida el JWT él mismo.
func withAuthorization(ctx context.Context, authorization string) context.Context {
	if authorization == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", authorization)
}
