package logic

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"auth/auth"
	"auth/internal/config"
	"auth/internal/correo"
	"auth/internal/repository"
	"auth/internal/security"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Registro manual con verificación del correo, contra un PostgreSQL
// DESECHABLE (AUTH_DB_AISLADA_URL, ver google_flujos_test.go). El proveedor
// de correo se simula: guarda los mensajes para leer el código.

type correoFalso struct {
	mu       sync.Mutex
	mensajes []correo.Mensaje
	falla    error
	noConfig bool
}

func (c *correoFalso) Configurado() bool { return !c.noConfig }

func (c *correoFalso) Enviar(_ context.Context, m correo.Mensaje) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.falla != nil {
		return c.falla
	}
	c.mensajes = append(c.mensajes, m)
	return nil
}

var reCodigoCorreo = regexp.MustCompile(`(?m)^    ([0-9]{6})$`)

// ultimo: último mensaje enviado a esa dirección y el código que contiene.
func (c *correoFalso) ultimo(t *testing.T, para string) (correo.Mensaje, string) {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := len(c.mensajes) - 1; i >= 0; i-- {
		if c.mensajes[i].Para == para {
			m := c.mensajes[i]
			if g := reCodigoCorreo.FindStringSubmatch(m.Texto); g != nil {
				return m, g[1]
			}
			return m, ""
		}
	}
	t.Fatalf("no se envió ningún correo a la dirección de prueba")
	return correo.Mensaje{}, ""
}

func (c *correoFalso) cantidad(para string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, m := range c.mensajes {
		if m.Para == para {
			n++
		}
	}
	return n
}

type entornoRegistro struct {
	*entorno
	correo *correoFalso
}

