package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"psicologia/internal/clinicalcrypto"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Cifrador: cifrado del contenido clínico (internal/clinicalcrypto.Cipher).
// El repositorio cifra al escribir y descifra al leer; ninguna otra capa
// (lógica, Gateway, frontend) conoce la clave ni ve el contenido cifrado.
type Cifrador interface {
	Encrypt(plaintext, aad []byte) ([]byte, string, error)
	Decrypt(blob []byte, keyID string, aad []byte) ([]byte, error)
}

// ErrContenidoSinCifrar: una fila todavía tiene su contenido en texto claro
// (no migrada con cmd/cifrar-contenido). No hay lectura de respaldo en claro.
var ErrContenidoSinCifrar = errors.New("contenido clinico pendiente de cifrado")

// ContenidoCifrable: los textos libres de una versión de la Hoja de Atención
// (y su motivo de corrección), que se guardan juntos en un único bloque
// cifrado (atencion_versiones.contenido_cifrado). Fecha, hora y las
// categorías (realizada_por, motivos, derivaciones) quedan en claro: son
// códigos de catálogo que el listado y los reportes agregados necesitan
// (decisión D1, FASE 6N-3).
type ContenidoCifrable struct {
	RealizadaPorOtros     string `json:"realizada_por_otros"`
	MotivoOtros           string `json:"motivo_otros"`
	DescripcionMotivo     string `json:"descripcion_motivo"`
	AnalisisIntegral      string `json:"analisis_integral"`
	Estrategias           string `json:"estrategias"`
	DerivacionOtro        string `json:"derivacion_otro"`
	SeguimientoResultados string `json:"seguimiento_resultados"`
	Acuerdos              string `json:"acuerdos"`
	CompromisoEstudiante  string `json:"compromiso_estudiante"`
	CompromisoFamilia     string `json:"compromiso_familia"`
	MotivoCorreccion      string `json:"motivo_correccion"`
}

func cifrableDesde(c AtencionContenido, motivoCorreccion string) ContenidoCifrable {
	return ContenidoCifrable{
		RealizadaPorOtros: c.RealizadaPorOtros, MotivoOtros: c.MotivoOtros,
		DescripcionMotivo: c.DescripcionMotivo, AnalisisIntegral: c.AnalisisIntegral,
		Estrategias: c.Estrategias, DerivacionOtro: c.DerivacionOtro,
		SeguimientoResultados: c.SeguimientoResultados, Acuerdos: c.Acuerdos,
		CompromisoEstudiante: c.CompromisoEstudiante, CompromisoFamilia: c.CompromisoFamilia,
		MotivoCorreccion: motivoCorreccion,
	}
}

func (cc ContenidoCifrable) aplicarA(c *AtencionContenido) {
	c.RealizadaPorOtros, c.MotivoOtros = cc.RealizadaPorOtros, cc.MotivoOtros
	c.DescripcionMotivo, c.AnalisisIntegral, c.Estrategias = cc.DescripcionMotivo, cc.AnalisisIntegral, cc.Estrategias
	c.DerivacionOtro, c.SeguimientoResultados, c.Acuerdos = cc.DerivacionOtro, cc.SeguimientoResultados, cc.Acuerdos
	c.CompromisoEstudiante, c.CompromisoFamilia = cc.CompromisoEstudiante, cc.CompromisoFamilia
}

// CifrarContenidoVersion cifra los textos libres de una versión, ligados a su
// fila. Exportada para cmd/cifrar-contenido (misma serialización y AAD).
func CifrarContenidoVersion(c Cifrador, versionID, atencionID string, cc ContenidoCifrable) ([]byte, string, error) {
	plano, err := json.Marshal(cc)
	if err != nil {
		return nil, "", err
	}
	return c.Encrypt(plano, clinicalcrypto.AADVersion(versionID, atencionID))
}

// DescifrarContenidoVersion es la operación inversa de CifrarContenidoVersion.
func DescifrarContenidoVersion(c Cifrador, blob []byte, keyID, versionID, atencionID string) (ContenidoCifrable, error) {
	var cc ContenidoCifrable
	plano, err := c.Decrypt(blob, keyID, clinicalcrypto.AADVersion(versionID, atencionID))
	if err != nil {
		return cc, err
	}
	if err := json.Unmarshal(plano, &cc); err != nil {
		return cc, clinicalcrypto.ErrDescifrado
	}
	return cc, nil
}

