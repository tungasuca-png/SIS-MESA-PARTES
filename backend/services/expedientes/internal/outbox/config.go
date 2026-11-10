package outbox

import (
	"fmt"
	"time"
)

// LeaseMaximo: tope que impone el trigger trg_outbox_proteger (migración 004):
// un reclamo con reclamado_hasta > ahora + 10 minutos se rechaza con 42501.
const LeaseMaximo = 10 * time.Minute

// margenCiclo: holgura que Ciclo reserva, además del TimeoutRPC, antes de
// procesar cada evento reclamado (ver Ciclo).
const margenCiclo = 2 * time.Second

// Validar rechaza configuraciones con las que el publicador arrancaría pero
// no publicaría nunca:
//   - Lease > LeaseMaximo: la base rechaza todos los reclamos.
//   - Lease <= TimeoutRPC + margen: Ciclo reclama (suma un intento) y corta
//     antes de procesar el primer evento; los intentos crecen sin publicar.
//
// Devuelve además un aviso (no bloqueante) si un lote completo no cabe en el
// lease en el peor caso: los eventos que no alcancen se retoman al vencer
// el lease, con un intento más.
func (c Config) Validar() (aviso string, err error) {
	if c.Lease > LeaseMaximo {
		return "", fmt.Errorf("LeaseSegundos=%d supera el máximo de %d que admite la base (trg_outbox_proteger)",
			int(c.Lease/time.Second), int(LeaseMaximo/time.Second))
	}
	if c.Lease <= c.TimeoutRPC+margenCiclo {
		return "", fmt.Errorf("LeaseSegundos=%d debe superar TimeoutRPCMs=%d + %d ms: con ese lease no se procesa ningún evento",
			int(c.Lease/time.Second), c.TimeoutRPC.Milliseconds(), margenCiclo.Milliseconds())
	}
	if peor := time.Duration(c.Lote)*c.TimeoutRPC + margenCiclo; peor > c.Lease {
		aviso = fmt.Sprintf("un lote de %d eventos puede tardar %s y el lease es %s: los que no alcancen se retomarán con un intento más",
			c.Lote, peor, c.Lease)
	}
	return aviso, nil
}