func nuevoEntornoRegistro(t *testing.T, cfg config.RegistroCorreoConfig) *entornoRegistro {
	t.Helper()
	e := nuevoEntorno(t)
	c := &correoFalso{}
	codigos, err := security.NewCodigosVerificacion("secreto-de-prueba-para-jwt-0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	e.sc.Config.RegistroCorreo = cfg
	e.sc.RegistroCorreoRepository = repository.NewRegistroCorreoRepository(e.sc.DB)
	e.sc.Correo = c
	e.sc.CodigosVerificacion = codigos
	return &entornoRegistro{entorno: e, correo: c}
}

type datosRegistro struct {
	email, dni, clave string
}

func (e *entornoRegistro) datos() datosRegistro {
	return datosRegistro{email: e.unico("reg") + "@correo.example", dni: e.dniNuevo(), clave: "ClaveSegura123"}
}

func (e *entornoRegistro) registrar(d datosRegistro) (*auth.EmailRegisterResponse, error) {
	return NewEmailRegisterLogic(e.ctx, e.sc).EmailRegister(&auth.EmailRegisterRequest{
		Email: d.email, Password: d.clave, Dni: d.dni, Nombres: "Ana María", ApellidoPaterno: "Quispe",
		ApellidoMaterno: "Huamán", AcceptTerms: true, TermsVersion: "2026-prueba",
	})
}

func (e *entornoRegistro) verificar(id, codigo string) (*auth.EmailVerifyResponse, error) {
	return NewEmailVerifyLogic(e.ctx, e.sc).EmailVerify(&auth.EmailVerifyRequest{RegistrationId: id, Code: codigo})
}

func (e *entornoRegistro) completar(id, codigo string) (*auth.EmailCompleteResponse, error) {
	return NewEmailCompleteRegistrationLogic(e.ctx, e.sc).EmailCompleteRegistration(&auth.EmailRegistrationRequest{RegistrationId: id, Code: codigo})
}

func (e *entornoRegistro) reenviar(id string) (*auth.EmailResendCodeResponse, error) {
	return NewEmailResendCodeLogic(e.ctx, e.sc).EmailResendCode(&auth.EmailResendCodeRequest{RegistrationId: id})
}

func (e *entornoRegistro) login(dni, clave string) (*auth.LoginResponse, error) {
	return NewLoginLogic(e.ctx, e.sc).Login(&auth.LoginRequest{Dni: dni, Password: clave})
}

// loginCorreo: login de un solicitante con DNI todavía no verificado.
func (e *entornoRegistro) loginCorreo(email, clave string) (*auth.LoginResponse, error) {
	return NewLoginLogic(e.ctx, e.sc).Login(&auth.LoginRequest{Email: email, Password: clave})
}

// verificarIdentidad: Secretaría coteja la identidad (la lógica corre con la
// sesión del personal; el permiso lo exige el interceptor de autorización,
// probado aparte).
func (e *entornoRegistro) verificarIdentidad(personal, usuarioID, dni string) error {
	_, err := e.conSesion(personal, "SECRETARIA", "/auth.Auth/VerifySolicitanteIdentity", func(ctx context.Context) (any, error) {
		return NewVerifySolicitanteIdentityLogic(ctx, e.sc).VerifySolicitanteIdentity(&auth.VerifySolicitanteIdentityRequest{
			UserId: usuarioID, DniEsperado: dni, Metodo: "PRESENCIAL",
		})
	})
	return err
}

// claveRegistro: clave estable de "registro_<clave>: ..." o de los errores
// del login ("correo_no_verificado: ...").
func claveRegistro(err error) string {
	msg := status.Convert(err).Message()
	i := strings.Index(msg, ":")
	switch {
	case strings.HasPrefix(msg, "registro_") && i > 0:
		return msg[len("registro_"):i]
	case strings.HasPrefix(msg, "correo_no_verificado"):
		return "correo_no_verificado"
	}
	return status.Code(err).String()
}

// otroCodigo: un código válido en formato pero distinto del enviado.
func otroCodigo(c string) string {
	if c == "000000" {
		return "000001"
	}
	return "000000"
}

func TestRegistroCorreo_FlujoCompletoYLogin(t *testing.T) {
	e := nuevoEntornoRegistro(t, config.RegistroCorreoConfig{})
	d := e.datos()
	r, err := e.registrar(d)
	if err != nil || r.RegistrationId == "" || r.ExpiresIn != 600 || r.ResendIn != 60 {
		t.Fatalf("registro: %+v %v", r, err)
	}
	m, codigo := e.correo.ultimo(t, d.email)
	if codigo == "" {
		t.Fatal("el correo debe traer un código de 6 dígitos")
	}
	for _, debe := range []string{"SIS-MESA-PARTES", "Tungasuca", "vence en 10 minutos", "No compartas", "Si no solicitaste"} {
		if !strings.Contains(m.Texto, debe) {
			t.Errorf("el correo no menciona %q", debe)
		}
	}
	// La base nunca guarda el código ni la clave en claro.
	if e.contar(`SELECT count(*) FROM registros_correo WHERE id = $1 AND code_hash <> convert_to($2, 'UTF8') AND password_hash LIKE '$2%'`, r.RegistrationId, codigo) != 1 {
		t.Fatal("código y clave deben guardarse como hash")
	}

	// Antes de verificar el correo: no hay cuenta; con la clave correcta se avisa.
	if _, err := e.loginCorreo(d.email, d.clave); status.Code(err) != codes.PermissionDenied || claveRegistro(err) != "correo_no_verificado" {
		t.Fatalf("login sin verificar: %v", err)
	}
	if _, err := e.loginCorreo(d.email, "otra-clave-123"); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("login sin verificar con clave incorrecta: %v", err)
	}
	// El DNI declarado nunca responde distinto a un DNI inexistente.
	if _, err := e.login(d.dni, d.clave); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("login por DNI sin verificar: %v", err)
	}
	if e.contar(`SELECT count(*) FROM usuarios WHERE email = $1`, d.email) != 0 {
		t.Fatal("la cuenta no debe existir antes de verificar")
	}

	v, err := e.verificar(r.RegistrationId, codigo)
	if err != nil || v.UserId == "" || v.ProfileToken == "" || v.Dni != d.dni || v.Email != d.email || v.Nombres != "Ana María" || v.Apellidos != "Quispe Huamán" {
		t.Fatalf("verificación: %+v %v", v, err)
	}
	if e.contar(`SELECT count(*) FROM usuarios u JOIN usuario_roles ur ON ur.usuario_id = u.id JOIN roles r ON r.id = ur.rol_id
		WHERE u.id = $1 AND u.email_verified AND u.email_verified_at IS NOT NULL AND r.nombre = 'SOLICITANTE'`, v.UserId) != 1 {
		t.Fatal("la cuenta debe quedar SOLICITANTE con el correo verificado")
	}
	// Verificar el correo NO verifica la identidad: el DNI solo queda declarado.
	if e.contar(`SELECT count(*) FROM usuarios u JOIN identidad_solicitantes i ON i.usuario_id = u.id
		WHERE u.id = $1 AND u.dni IS NULL AND i.dni_declarado = $2 AND i.estado = 'PENDIENTE' AND i.origen = 'correo'`, v.UserId, d.dni) != 1 {
		t.Fatal("el DNI debe quedar solo declarado, pendiente de verificación")
	}
	// Perfil todavía no creado: no puede iniciar sesión.
	if _, err := e.loginCorreo(d.email, d.clave); status.Code(err) != codes.PermissionDenied || !strings.HasPrefix(status.Convert(err).Message(), "registro_incompleto") {
		t.Fatalf("login con registro incompleto: %v", err)
	}
	// Reintento de la verificación (perfil pendiente): misma cuenta.
	if v2, err := e.verificar(r.RegistrationId, codigo); err != nil || v2.UserId != v.UserId {
		t.Fatalf("reintento de verificación: %v", err)
	}
	fin, err := e.completar(r.RegistrationId, codigo)
	if err != nil || fin.Dni != d.dni {
		t.Fatalf("completar: %v", err)
	}
	// Identidad pendiente: entra con su correo; el DNI todavía no sirve.
	ses, err := e.loginCorreo(d.email, d.clave)
	if err != nil || ses.UserId != v.UserId || ses.Role != "SOLICITANTE" {
		t.Fatalf("login por correo de la cuenta activada: %v", err)
	}
	if _, err := e.loginCorreo(d.email, "clave-incorrecta-9"); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("contraseña incorrecta: %v", err)
	}
	if _, err := e.login(d.dni, d.clave); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("login por DNI antes de verificar la identidad: %v", err)
	}
	// Secretaría verifica la identidad: desde ahí también entra con su DNI.
	personal := e.crearUsuario(e.dniNuevo(), e.unico("sec")+"@colegio.example", "ClaveSecre123", "SECRETARIA", true)
	if err := e.verificarIdentidad(personal, v.UserId, d.dni); err != nil {
		t.Fatalf("verificación de identidad: %v", err)
	}
	if ses, err := e.login(d.dni, d.clave); err != nil || ses.UserId != v.UserId || ses.Role != "SOLICITANTE" {
		t.Fatalf("login por DNI tras verificar la identidad: %v", err)
	}
	if ses, err := e.loginCorreo(d.email, d.clave); err != nil || ses.UserId != v.UserId {
		t.Fatalf("el login por correo sigue funcionando: %v", err)
	}
	// Código reutilizado tras completar: inválido.
	if _, err := e.verificar(r.RegistrationId, codigo); claveRegistro(err) != "invalido" {
		t.Fatalf("código reutilizado: %v", err)
	}
	if _, err := e.completar(r.RegistrationId, codigo); claveRegistro(err) != "invalido" {
		t.Fatalf("completar dos veces: %v", err)
	}
	// Datos personales y clave borrados del registro completado.
	if e.contar(`SELECT count(*) FROM registros_correo WHERE id = $1 AND password_hash IS NULL AND nombres IS NULL AND completado_at IS NOT NULL`, r.RegistrationId) != 1 {
		t.Fatal("el registro completado no debe conservar la clave ni los nombres")
	}
	// Auditoría sin datos sensibles.
	if e.contar(`SELECT count(*) FROM auth_eventos WHERE usuario_id = $1 AND proveedor = 'correo' AND resultado = 'OK'`, v.UserId) < 2 {
		t.Fatal("auditoría de la verificación y la activación")
	}
	if e.contar(`SELECT count(*) FROM auth_eventos WHERE motivo LIKE '%' || $1 || '%' OR motivo LIKE '%' || $2 || '%'`, codigo, d.dni) != 0 {
		t.Fatal("la auditoría no debe contener códigos ni DNI")
	}
}