// cifrarTexto / descifrarTexto: un campo de texto suelto (motivo de anulación,
// resumen del caso, motivo de cierre). "" se guarda como NULL (sin bloque).
func cifrarTexto(c Cifrador, texto string, aad []byte) ([]byte, *string, error) {
	if texto == "" {
		return nil, nil, nil
	}
	blob, kid, err := c.Encrypt([]byte(texto), aad)
	if err != nil {
		return nil, nil, err
	}
	return blob, &kid, nil
}

func descifrarTexto(c Cifrador, blob []byte, keyID *string, aad []byte) (*string, error) {
	if blob == nil {
		return nil, nil
	}
	if keyID == nil {
		return nil, clinicalcrypto.ErrDescifrado
	}
	plano, err := c.Decrypt(blob, *keyID, aad)
	if err != nil {
		return nil, err
	}
	s := string(plano)
	return &s, nil
}

// Estados de una atención (VARCHAR + CHECK en la BD). FIRMADA no existe
// todavía: depende de una decisión institucional sobre la firma.
const (
	AtencionBorrador   = "BORRADOR"
	AtencionRegistrada = "REGISTRADA"
	AtencionAnulada    = "ANULADA"
	AtencionDescartada = "DESCARTADA"
)

const (
	AccionRegister = "REGISTER"
	AccionCorrect  = "CORRECT"
	AccionAnnul    = "ANNUL"
	AccionDiscard  = "DISCARD"
	AccionExport   = "EXPORT"

	RecursoAtencion = "ATENCION_PSICOLOGICA"
)

var (
	ErrAtencionNotFound  = errors.New("atencion psicologica not found")
	ErrAtencionEstado    = errors.New("el estado de la atencion no permite esta operacion")
	ErrAtencionConflicto = errors.New("la atencion cambio desde que se cargo")
	ErrCatalogoInvalido  = errors.New("codigo de catalogo invalido o inactivo")
	ErrExpedienteCerrado = errors.New("el expediente psicologico esta cerrado")
	ErrVersionNotFound   = errors.New("version de atencion not found")
)

// Tablas de catálogo admitidas (lista blanca: el nombre nunca viene del usuario).
const (
	CatalogoMotivos  = "catalogo_motivos_atencion"
	CatalogoDestinos = "catalogo_destinos_derivacion"
)

type OpcionCatalogo struct {
	Codigo   string `json:"codigo"`
	Etiqueta string `json:"etiqueta"`
}

// AtencionContenido es un bloque de la Hoja de Atención. Fecha y hora vacías
// solo son válidas en un borrador.
type AtencionContenido struct {
	FechaAtencion         string // YYYY-MM-DD o ""
	HoraAtencion          string // HH:MM o ""
	RealizadaPor          []string
	RealizadaPorOtros     string
	Motivos               []OpcionCatalogo
	MotivoOtros           string
	DescripcionMotivo     string
	AnalisisIntegral      string
	Estrategias           string
	Derivaciones          []OpcionCatalogo
	DerivacionOtro        string
	SeguimientoResultados string
	Acuerdos              string
	CompromisoEstudiante  string
	CompromisoFamilia     string
}

type Atencion struct {
	ID              string
	ExpedienteID    string
	Numero          *int
	Estado          string
	AutorID         string
	RegistradaPor   *string
	RegistradaEn    *time.Time
	VersionVigente  int
	Revision        int
	AnuladaPor      *string
	AnuladaEn       *time.Time
	MotivoAnulacion *string // descifrado por el repositorio; nil en listados
	CreatedAt       time.Time
	UpdatedAt       time.Time

	motivoAnulacionCifrado []byte
	cifradoKeyID           *string
}

type AtencionVersion struct {
	ID               string
	AtencionID       string
	NumeroVersion    int
	Contenido        AtencionContenido
	MotivoCorreccion *string
	CreadaPor        string
	CreadaEn         time.Time
}

// AtencionResumen: fila del listado, SIN contenido clínico.
type AtencionResumen struct {
	Atencion
	FechaAtencion string
	HoraAtencion  string
	Motivos       []OpcionCatalogo
}

