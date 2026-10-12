package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Estudiante struct {
	ID              string
	Codigo          string
	Nombres         string
	Apellidos       string
	NumeroDocumento string
	Nivel           string
	Grado           string
	Seccion         string
	Turno           string
	EstadoMatricula string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	// FASE E-04: identificación básica completa. Los datos personales y
	// sensibles (fecha de nacimiento, sexo, lugar de nacimiento, lenguas,
	// foto) NO se leen: ningún RPC actual los expone.
	TipoDocumento   string
	CodigoSiagie    string
	ApellidoPaterno string
	ApellidoMaterno string
	Estado          string
	// FASE E-05: versión del registro (control de concurrencia).
	Version int
}

func (e *Estudiante) NombreCompleto() string {
	return strings.TrimSpace(e.Nombres + " " + e.Apellidos)
}

// estudianteColumns (FASE 5D): agrega numero_documento — el selector de
// estudiantes del frontend lo necesita para el buscador por DNI y para
// mostrarlo en el resultado (ver EstudianteBasico).
//
// FASE E-03: numero_documento admite NULL (estudiante SIN_DOCUMENTO); se
// devuelve como "" para no cambiar el contrato protobuf existente.
//
// FASE E-04: agrega tipo_documento, codigo_siagie, apellidos separados y
// estado del registro (los opcionales como ""). texto_busqueda no se lee.
//
// FASE E-05: agrega version (no es un dato personal).
const estudianteColumns = `
	id, codigo, nombres, apellidos, COALESCE(numero_documento, '') AS numero_documento, nivel, grado, seccion, turno,
	estado_matricula, created_at, updated_at,
	tipo_documento, COALESCE(codigo_siagie, ''), COALESCE(apellido_paterno, ''), COALESCE(apellido_materno, ''), estado,
	version
`

func scanEstudiante(row pgx.Row) (*Estudiante, error) {
	e := &Estudiante{}
	err := row.Scan(
		&e.ID, &e.Codigo, &e.Nombres, &e.Apellidos, &e.NumeroDocumento, &e.Nivel, &e.Grado,
		&e.Seccion, &e.Turno, &e.EstadoMatricula, &e.CreatedAt, &e.UpdatedAt,
		&e.TipoDocumento, &e.CodigoSiagie, &e.ApellidoPaterno, &e.ApellidoMaterno, &e.Estado,
		&e.Version,
	)
	if err != nil {
		return nil, err
	}
	return e, nil
}

type EstudianteRepository struct {
	db *pgxpool.Pool
}

func NewEstudianteRepository(db *pgxpool.Pool) *EstudianteRepository {
	return &EstudianteRepository{db: db}
}

func (r *EstudianteRepository) GetByID(ctx context.Context, id string) (*Estudiante, error) {
	found, err := scanEstudiante(r.db.QueryRow(ctx, `
		SELECT `+estudianteColumns+`
		FROM estudiantes
		WHERE id = $1
	`, id))
	if err != nil {
		return nil, classifyNotFound(err)
	}
	return found, nil
}

// GetBasicByIDs resuelve varios estudiantes en una sola consulta (mismo
// patrón que Usuarios.FindManyByIDs): evita N llamadas cuando otro servicio
// ya trae una lista de estudiante_id y solo necesita mostrarlos.
func (r *EstudianteRepository) GetBasicByIDs(ctx context.Context, ids []string) ([]*Estudiante, error) {
	if len(ids) == 0 {
		return []*Estudiante{}, nil
	}

	rows, err := r.db.Query(ctx, `
		SELECT `+estudianteColumns+`
		FROM estudiantes
		WHERE id = ANY($1)
		ORDER BY apellidos, nombres, id
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectEstudiantes(rows)
}

// SearchFilter (FASE E-04). Terms: palabras ya separadas por la lógica (en
// cualquier orden: "Perez Juan" equivale a "Juan Perez"); vacío = listado
// general. Un filtro vacío no se aplica. Nivel/Grado/Seccion/Turno son
// temporales: pasarán al dominio Matrícula.
type SearchFilter struct {
	Terms    []string
	Estado   string
	Nivel    string
	Grado    string
	Seccion  string
	Turno    string
	Page     int
	PageSize int
}

// likeEscaper neutraliza los comodines de LIKE en lo que escribe el usuario:
// "%" y "_" se buscan como texto literal, no como patrón.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// Search (FASE E-04): cada palabra buscada debe aparecer en texto_busqueda,
// columna generada (migración 003) con código, código SIAGIE, documento,
// nombres y apellidos, normalizada por estudiantes_normalizar (sin tildes, en
// minúsculas, espacios colapsados). La misma función normaliza la consulta,
// así "JOSÉ", "jose" y "José" coinciden. Cada condición puede usar el índice
// de trigramas idx_estudiantes_texto_busqueda_trgm.
//
// Todo va parametrizado: lo que envía el usuario nunca se concatena al SQL;
// los nombres de columna de los filtros son constantes de este archivo. El
// orden es fijo y determinista (apellidos, nombres, id): no hay orden elegido
// por el cliente.
func (r *EstudianteRepository) Search(ctx context.Context, filter SearchFilter) ([]*Estudiante, int, error) {
	conds := make([]string, 0, len(filter.Terms)+5)
	args := make([]any, 0, len(filter.Terms)+7)
	param := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	for _, term := range filter.Terms {
		conds = append(conds, `texto_busqueda LIKE '%' || estudiantes_normalizar(`+param(likeEscaper.Replace(term))+`) || '%'`)
	}
	for _, f := range []struct{ col, val string }{
		{"estado", filter.Estado},
		{"nivel", filter.Nivel},
		{"grado", filter.Grado},
		{"seccion", filter.Seccion},
		{"turno", filter.Turno},
	} {
		if f.val != "" {
			conds = append(conds, f.col+" = "+param(f.val))
		}
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM estudiantes `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := param(filter.PageSize)
	offset := param((filter.Page - 1) * filter.PageSize)
	rows, err := r.db.Query(ctx, `
		SELECT `+estudianteColumns+`
		FROM estudiantes `+where+`
		ORDER BY apellidos, nombres, id
		LIMIT `+limit+` OFFSET `+offset, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items, err := collectEstudiantes(rows)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func collectEstudiantes(rows pgx.Rows) ([]*Estudiante, error) {
	items := make([]*Estudiante, 0)
	for rows.Next() {
		e, err := scanEstudiante(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	return items, rows.Err()
}