func TestRegistroCorreo_CorreoExistenteNoSeRevela(t *testing.T) {
	e := nuevoEntornoRegistro(t, config.RegistroCorreoConfig{EsperaReenvio: time.Millisecond})
	d := e.datos()
	e.crearUsuario(e.dniNuevo(), d.email, "ClaveInstitucional1", "SOLICITANTE", true)
	nuevo := e.datos()
	rNuevo, err1 := e.registrar(nuevo)
	rExistente, err2 := e.registrar(d)
	if err1 != nil || err2 != nil || rExistente.RegistrationId == "" ||
		rExistente.ExpiresIn != rNuevo.ExpiresIn || rExistente.ResendIn != rNuevo.ResendIn {
		t.Fatalf("la respuesta debe ser idéntica: %+v %v / %+v %v", rNuevo, err1, rExistente, err2)
	}
	m, codigo := e.correo.ultimo(t, d.email)
	if codigo != "" || !strings.Contains(m.Texto, "ya existe una cuenta") {
		t.Fatal("al correo existente se le envía un aviso, sin código")
	}
	for _, c := range []string{"000000", "123456", "999999"} {
		if _, err := e.verificar(rExistente.RegistrationId, c); claveRegistro(err) != "codigo_incorrecto" && claveRegistro(err) != "intentos_agotados" {
			t.Fatalf("ningún código vale para un correo existente: %v", err)
		}
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := e.reenviar(rExistente.RegistrationId); err != nil {
		t.Fatalf("reenvío a un correo existente: %v", err)
	}
	if m, c := e.correo.ultimo(t, d.email); c != "" || !strings.Contains(m.Texto, "ya existe una cuenta") {
		t.Fatal("el reenvío también es un aviso")
	}
	if e.contar(`SELECT count(*) FROM usuarios WHERE email = $1`, d.email) != 1 {
		t.Fatal("no debe crearse otra cuenta")
	}
}

func TestRegistroCorreo_CodigoIncorrectoIntentosYExpiracion(t *testing.T) {
	e := nuevoEntornoRegistro(t, config.RegistroCorreoConfig{EsperaReenvio: time.Millisecond})
	d := e.datos()
	r, err := e.registrar(d)
	if err != nil {
		t.Fatal(err)
	}
	_, codigo := e.correo.ultimo(t, d.email)
	malo := otroCodigo(codigo)
	for i := 4; i >= 1; i-- {
		_, err := e.verificar(r.RegistrationId, malo)
		if claveRegistro(err) != "codigo_incorrecto" || status.Code(err) != codes.InvalidArgument {
			t.Fatalf("intento incorrecto: %v", err)
		}
		if i == 1 && !strings.Contains(status.Convert(err).Message(), "Te queda 1 intento") {
			t.Fatalf("debe informar el intento restante: %v", err)
		}
	}
	if _, err := e.verificar(r.RegistrationId, malo); claveRegistro(err) != "intentos_agotados" || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("quinto intento: %v", err)
	}
	// Agotados: ni el código correcto sirve.
	if _, err := e.verificar(r.RegistrationId, codigo); claveRegistro(err) != "intentos_agotados" {
		t.Fatalf("código correcto con intentos agotados: %v", err)
	}
	if _, err := e.verificar(r.RegistrationId, "12ab56"); claveRegistro(err) != "codigo_formato" {
		t.Fatalf("formato: %v", err)
	}
	if _, err := e.verificar("no-es-un-uuid", codigo); claveRegistro(err) != "invalido" {
		t.Fatalf("id inválido: %v", err)
	}
	// Reenvío: código nuevo, intentos en cero; el anterior ya no sirve.
	time.Sleep(5 * time.Millisecond)
	if _, err := e.reenviar(r.RegistrationId); err != nil {
		t.Fatal(err)
	}
	_, nuevo := e.correo.ultimo(t, d.email)
	if nuevo != codigo {
		if _, err := e.verificar(r.RegistrationId, codigo); claveRegistro(err) != "codigo_incorrecto" {
			t.Fatalf("el código anterior debe quedar invalidado: %v", err)
		}
	}
	// Expirado.
	if _, err := e.admin.Exec(e.ctx, `UPDATE registros_correo SET created_at = now() - interval '1 hour', expires_at = now() - interval '1 minute' WHERE id = $1`, r.RegistrationId); err != nil {
		t.Fatal(err)
	}
	if _, err := e.verificar(r.RegistrationId, nuevo); claveRegistro(err) != "codigo_expirado" {
		t.Fatalf("código expirado: %v", err)
	}
	if e.contar(`SELECT count(*) FROM usuarios WHERE email = $1`, d.email) != 0 {
		t.Fatal("ningún intento fallido crea la cuenta")
	}
}

