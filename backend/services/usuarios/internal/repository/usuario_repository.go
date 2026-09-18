package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Usuario struct {
	ID          string
	Nombres     string
	Apellidos   string
	DNI         string
	Telefono    string
	Correo      string
	Direccion   string
	TipoUsuario string
	Activo      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (u *Usuario) NombreCompleto() string {
	return strings.TrimSpace(u.Nombres + " " + u.Apellidos)
}

type ListFilter struct {
	TipoUsuario string
	Query       string
	Page        int
	PageSize    int
}

type UsuarioRepository struct {
	db *pgxpool.Pool
}

func NewUsuarioRepository(db *pgxpool.Pool) *UsuarioRepository {
	return &UsuarioRepository{db: db}
}

const usuarioColumns = `
	id, nombres, apellidos, dni, telefono, correo, direccion,
	tipo_usuario, activo, created_at, updated_at
`

func scanUsuario(row pgx.Row) (*Usuario, error) {
	u := &Usuario{}
	err := row.Scan(
		&u.ID, &u.Nombres, &u.Apellidos, &u.DNI, &u.Telefono, &u.Correo,
		&u.Direccion, &u.TipoUsuario, &u.Activo, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *UsuarioRepository) FindByID(ctx context.Context, id string) (*Usuario, error) {
	found, err := scanUsuario(r.db.QueryRow(ctx, `
		SELECT `+usuarioColumns+`
		FROM usuarios
		WHERE id = $1
	`, id))
	if err != nil {
		return nil, classifyNotFound(err, ErrUsuarioNotFound)
	}
	return found, nil
}

// FindManyByIDs resuelve varios usuarios en una sola consulta: evita N
// llamadas cuando hay que resolver una lista de expedientes.
func (r *UsuarioRepository) FindManyByIDs(ctx context.Context, ids []string) ([]*Usuario, error) {
	if len(ids) == 0 {
		return []*Usuario{}, nil
	}

	rows, err := r.db.Query(ctx, `
		SELECT `+usuarioColumns+`
		FROM usuarios
		WHERE id = ANY($1)
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectUsuarios(rows)
}

func (r *UsuarioRepository) List(ctx context.Context, filter ListFilter) ([]*Usuario, int, error) {
	conditions := []string{"TRUE"}
	args := make([]any, 0, 2)

	if filter.TipoUsuario != "" {
		args = append(args, filter.TipoUsuario)
		conditions = append(conditions, fmt.Sprintf("tipo_usuario = $%d", len(args)))
	}
	if filter.Query != "" {
		args = append(args, "%"+strings.ToLower(filter.Query)+"%")
		conditions = append(conditions, fmt.Sprintf(
			"(LOWER(nombres) LIKE $%d OR LOWER(apellidos) LIKE $%d OR LOWER(dni) LIKE $%d)",
			len(args), len(args), len(args),
		))
	}
	whereClause := strings.Join(conditions, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM usuarios WHERE "+whereClause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	pagedArgs := append(append([]any{}, args...), filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT %s
		FROM usuarios
		WHERE %s
		ORDER BY apellidos, nombres
		LIMIT $%d OFFSET $%d
	`, usuarioColumns, whereClause, len(pagedArgs)-1, len(pagedArgs)), pagedArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items, err := collectUsuarios(rows)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Upsert crea o actualiza el perfil asociado a un UUID que ya existe en Auth.
// El id siempre viene de afuera: este servicio no genera identidades.
func (r *UsuarioRepository) Upsert(ctx context.Context, u *Usuario) (*Usuario, error) {
	saved, err := scanUsuario(r.db.QueryRow(ctx, `
		INSERT INTO usuarios (id, nombres, apellidos, dni, telefono, correo, direccion, tipo_usuario)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			nombres = EXCLUDED.nombres,
			apellidos = EXCLUDED.apellidos,
			dni = EXCLUDED.dni,
			telefono = EXCLUDED.telefono,
			correo = EXCLUDED.correo,
			direccion = EXCLUDED.direccion,
			tipo_usuario = EXCLUDED.tipo_usuario,
			updated_at = NOW()
		RETURNING `+usuarioColumns,
		u.ID, u.Nombres, u.Apellidos, u.DNI, u.Telefono, u.Correo, u.Direccion, u.TipoUsuario,
	))
	if err != nil {
		return nil, classifyUniqueViolation(err)
	}
	return saved, nil
}

// UpdateOwnProfile (Paso 22E) actualiza ÚNICAMENTE telefono/correo/direccion
// del perfil identificado por id — a propósito NO es un UPDATE genérico: no
// toca nombres, apellidos, dni, tipo_usuario ni activo, precisamente para
// que este método nunca pueda usarse (por error o a futuro) para más de lo
// que la edición del propio perfil debe permitir. Si el id no tiene perfil
// registrado, la sentencia no afecta ninguna fila y RETURNING no devuelve
// nada: se traduce en ErrUsuarioNotFound, sin crear un perfil nuevo.
func (r *UsuarioRepository) UpdateOwnProfile(ctx context.Context, id, telefono, correo, direccion string) (*Usuario, error) {
	found, err := scanUsuario(r.db.QueryRow(ctx, `
		UPDATE usuarios
		SET telefono = $2, correo = $3, direccion = $4, updated_at = NOW()
		WHERE id = $1
		RETURNING `+usuarioColumns,
		id, telefono, correo, direccion,
	))
	if err != nil {
		return nil, classifyNotFound(err, ErrUsuarioNotFound)
	}
	return found, nil
}

func collectUsuarios(rows pgx.Rows) ([]*Usuario, error) {
	items := make([]*Usuario, 0)
	for rows.Next() {
		item, err := scanUsuario(rows)
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