const atencionColumns = `a.id::text, a.expediente_id::text, a.numero, a.estado, a.autor_id::text,
	a.registrada_por::text, a.registrada_en, a.version_vigente, a.revision, a.anulada_por::text,
	a.anulada_en, a.motivo_anulacion_cifrado, a.cifrado_key_id, a.created_at, a.updated_at`

func scanAtencionInto(a *Atencion, extra ...any) []any {
	return append([]any{&a.ID, &a.ExpedienteID, &a.Numero, &a.Estado, &a.AutorID, &a.RegistradaPor,
		&a.RegistradaEn, &a.VersionVigente, &a.Revision, &a.AnuladaPor, &a.AnuladaEn,
		&a.motivoAnulacionCifrado, &a.cifradoKeyID, &a.CreatedAt, &a.UpdatedAt}, extra...)
}

// abrir descifra el motivo de anulación de una atención leída de la BD.
func (r *AtencionRepository) abrir(a *Atencion, err error) (*Atencion, error) {
	if err != nil || a == nil {
		return a, err
	}
	a.MotivoAnulacion, err = descifrarTexto(r.cifrador, a.motivoAnulacionCifrado, a.cifradoKeyID,
		clinicalcrypto.AADMotivoAnulacion(a.ID))
	if err != nil {
		return nil, err
	}
	return a, nil
}

func scanAtencion(row pgx.Row) (*Atencion, error) {
	a := &Atencion{}
	if err := row.Scan(scanAtencionInto(a)...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAtencionNotFound
		}
		return nil, err
	}
	return a, nil
}

// Solo metadatos y categorías en claro; los textos libres vienen en
// contenido_cifrado y se descifran en scanVersion.
const versionColumns = `v.id::text, v.atencion_id::text, v.numero_version, to_char(v.fecha_atencion, 'YYYY-MM-DD'),
	to_char(v.hora_atencion, 'HH24:MI'), v.realizada_por, v.motivos, v.derivaciones,
	v.contenido_cifrado, v.cifrado_key_id, v.creada_por::text, v.creada_en`

// scanVersion lee una versión y descifra su contenido. Una fila sin bloque
// cifrado (no migrada) es un error: no existe lectura de respaldo en claro.
func (r *AtencionRepository) scanVersion(row pgx.Row) (*AtencionVersion, error) {
	v := &AtencionVersion{}
	var fecha, hora, keyID *string
	var motivos, derivaciones, blob []byte
	c := &v.Contenido
	err := row.Scan(&v.ID, &v.AtencionID, &v.NumeroVersion, &fecha, &hora, &c.RealizadaPor,
		&motivos, &derivaciones, &blob, &keyID, &v.CreadaPor, &v.CreadaEn)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrVersionNotFound
		}
		return nil, err
	}
	if fecha != nil {
		c.FechaAtencion = *fecha
	}
	if hora != nil {
		c.HoraAtencion = *hora
	}
	if err := json.Unmarshal(motivos, &c.Motivos); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(derivaciones, &c.Derivaciones); err != nil {
		return nil, err
	}
	if blob == nil || keyID == nil {
		return nil, ErrContenidoSinCifrar
	}
	cc, err := DescifrarContenidoVersion(r.cifrador, blob, *keyID, v.ID, v.AtencionID)
	if err != nil {
		return nil, err
	}
	cc.aplicarA(c)
	if cc.MotivoCorreccion != "" {
		m := cc.MotivoCorreccion
		v.MotivoCorreccion = &m
	}
	return v, nil
}

type AtencionRepository struct {
	db       *pgxpool.Pool
	cifrador Cifrador
}

func NewAtencionRepository(db *pgxpool.Pool, cifrador Cifrador) *AtencionRepository {
	return &AtencionRepository{db: db, cifrador: cifrador}
}

func (r *AtencionRepository) withTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// ResolverCatalogo devuelve las opciones (con su etiqueta vigente) de los
// códigos pedidos. Falla si alguno no existe o está inactivo.
func ResolverCatalogo(ctx context.Context, q querier, tabla string, codigos []string) ([]OpcionCatalogo, error) {
	if tabla != CatalogoMotivos && tabla != CatalogoDestinos {
		return nil, fmt.Errorf("catalogo no admitido: %s", tabla)
	}
	if len(codigos) == 0 {
		return []OpcionCatalogo{}, nil
	}
	rows, err := q.Query(ctx, `SELECT codigo, etiqueta FROM `+tabla+` WHERE codigo = ANY($1) AND activo ORDER BY orden`, codigos)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OpcionCatalogo{}
	for rows.Next() {
		var o OpcionCatalogo
		if err := rows.Scan(&o.Codigo, &o.Etiqueta); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) != len(codigos) {
		return nil, ErrCatalogoInvalido
	}
	return out, nil
}