func TestRegistroCorreo_ReenvioEsperaYLimite(t *testing.T) {
	e := nuevoEntornoRegistro(t, config.RegistroCorreoConfig{EsperaReenvio: time.Hour})
	d := e.datos()
	r, err := e.registrar(d)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.reenviar(r.RegistrationId); claveRegistro(err) != "espera_reenvio" || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("reenvío inmediato: %v", err)
	}

	e2 := nuevoEntornoRegistro(t, config.RegistroCorreoConfig{EsperaReenvio: time.Millisecond, MaxEnvios: 3})
	d2 := e2.datos()
	r2, err := e2.registrar(d2)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		time.Sleep(5 * time.Millisecond)
		if _, err := e2.reenviar(r2.RegistrationId); err != nil {
			t.Fatalf("reenvío %d: %v", i+1, err)
		}
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := e2.reenviar(r2.RegistrationId); claveRegistro(err) != "reenvios_agotados" {
		t.Fatalf("máximo de envíos: %v", err)
	}
	if e2.correo.cantidad(d2.email) != 3 {
		t.Fatalf("se esperaban 3 correos, hubo %d", e2.correo.cantidad(d2.email))
	}
	// El último código enviado funciona.
	_, codigo := e2.correo.ultimo(t, d2.email)
	if _, err := e2.verificar(r2.RegistrationId, codigo); err != nil {
		t.Fatalf("verificar con el último código: %v", err)
	}
	if _, err := e2.reenviar(r2.RegistrationId); claveRegistro(err) != "invalido" {
		t.Fatalf("reenvío de un registro verificado: %v", err)
	}
}

