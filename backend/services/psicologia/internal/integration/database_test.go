package integration

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// TestBaseDeDatos prueba las garantías FÍSICAS de PostgreSQL, sin pasar por
// Go: 1 estudiante = 1 expediente, CHECK de estados/cierre, y que la bitácora
// sea append-only incluso para el dueño de la tabla.
func TestBaseDeDatos(t *testing.T) {
	h := newHarness(t)
	base := time.Now().UnixNano()
	estudiante := fixedUUID(base, 1)
	otroEstudiante := fixedUUID(base, 2)
	usuario := fixedUUID(base, 3)
	h.actors = append(h.actors, usuario)

	insert := func(estudianteID, codigo, estado string) (string, error) {
		var id string
		err := h.admin.QueryRow(h.ctx, `
			INSERT INTO expedientes_psicologicos (estudiante_id, codigo, estado, responsable_id, abierto_por)
			VALUES ($1::uuid, $2, $3, $4::uuid, $4::uuid) RETURNING id::text
		`, estudianteID, codigo, estado, usuario).Scan(&id)
		if err == nil {
			h.created = append(h.created, id)
		}
		return id, err
	}
	constraintOf := func(err error) string {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return pgErr.ConstraintName
		}
		return ""
	}

	t.Run("UnEstudiante_UnExpediente_UNIQUE", func(t *testing.T) {
		if _, err := insert(estudiante, "PSI-9999-000001", "ABIERTO"); err != nil {
			t.Fatalf("el primer INSERT debe funcionar: %v", err)
		}
		_, err := insert(estudiante, "PSI-9999-000002", "ABIERTO")
		if constraintOf(err) != "uq_expedientes_psicologicos_estudiante" {
			t.Fatalf("el segundo INSERT del mismo estudiante debe violar UNIQUE(estudiante_id), got %v", err)
		}
	})

	t.Run("CodigoUnicoYFormato", func(t *testing.T) {
		if _, err := insert(otroEstudiante, "PSI-9999-000001", "ABIERTO"); constraintOf(err) != "uq_expedientes_psicologicos_codigo" {
			t.Fatalf("un código repetido debe violar UNIQUE(codigo), got %v", err)
		}
		if _, err := insert(otroEstudiante, "EXP-1", "ABIERTO"); constraintOf(err) != "chk_expedientes_psicologicos_codigo" {
			t.Fatalf("un código con formato inválido debe violar el CHECK, got %v", err)
		}
	})

	t.Run("EstadoSoloLosTresValidos_VARCHARyCHECK", func(t *testing.T) {
		if _, err := insert(otroEstudiante, "PSI-9999-000003", "INVENTADO"); constraintOf(err) != "chk_expedientes_psicologicos_estado" {
			t.Fatalf("un estado fuera del catálogo debe violar el CHECK, got %v", err)
		}
	})

	t.Run("CerradoExigeFechaYMotivo", func(t *testing.T) {
		if _, err := insert(otroEstudiante, "PSI-9999-000004", "CERRADO"); constraintOf(err) != "chk_expedientes_psicologicos_cierre" {
			t.Fatalf("CERRADO sin fecha/motivo debe violar el CHECK, got %v", err)
		}
	})

	t.Run("AplicacionSinDeleteNiUpdateSobreAuditoria", func(t *testing.T) {
		if _, err := h.app.Exec(h.ctx, `INSERT INTO auditoria_psicologica (usuario_id, accion, recurso, resultado) VALUES ($1::uuid, 'VIEW', 'EXPEDIENTE_PSICOLOGICO', 'PERMITIDO')`, usuario); err != nil {
			t.Fatalf("la aplicación sí puede INSERTAR auditoría: %v", err)
		}
		if _, err := h.app.Exec(h.ctx, `UPDATE auditoria_psicologica SET motivo = 'x' WHERE usuario_id = $1::uuid`, usuario); err == nil {
			t.Fatal("la aplicación NO debe poder hacer UPDATE sobre la auditoría")
		}
		if _, err := h.app.Exec(h.ctx, `DELETE FROM auditoria_psicologica WHERE usuario_id = $1::uuid`, usuario); err == nil {
			t.Fatal("la aplicación NO debe poder hacer DELETE sobre la auditoría")
		}
		if _, err := h.app.Exec(h.ctx, `TRUNCATE auditoria_psicologica`); err == nil {
			t.Fatal("la aplicación NO debe poder hacer TRUNCATE sobre la auditoría")
		}
	})

	t.Run("AplicacionSinDeleteSobreExpedientes", func(t *testing.T) {
		if _, err := h.app.Exec(h.ctx, `DELETE FROM expedientes_psicologicos WHERE estudiante_id = $1::uuid`, estudiante); err == nil {
			t.Fatal("la aplicación no borra físicamente expedientes")
		}
	})

	t.Run("TriggerBloqueaTambienAlDueno", func(t *testing.T) {
		// Segunda capa: aunque el rol conserve privilegios (el dueño los tiene),
		// el trigger rechaza UPDATE y DELETE.
		_, err := h.admin.Exec(h.ctx, `UPDATE auditoria_psicologica SET motivo = 'x' WHERE usuario_id = $1::uuid`, usuario)
		if err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Fatalf("el trigger debe rechazar UPDATE incluso al dueño, got %v", err)
		}
		_, err = h.admin.Exec(h.ctx, `DELETE FROM auditoria_psicologica WHERE usuario_id = $1::uuid`, usuario)
		if err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Fatalf("el trigger debe rechazar DELETE incluso al dueño, got %v", err)
		}
	})
}
