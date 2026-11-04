package repository

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Privilegios REALES de auth_app sobre el acceso con Google y la función de
// compensación. Solo en un PostgreSQL DESECHABLE con los roles reales
// (auth_owner dueño, auth_migrator, auth_app, auth_tester):
//   AUTH_DB_AISLADA_URL       -> conexión como auth_app
//   AUTH_DB_AISLADA_ADMIN_URL -> conexión como auth_tester (siembra y verificación)
// Ver scratchpad/auth-roles-aislado.sh. Nunca contra auth_db real.

const funcionCompensacion = "public.cancelar_registro_google_pendiente(uuid)"

type rolesReales struct {
	t     *testing.T
	ctx   context.Context
	app   *pgxpool.Pool
	admin *pgxpool.Pool
}

func conRolesReales(t *testing.T) *rolesReales {
	t.Helper()
	appURL, adminURL := os.Getenv("AUTH_DB_AISLADA_URL"), os.Getenv("AUTH_DB_AISLADA_ADMIN_URL")
	if appURL == "" || adminURL == "" {
		t.Skip("se requieren AUTH_DB_AISLADA_URL (auth_app) y AUTH_DB_AISLADA_ADMIN_URL (auth_tester) de un PostgreSQL desechable")
	}
	ctx := context.Background()
	app, err := pgxpool.New(ctx, appURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Close)
	admin, err := pgxpool.New(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	r := &rolesReales{t: t, ctx: ctx, app: app, admin: admin}
	var usuario string
	_ = app.QueryRow(ctx, `SELECT current_user`).Scan(&usuario)
	if usuario != "auth_app" {
		t.Fatalf("AUTH_DB_AISLADA_URL debe conectar como auth_app, no %q", usuario)
	}
	for _, rol := range []string{"ADMIN", "SOLICITANTE"} {
		if _, err := admin.Exec(ctx, `INSERT INTO roles (nombre) VALUES ($1) ON CONFLICT (nombre) DO NOTHING`, rol); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

func (r *rolesReales) n(sql string, args ...any) int {
	r.t.Helper()
	var n int
	if err := r.admin.QueryRow(r.ctx, sql, args...).Scan(&n); err != nil {
		r.t.Fatal(err)
	}
	return n
}

// usuario siembra una cuenta (con un rol) usando auth_tester.
func (r *rolesReales) usuario(rol string) string {
	r.t.Helper()
	v, _ := rand.Int(rand.Reader, big.NewInt(89999999))
	dni := fmt.Sprintf("%08d", 10000000+v.Int64())
	var id string
	err := r.admin.QueryRow(r.ctx, `
		INSERT INTO usuarios (username, email, password_hash, dni) VALUES ($1, $2, 'x', $3) RETURNING id
	`, "perm_"+dni, "perm"+dni+"@prueba.example", dni).Scan(&id)
	if err != nil {
		r.t.Fatal(err)
	}
	if _, err := r.admin.Exec(r.ctx, `INSERT INTO usuario_roles (usuario_id, rol_id) SELECT $1, id FROM roles WHERE nombre = $2`, id, rol); err != nil {
		r.t.Fatal(err)
	}
	return id
}

func (r *rolesReales) identidad(usuarioID string, completo bool) {
	r.t.Helper()
	if _, err := r.admin.Exec(r.ctx, `
		INSERT INTO user_auth_providers (usuario_id, provider, provider_user_id, provider_email, registro_completo)
		VALUES ($1, 'google', $2, 'g@prueba.example', $3)
	`, usuarioID, "sub-"+usuarioID, completo); err != nil {
		r.t.Fatal(err)
	}
}

func denegado(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42501" // insufficient_privilege
}

func TestPermisosAuthApp_TablasGoogleYUsuarios(t *testing.T) {
	r := conRolesReales(t)

	// Matriz exacta de privilegios (catálogo).
	esperado := map[string]string{
		"user_auth_providers": "SELECT,INSERT,UPDATE",
		"oauth_login_codes":   "SELECT,INSERT,UPDATE",
		"auth_eventos":        "INSERT",
		"usuarios":            "SELECT,INSERT,UPDATE", // sin DELETE (02_propiedad_y_permisos.sql)
	}
	for tabla, quiere := range esperado {
		var tiene []string
		for _, p := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER"} {
			var ok bool
			if err := r.admin.QueryRow(r.ctx, `SELECT has_table_privilege('auth_app', $1, $2)`, "public."+tabla, p).Scan(&ok); err != nil {
				t.Fatal(err)
			}
			if ok {
				tiene = append(tiene, p)
			}
		}
		if strings.Join(tiene, ",") != quiere {
			t.Errorf("auth_app en %s: %v, esperado %s", tabla, tiene, quiere)
		}
	}
	var dueno string
	_ = r.admin.QueryRow(r.ctx, `SELECT tableowner FROM pg_tables WHERE tablename='user_auth_providers'`).Scan(&dueno)
	if dueno != "auth_owner" {
		t.Errorf("dueño de user_auth_providers: %s", dueno)
	}

	// Operaciones reales como auth_app.
	u := r.usuario("SOLICITANTE")
	if _, err := r.app.Exec(r.ctx, `INSERT INTO user_auth_providers (usuario_id, provider, provider_user_id, provider_email) VALUES ($1,'google',$2,'a@prueba.example')`, u, "sub-ok-"+u); err != nil {
		t.Fatalf("INSERT user_auth_providers: %v", err)
	}
	if _, err := r.app.Exec(r.ctx, `UPDATE user_auth_providers SET last_used_at = now() WHERE usuario_id=$1`, u); err != nil {
		t.Fatalf("UPDATE user_auth_providers: %v", err)
	}
	var n int
	if err := r.app.QueryRow(r.ctx, `SELECT count(*) FROM user_auth_providers WHERE usuario_id=$1`, u).Scan(&n); err != nil || n != 1 {
		t.Fatalf("SELECT user_auth_providers: %v %d", err, n)
	}
	hash := make([]byte, 32)
	_, _ = rand.Read(hash)
	if _, err := r.app.Exec(r.ctx, `INSERT INTO oauth_login_codes (code_hash, purpose, usuario_id, expires_at) VALUES ($1,'LOGIN',$2, now()+interval '1 minute')`, hash, u); err != nil {
		t.Fatalf("INSERT oauth_login_codes: %v", err)
	}
	tx, err := r.app.Begin(r.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(r.ctx, `SELECT 1 FROM oauth_login_codes WHERE code_hash=$1 FOR UPDATE`, hash); err != nil {
		t.Fatalf("SELECT ... FOR UPDATE oauth_login_codes: %v", err)
	}
	if _, err := tx.Exec(r.ctx, `UPDATE oauth_login_codes SET used_at = now() WHERE code_hash=$1`, hash); err != nil {
		t.Fatalf("UPDATE oauth_login_codes: %v", err)
	}
	_ = tx.Commit(r.ctx)
	if _, err := r.app.Exec(r.ctx, `INSERT INTO auth_eventos (usuario_id, evento, resultado) VALUES ($1,'prueba_permisos','OK')`, u); err != nil {
		t.Fatalf("INSERT auth_eventos: %v", err)
	}

	// Lo que auth_app NO puede hacer.
	prohibidas := map[string]string{
		"DELETE user_auth_providers": `DELETE FROM user_auth_providers WHERE usuario_id = '` + u + `'`,
		"DELETE oauth_login_codes":   `DELETE FROM oauth_login_codes WHERE usuario_id = '` + u + `'`,
		"DELETE auth_eventos":        `DELETE FROM auth_eventos WHERE usuario_id = '` + u + `'`,
		"SELECT auth_eventos":        `SELECT count(*) FROM auth_eventos`,
		"UPDATE auth_eventos":        `UPDATE auth_eventos SET motivo = 'x' WHERE usuario_id = '` + u + `'`,
		"DELETE usuarios":            `DELETE FROM usuarios WHERE id = '` + u + `'`,
		"TRUNCATE oauth_login_codes": `TRUNCATE oauth_login_codes`,
	}
	for nombre, sql := range prohibidas {
		if _, err := r.app.Exec(r.ctx, sql); !denegado(err) {
			t.Errorf("%s debía ser rechazado por privilegios: %v", nombre, err)
		}
	}
	if r.n(`SELECT count(*) FROM usuarios WHERE id=$1`, u) != 1 || r.n(`SELECT count(*) FROM user_auth_providers WHERE usuario_id=$1`, u) != 1 {
		t.Fatal("nada debía eliminarse")
	}
}

func TestFuncionCompensacion_PropiedadesDeSeguridad(t *testing.T) {
	r := conRolesReales(t)
	var dueno string
	var secdef bool
	var config []string
	err := r.admin.QueryRow(r.ctx, `
		SELECT pg_get_userbyid(p.proowner), p.prosecdef, coalesce(p.proconfig, '{}')
		FROM pg_proc p WHERE p.oid = $1::regprocedure
	`, funcionCompensacion).Scan(&dueno, &secdef, &config)
	if err != nil {
		t.Fatal(err)
	}
	if dueno != "auth_owner" || !secdef || strings.Join(config, ";") != "search_path=pg_catalog, pg_temp" {
		t.Fatalf("función: dueño=%s secdef=%v config=%v", dueno, secdef, config)
	}
	// EXECUTE: solo auth_app (y su dueño); PUBLIC no.
	var appOK, testerOK, publico bool
	_ = r.admin.QueryRow(r.ctx, `SELECT has_function_privilege('auth_app', $1::regprocedure, 'EXECUTE'),
		has_function_privilege('auth_tester', $1::regprocedure, 'EXECUTE'),
		EXISTS (SELECT 1 FROM pg_proc p, aclexplode(p.proacl) a WHERE p.oid = $1::regprocedure AND a.grantee = 0)`, funcionCompensacion).Scan(&appOK, &testerOK, &publico)
	if !appOK || testerOK || publico {
		t.Fatalf("EXECUTE: auth_app=%v auth_tester=%v PUBLIC=%v", appOK, testerOK, publico)
	}
	// auth_app no puede alterarla, reemplazarla, borrarla ni ceder su ejecución.
	for nombre, sql := range map[string]string{
		"CREATE OR REPLACE": `CREATE OR REPLACE FUNCTION public.cancelar_registro_google_pendiente(p_usuario_id uuid) RETURNS text LANGUAGE sql AS $$ SELECT 'X' $$`,
		"ALTER":             `ALTER FUNCTION ` + funcionCompensacion + ` SET search_path = public`,
		"DROP":              `DROP FUNCTION ` + funcionCompensacion,
		"CREATE en public":  `CREATE TABLE public.usuarios_sombra (id uuid)`,
		"CREATE TEMP":       `CREATE TEMP TABLE usuarios (id uuid)`,
		"CREATE SCHEMA":     `CREATE SCHEMA sombra`,
	} {
		if _, err := r.app.Exec(r.ctx, sql); err == nil {
			t.Errorf("auth_app pudo ejecutar %s", nombre)
		}
	}
	// GRANT sin la opción de concesión no falla en PostgreSQL (solo emite
	// WARNING "no privileges were granted"): se verifica el efecto.
	_, _ = r.app.Exec(r.ctx, `GRANT EXECUTE ON FUNCTION `+funcionCompensacion+` TO auth_tester`)
	_ = r.admin.QueryRow(r.ctx, `SELECT has_function_privilege('auth_tester', $1::regprocedure, 'EXECUTE')`, funcionCompensacion).Scan(&testerOK)
	if testerOK {
		t.Fatal("auth_app pudo ceder EXECUTE de la función")
	}
}

func TestFuncionCompensacion_SoloBorraRegistroGooglePendiente(t *testing.T) {
	r := conRolesReales(t)
	llamar := func(id any) string {
		t.Helper()
		var res string
		if err := r.app.QueryRow(r.ctx, `SELECT public.cancelar_registro_google_pendiente($1)`, id).Scan(&res); err != nil {
			t.Fatalf("invocar como auth_app: %v", err)
		}
		return res
	}
	pendiente := r.usuario("SOLICITANTE")
	r.identidad(pendiente, false)
	hash := make([]byte, 32)
	_, _ = rand.Read(hash)
	if _, err := r.admin.Exec(r.ctx, `INSERT INTO oauth_login_codes (code_hash, purpose, usuario_id, provider_user_id, provider_email, expires_at)
		VALUES ($1,'REGISTRO',$2,'s','g@prueba.example', now()+interval '5 minutes')`, hash, pendiente); err != nil {
		t.Fatal(err)
	}
	completo := r.usuario("SOLICITANTE")
	r.identidad(completo, true)
	institucional := r.usuario("SOLICITANTE")
	conSesion := r.usuario("SOLICITANTE")
	r.identidad(conSesion, false)
	token := make([]byte, 32)
	_, _ = rand.Read(token)
	if _, err := r.admin.Exec(r.ctx, `INSERT INTO refresh_tokens (usuario_id, token_hash, expires_at) VALUES ($1,$2, now()+interval '1 hour')`, conSesion, token); err != nil {
		t.Fatal(err)
	}
	adminPendiente := r.usuario("ADMIN")
	r.identidad(adminPendiente, false)

	// Conteos de las cuentas sembradas aquí (otras pruebas pueden escribir en
	// la misma base en paralelo) y de los catálogos globales.
	sembradas := []string{pendiente, completo, institucional, conSesion, adminPendiente}
	contar := func() [5]int {
		return [5]int{
			r.n(`SELECT count(*) FROM usuarios WHERE id = ANY($1::uuid[])`, sembradas),
			r.n(`SELECT count(*) FROM usuario_roles WHERE usuario_id = ANY($1::uuid[])`, sembradas),
			r.n(`SELECT count(*) FROM refresh_tokens WHERE usuario_id = ANY($1::uuid[])`, sembradas),
			r.n(`SELECT count(*) FROM roles`), r.n(`SELECT count(*) FROM permissions`),
		}
	}
	antes := contar()

	// Rechazados: nada cambia en ninguna tabla.
	casos := []struct {
		nombre string
		id     any
		quiere string
	}{
		{"registro Google completo", completo, "NO_PENDIENTE"},
		{"cuenta institucional sin Google", institucional, "NO_PENDIENTE"},
		{"registro pendiente con refresh token", conSesion, "CON_SESION"},
		{"pendiente con rol ADMIN", adminPendiente, "NO_PENDIENTE"},
		{"UUID inexistente", "00000000-0000-4000-8000-00000000dead", "NO_EXISTE"},
		{"UUID nulo", nil, "NO_EXISTE"},
	}
	for _, c := range casos {
		if res := llamar(c.id); res != c.quiere {
			t.Errorf("%s: %s, esperado %s", c.nombre, res, c.quiere)
		}
	}
	if contar() != antes {
		t.Fatalf("un rechazo no debe modificar nada: %v -> %v", antes, contar())
	}
	for _, id := range []string{completo, institucional, conSesion, adminPendiente} {
		if r.n(`SELECT count(*) FROM usuarios WHERE id=$1`, id) != 1 {
			t.Fatalf("la cuenta %s no debía eliminarse", id)
		}
	}

	// Permitido: solo la cuenta pendiente, con sus filas dependientes.
	if res := llamar(pendiente); res != "ELIMINADA" {
		t.Fatalf("registro pendiente: %s", res)
	}
	despues := contar()
	if despues[0] != antes[0]-1 || despues[1] != antes[1]-1 || despues[2] != antes[2] || despues[3] != antes[3] || despues[4] != antes[4] {
		t.Fatalf("solo debía eliminarse una cuenta (y su rol): %v -> %v", antes, despues)
	}
	if r.n(`SELECT count(*) FROM usuarios WHERE id=$1`, pendiente) != 0 ||
		r.n(`SELECT count(*) FROM user_auth_providers WHERE usuario_id=$1`, pendiente) != 0 ||
		r.n(`SELECT count(*) FROM oauth_login_codes WHERE usuario_id=$1`, pendiente) != 0 {
		t.Fatal("la cuenta pendiente y sus filas dependientes debían eliminarse")
	}
	// Repetir la llamada es seguro.
	if res := llamar(pendiente); res != "NO_EXISTE" {
		t.Fatalf("segunda llamada: %s", res)
	}

	// La compensación del repositorio (como auth_app) usa la función.
	g := NewGoogleRepository(r.app)
	otro := r.usuario("SOLICITANTE")
	r.identidad(otro, false)
	hash2 := make([]byte, 32)
	_, _ = rand.Read(hash2)
	if _, err := r.admin.Exec(r.ctx, `INSERT INTO oauth_login_codes (code_hash, purpose, usuario_id, provider_user_id, provider_email, expires_at)
		VALUES ($1,'REGISTRO',$2,'s2','g2@prueba.example', now()+interval '5 minutes')`, hash2, otro); err != nil {
		t.Fatal(err)
	}
	if id, err := g.CancelRegistration(r.ctx, hash2); err != nil || id != otro {
		t.Fatalf("CancelRegistration como auth_app: %v", err)
	}
	// Con un código de una cuenta completa, el repositorio no borra.
	hash3 := make([]byte, 32)
	_, _ = rand.Read(hash3)
	if _, err := r.admin.Exec(r.ctx, `INSERT INTO oauth_login_codes (code_hash, purpose, usuario_id, provider_user_id, provider_email, expires_at)
		VALUES ($1,'REGISTRO',$2,'s3','g3@prueba.example', now()+interval '5 minutes')`, hash3, completo); err != nil {
		t.Fatal(err)
	}
	if _, err := g.CancelRegistration(r.ctx, hash3); !errors.Is(err, ErrRegistroNoCancelable) || r.n(`SELECT count(*) FROM usuarios WHERE id=$1`, completo) != 1 {
		t.Fatalf("CancelRegistration de una cuenta completa: %v", err)
	}
}