func TestRegistroCorreo_NuevoRegistroAnulaElAnteriorYLimitePorHora(t *testing.T) {
	e := nuevoEntornoRegistro(t, config.RegistroCorreoConfig{MaxRegistrosPorHora: 2})
	d := e.datos()
	r1, err := e.registrar(d)
	if err != nil {
		t.Fatal(err)
	}
	_, c1 := e.correo.ultimo(t, d.email)
	if _, err := e.registrar(d); err != nil {
		t.Fatal(err)
	}
	if _, err := e.verificar(r1.RegistrationId, c1); claveRegistro(err) != "invalido" {
		t.Fatalf("el registro anterior debe quedar anulado: %v", err)
	}
	if _, err := e.registrar(d); claveRegistro(err) != "demasiadas_solicitudes" || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("límite por hora: %v", err)
	}
}

func TestRegistroCorreo_FalloDelProveedor(t *testing.T) {
	e := nuevoEntornoRegistro(t, config.RegistroCorreoConfig{EsperaReenvio: time.Millisecond})
	d := e.datos()
	e.correo.falla = &correo.ErrEnvio{Etapa: "conexión"}
	if _, err := e.registrar(d); claveRegistro(err) != "envio_fallido" || status.Code(err) != codes.Unavailable {
		t.Fatalf("fallo del proveedor al registrar: %v", err)
	}
	if e.contar(`SELECT count(*) FROM registros_correo WHERE email = $1`, d.email) != 0 {
		t.Fatal("si el correo no sale, no se guarda el registro")
	}
	e.correo.falla = nil
	r, err := e.registrar(d)
	if err != nil {
		t.Fatal(err)
	}
	_, codigo := e.correo.ultimo(t, d.email)
	time.Sleep(5 * time.Millisecond)
	e.correo.falla = errors.New("smtp caído")
	if _, err := e.reenviar(r.RegistrationId); claveRegistro(err) != "envio_fallido" {
		t.Fatalf("fallo del proveedor al reenviar: %v", err)
	}
	e.correo.falla = nil
	if _, err := e.verificar(r.RegistrationId, codigo); err != nil {
		t.Fatalf("si el reenvío falla, el código anterior sigue valiendo: %v", err)
	}

	sinSMTP := nuevoEntornoRegistro(t, config.RegistroCorreoConfig{})
	sinSMTP.correo.noConfig = true
	if _, err := sinSMTP.registrar(sinSMTP.datos()); claveRegistro(err) != "no_disponible" || status.Code(err) != codes.Unavailable {
		t.Fatalf("sin proveedor configurado: %v", err)
	}
}

