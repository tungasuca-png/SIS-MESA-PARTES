package logic

import (
	"testing"

	"auth/internal/config"

	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Alcance del registro manual: SOLO crea cuentas SOLICITANTE, sin permisos
// adicionales, y no altera cuentas, roles, permisos ni el login de los
// usuarios institucionales.

// huellaAutorizacion: contenido completo de roles, permisos y sus
// asignaciones (cualquier cambio altera la huella).
func (e *entornoRegistro) huellaAutorizacion() string {
	e.t.Helper()
	var h string
	err := e.admin.QueryRow(e.ctx, `
		SELECT md5(
			coalesce((SELECT string_agg(id::text || nombre || estado::text, ',' ORDER BY id) FROM roles), '') || '|' ||
			coalesce((SELECT string_agg(id::text || codigo || estado::text, ',' ORDER BY id) FROM permissions), '') || '|' ||
			coalesce((SELECT string_agg(role_id::text || permission_id::text, ',' ORDER BY role_id, permission_id) FROM role_permissions), '')
		)`).Scan(&h)
	if err != nil {
		e.t.Fatal(err)
	}
	return h
}

func TestRegistroCorreo_SoloCreaSolicitanteSinPermisosAdicionales(t *testing.T) {
	e := nuevoEntornoRegistro(t, config.RegistroCorreoConfig{})
	antes := e.huellaAutorizacion()
	d := e.datos()
	r, err := e.registrar(d)
	if err != nil {
		t.Fatal(err)
	}
	_, codigo := e.correo.ultimo(t, d.email)
	v, err := e.verificar(r.RegistrationId, codigo)
	if err != nil {
		t.Fatal(err)
	}
	// El token interno para crear el perfil es de SOLICITANTE.
	claims, err := e.sc.JWTManager.ValidateAccessToken(v.ProfileToken)
	if err != nil || claims.Role != "SOLICITANTE" || claims.Subject != v.UserId {
		t.Fatalf("token de perfil: %+v %v", claims, err)
	}
	if _, err := e.completar(r.RegistrationId, codigo); err != nil {
		t.Fatal(err)
	}
	// Exactamente un rol, SOLICITANTE.
	if e.contar(`SELECT count(*) FROM usuario_roles WHERE usuario_id = $1`, v.UserId) != 1 ||
		e.contar(`SELECT count(*) FROM usuario_roles ur JOIN roles r ON r.id = ur.rol_id WHERE ur.usuario_id = $1 AND r.nombre = 'SOLICITANTE'`, v.UserId) != 1 {
		t.Fatal("la cuenta debe tener solo el rol SOLICITANTE")
	}
	// Permisos efectivos: exactamente los del rol SOLICITANTE (ni uno más).
	if e.contar(`
		SELECT count(*) FROM (
			(SELECT rp.permission_id FROM usuario_roles ur JOIN role_permissions rp ON rp.role_id = ur.rol_id WHERE ur.usuario_id = $1
			 EXCEPT
			 SELECT rp.permission_id FROM role_permissions rp JOIN roles r ON r.id = rp.role_id WHERE r.nombre = 'SOLICITANTE')
		) extra`, v.UserId) != 0 {
		t.Fatal("la cuenta no debe tener permisos ajenos al rol SOLICITANTE")
	}
	// La sesión emitida por el login también es SOLICITANTE.
	ses, err := e.loginCorreo(d.email, d.clave)
	if err != nil || ses.Role != "SOLICITANTE" {
		t.Fatalf("login: %v", err)
	}
	// Ni el registro ni la verificación tocaron roles, permisos ni asignaciones.
	if e.huellaAutorizacion() != antes {
		t.Fatal("roles, permisos o asignaciones cambiaron")
	}
	// Repetir la verificación no cambia el rol ni concede nada.
	_, _ = e.verificar(r.RegistrationId, codigo)
	if e.contar(`SELECT count(*) FROM usuario_roles WHERE usuario_id = $1`, v.UserId) != 1 || e.huellaAutorizacion() != antes {
		t.Fatal("una verificación repetida no debe cambiar roles ni permisos")
	}
}

func TestRegistroCorreo_NoAlteraCuentasInstitucionales(t *testing.T) {
	e := nuevoEntornoRegistro(t, config.RegistroCorreoConfig{})
	antes := e.huellaAutorizacion()
	dniAdmin, correoAdmin := e.dniNuevo(), e.unico("adm")+"@colegio.example"
	admin := e.crearUsuario(dniAdmin, correoAdmin, "ClaveAdmin123", "ADMIN", true)
	secretaria := e.crearUsuario(e.dniNuevo(), e.unico("sec")+"@colegio.example", "ClaveSecre123", "SECRETARIA", true)
	var hashAntes string
	var verificadoAntes bool
	_ = e.admin.QueryRow(e.ctx, `SELECT password_hash, email_verified FROM usuarios WHERE id = $1`, admin).Scan(&hashAntes, &verificadoAntes)

	// 1. Registro con el correo del ADMIN: respuesta normal, aviso sin código.
	r1, err := e.registrar(datosRegistro{email: correoAdmin, dni: e.dniNuevo(), clave: "OtraClave999"})
	if err != nil {
		t.Fatal(err)
	}
	if _, c := e.correo.ultimo(t, correoAdmin); c != "" {
		t.Fatal("al correo institucional no se le envía un código")
	}
	for _, c := range []string{"000000", "111111"} {
		if _, err := e.verificar(r1.RegistrationId, c); status.Code(err) == codes.OK {
			t.Fatal("ningún código activa un registro con el correo del ADMIN")
		}
	}

	// 2. Registro con el DNI del ADMIN y otro correo: el correo se verifica,
	// pero la cuenta nueva NO obtiene el DNI (solo queda declarado).
	d2 := datosRegistro{email: e.unico("x") + "@correo.example", dni: dniAdmin, clave: "OtraClave999"}
	r2, err := e.registrar(d2)
	if err != nil {
		t.Fatal(err)
	}
	_, c2 := e.correo.ultimo(t, d2.email)
	v2, err := e.verificar(r2.RegistrationId, c2)
	if err != nil {
		t.Fatalf("verificar el correo propio: %v", err)
	}
	if e.contar(`SELECT count(*) FROM usuarios WHERE id = $1 AND dni IS NULL`, v2.UserId) != 1 ||
		e.contar(`SELECT count(*) FROM usuarios WHERE dni = $1`, dniAdmin) != 1 {
		t.Fatal("el DNI del ADMIN no se reserva ni se toma")
	}
	if err := e.verificarIdentidad(secretaria, v2.UserId, dniAdmin); status.Code(err) != codes.AlreadyExists {
		t.Fatalf("verificar con el DNI del ADMIN: %v", err)
	}

	// La cuenta ADMIN no cambió: clave, verificación, rol.
	var hashDespues string
	var verificadoDespues bool
	_ = e.admin.QueryRow(e.ctx, `SELECT password_hash, email_verified FROM usuarios WHERE id = $1`, admin).Scan(&hashDespues, &verificadoDespues)
	if hashDespues != hashAntes || verificadoDespues != verificadoAntes ||
		e.contar(`SELECT count(*) FROM usuario_roles ur JOIN roles r ON r.id = ur.rol_id WHERE ur.usuario_id = $1 AND r.nombre = 'ADMIN'`, admin) != 1 ||
		e.contar(`SELECT count(*) FROM usuario_roles WHERE usuario_id = $1`, admin) != 1 {
		t.Fatal("la cuenta institucional no debe cambiar")
	}
	if bcrypt.CompareHashAndPassword([]byte(hashDespues), []byte("OtraClave999")) == nil {
		t.Fatal("la clave del registro nunca reemplaza la institucional")
	}

	// Login institucional intacto: ADMIN y SECRETARIA entran con su rol; la
	// clave del registro no sirve para el DNI del ADMIN (credenciales
	// inválidas, sin mencionar el registro).
	if ses, err := e.login(dniAdmin, "ClaveAdmin123"); err != nil || ses.Role != "ADMIN" || ses.UserId != admin {
		t.Fatalf("login ADMIN: %v", err)
	}
	if _, err := e.login(dniAdmin, "OtraClave999"); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("clave del registro con el DNI del ADMIN: %v", err)
	}
	var dniSecretaria string
	_ = e.admin.QueryRow(e.ctx, `SELECT dni FROM usuarios WHERE id = $1`, secretaria).Scan(&dniSecretaria)
	if ses, err := e.login(dniSecretaria, "ClaveSecre123"); err != nil || ses.Role != "SECRETARIA" {
		t.Fatalf("login SECRETARIA: %v", err)
	}
	if e.huellaAutorizacion() != antes {
		t.Fatal("roles, permisos o asignaciones cambiaron")
	}
}
