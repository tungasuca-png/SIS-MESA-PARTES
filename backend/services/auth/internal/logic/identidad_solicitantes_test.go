package logic

import (
	"context"
	"sync"
	"testing"

	"auth/auth"
	"auth/internal/authorization"
	"auth/internal/config"
	"auth/internal/interceptor"
	"auth/internal/repository"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Verificación de identidad de solicitantes (PostgreSQL desechable, ver
// google_flujos_test.go). Las llamadas pasan por los interceptores REALES de
// autenticación y autorización: el permiso se lee de role_permissions.

const permisoIdentidad = "solicitantes.identidad.verify"

// conPermisos ejecuta la lógica a través de autenticación + autorización.
func (e *entornoRegistro) conPermisos(usuarioID, rol, metodo string, f func(ctx context.Context) (any, error)) (any, error) {
	token, _, _ := e.sc.JWTManager.GenerateAccessToken(usuarioID, "u", rol)
	ctx := metadata.NewIncomingContext(e.ctx, metadata.Pairs("authorization", "Bearer "+token))
	politicas := interceptor.DefaultMethodPolicies()
	autz := authorization.NewAuthorizationService(e.sc.UserRepository, repository.NewRoleRepository(e.sc.DB), repository.NewRolePermissionRepository(e.sc.DB))
	autenticar := interceptor.AuthenticationInterceptor(e.sc.JWTManager, politicas)
	autorizar := interceptor.AuthorizationInterceptor(autz, politicas)
	info := &grpc.UnaryServerInfo{FullMethod: metodo}
	return autenticar(ctx, nil, info, func(ctx context.Context, req any) (any, error) {
		return autorizar(ctx, req, info, func(ctx context.Context, _ any) (any, error) { return f(ctx) })
	})
}

// concederASecretaria asigna el permiso a SECRETARIA solo durante la prueba.
func (e *entornoRegistro) concederASecretaria() {
	e.t.Helper()
	if _, err := e.admin.Exec(e.ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		SELECT r.id, p.id FROM roles r, permissions p WHERE r.nombre = 'SECRETARIA' AND p.codigo = $1
		ON CONFLICT DO NOTHING`, permisoIdentidad); err != nil {
		e.t.Fatal(err)
	}
	e.t.Cleanup(func() {
		_, _ = e.admin.Exec(context.Background(), `
			DELETE FROM role_permissions WHERE permission_id = (SELECT id FROM permissions WHERE codigo = $1)`, permisoIdentidad)
	})
}

// solicitantePendiente: registro manual completo (correo verificado, DNI
// declarado y pendiente).
func (e *entornoRegistro) solicitantePendiente(dni string) (usuarioID string, d datosRegistro) {
	e.t.Helper()
	d = e.datos()
	if dni != "" {
		d.dni = dni
	}
	r, err := e.registrar(d)
	if err != nil {
		e.t.Fatal(err)
	}
	_, codigo := e.correo.ultimo(e.t, d.email)
	v, err := e.verificar(r.RegistrationId, codigo)
	if err != nil {
		e.t.Fatal(err)
	}
	if _, err := e.completar(r.RegistrationId, codigo); err != nil {
		e.t.Fatal(err)
	}
	return v.UserId, d
}

func verificarCon(ctx context.Context, e *entornoRegistro, usuarioID, dni string) (any, error) {
	return NewVerifySolicitanteIdentityLogic(ctx, e.sc).VerifySolicitanteIdentity(&auth.VerifySolicitanteIdentityRequest{
		UserId: usuarioID, DniEsperado: dni, Metodo: "DOCUMENTO",
	})
}

func TestIdentidad_PermisoObligatorioEnBackend(t *testing.T) {
	e := nuevoEntornoRegistro(t, config.RegistroCorreoConfig{})
	if e.contar(`SELECT count(*) FROM permissions WHERE codigo = $1 AND estado`, permisoIdentidad) != 1 ||
		e.contar(`SELECT count(*) FROM role_permissions rp JOIN permissions p ON p.id = rp.permission_id WHERE p.codigo = $1`, permisoIdentidad) != 0 {
		t.Fatal("el permiso existe y no está asignado a ningún rol")
	}
	sol, d := e.solicitantePendiente("")
	secretaria := e.crearUsuario(e.dniNuevo(), e.unico("sec")+"@colegio.example", "ClaveSecre123", "SECRETARIA", true)

	// Sin el permiso: ni consultar, ni verificar, ni rechazar.
	llamadas := map[string]func(ctx context.Context) (any, error){
		"/auth.Auth/ListSolicitanteIdentities": func(ctx context.Context) (any, error) {
			return NewListSolicitanteIdentitiesLogic(ctx, e.sc).ListSolicitanteIdentities(&auth.ListSolicitanteIdentitiesRequest{})
		},
		"/auth.Auth/GetSolicitanteIdentity": func(ctx context.Context) (any, error) {
			return NewGetSolicitanteIdentityLogic(ctx, e.sc).GetSolicitanteIdentity(&auth.GetSolicitanteIdentityRequest{UserId: sol})
		},
		"/auth.Auth/VerifySolicitanteIdentity": func(ctx context.Context) (any, error) { return verificarCon(ctx, e, sol, d.dni) },
		"/auth.Auth/RejectSolicitanteIdentity": func(ctx context.Context) (any, error) {
			return NewRejectSolicitanteIdentityLogic(ctx, e.sc).RejectSolicitanteIdentity(&auth.RejectSolicitanteIdentityRequest{UserId: sol, Motivo: "No coincide"})
		},
	}
	for metodo, f := range llamadas {
		if _, err := e.conPermisos(secretaria, "SECRETARIA", metodo, f); status.Code(err) != codes.PermissionDenied {
			t.Fatalf("%s sin permiso: %v", metodo, err)
		}
		// Un solicitante tampoco (ni sobre sí mismo).
		if _, err := e.conPermisos(sol, "SOLICITANTE", metodo, f); status.Code(err) != codes.PermissionDenied {
			t.Fatalf("%s como solicitante: %v", metodo, err)
		}
	}
	if e.contar(`SELECT count(*) FROM usuarios WHERE id = $1 AND dni IS NULL`, sol) != 1 {
		t.Fatal("sin permiso no cambia nada")
	}

	// Con el permiso asignado al rol: consulta y verifica.
	e.concederASecretaria()
	lista, err := e.conPermisos(secretaria, "SECRETARIA", "/auth.Auth/ListSolicitanteIdentities", llamadas["/auth.Auth/ListSolicitanteIdentities"])
	if err != nil || lista.(*auth.ListSolicitanteIdentitiesResponse).Total < 1 {
		t.Fatalf("listar con permiso: %v", err)
	}
	if _, err := e.conPermisos(secretaria, "SECRETARIA", "/auth.Auth/VerifySolicitanteIdentity", llamadas["/auth.Auth/VerifySolicitanteIdentity"]); err != nil {
		t.Fatalf("verificar con permiso: %v", err)
	}
	if e.contar(`SELECT count(*) FROM usuarios WHERE id = $1 AND dni = $2`, sol, d.dni) != 1 ||
		e.contar(`SELECT count(*) FROM identidad_solicitantes WHERE usuario_id = $1 AND estado = 'VERIFICADA' AND metodo = 'DOCUMENTO' AND verificado_por = $2`, sol, secretaria) != 1 {
		t.Fatal("verificada: DNI definitivo y quién verificó")
	}
	if ses, err := e.login(d.dni, d.clave); err != nil || ses.UserId != sol || ses.Role != "SOLICITANTE" {
		t.Fatalf("login por DNI tras verificar: %v", err)
	}
	// Verificar otra vez: idempotente. Rechazar una verificada: no.
	if _, err := e.conPermisos(secretaria, "SECRETARIA", "/auth.Auth/VerifySolicitanteIdentity", llamadas["/auth.Auth/VerifySolicitanteIdentity"]); err != nil {
		t.Fatalf("verificación repetida: %v", err)
	}
	if _, err := e.conPermisos(secretaria, "SECRETARIA", "/auth.Auth/RejectSolicitanteIdentity", llamadas["/auth.Auth/RejectSolicitanteIdentity"]); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("rechazar una identidad verificada: %v", err)
	}
	// Auditoría sin DNI.
	if e.contar(`SELECT count(*) FROM auth_eventos WHERE usuario_id = $1 AND proveedor = 'identidad' AND evento = 'identidad_verificacion' AND resultado = 'OK'`, sol) != 1 ||
		e.contar(`SELECT count(*) FROM auth_eventos WHERE motivo LIKE '%' || $1 || '%'`, d.dni) != 0 {
		t.Fatal("auditoría de la verificación, sin DNI")
	}
}

func TestIdentidad_RechazoYPropiaIdentidad(t *testing.T) {
	e := nuevoEntornoRegistro(t, config.RegistroCorreoConfig{})
	sol, d := e.solicitantePendiente("")
	personal := e.crearUsuario(e.dniNuevo(), e.unico("sec")+"@colegio.example", "ClaveSecre123", "SECRETARIA", true)
	rechazar := func(quien, motivo string) error {
		_, err := e.conSesion(quien, "SECRETARIA", "/auth.Auth/RejectSolicitanteIdentity", func(ctx context.Context) (any, error) {
			return NewRejectSolicitanteIdentityLogic(ctx, e.sc).RejectSolicitanteIdentity(&auth.RejectSolicitanteIdentityRequest{UserId: sol, Motivo: motivo})
		})
		return err
	}
	if err := rechazar(personal, "x"); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("motivo obligatorio: %v", err)
	}
	if err := rechazar(sol, "Quiero rechazarme"); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("propia identidad: %v", err)
	}
	if err := rechazar(personal, "Los datos no coinciden con el documento"); err != nil {
		t.Fatal(err)
	}
	if e.contar(`SELECT count(*) FROM identidad_solicitantes WHERE usuario_id = $1 AND estado = 'RECHAZADA' AND verificado_por = $2`, sol, personal) != 1 ||
		e.contar(`SELECT count(*) FROM usuarios WHERE id = $1 AND dni IS NULL`, sol) != 1 {
		t.Fatal("rechazada: sin DNI definitivo")
	}
	// Sigue entrando con su correo y puede presentar trámites.
	if _, err := e.loginCorreo(d.email, d.clave); err != nil {
		t.Fatalf("login por correo tras el rechazo: %v", err)
	}
	// Tras un cotejo posterior, se puede verificar.
	if err := e.verificarIdentidad(personal, sol, d.dni); err != nil {
		t.Fatalf("verificar tras un rechazo: %v", err)
	}
	// Un DNI esperado distinto del declarado no verifica nada.
	otro, d2 := e.solicitantePendiente("")
	if err := e.verificarIdentidad(personal, otro, otroCodigo(d2.dni[:6])+"00"); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("DNI esperado distinto: %v", err)
	}
}

func TestIdentidad_VerificacionesConcurrentes(t *testing.T) {
	e := nuevoEntornoRegistro(t, config.RegistroCorreoConfig{})
	personal := e.crearUsuario(e.dniNuevo(), e.unico("sec")+"@colegio.example", "ClaveSecre123", "SECRETARIA", true)
	dni := e.dniNuevo()
	// Tres personas declaran el mismo DNI (ninguna lo reserva).
	var ids []string
	for i := 0; i < 3; i++ {
		id, _ := e.solicitantePendiente(dni)
		ids = append(ids, id)
	}
	if e.contar(`SELECT count(*) FROM usuarios WHERE dni = $1`, dni) != 0 {
		t.Fatal("declarar no reserva")
	}
	// Se verifican las tres a la vez (y la primera dos veces): solo una gana.
	objetivos := append([]string{ids[0]}, ids...)
	errs := make([]error, len(objetivos))
	var wg sync.WaitGroup
	for i, id := range objetivos {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			errs[i] = e.verificarIdentidad(personal, id, dni)
		}(i, id)
	}
	wg.Wait()
	exitos := map[string]bool{}
	for i, err := range errs {
		switch {
		case err == nil:
			exitos[objetivos[i]] = true
		case status.Code(err) != codes.AlreadyExists:
			t.Fatalf("error inesperado: %v", err)
		}
	}
	if len(exitos) != 1 {
		t.Fatalf("exactamente una cuenta debe quedar verificada: %v", exitos)
	}
	if e.contar(`SELECT count(*) FROM usuarios WHERE dni = $1`, dni) != 1 ||
		e.contar(`SELECT count(*) FROM identidad_solicitantes WHERE dni_declarado = $1 AND estado = 'VERIFICADA'`, dni) != 1 ||
		e.contar(`SELECT count(*) FROM identidad_solicitantes i JOIN usuarios u ON u.id = i.usuario_id
			WHERE i.dni_declarado = $1 AND i.estado = 'PENDIENTE' AND u.dni IS NULL`, dni) != 2 {
		t.Fatal("estado consistente: una verificada con DNI, dos pendientes sin DNI")
	}
}

func TestLoginPorCorreo_SoloSolicitantes(t *testing.T) {
	e := nuevoEntornoRegistro(t, config.RegistroCorreoConfig{})
	correoAdmin := e.unico("adm") + "@colegio.example"
	dniAdmin := e.dniNuevo()
	admin := e.crearUsuario(dniAdmin, correoAdmin, "ClaveAdmin123", "ADMIN", true)
	// Correo + clave correcta de un ADMIN: credenciales inválidas, sin contar intentos.
	if _, err := e.loginCorreo(correoAdmin, "ClaveAdmin123"); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("ADMIN por correo: %v", err)
	}
	if e.contar(`SELECT failed_login_attempts FROM usuarios WHERE id = $1`, admin) != 0 {
		t.Fatal("el intento por correo no debe afectar a la cuenta institucional")
	}
	if ses, err := e.login(dniAdmin, "ClaveAdmin123"); err != nil || ses.Role != "ADMIN" {
		t.Fatalf("login institucional por DNI: %v", err)
	}
	// Formato inválido y correo inexistente.
	if _, err := e.loginCorreo("no-es-correo", "x"); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("formato: %v", err)
	}
	if _, err := e.loginCorreo(e.unico("nadie")+"@correo.example", "ClaveSegura123"); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("correo inexistente: %v", err)
	}
	// Bloqueo por intentos también en el login por correo.
	sol, d := e.solicitantePendiente("")
	for i := 0; i < 5; i++ {
		_, _ = e.loginCorreo(d.email, "clave-equivocada")
	}
	if _, err := e.loginCorreo(d.email, d.clave); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("bloqueo tras 5 intentos: %v", err)
	}
	_ = sol
}

// Sincronización con Usuarios y compensación: estado explícito en Auth.
func TestIdentidad_SincronizacionYCompensacion(t *testing.T) {
	e := nuevoEntornoRegistro(t, config.RegistroCorreoConfig{})
	personal := e.crearUsuario(e.dniNuevo(), e.unico("sec")+"@colegio.example", "ClaveSecre123", "SECRETARIA", true)
	llamar := func(metodo string, f func(ctx context.Context) (any, error)) error {
		_, err := e.conPermisos(personal, "SECRETARIA", metodo, f)
		return err
	}
	confirmar := func(id, dni string) error {
		return llamar("/auth.Auth/ConfirmSolicitanteIdentitySync", func(ctx context.Context) (any, error) {
			return NewConfirmSolicitanteIdentitySyncLogic(ctx, e.sc).ConfirmSolicitanteIdentitySync(&auth.IdentitySyncRequest{UserId: id, Dni: dni})
		})
	}
	revertir := func(id, dni string) error {
		return llamar("/auth.Auth/RevertSolicitanteIdentity", func(ctx context.Context) (any, error) {
			return NewRevertSolicitanteIdentityLogic(ctx, e.sc).RevertSolicitanteIdentity(&auth.IdentitySyncRequest{UserId: id, Dni: dni})
		})
	}
	fallo := func(id, codigo string) error {
		return llamar("/auth.Auth/RecordSolicitanteIdentitySyncFailure", func(ctx context.Context) (any, error) {
			return NewRecordSolicitanteIdentitySyncFailureLogic(ctx, e.sc).RecordSolicitanteIdentitySyncFailure(&auth.IdentitySyncFailureRequest{UserId: id, Codigo: codigo})
		})
	}
	verificar := func(id, dni string) error {
		return llamar("/auth.Auth/VerifySolicitanteIdentity", func(ctx context.Context) (any, error) { return verificarCon(ctx, e, id, dni) })
	}
	sol, d := e.solicitantePendiente("")

	// Sin el permiso: ninguna de las tres operaciones.
	for nombre, err := range map[string]error{"confirmar": confirmar(sol, d.dni), "revertir": revertir(sol, d.dni), "fallo": fallo(sol, "usuarios_error")} {
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("%s sin permiso: %v", nombre, err)
		}
	}
	e.concederASecretaria()

	// Verificar deja la sincronización PERFIL_PENDIENTE (Auth ya tiene el DNI).
	if err := verificar(sol, d.dni); err != nil {
		t.Fatal(err)
	}
	if e.contar(`SELECT count(*) FROM identidad_solicitantes i JOIN usuarios u ON u.id = i.usuario_id
		WHERE i.usuario_id = $1 AND i.estado = 'VERIFICADA' AND i.sincronizacion = 'PERFIL_PENDIENTE' AND u.dni = $2`, sol, d.dni) != 1 {
		t.Fatal("verificada en Auth con el perfil pendiente")
	}
	// Reconciliación: la bandeja filtra las pendientes.
	lista, err := e.conPermisos(personal, "SECRETARIA", "/auth.Auth/ListSolicitanteIdentities", func(ctx context.Context) (any, error) {
		return NewListSolicitanteIdentitiesLogic(ctx, e.sc).ListSolicitanteIdentities(&auth.ListSolicitanteIdentitiesRequest{Estado: "VERIFICADA", SincronizacionPendiente: true})
	})
	encontrada := false
	for _, i := range lista.(*auth.ListSolicitanteIdentitiesResponse).GetItems() {
		if i.Sincronizacion != "PERFIL_PENDIENTE" {
			t.Fatalf("el filtro devolvió una no pendiente: %+v", i)
		}
		encontrada = encontrada || i.UserId == sol
	}
	if err != nil || !encontrada {
		t.Fatalf("listar pendientes: %v", err)
	}
	// Fallo registrado: traza sin DNI, sigue pendiente.
	if err := fallo(sol, "usuarios_no_disponible"); err != nil {
		t.Fatal(err)
	}
	if err := fallo(sol, "codigo con espacios"); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("código de error inválido: %v", err)
	}
	if e.contar(`SELECT sincronizacion_intentos FROM identidad_solicitantes WHERE usuario_id = $1 AND sincronizacion_error = 'usuarios_no_disponible'`, sol) != 1 {
		t.Fatal("traza del intento fallido")
	}
	// Confirmar: solo con el DNI definitivo correcto; idempotente.
	if err := confirmar(sol, e.dniNuevo()); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("confirmar con otro DNI: %v", err)
	}
	if err := confirmar(sol, d.dni); err != nil {
		t.Fatal(err)
	}
	if err := confirmar(sol, d.dni); err != nil {
		t.Fatalf("confirmar dos veces: %v", err)
	}
	// Una verificación COMPLETA no se revierte.
	if err := revertir(sol, d.dni); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("revertir una completa: %v", err)
	}
	if e.contar(`SELECT count(*) FROM usuarios WHERE id = $1 AND dni = $2`, sol, d.dni) != 1 {
		t.Fatal("la completa conserva su DNI")
	}

	// Compensación: Usuarios encontró el DNI en otro perfil.
	sol2, d2 := e.solicitantePendiente("")
	if err := verificar(sol2, d2.dni); err != nil {
		t.Fatal(err)
	}
	if err := revertir(sol2, d2.dni); err != nil {
		t.Fatalf("compensación: %v", err)
	}
	if e.contar(`SELECT count(*) FROM usuarios WHERE id = $1 AND dni IS NULL`, sol2) != 1 ||
		e.contar(`SELECT count(*) FROM identidad_solicitantes WHERE usuario_id = $1 AND estado = 'PENDIENTE'
			AND sincronizacion IS NULL AND sincronizacion_error = 'conflicto_perfil'`, sol2) != 1 {
		t.Fatal("revertida: sin DNI, PENDIENTE y con el conflicto registrado")
	}
	if i, err := e.sc.IdentidadRepository.Obtener(e.ctx, sol2); err != nil || !i.Conflicto {
		t.Fatalf("el conflicto queda visible (no se reintenta a ciegas): %+v %v", i, err)
	}
	// La cuenta no se eliminó: entra con su correo; ya no con el DNI.
	if _, err := e.loginCorreo(d2.email, d2.clave); err != nil {
		t.Fatalf("la cuenta sigue existiendo: %v", err)
	}
	if _, err := e.login(d2.dni, d2.clave); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("sin DNI tras la compensación: %v", err)
	}
	// Compensar otra vez: no hace nada.
	if err := revertir(sol2, d2.dni); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("revertir dos veces: %v", err)
	}
	// Nunca sobre una cuenta institucional (no tiene identidad declarada).
	dniInst := e.dniNuevo()
	inst := e.crearUsuario(dniInst, e.unico("adm")+"@colegio.example", "ClaveAdmin123", "ADMIN", true)
	if err := revertir(inst, dniInst); status.Code(err) != codes.NotFound {
		t.Fatalf("revertir una cuenta institucional: %v", err)
	}
	if e.contar(`SELECT count(*) FROM usuarios WHERE id = $1 AND dni = $2`, inst, dniInst) != 1 {
		t.Fatal("la cuenta institucional no cambia")
	}
	// Auditoría de cada etapa, sin DNI.
	if e.contar(`SELECT count(*) FROM auth_eventos WHERE usuario_id = $1 AND evento = 'identidad_sincronizacion'`, sol) != 3 ||
		e.contar(`SELECT count(*) FROM auth_eventos WHERE usuario_id = $1 AND evento = 'identidad_reversion' AND resultado = 'OK'`, sol2) != 1 ||
		e.contar(`SELECT count(*) FROM auth_eventos WHERE motivo LIKE '%' || $1 || '%' OR motivo LIKE '%' || $2 || '%'`, d.dni, d2.dni) != 0 {
		t.Fatal("auditoría de la sincronización")
	}
}