func TestRegistroCorreo_ValidacionDelFormulario(t *testing.T) {
	e := nuevoEntornoRegistro(t, config.RegistroCorreoConfig{})
	base := func() *auth.EmailRegisterRequest {
		d := e.datos()
		return &auth.EmailRegisterRequest{Email: d.email, Password: d.clave, Dni: d.dni, Nombres: "Ana",
			ApellidoPaterno: "Quispe", AcceptTerms: true, TermsVersion: "v1"}
	}
	casos := map[string]func(r *auth.EmailRegisterRequest){
		"sin términos":       func(r *auth.EmailRegisterRequest) { r.AcceptTerms = false },
		"sin versión":        func(r *auth.EmailRegisterRequest) { r.TermsVersion = " " },
		"correo inválido":    func(r *auth.EmailRegisterRequest) { r.Email = "no-es-correo" },
		"correo con salto":   func(r *auth.EmailRegisterRequest) { r.Email = "a@b.example\r\nBcc: x@y.example" },
		"clave corta":        func(r *auth.EmailRegisterRequest) { r.Password = "1234567" },
		"clave larga":        func(r *auth.EmailRegisterRequest) { r.Password = strings.Repeat("a", 73) },
		"dni inválido":       func(r *auth.EmailRegisterRequest) { r.Dni = "1234" },
		"sin nombres":        func(r *auth.EmailRegisterRequest) { r.Nombres = "  " },
		"sin paterno":        func(r *auth.EmailRegisterRequest) { r.ApellidoPaterno = "" },
		"nombres muy largos": func(r *auth.EmailRegisterRequest) { r.Nombres = strings.Repeat("a", 101) },
	}
	for nombre, mod := range casos {
		r := base()
		mod(r)
		if _, err := NewEmailRegisterLogic(e.ctx, e.sc).EmailRegister(r); status.Code(err) != codes.InvalidArgument {
			t.Errorf("%s: %v", nombre, err)
		}
	}
	if len(e.correo.mensajes) != 0 {
		t.Fatal("un formulario inválido no envía correos")
	}
}

// Verificar un correo propio con el DNI de OTRA persona no da ese DNI: la
// cuenta nueva queda sin DNI, el titular no cambia, el registro no revela
// nada y Secretaría ve el conflicto (y no puede verificarlo).
func TestRegistroCorreo_DNIAjenoNoSeApropia(t *testing.T) {
	e := nuevoEntornoRegistro(t, config.RegistroCorreoConfig{})
	d := e.datos()
	titular := e.crearUsuario(d.dni, e.unico("tit")+"@colegio.example", "ClaveTitular123", "SOLICITANTE", true)
	r, err := e.registrar(d)
	if err != nil {
		t.Fatalf("el registro no revela que el DNI existe: %v", err)
	}
	_, codigo := e.correo.ultimo(t, d.email)
	v, err := e.verificar(r.RegistrationId, codigo)
	if err != nil {
		t.Fatalf("verificar el correo propio: %v", err)
	}
	if _, err := e.completar(r.RegistrationId, codigo); err != nil {
		t.Fatal(err)
	}
	if e.contar(`SELECT count(*) FROM usuarios WHERE id = $1 AND dni IS NULL`, v.UserId) != 1 ||
		e.contar(`SELECT count(*) FROM usuarios WHERE dni = $1`, d.dni) != 1 ||
		e.contar(`SELECT count(*) FROM usuarios WHERE id = $1 AND dni = $2`, titular, d.dni) != 1 {
		t.Fatal("el DNI sigue siendo solo del titular")
	}
	// Con el DNI ajeno no entra; el titular sí.
	if _, err := e.login(d.dni, d.clave); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("login con el DNI ajeno: %v", err)
	}
	if ses, err := e.login(d.dni, "ClaveTitular123"); err != nil || ses.UserId != titular {
		t.Fatalf("login del titular: %v", err)
	}
	// Secretaría: conflicto visible y verificación rechazada sin cambios.
	if i, err := e.sc.IdentidadRepository.Obtener(e.ctx, v.UserId); err != nil || !i.Conflicto {
		t.Fatalf("conflicto visible para el personal: %+v %v", i, err)
	}
	personal := e.crearUsuario(e.dniNuevo(), e.unico("sec")+"@colegio.example", "ClaveSecre123", "SECRETARIA", true)
	if err := e.verificarIdentidad(personal, v.UserId, d.dni); status.Code(err) != codes.AlreadyExists ||
		!strings.HasPrefix(status.Convert(err).Message(), "identidad_conflicto") {
		t.Fatalf("verificación con DNI ajeno: %v", err)
	}
	if e.contar(`SELECT count(*) FROM usuarios WHERE id = $1 AND dni IS NULL`, v.UserId) != 1 ||
		e.contar(`SELECT count(*) FROM identidad_solicitantes WHERE usuario_id = $1 AND estado = 'PENDIENTE'`, v.UserId) != 1 ||
		e.contar(`SELECT count(*) FROM usuarios WHERE id = $1 AND dni = $2`, titular, d.dni) != 1 {
		t.Fatal("un conflicto no deja cambios parciales")
	}
}