// ContenidoInput es el contenido que envía el usuario (códigos, sin etiquetas).
type ContenidoInput struct {
	AtencionContenido
	MotivoCodigos     []string
	DerivacionCodigos []string
}

// resolver completa las etiquetas congeladas desde los catálogos.
func (in ContenidoInput) resolver(ctx context.Context, q querier) (AtencionContenido, error) {
	c := in.AtencionContenido
	var err error
	if c.Motivos, err = ResolverCatalogo(ctx, q, CatalogoMotivos, in.MotivoCodigos); err != nil {
		return c, err
	}
	if c.Derivaciones, err = ResolverCatalogo(ctx, q, CatalogoDestinos, in.DerivacionCodigos); err != nil {
		return c, err
	}
	if c.RealizadaPor == nil {
		c.RealizadaPor = []string{}
	}
	return c, nil
}

// contenidoArgs: las 5 columnas en claro de una versión más su bloque cifrado
// y key_id (7 valores, en el orden de contenidoColumnas).
func (r *AtencionRepository) contenidoArgs(versionID, atencionID string, c AtencionContenido, motivoCorreccion string) ([]any, error) {
	motivos, err := json.Marshal(c.Motivos)
	if err != nil {
		return nil, err
	}
	derivaciones, err := json.Marshal(c.Derivaciones)
	if err != nil {
		return nil, err
	}
	blob, keyID, err := CifrarContenidoVersion(r.cifrador, versionID, atencionID, cifrableDesde(c, motivoCorreccion))
	if err != nil {
		return nil, err
	}
	return []any{c.FechaAtencion, c.HoraAtencion, c.RealizadaPor, motivos, derivaciones, blob, keyID}, nil
}

// Los 7 valores de contenidoArgs, a partir de $offset.
func contenidoPlaceholders(offset int) string {
	p := func(i int) string { return fmt.Sprintf("$%d", offset+i) }
	return fmt.Sprintf(`NULLIF(%s::text, '')::date, NULLIF(%s::text, '')::time, %s::text[], %s::jsonb, %s::jsonb, %s::bytea, %s::text`,
		p(0), p(1), p(2), p(3), p(4), p(5), p(6))
}

const contenidoColumnas = `fecha_atencion, hora_atencion, realizada_por, motivos, derivaciones, contenido_cifrado, cifrado_key_id`

