package repository

import (
	"context"
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

func (r *DocumentoRepository) ListByExpediente(ctx context.Context, expedienteID string) ([]*Documento, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+metadataColumns+`
		FROM documentos
		WHERE expediente_id = $1 AND estado = 'ACTIVO'
		ORDER BY fecha_registro DESC
	`, expedienteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*Documento, 0)
	for rows.Next() {
		item, err := scanMetadata(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
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
