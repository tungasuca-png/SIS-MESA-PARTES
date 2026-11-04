package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
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
	// DNIDeclarado: DNI escrito en el registro público, sin verificar.
	DNIDeclarado string
}

// IdentidadPendiente: declaró un DNI que todavía no se verificó.
func (u *Usuario) IdentidadPendiente() bool {
	return u.DNI == "" && u.DNIDeclarado != ""
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

	// Migración 003 (dni_declarado): si todavía no se aplicó, los perfiles se
	// leen como antes y las operaciones del DNI declarado responden
	// ErrEsquemaPendiente. Se vuelve a comprobar cada 30 s mientras falte, así
	// que aplicar la migración no requiere reiniciar el servicio.
	esquemaMu    sync.Mutex
	conDeclarado bool
	comprobado   time.Time
}

// ErrEsquemaPendiente: la migración 003 (dni_declarado) no está aplicada.
var ErrEsquemaPendiente = errors.New("migración de dni_declarado pendiente")

const usuarioColumnsSinDeclarado = `
	id, nombres, apellidos, dni, telefono, correo, direccion,
	tipo_usuario, activo, created_at, updated_at, '' AS dni_declarado
`

// tieneDeclarado: la columna usuarios.dni_declarado existe. Una vez vista,
// queda fija; si falta, se vuelve a consultar como mucho cada 30 s.
func (r *UsuarioRepository) tieneDeclarado(ctx context.Context) bool {
	r.esquemaMu.Lock()
	defer r.esquemaMu.Unlock()
	if r.conDeclarado || time.Since(r.comprobado) < 30*time.Second {
		return r.conDeclarado
	}
	var existe bool
	if err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = 'usuarios' AND column_name = 'dni_declarado'
		)`).Scan(&existe); err != nil {
		return false
	}
	r.conDeclarado, r.comprobado = existe, time.Now()
	return existe
}

// columnas: lista de columnas según el esquema disponible.
func (r *UsuarioRepository) columnas(ctx context.Context) string {
	if r.tieneDeclarado(ctx) {
		return usuarioColumns
	}
	return usuarioColumnsSinDeclarado
}

func NewUsuarioRepository(db *pgxpool.Pool) *UsuarioRepository {
	return &UsuarioRepository{db: db}
}

const usuarioColumns = `
	id, nombres, apellidos, dni, telefono, correo, direccion,
	tipo_usuario, activo, created_at, updated_at, dni_declarado
