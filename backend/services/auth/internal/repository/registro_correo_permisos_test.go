package repository

import (
	"crypto/rand"
	"strings"
	"testing"
)

// Privilegios REALES de auth_app sobre el registro manual (migración 024).
// Mismo entorno que google_permisos_test.go (roles reales, base desechable).

const funcionCompensacionCorreo = "public.cancelar_registro_correo_pendiente(uuid)"

// registroVerificado siembra (como auth_tester) un registro verificado con su
// cuenta SOLICITANTE, como lo deja EmailVerify.
func (r *rolesReales) registroVerificado(rol string) (registroID, usuarioID string) {
	r.t.Helper()
	usuarioID = r.usuario(rol)
	hash := make([]byte, 32)
	_, _ = rand.Read(hash)
	var email, dni string
	if err := r.admin.QueryRow(r.ctx, `SELECT email::text, dni FROM usuarios WHERE id = $1`, usuarioID).Scan(&email, &dni); err != nil {
		r.t.Fatal(err)
	}
	err := r.admin.QueryRow(r.ctx, `
		INSERT INTO registros_correo (email, dni, password_hash, code_hash, expires_at, terminos_version,
		                              terminos_aceptados_at, usuario_id, verificado_at)
		VALUES ($1, $2, '$2a$10$x', $3, now() + interval '10 minutes', 'v1', now(), $4, now())
		RETURNING id
	`, email, dni, hash, usuarioID).Scan(&registroID)
	if err != nil {
		r.t.Fatal(err)
	}
	return registroID, usuarioID
}

func TestPermisosAuthApp_RegistrosCorreo(t *testing.T) {
	r := conRolesReales(t)
	var tiene []string
	for _, p := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER"} {
		var ok bool
		if err := r.admin.QueryRow(r.ctx, `SELECT has_table_privilege('auth_app', 'public.registros_correo', $1)`, p).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		if ok {
			tiene = append(tiene, p)
		}
	}
	if strings.Join(tiene, ",") != "SELECT,INSERT,UPDATE" {
		t.Fatalf("auth_app en registros_correo: %v", tiene)
	}
	reg, _ := r.registroVerificado("SOLICITANTE")
	if _, err := r.app.Exec(r.ctx, `UPDATE registros_correo SET intentos = intentos + 1 WHERE id = $1`, reg); err != nil {
		t.Fatalf("UPDATE como auth_app: %v", err)
	}
	if _, err := r.app.Exec(r.ctx, `DELETE FROM registros_correo WHERE id = $1`, reg); !denegado(err) {
		t.Fatalf("DELETE debía rechazarse: %v", err)
	}

	var dueno string
	var secdef bool
	var config []string
	if err := r.admin.QueryRow(r.ctx, `
		SELECT pg_get_userbyid(p.proowner), p.prosecdef, coalesce(p.proconfig, '{}')
		FROM pg_proc p WHERE p.oid = $1::regprocedure
	`, funcionCompensacionCorreo).Scan(&dueno, &secdef, &config); err != nil {
		t.Fatal(err)
	}
	if dueno != "auth_owner" || !secdef || strings.Join(config, ";") != "search_path=pg_catalog, pg_temp" {
		t.Fatalf("función: dueño=%s secdef=%v config=%v", dueno, secdef, config)
	}
	var appOK, testerOK, publico bool
	_ = r.admin.QueryRow(r.ctx, `SELECT has_function_privilege('auth_app', $1::regprocedure, 'EXECUTE'),
		has_function_privilege('auth_tester', $1::regprocedure, 'EXECUTE'),
		EXISTS (SELECT 1 FROM pg_proc p, aclexplode(p.proacl) a WHERE p.oid = $1::regprocedure AND a.grantee = 0)`,
		funcionCompensacionCorreo).Scan(&appOK, &testerOK, &publico)
	if !appOK || testerOK || publico {
		t.Fatalf("EXECUTE: auth_app=%v auth_tester=%v PUBLIC=%v", appOK, testerOK, publico)
	}
	for nombre, sql := range map[string]string{
		"CREATE OR REPLACE": `CREATE OR REPLACE FUNCTION public.cancelar_registro_correo_pendiente(p_registro_id uuid) RETURNS text LANGUAGE sql AS $$ SELECT 'X' $$`,
		"ALTER":             `ALTER FUNCTION ` + funcionCompensacionCorreo + ` SET search_path = public`,
		"DROP":              `DROP FUNCTION ` + funcionCompensacionCorreo,
	} {
		if _, err := r.app.Exec(r.ctx, sql); err == nil {
			t.Errorf("auth_app pudo ejecutar %s", nombre)
		}
	}
}

