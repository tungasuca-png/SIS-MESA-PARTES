package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Acciones y resultados de la bitácora (coinciden con los CHECK de la BD).
const (
	AccionCreate = "CREATE"
	AccionView   = "VIEW"
	AccionUpdate = "UPDATE"
	AccionClose  = "CLOSE"
	AccionReopen = "REOPEN"
	AccionDenied = "DENIED"

	ResultadoPermitido = "PERMITIDO"
	ResultadoDenegado  = "DENEGADO"

	RecursoExpediente = "EXPEDIENTE_PSICOLOGICO"
)

// AuditEntry es un evento de auditoría. NUNCA debe llevar contenido clínico,
// JWT, claves ni texto libre del usuario: Motivo es un código corto del sistema
// (p. ej. PERMISO_INSUFICIENTE) o la lista de nombres de campos modificados.
type AuditEntry struct {
	UsuarioID    string
	Accion       string
	Recurso      string
	RecursoID    string // vacío = no aplica (p. ej. un listado)
	EstudianteID string // vacío = no aplica
	Resultado    string
	Motivo       string
}

type execer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func insertAuditoria(ctx context.Context, q execer, e AuditEntry) error {
	_, err := q.Exec(ctx, `
		INSERT INTO auditoria_psicologica (usuario_id, accion, recurso, recurso_id, estudiante_id, resultado, motivo)
		VALUES ($1::uuid, $2, $3, NULLIF($4::text, '')::uuid, NULLIF($5::text, '')::uuid, $6, NULLIF($7::text, ''))
	`, e.UsuarioID, e.Accion, e.Recurso, e.RecursoID, e.EstudianteID, e.Resultado, e.Motivo)
	return err
}

// AuditoriaRepository registra eventos que no acompañan a una mutación (VIEW y
// DENIED). Las mutaciones escriben su evento dentro de su propia transacción
// (ver ExpedienteRepository), no por aquí. La aplicación solo tiene INSERT y
// SELECT sobre la tabla; no existe ninguna operación de modificación.
type AuditoriaRepository struct {
	db *pgxpool.Pool
}

func NewAuditoriaRepository(db *pgxpool.Pool) *AuditoriaRepository {
	return &AuditoriaRepository{db: db}
}

func (r *AuditoriaRepository) Registrar(ctx context.Context, e AuditEntry) error {
	return insertAuditoria(ctx, r.db, e)
}