// nuevoID genera el id de una fila antes de insertarla: el AAD del bloque
// cifrado lo necesita.
func nuevoID(ctx context.Context, tx pgx.Tx) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text`).Scan(&id)
	return id, err
}

// ---------------------------------------------------------------------------
// Lecturas
// ---------------------------------------------------------------------------

func (r *AtencionRepository) GetByID(ctx context.Context, id string) (*Atencion, error) {
	return r.abrir(scanAtencion(r.db.QueryRow(ctx, `SELECT `+atencionColumns+` FROM atenciones_psicologicas a WHERE a.id = $1::uuid`, id)))
}

func (r *AtencionRepository) GetVersion(ctx context.Context, atencionID string, numeroVersion int) (*AtencionVersion, error) {
	return r.scanVersion(r.db.QueryRow(ctx, `SELECT `+versionColumns+` FROM atencion_versiones v
		WHERE v.atencion_id = $1::uuid AND v.numero_version = $2`, atencionID, numeroVersion))
}

// ListVersions devuelve solo las versiones oficiales (sin el borrador).
func (r *AtencionRepository) ListVersions(ctx context.Context, atencionID string) ([]*AtencionVersion, error) {
	rows, err := r.db.Query(ctx, `SELECT `+versionColumns+` FROM atencion_versiones v
		WHERE v.atencion_id = $1::uuid AND v.numero_version > 0 ORDER BY v.numero_version`, atencionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*AtencionVersion{}
	for rows.Next() {
		v, err := r.scanVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

type ListAtencionesFilter struct {
	ExpedienteID string
	ActorID      string // los borradores solo aparecen para su autor
	Estado       string
	Page         int
	PageSize     int
}

// ListByExpediente: registradas y anuladas de todos, borradores solo del
// autor, descartadas nunca. Toma fecha, hora y motivos de la versión vigente
// (la 0 para un borrador); nunca devuelve texto clínico.
func (r *AtencionRepository) ListByExpediente(ctx context.Context, f ListAtencionesFilter) ([]*AtencionResumen, int, error) {
	where := `a.expediente_id = $1::uuid
		AND (a.estado IN ('REGISTRADA', 'ANULADA') OR (a.estado = 'BORRADOR' AND a.autor_id = $2::uuid))
		AND ($3::text = '' OR a.estado = $3)`
	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM atenciones_psicologicas a WHERE `+where,
		f.ExpedienteID, f.ActorID, f.Estado).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(ctx, `
		SELECT `+atencionColumns+`, to_char(v.fecha_atencion, 'YYYY-MM-DD'), to_char(v.hora_atencion, 'HH24:MI'), v.motivos
		FROM atenciones_psicologicas a
		JOIN atencion_versiones v ON v.atencion_id = a.id AND v.numero_version = a.version_vigente
		WHERE `+where+`
		ORDER BY a.numero DESC NULLS FIRST, a.created_at DESC
		LIMIT $4 OFFSET $5`,
		f.ExpedienteID, f.ActorID, f.Estado, f.PageSize, (f.Page-1)*f.PageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []*AtencionResumen{}
	for rows.Next() {
		item := &AtencionResumen{}
		var fecha, hora *string
		var motivos []byte
		if err := rows.Scan(scanAtencionInto(&item.Atencion, &fecha, &hora, &motivos)...); err != nil {
			return nil, 0, err
		}
		if fecha != nil {
			item.FechaAtencion = *fecha
		}
		if hora != nil {
			item.HoraAtencion = *hora
		}
		if err := json.Unmarshal(motivos, &item.Motivos); err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	return out, total, rows.Err()
}

// ---------------------------------------------------------------------------
// Escrituras: todas en una transacción que (1) bloquea el expediente
// verificando el alcance, (2) bloquea la atención y (3) escribe el evento de
// auditoría. El orden de bloqueo (expediente -> atención) es siempre el mismo.
// ---------------------------------------------------------------------------

type CreateDraftInput struct {
	ExpedienteID       string
	ScopeResponsableID string
	ActorID            string
	Contenido          ContenidoInput
}

func (r *AtencionRepository) CreateDraft(ctx context.Context, in CreateDraftInput) (*Atencion, error) {
	var created *Atencion
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		estado, estudianteID, err := lockScoped(ctx, tx, in.ExpedienteID, in.ScopeResponsableID)
		if err != nil {
			return err
		}
		if estado == "CERRADO" {
			return ErrExpedienteCerrado
		}
		contenido, err := in.Contenido.resolver(ctx, tx)
		if err != nil {
			return err
		}
		created, err = scanAtencion(tx.QueryRow(ctx, `
			INSERT INTO atenciones_psicologicas AS a (expediente_id, estado, autor_id)
			VALUES ($1::uuid, 'BORRADOR', $2::uuid)
			RETURNING `+atencionColumns, in.ExpedienteID, in.ActorID))
		if err != nil {
			return err
		}
		versionID, err := nuevoID(ctx, tx)
		if err != nil {
			return err
		}
		args, err := r.contenidoArgs(versionID, created.ID, contenido, "")
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO atencion_versiones (id, atencion_id, numero_version, creada_por, `+contenidoColumnas+`)
			VALUES ($1::uuid, $2::uuid, 0, $3::uuid, `+contenidoPlaceholders(4)+`)`,
			append([]any{versionID, created.ID, in.ActorID}, args...)...); err != nil {
			return err
		}
		return insertAuditoria(ctx, tx, AuditEntry{
			UsuarioID: in.ActorID, Accion: AccionCreate, Recurso: RecursoAtencion,
			RecursoID: created.ID, EstudianteID: estudianteID, Resultado: ResultadoPermitido, Motivo: "BORRADOR",
		})
	})
	if err != nil {
		return nil, err
	}
	return r.abrir(created, nil)
}

// lockAtencion bloquea el expediente (con alcance) y la atención, en ese
// orden. Devuelve la atención bloqueada y el estado/estudiante del expediente.
func lockAtencion(ctx context.Context, tx pgx.Tx, id, scopeResponsableID string) (*Atencion, string, string, error) {
	var expedienteID string
	if err := tx.QueryRow(ctx, `SELECT expediente_id::text FROM atenciones_psicologicas WHERE id = $1::uuid`, id).Scan(&expedienteID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", "", ErrAtencionNotFound
		}
		return nil, "", "", err
	}
	expEstado, estudianteID, err := lockScoped(ctx, tx, expedienteID, scopeResponsableID)
	if err != nil {
		if errors.Is(err, ErrExpedienteNotFound) {
			return nil, "", "", ErrAtencionNotFound
		}
		return nil, "", "", err
	}
	a, err := scanAtencion(tx.QueryRow(ctx, `SELECT `+atencionColumns+` FROM atenciones_psicologicas a WHERE a.id = $1::uuid FOR UPDATE`, id))
	if err != nil {
		return nil, "", "", err
	}
	return a, expEstado, estudianteID, nil
}

// lockBorradorPropio: la atención debe ser un borrador del actor con la
// revisión esperada. Un borrador ajeno es indistinguible de uno inexistente.
func lockBorradorPropio(ctx context.Context, tx pgx.Tx, id, scope, actorID string, revision int) (*Atencion, string, string, error) {
	a, expEstado, estudianteID, err := lockAtencion(ctx, tx, id, scope)
	if err != nil {
		return nil, "", "", err
	}
	if a.Estado != AtencionBorrador || a.AutorID != actorID {
		if a.Estado == AtencionBorrador || a.Estado == AtencionDescartada {
			return nil, "", "", ErrAtencionNotFound
		}
		return nil, "", "", ErrAtencionEstado
	}
	if a.Revision != revision {
		return nil, "", "", ErrAtencionConflicto
	}
	return a, expEstado, estudianteID, nil
}

type UpdateDraftInput struct {
	ID                 string
	Revision           int
	ScopeResponsableID string
	ActorID            string
	Contenido          ContenidoInput
}

func (r *AtencionRepository) UpdateDraft(ctx context.Context, in UpdateDraftInput) (*Atencion, error) {
	var updated *Atencion
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		a, expEstado, estudianteID, err := lockBorradorPropio(ctx, tx, in.ID, in.ScopeResponsableID, in.ActorID, in.Revision)
		if err != nil {
			return err
		}
		if expEstado == "CERRADO" {
			return ErrExpedienteCerrado
		}
		contenido, err := in.Contenido.resolver(ctx, tx)
		if err != nil {
			return err
		}
		var versionID string
		if err := tx.QueryRow(ctx, `SELECT id::text FROM atencion_versiones
			WHERE atencion_id = $1::uuid AND numero_version = 0 FOR UPDATE`, a.ID).Scan(&versionID); err != nil {
			return err
		}
		args, err := r.contenidoArgs(versionID, a.ID, contenido, "")
		if err != nil {
			return err
		}
		// Cada guardado vuelve a cifrar el borrador completo con un nonce nuevo.
		if _, err := tx.Exec(ctx, `
			UPDATE atencion_versiones SET (`+contenidoColumnas+`) = (`+contenidoPlaceholders(2)+`),
				creada_en = CURRENT_TIMESTAMP
			WHERE id = $1::uuid`,
			append([]any{versionID}, args...)...); err != nil {
			return err
		}
		updated, err = scanAtencion(tx.QueryRow(ctx, `
			UPDATE atenciones_psicologicas a SET revision = revision + 1, updated_at = CURRENT_TIMESTAMP
			WHERE a.id = $1::uuid RETURNING `+atencionColumns, a.ID))
		if err != nil {
			return err
		}
		return insertAuditoria(ctx, tx, AuditEntry{
			UsuarioID: in.ActorID, Accion: AccionUpdate, Recurso: RecursoAtencion,
			RecursoID: a.ID, EstudianteID: estudianteID, Resultado: ResultadoPermitido, Motivo: "BORRADOR",
		})
	})
	if err != nil {
		return nil, err
	}
	return r.abrir(updated, nil)
}

type TransitionInput struct {
	ID                 string
	Revision           int
	ScopeResponsableID string
	ActorID            string
	// Validar recibe el contenido que se va a oficializar (registrar o
	// corregir). Un error de validación revierte toda la transacción.
	Validar func(AtencionContenido) error
}

// Register convierte el borrador (versión 0) en la versión 1 inmutable y le
// asigna el siguiente número del expediente.
func (r *AtencionRepository) Register(ctx context.Context, in TransitionInput) (*Atencion, error) {
	var registered *Atencion
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		a, expEstado, estudianteID, err := lockBorradorPropio(ctx, tx, in.ID, in.ScopeResponsableID, in.ActorID, in.Revision)
		if err != nil {
			return err
		}
		if expEstado == "CERRADO" {
			return ErrExpedienteCerrado
		}
		// El borrador se renombra a versión 1 sin volver a cifrarlo: su AAD usa
		// el id de la fila, no numero_version.
		borrador, err := r.scanVersion(tx.QueryRow(ctx, `SELECT `+versionColumns+` FROM atencion_versiones v
			WHERE v.atencion_id = $1::uuid AND v.numero_version = 0`, a.ID))
		if err != nil {
			return err
		}
		if in.Validar != nil {
			if err := in.Validar(borrador.Contenido); err != nil {
				return err
			}
		}
		var numero int
		if err := tx.QueryRow(ctx, `
			INSERT INTO atencion_numero_counters (expediente_id, ultimo) VALUES ($1::uuid, 1)
			ON CONFLICT (expediente_id) DO UPDATE SET ultimo = atencion_numero_counters.ultimo + 1
			RETURNING ultimo`, a.ExpedienteID).Scan(&numero); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE atencion_versiones SET numero_version = 1, creada_por = $2::uuid, creada_en = CURRENT_TIMESTAMP
			WHERE atencion_id = $1::uuid AND numero_version = 0`, a.ID, in.ActorID); err != nil {
			return err
		}
		registered, err = scanAtencion(tx.QueryRow(ctx, `
			UPDATE atenciones_psicologicas a SET estado = 'REGISTRADA', numero = $2, registrada_por = $3::uuid,
				registrada_en = CURRENT_TIMESTAMP, version_vigente = 1, revision = revision + 1,
				updated_at = CURRENT_TIMESTAMP
			WHERE a.id = $1::uuid RETURNING `+atencionColumns, a.ID, numero, in.ActorID))
		if err != nil {
			return err
		}
		return insertAuditoria(ctx, tx, AuditEntry{
			UsuarioID: in.ActorID, Accion: AccionRegister, Recurso: RecursoAtencion,
			RecursoID: a.ID, EstudianteID: estudianteID, Resultado: ResultadoPermitido, Motivo: "VERSION_1",
		})
	})
	if err != nil {
		return nil, err
	}
	return r.abrir(registered, nil)
}