func TestRegistroCorreo_Compensacion(t *testing.T) {
	e := nuevoEntornoRegistro(t, config.RegistroCorreoConfig{})
	d := e.datos()
	r, err := e.registrar(d)
	if err != nil {
		t.Fatal(err)
	}
	_, codigo := e.correo.ultimo(t, d.email)
	cancelar := func() error {
		_, err := NewEmailCancelRegistrationLogic(e.ctx, e.sc).EmailCancelRegistration(&auth.EmailRegistrationRequest{RegistrationId: r.RegistrationId, Code: codigo})
		return err
	}
	if err := cancelar(); claveRegistro(err) != "no_verificado" {
		t.Fatalf("cancelar sin verificar: %v", err)
	}
	v, err := e.verificar(r.RegistrationId, codigo)
	if err != nil {
		t.Fatal(err)
	}
	if err := cancelar(); err != nil {
		t.Fatalf("compensación: %v", err)
	}
	if e.contar(`SELECT count(*) FROM usuarios WHERE id = $1`, v.UserId) != 0 ||
		e.contar(`SELECT count(*) FROM registros_correo WHERE id = $1 AND anulado_at IS NOT NULL AND usuario_id IS NULL`, r.RegistrationId) != 1 {
		t.Fatal("la compensación elimina la cuenta y anula el registro")
	}
	if _, err := e.login(d.dni, d.clave); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("login tras la compensación: %v", err)
	}
	if err := cancelar(); claveRegistro(err) != "invalido" {
		t.Fatalf("segunda compensación: %v", err)
	}
}

func TestRegistroCorreo_VerificacionesConcurrentesUnaSolaCuenta(t *testing.T) {
	e := nuevoEntornoRegistro(t, config.RegistroCorreoConfig{})
	d := e.datos()
	r, err := e.registrar(d)
	if err != nil {
		t.Fatal(err)
	}
	_, codigo := e.correo.ultimo(t, d.email)
	const n = 8
	ids := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v, err := e.verificar(r.RegistrationId, codigo)
			errs[i] = err
			if err == nil {
				ids[i] = v.UserId
			}
		}(i)
	}
	wg.Wait()
	for i := 0; i < n; i++ {
		if errs[i] != nil || ids[i] != ids[0] {
			t.Fatalf("verificación concurrente %d: %v", i, errs[i])
		}
	}
	if e.contar(`SELECT count(*) FROM usuarios WHERE email = $1`, d.email) != 1 {
		t.Fatal("solo una cuenta")
	}

	// Registros concurrentes del mismo correo: solo uno queda vigente.
	d2 := e.datos()
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = e.registrar(d2) }()
	}
	wg.Wait()
	if e.contar(`SELECT count(*) FROM registros_correo WHERE email = $1 AND anulado_at IS NULL`, d2.email) != 1 {
		t.Fatal("solo un registro vigente por correo")
	}
}