func TestFuncionCompensacionCorreo_SoloBorraRegistroVerificadoPendiente(t *testing.T) {
	r := conRolesReales(t)
	llamar := func(id any) string {
		t.Helper()
		var res string
		if err := r.app.QueryRow(r.ctx, `SELECT public.cancelar_registro_correo_pendiente($1)`, id).Scan(&res); err != nil {
			t.Fatalf("invocar como auth_app: %v", err)
		}
		return res
	}
	pendiente, uPendiente := r.registroVerificado("SOLICITANTE")
	completo, uCompleto := r.registroVerificado("SOLICITANTE")
	if _, err := r.admin.Exec(r.ctx, `UPDATE registros_correo SET completado_at = now() WHERE id = $1`, completo); err != nil {
		t.Fatal(err)
	}
	conSesion, uSesion := r.registroVerificado("SOLICITANTE")
	token := make([]byte, 32)
	_, _ = rand.Read(token)
	if _, err := r.admin.Exec(r.ctx, `INSERT INTO refresh_tokens (usuario_id, token_hash, expires_at) VALUES ($1, $2, now() + interval '1 hour')`, uSesion, token); err != nil {
		t.Fatal(err)
	}
	admin, uAdmin := r.registroVerificado("ADMIN")
	conGoogle, uGoogle := r.registroVerificado("SOLICITANTE")
	r.identidad(uGoogle, true)

	casos := []struct {
		nombre, id, usuario, quiere string
	}{
		{"registro completado", completo, uCompleto, "NO_PENDIENTE"},
		{"cuenta con refresh token", conSesion, uSesion, "CON_SESION"},
		{"rol ADMIN", admin, uAdmin, "NO_PENDIENTE"},
		{"cuenta con identidad Google", conGoogle, uGoogle, "NO_PENDIENTE"},
	}
	for _, c := range casos {
		if res := llamar(c.id); res != c.quiere {
			t.Errorf("%s: %s, esperado %s", c.nombre, res, c.quiere)
		}
		if r.n(`SELECT count(*) FROM usuarios WHERE id = $1`, c.usuario) != 1 {
			t.Fatalf("%s: la cuenta no debía eliminarse", c.nombre)
		}
	}
	if res := llamar("00000000-0000-4000-8000-00000000beef"); res != "NO_EXISTE" {
		t.Errorf("registro inexistente: %s", res)
	}
	if res := llamar(nil); res != "NO_EXISTE" {
		t.Errorf("nulo: %s", res)
	}
	if res := llamar(pendiente); res != "ELIMINADA" {
		t.Fatalf("registro verificado pendiente: %s", res)
	}
	if r.n(`SELECT count(*) FROM usuarios WHERE id = $1`, uPendiente) != 0 ||
		r.n(`SELECT count(*) FROM registros_correo WHERE id = $1 AND anulado_at IS NOT NULL AND usuario_id IS NULL`, pendiente) != 1 {
		t.Fatal("la cuenta se elimina y el registro queda anulado como rastro")
	}
	if res := llamar(pendiente); res != "NO_PENDIENTE" {
		t.Fatalf("segunda llamada: %s", res)
	}
}

// auth_tester (pruebas de integración) siembra y lee las tablas del registro
// público, pero no puede borrarlas (migración 026). Al eliminar uno de sus
// usuarios de prueba, las claves foráneas actúan como el dueño de la tabla:
// la identidad se borra en cascada y el registro queda con usuario_id NULL.
func TestPermisosAuthTester_RegistroPublico(t *testing.T) {
	r := conRolesReales(t)
	for _, tabla := range []string{"registros_correo", "identidad_solicitantes"} {
		var tiene []string
		for _, p := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER"} {
			var ok bool
			if err := r.admin.QueryRow(r.ctx, `SELECT has_table_privilege('auth_tester', 'public.' || $1, $2)`, tabla, p).Scan(&ok); err != nil {
				t.Fatal(err)
			}
			if ok {
				tiene = append(tiene, p)
			}
		}
		if strings.Join(tiene, ",") != "SELECT,INSERT,UPDATE" {
			t.Fatalf("auth_tester en %s: %v", tabla, tiene)
		}
	}

	reg, usuarioID := r.registroVerificado("SOLICITANTE")
	if _, err := r.admin.Exec(r.ctx, `INSERT INTO identidad_solicitantes (usuario_id, dni_declarado, origen) VALUES ($1, '70000099', 'correo')`, usuarioID); err != nil {
		t.Fatalf("auth_tester siembra una identidad: %v", err)
	}
	if _, err := r.admin.Exec(r.ctx, `DELETE FROM registros_correo WHERE id = $1`, reg); !denegado(err) {
		t.Fatalf("auth_tester no debe borrar registros_correo: %v", err)
	}
	if _, err := r.admin.Exec(r.ctx, `DELETE FROM identidad_solicitantes WHERE usuario_id = $1`, usuarioID); !denegado(err) {
		t.Fatalf("auth_tester no debe borrar identidad_solicitantes: %v", err)
	}
	// Limpieza de un usuario de prueba: sigue funcionando.
	if _, err := r.admin.Exec(r.ctx, `DELETE FROM usuarios WHERE id = $1`, usuarioID); err != nil {
		t.Fatalf("auth_tester elimina su usuario de prueba: %v", err)
	}
	if r.n(`SELECT count(*) FROM identidad_solicitantes WHERE usuario_id = $1`, usuarioID) != 0 ||
		r.n(`SELECT count(*) FROM registros_correo WHERE id = $1 AND usuario_id IS NULL`, reg) != 1 {
		t.Fatal("las claves foráneas debían borrar la identidad y dejar el registro sin usuario")
	}
}
