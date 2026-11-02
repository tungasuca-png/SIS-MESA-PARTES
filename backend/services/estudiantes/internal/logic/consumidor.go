package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/interceptor"
	"estudiantes/internal/repository"
)

// Sistemas consumidores (client credentials) SIN estudiantes.view: solo datos
// mínimos para identificar y seleccionar a un estudiante. Los usuarios humanos
// y los sistemas con estudiantes.view conservan la respuesta completa.

// consumidorMinimo: la llamada la hace un sistema consumidor sin
// estudiantes.view (p. ej. solo estudiantes.basic.view).
func consumidorMinimo(ctx context.Context) bool {
	s, ok := interceptor.ServiceFromContext(ctx)
	return ok && !s.Permisos["estudiantes.view"]
}

// basicoParaLlamador: EstudianteBasico completo, o para un consumidor mínimo
// solo id, nombre completo, grado y sección (nunca documento, SIAGIE, código,
// nombres/apellidos separados, nivel, turno ni estados).
func basicoParaLlamador(ctx context.Context, e *repository.Estudiante) *estudiantes.EstudianteBasico {
	if consumidorMinimo(ctx) {
		return &estudiantes.EstudianteBasico{Id: e.ID, NombreCompleto: e.NombreCompleto(), Grado: e.Grado, Seccion: e.Seccion}
	}
	return toProtoBasic(e)
}