// Discard marca un borrador propio como DESCARTADA (no se borra físicamente).
// Se permite aunque el expediente esté cerrado: solo retira un borrador.
func (r *AtencionRepository) Discard(ctx context.Context, in TransitionInput) error {
	return r.withTx(ctx, func(tx pgx.Tx) error {
		a, _, estudianteID, err := lockBorradorPropio(ctx, tx, in.ID, in.ScopeResponsableID, in.ActorID, in.Revision)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE atenciones_psicologicas SET estado = 'DESCARTADA', revision = revision + 1,
			updated_at = CURRENT_TIMESTAMP WHERE id = $1::uuid`, a.ID); err != nil {
			return err
		}
		return insertAuditoria(ctx, tx, AuditEntry{
			UsuarioID: in.ActorID, Accion: AccionDiscard, Recurso: RecursoAtencion,
			RecursoID: a.ID, EstudianteID: estudianteID, Resultado: ResultadoPermitido, Motivo: "BORRADOR",
		})
	})
}

type CorrectInput struct {
	ID                 string
	VersionBase        int
	MotivoCorreccion   string
	ScopeResponsableID string
	ActorID            string
	Contenido          ContenidoInput
	Validar            func(AtencionContenido) error
}

// Correct INSERTA la versión siguiente (nunca modifica la anterior) y la deja
// como vigente. version_base protege contra correcciones concurrentes.
func (r *AtencionRepository) Correct(ctx context.Context, in CorrectInput) (*Atencion, error) {
	var corrected *Atencion
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		a, expEstado, estudianteID, err := lockAtencion(ctx, tx, in.ID, in.ScopeResponsableID)
		if err != nil {
			return err
		}
		if a.Estado == AtencionDescartada {
			return ErrAtencionNotFound
		}
		if expEstado == "CERRADO" {
			return ErrExpedienteCerrado
		}
		if a.Estado != AtencionRegistrada {
			return ErrAtencionEstado
		}
		if a.VersionVigente != in.VersionBase {
			return ErrAtencionConflicto
		}
		contenido, err := in.Contenido.resolver(ctx, tx)
		if err != nil {
			return err
		}
		if in.Validar != nil {
			if err := in.Validar(contenido); err != nil {
				return err
			}
		}
		versionID, err := nuevoID(ctx, tx)
		if err != nil {
			return err
		}
		// El motivo de la corrección viaja dentro del bloque cifrado.
		args, err := r.contenidoArgs(versionID, a.ID, contenido, in.MotivoCorreccion)
		if err != nil {
			return err
		}
		nueva := a.VersionVigente + 1
		if _, err := tx.Exec(ctx, `
			INSERT INTO atencion_versiones (id, atencion_id, numero_version, version_anterior_id, creada_por, `+contenidoColumnas+`)
			SELECT $1::uuid, $2::uuid, $3, v.id, $4::uuid, `+contenidoPlaceholders(6)+`
			FROM atencion_versiones v WHERE v.atencion_id = $2::uuid AND v.numero_version = $5`,
			append([]any{versionID, a.ID, nueva, in.ActorID, a.VersionVigente}, args...)...); err != nil {
			return err
		}
		corrected, err = scanAtencion(tx.QueryRow(ctx, `
			UPDATE atenciones_psicologicas a SET version_vigente = $2, revision = revision + 1, updated_at = CURRENT_TIMESTAMP
			WHERE a.id = $1::uuid RETURNING `+atencionColumns, a.ID, nueva))
		if err != nil {
			return err
		}
		return insertAuditoria(ctx, tx, AuditEntry{
			UsuarioID: in.ActorID, Accion: AccionCorrect, Recurso: RecursoAtencion,
			RecursoID: a.ID, EstudianteID: estudianteID, Resultado: ResultadoPermitido,
			Motivo: fmt.Sprintf("VERSION_%d", nueva),
		})
	})
	if err != nil {
		return nil, err
	}
	return r.abrir(corrected, nil)
}

type AnnulInput struct {
	ID                 string
	Motivo             string
	ScopeResponsableID string
	ActorID            string
}

// Annul pasa una atención REGISTRADA a ANULADA con motivo. No borra nada.
func (r *AtencionRepository) Annul(ctx context.Context, in AnnulInput) (*Atencion, error) {
	var annulled *Atencion
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		a, expEstado, estudianteID, err := lockAtencion(ctx, tx, in.ID, in.ScopeResponsableID)
		if err != nil {
			return err
		}
		if a.Estado == AtencionDescartada {
			return ErrAtencionNotFound
		}
		if expEstado == "CERRADO" {
			return ErrExpedienteCerrado
		}
		if a.Estado != AtencionRegistrada {
			return ErrAtencionEstado
		}
		blob, keyID, err := cifrarTexto(r.cifrador, in.Motivo, clinicalcrypto.AADMotivoAnulacion(a.ID))
		if err != nil {
			return err
		}
		annulled, err = scanAtencion(tx.QueryRow(ctx, `
			UPDATE atenciones_psicologicas a SET estado = 'ANULADA', anulada_por = $2::uuid,
				anulada_en = CURRENT_TIMESTAMP, motivo_anulacion_cifrado = $3, cifrado_key_id = $4,
				revision = revision + 1, updated_at = CURRENT_TIMESTAMP
			WHERE a.id = $1::uuid RETURNING `+atencionColumns, a.ID, in.ActorID, blob, keyID))
		if err != nil {
			return err
		}
		return insertAuditoria(ctx, tx, AuditEntry{
			UsuarioID: in.ActorID, Accion: AccionAnnul, Recurso: RecursoAtencion,
			RecursoID: a.ID, EstudianteID: estudianteID, Resultado: ResultadoPermitido,
			Motivo: fmt.Sprintf("VERSION_%d", a.VersionVigente),
		})
	})
	if err != nil {
		return nil, err
	}
	return r.abrir(annulled, nil)
}
