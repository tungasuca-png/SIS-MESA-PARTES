package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Documento son los metadatos de un documento, SIN su contenido binario:
// se usa para Get/List, donde devolver el archivo completo seria
// desperdiciar ancho de banda. Download usa DocumentoConContenido.
type Documento struct {
	ID            string
	ExpedienteID  string
	Nombre        string
	TipoDocumento string
	Extension     string
	TamanoBytes   int64
	SubidoPor     string
	Estado        string
	FechaRegistro time.Time
}

type DocumentoConContenido struct {
	Documento
	Contenido []byte
}

type DocumentoRepository struct {
	db *pgxpool.Pool
}

func NewDocumentoRepository(db *pgxpool.Pool) *DocumentoRepository {
	return &DocumentoRepository{db: db}
}

const metadataColumns = `
	id, expediente_id, nombre, tipo_documento, extension, tamano_bytes,
	subido_por, estado, fecha_registro
`

func scanMetadata(row pgx.Row) (*Documento, error) {
	d := &Documento{}
	err := row.Scan(
		&d.ID, &d.ExpedienteID, &d.Nombre, &d.TipoDocumento, &d.Extension,
		&d.TamanoBytes, &d.SubidoPor, &d.Estado, &d.FechaRegistro,
	)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (r *DocumentoRepository) Upload(ctx context.Context, expedienteID, nombre, tipoDocumento, extension, subidoPor string, contenido []byte) (*Documento, error) {
	created, err := scanMetadata(r.db.QueryRow(ctx, `
		INSERT INTO documentos (expediente_id, nombre, tipo_documento, extension, tamano_bytes, contenido, subido_por)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+metadataColumns,
		expedienteID, nombre, tipoDocumento, extension, len(contenido), contenido, subidoPor,
	))
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (r *DocumentoRepository) GetMetadata(ctx context.Context, id string) (*Documento, error) {
	found, err := scanMetadata(r.db.QueryRow(ctx, `
		SELECT `+metadataColumns+`
		FROM documentos
		WHERE id = $1 AND estado = 'ACTIVO'
	`, id))
	if err != nil {
		return nil, classifyNotFound(err, ErrDocumentoNotFound)
	}
	return found, nil
}

func (r *DocumentoRepository) GetWithContent(ctx context.Context, id string) (*DocumentoConContenido, error) {
	full := &DocumentoConContenido{}
	err := r.db.QueryRow(ctx, `
		SELECT id, expediente_id, nombre, tipo_documento, extension, tamano_bytes,
		       subido_por, estado, fecha_registro, contenido
		FROM documentos
		WHERE id = $1 AND estado = 'ACTIVO'
	`, id).Scan(
		&full.ID, &full.ExpedienteID, &full.Nombre, &full.TipoDocumento, &full.Extension,
		&full.TamanoBytes, &full.SubidoPor, &full.Estado, &full.FechaRegistro, &full.Contenido,
	)
	if err != nil {
		return nil, classifyNotFound(err, ErrDocumentoNotFound)
	}
	return full, nil
}

// ListFilter filtra el listado de documentos. Los campos vacios no filtran.
//
// ExpedienteIDs (Paso 19C) acota el listado a un conjunto de expedientes
// (usado para "documentos.view_own" SIN expediente_id puntual: la lista de
// expedientes propios del solicitante se resuelve consultando a Expedientes
// Service, nunca con un JOIN entre bases de datos distintas). Mutuamente
// excluyente en la práctica con ExpedienteID (un id puntual ya implica
// ownership confirmado sobre ESE expediente, ver ListDocumentosLogic).
//
// SubidoPor queda SIN USO para decidir ownership (Paso 19C: "quien subió el
// documento" no es "el dueño del expediente" — ver Paso 19). Se conserva el
// campo porque otras rutas de código externas al alcance de este paso no lo
// usan tampoco hoy, pero no se elimina para no tocar más de lo necesario.
type ListFilter struct {
	ExpedienteID  string
	ExpedienteIDs []string
	TipoDocumento string
	SubidoPor     string
	Page          int
	PageSize      int
}

func (r *DocumentoRepository) List(ctx context.Context, filter ListFilter) ([]*Documento, int, error) {
	conditions := []string{"estado = 'ACTIVO'"}
	args := make([]any, 0, 4)

	addCondition := func(column, value string) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	if filter.ExpedienteID != "" {
		addCondition("expediente_id", filter.ExpedienteID)
	} else if filter.ExpedienteIDs != nil {
		// nil = sin filtro (comportamiento previo, usado cuando hasViewAll).
		// No-nil pero vacío = el llamador SÍ quiso filtrar por un conjunto
		// de expedientes y ese conjunto resultó vacío (p. ej. un
		// solicitante sin ningún expediente propio) — debe devolver CERO
		// filas, nunca "sin filtro" (que expondría todo el listado).
		if len(filter.ExpedienteIDs) == 0 {
			conditions = append(conditions, "1 = 0")
		} else {
			args = append(args, filter.ExpedienteIDs)
			conditions = append(conditions, fmt.Sprintf("expediente_id = ANY($%d::uuid[])", len(args)))
		}
	}
	if filter.TipoDocumento != "" {
		addCondition("tipo_documento", filter.TipoDocumento)
	}
	if filter.SubidoPor != "" {
		addCondition("subido_por", filter.SubidoPor)
	}
	whereClause := strings.Join(conditions, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM documentos WHERE "+whereClause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limitArgs := append(append([]any{}, args...), filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT %s
		FROM documentos
		WHERE %s
		ORDER BY fecha_registro DESC
		LIMIT $%d OFFSET $%d
	`, metadataColumns, whereClause, len(limitArgs)-1, len(limitArgs)), limitArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*Documento, 0)
	for rows.Next() {
		item, err := scanMetadata(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *DocumentoRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE documentos
		SET estado = 'ELIMINADO'
		WHERE id = $1 AND estado = 'ACTIVO'
	`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrDocumentoNotFound
	}
	return nil
}