`

func scanUsuario(row pgx.Row) (*Usuario, error) {
	u := &Usuario{}
	err := row.Scan(
		&u.ID, &u.Nombres, &u.Apellidos, &u.DNI, &u.Telefono, &u.Correo,
		&u.Direccion, &u.TipoUsuario, &u.Activo, &u.CreatedAt, &u.UpdatedAt, &u.DNIDeclarado,
	)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *UsuarioRepository) FindByID(ctx context.Context, id string) (*Usuario, error) {
	found, err := scanUsuario(r.db.QueryRow(ctx, `
		SELECT `+r.columnas(ctx)+`
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
		SELECT `+r.columnas(ctx)+`
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
	`, r.columnas(ctx), whereClause, len(pagedArgs)-1, len(pagedArgs)), pagedArgs...)
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
// CreateIfNotExists: crea el perfil solo si no existe (ON CONFLICT DO
// NOTHING). Devuelve el existente con creado = false si ya estaba: un
// reintento nunca sobrescribe datos. El índice único de DNI decide los
// duplicados (ErrDniExists).
func (r *UsuarioRepository) CreateIfNotExists(ctx context.Context, u *Usuario) (*Usuario, bool, error) {
	var row pgx.Row
	if r.tieneDeclarado(ctx) {
		row = r.db.QueryRow(ctx, `
			INSERT INTO usuarios (id, nombres, apellidos, dni, telefono, correo, direccion, tipo_usuario, dni_declarado)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (id) DO NOTHING
			RETURNING `+usuarioColumns,
			u.ID, u.Nombres, u.Apellidos, u.DNI, u.Telefono, u.Correo, u.Direccion, u.TipoUsuario, u.DNIDeclarado,
		)
	} else {
		// Sin la migración 003 un DNI declarado no puede guardarse sin
		// reservarlo: no se crea nada.
		if u.DNIDeclarado != "" {
			return nil, false, ErrEsquemaPendiente
		}
		row = r.db.QueryRow(ctx, `
			INSERT INTO usuarios (id, nombres, apellidos, dni, telefono, correo, direccion, tipo_usuario)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (id) DO NOTHING
			RETURNING `+usuarioColumnsSinDeclarado,
			u.ID, u.Nombres, u.Apellidos, u.DNI, u.Telefono, u.Correo, u.Direccion, u.TipoUsuario,
		)
	}
	saved, err := scanUsuario(row)
	if err == nil {
		return saved, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, classifyUniqueViolation(err)
	}
	existing, err := r.FindByID(ctx, u.ID)
	if err != nil {
		return nil, false, err
	}
	return existing, false, nil
}

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
		RETURNING `+r.columnas(ctx),
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
		RETURNING `+r.columnas(ctx),
		id, telefono, correo, direccion,
	))
	if err != nil {
		return nil, classifyNotFound(err, ErrUsuarioNotFound)
	}
	return found, nil
}

// UpdateTipoUsuario (sincronización Usuario -> Rol) actualiza ÚNICAMENTE
// tipo_usuario — a propósito NO es un Upsert: no crea perfiles nuevos ni
// toca nombres/apellidos/dni/telefono/correo/direccion/activo. Auth Service
// sigue siendo la fuente de verdad real del rol; este método solo refleja
// ese resultado YA confirmado en la copia denormalizada de este servicio.
// Si el id no tiene perfil registrado, la sentencia no afecta ninguna fila
// y se traduce en ErrUsuarioNotFound, igual que UpdateOwnProfile.
func (r *UsuarioRepository) UpdateTipoUsuario(ctx context.Context, id, tipoUsuario string) (*Usuario, error) {
	found, err := scanUsuario(r.db.QueryRow(ctx, `
		UPDATE usuarios
		SET tipo_usuario = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING `+r.columnas(ctx),
		id, tipoUsuario,
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

// ErrDNINoAsignable: el perfil no es de un solicitante, no declaró ese DNI o
// ya tiene otro DNI definitivo.
var ErrDNINoAsignable = errors.New("dni no asignable a este perfil")

// AsignarDNIVerificado pasa el DNI declarado al DNI definitivo de un perfil
// SOLICITANTE (verificación por Secretaría). Idempotente si ya lo tenía. Si
// el DNI pertenece a otro perfil, el índice único lo rechaza (ErrDniExists)
// y nada cambia.
func (r *UsuarioRepository) AsignarDNIVerificado(ctx context.Context, id, dni string) (*Usuario, error) {
	if !r.tieneDeclarado(ctx) {
		return nil, ErrEsquemaPendiente
	}
	found, err := scanUsuario(r.db.QueryRow(ctx, `
		UPDATE usuarios
		SET dni = $2, updated_at = NOW()
		WHERE id = $1 AND tipo_usuario = 'SOLICITANTE' AND dni_declarado = $2 AND (dni = '' OR dni = $2)
		RETURNING `+r.columnas(ctx),
		id, dni,
	))
	if err == nil {
		return found, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, classifyUniqueViolation(err)
	}
	if _, err := r.FindByID(ctx, id); err != nil {
		return nil, err
	}
	return nil, ErrDNINoAsignable
}

// RevertirDNIVerificado deshace AsignarDNIVerificado cuando Auth no pudo
// completar la verificación (p. ej. el DNI pertenece a otra cuenta en Auth).
// Solo afecta a un perfil SOLICITANTE cuyo DNI definitivo es exactamente el
// declarado; si no, no cambia nada (idempotente).
func (r *UsuarioRepository) RevertirDNIVerificado(ctx context.Context, id, dni string) (*Usuario, error) {
	if !r.tieneDeclarado(ctx) {
		return nil, ErrEsquemaPendiente
	}
	found, err := scanUsuario(r.db.QueryRow(ctx, `
		UPDATE usuarios
		SET dni = '', updated_at = NOW()
		WHERE id = $1 AND tipo_usuario = 'SOLICITANTE' AND dni_declarado = $2 AND dni = $2
		RETURNING `+r.columnas(ctx),
		id, dni,
	))
	if err == nil {
		return found, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

// RegistroPublicoDisponible comprueba, en cada llamada y sin escribir nada,
// que la base responde y que el esquema admite perfiles con DNI declarado
// (migración 003): falla si la columna no existe o no se puede leer.
func (r *UsuarioRepository) RegistroPublicoDisponible(ctx context.Context) error {
	rows, err := r.db.Query(ctx, `SELECT dni_declarado FROM usuarios LIMIT 0`)
	if err != nil {
		return err
	}
	rows.Close()
	return rows.Err()
}
