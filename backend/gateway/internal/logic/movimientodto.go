package logic

import (
	"expedientes/expedientesclient"
	"gateway/internal/errorsafe"
	"gateway/internal/types"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// toMovimientoDTO convierte el movimiento registrado por Expedientes (A + B).
func toMovimientoDTO(m *expedientesclient.Movimiento) types.MovimientoDTO {
	if m == nil {
		return types.MovimientoDTO{}
	}
	return types.MovimientoDTO{
		Id: m.Id, ExpedienteId: m.ExpedienteId, Secuencia: m.Secuencia, Tipo: m.Tipo,
		ActorId: m.ActorId, ActorRol: m.ActorRol, AreaOrigen: m.AreaOrigen, AreaDestino: m.AreaDestino,
		EstadoAnterior: m.EstadoAnterior, EstadoNuevo: m.EstadoNuevo, Motivo: m.Motivo, Condicion: m.Condicion,
		OcurridoEn: m.OcurridoEn,
	}
}

// derivacionDeMovimiento: compatibilidad con el campo "derivacion" anterior
// (antes el gateway creaba una derivación tipo DERIVACION tras mover el área;
// también para el rechazo).
func derivacionDeMovimiento(m *expedientesclient.Movimiento) types.DerivacionDTO {
	if m == nil {
		return types.DerivacionDTO{}
	}
	return types.DerivacionDTO{
		Id: m.Id, ExpedienteId: m.ExpedienteId, Tipo: "DERIVACION", Origen: m.AreaOrigen, Destino: m.AreaDestino,
		Motivo: m.Motivo, Condicion: m.Condicion, RegistradoPor: m.ActorId, FechaRegistro: m.OcurridoEn,
		Clase: "MOVIMIENTO", Secuencia: m.Secuencia, EstadoAnterior: m.EstadoAnterior, EstadoNuevo: m.EstadoNuevo,
		ActorRol: m.ActorRol,
	}
}

// errorMovimiento: en las operaciones con Idempotency-Key, un timeout o una
// conexión perdida dejan el resultado INCIERTO (pudo confirmarse). Se informa
// con un mensaje seguro y explícito: reintentar con la misma clave no duplica.
func errorMovimiento(err error) error {
	switch status.Code(err) {
	case codes.DeadlineExceeded, codes.Unavailable:
		return status.Error(status.Code(err), errorsafe.MensajeResultadoIncierto)
	}
	return err
}
