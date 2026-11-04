package logic

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"auth/auth"
	"auth/internal/config"
	"auth/internal/interceptor"
	"auth/internal/repository"
	"auth/internal/security"
	"auth/internal/svc"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Flujos de Google contra un PostgreSQL DESECHABLE con todas las migraciones
// de Auth. Nunca contra auth_db real.
//   - AUTH_DB_AISLADA_URL: conexión del código bajo prueba. Con los roles
//     reales es auth_app (privilegios mínimos de producción).
//   - AUTH_DB_AISLADA_ADMIN_URL (opcional): conexión para sembrar datos y
//     verificar (auth_tester); sin ella se usa la misma URL. Google se simula (googleFalso): la validación criptográfica
// del ID token se prueba aparte, con go-oidc, en internal/security.

type googleFalso struct {
	mu          sync.Mutex
	identidades map[string]*security.GoogleIdentity
}

func (g *googleFalso) Configurado() bool { return true }

func (g *googleFalso) AuthCodeURL(_ context.Context, state, _, _ string) (string, error) {
	return "https://proveedor.example/auth?state=" + state, nil
}

func (g *googleFalso) Exchange(_ context.Context, code, _, _ string) (*security.GoogleIdentity, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	id, ok := g.identidades[code]
	if !ok {
		return nil, security.ErrGoogleCodigo
	}
	delete(g.identidades, code)
	copia := *id
	return &copia, nil
}

type entorno struct {
	t      *testing.T
	ctx    context.Context
	sc     *svc.ServiceContext
	admin  *pgxpool.Pool // siembra y verificación (nunca la usa el código bajo prueba)
	google *googleFalso
	n      int
}

func nuevoEntorno(t *testing.T) *entorno {
	t.Helper()
	url := os.Getenv("AUTH_DB_AISLADA_URL")
	if url == "" {
		t.Skip("AUTH_DB_AISLADA_URL no definida: estas pruebas solo corren contra un PostgreSQL desechable")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	admin := pool
	if u := os.Getenv("AUTH_DB_AISLADA_ADMIN_URL"); u != "" {
		if admin, err = pgxpool.New(ctx, u); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(admin.Close)
	}
	for _, rol := range []string{"ADMIN", "SECRETARIA", "SOLICITANTE", "PSICOLOGO"} {
		if _, err := admin.Exec(ctx, `INSERT INTO roles (nombre) VALUES ($1) ON CONFLICT (nombre) DO NOTHING`, rol); err != nil {
			t.Fatal(err)
		}
	}
	g := &googleFalso{identidades: map[string]*security.GoogleIdentity{}}
	sc := &svc.ServiceContext{
		Config:                 config.Config{AccessTokenDuration: 15 * time.Minute, RefreshTokenDuration: time.Hour},
		DB:                     pool,
		UserRepository:         repository.NewUserRepository(pool),
		RefreshTokenRepository: repository.NewRefreshTokenRepository(pool),
		GoogleRepository:       repository.NewGoogleRepository(pool),
		IdentidadRepository:    repository.NewIdentidadRepository(pool),
		JWTManager:             security.NewJWTManager("secreto-de-prueba-para-jwt-0123456789abcdef", 15*time.Minute),
		Google:                 g,
	}
	return &entorno{t: t, ctx: ctx, sc: sc, admin: admin, google: g}
}

func aleatorio(n int64) int64 {
	v, _ := rand.Int(rand.Reader, big.NewInt(n))
	return v.Int64()
}

func (e *entorno) dniNuevo() string { return fmt.Sprintf("%08d", 10000000+aleatorio(89999999)) }

func (e *entorno) unico(prefijo string) string {
	e.n++
	return fmt.Sprintf("%s%d%d", prefijo, time.Now().UnixNano()%1e9, e.n)
}

// crearUsuario: cuenta "institucional" sembrada directamente.
func (e *entorno) crearUsuario(dni, email, clave, rol string, activo bool) string {
	e.t.Helper()
	hash, _ := bcrypt.GenerateFromPassword([]byte(clave), bcrypt.MinCost)
	var id string
	err := e.admin.QueryRow(e.ctx, `
		INSERT INTO usuarios (username, email, password_hash, dni, estado) VALUES ($1, $2, $3, $4, $5) RETURNING id
	`, e.unico("inst_"), email, string(hash), dni, activo).Scan(&id)
	if err != nil {
		e.t.Fatal(err)
	}
	if _, err := e.admin.Exec(e.ctx, `INSERT INTO usuario_roles (usuario_id, rol_id) SELECT $1, id FROM roles WHERE nombre = $2`, id, rol); err != nil {
		e.t.Fatal(err)
	}
	return id
}

// callback: simula el regreso desde Google con una identidad.
func (e *entorno) callback(sub, email string, verificado bool, linkCode string) (*auth.GoogleCallbackResponse, error) {
	code := e.unico("code-")
	e.google.mu.Lock()
	e.google.identidades[code] = &security.GoogleIdentity{Subject: sub, Email: strings.ToLower(email), EmailVerified: verificado, GivenName: "Ana", FamilyName: "Quispe"}
	e.google.mu.Unlock()
	return NewGoogleCallbackLogic(e.ctx, e.sc).GoogleCallback(&auth.GoogleCallbackRequest{Code: code, Verifier: "v", Nonce: "n", LinkCode: linkCode})
}

func (e *entorno) contar(sql string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.admin.QueryRow(e.ctx, sql, args...).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

// conSesion ejecuta la lógica a través del interceptor REAL (JWT en metadata).
func (e *entorno) conSesion(usuarioID, rol, metodo string, f func(ctx context.Context) (any, error)) (any, error) {
	token, _, _ := e.sc.JWTManager.GenerateAccessToken(usuarioID, "u", rol)
	ctx := metadata.NewIncomingContext(e.ctx, metadata.Pairs("authorization", "Bearer "+token))
	icpt := interceptor.AuthenticationInterceptor(e.sc.JWTManager, interceptor.DefaultMethodPolicies())
	return icpt(ctx, nil, &grpc.UnaryServerInfo{FullMethod: metodo}, func(ctx context.Context, _ any) (any, error) { return f(ctx) })
}

func clave(err error) string {
	msg := status.Convert(err).Message()
	if i := strings.Index(msg, ":"); strings.HasPrefix(msg, "google_") && i > 0 {
		return msg[len("google_"):i]
	}
	return status.Code(err).String()
}

func TestGoogle_RegistroNuevoYLuegoLogin(t *testing.T) {
	e := nuevoEntorno(t)
	sub, email, dni := e.unico("sub-"), e.unico("nuevo")+"@gmail.example", e.dniNuevo()

	cb, err := e.callback(sub, email, true, "")
	if err != nil || cb.Result != ResultadoRegistro || cb.LoginCode == "" {
		t.Fatalf("callback: %+v %v", cb, err)
	}
	ex, err := NewGoogleExchangeLogic(e.ctx, e.sc).GoogleExchange(&auth.GoogleExchangeRequest{LoginCode: cb.LoginCode})
	if err != nil || ex.Result != ResultadoRegistroPendiente || ex.Email != email || ex.Nombres != "Ana" || ex.Session != nil {
		t.Fatalf("canje de registro: %+v %v", ex, err)
	}
	reg, err := NewGoogleRegisterLogic(e.ctx, e.sc).GoogleRegister(&auth.GoogleRegisterRequest{LoginCode: cb.LoginCode, Dni: dni})
	if err != nil || reg.Dni != dni || reg.Email != email {
		t.Fatalf("registro: %+v %v", reg, err)
	}
	claims, err := e.sc.JWTManager.ValidateAccessToken(reg.ProfileToken)
	if err != nil || claims.Subject != reg.UserId || claims.Role != "SOLICITANTE" {
		t.Fatalf("token de perfil: %+v %v", claims, err)
	}
	// Cuenta creada con rol fijo SOLICITANTE, correo verificado y pendiente.
	// El DNI solo queda DECLARADO (usuarios.dni NULL hasta que Secretaría
	// verifique la identidad).
	if e.contar(`SELECT count(*) FROM usuarios u JOIN usuario_roles ur ON ur.usuario_id=u.id JOIN roles r ON r.id=ur.rol_id
		JOIN identidad_solicitantes i ON i.usuario_id=u.id
		WHERE u.id=$1 AND r.nombre='SOLICITANTE' AND u.email_verified AND u.dni IS NULL
		  AND i.dni_declarado=$2 AND i.estado='PENDIENTE' AND i.origen='google'`, reg.UserId, dni) != 1 ||
		e.contar(`SELECT count(*) FROM user_auth_providers WHERE usuario_id=$1 AND NOT registro_completo`, reg.UserId) != 1 {
		t.Fatal("estado de la cuenta pendiente")
	}
	// Mientras falte el perfil, volver con Google retoma el formulario (no hay login).
	cb2, err := e.callback(sub, email, true, "")
	if err != nil || cb2.Result != ResultadoRegistro {
		t.Fatalf("registro pendiente: %+v %v", cb2, err)
	}
	// Reintento idempotente: mismo usuario, sin duplicar.
	reg2, err := NewGoogleRegisterLogic(e.ctx, e.sc).GoogleRegister(&auth.GoogleRegisterRequest{LoginCode: cb.LoginCode, Dni: dni})
	if err != nil || reg2.UserId != reg.UserId {
		t.Fatalf("reintento: %+v %v", reg2, err)
	}
	if _, err := NewGoogleRegisterLogic(e.ctx, e.sc).GoogleRegister(&auth.GoogleRegisterRequest{LoginCode: cb.LoginCode, Dni: e.dniNuevo()}); clave(err) != "dni_distinto" {
		t.Fatalf("otro DNI en el reintento: %v", err)
	}
	fin, err := NewGoogleCompleteRegistrationLogic(e.ctx, e.sc).GoogleCompleteRegistration(&auth.GoogleRegistrationRequest{LoginCode: cb.LoginCode})
	if err != nil || fin.Session.AccessToken == "" || fin.Session.RefreshToken == "" || fin.Session.Role != "SOLICITANTE" || fin.Session.UserId != reg.UserId {
		t.Fatalf("completar: %+v %v", fin, err)
	}
	if _, err := NewGoogleCompleteRegistrationLogic(e.ctx, e.sc).GoogleCompleteRegistration(&auth.GoogleRegistrationRequest{LoginCode: cb.LoginCode}); clave(err) != "codigo_invalido" {
		t.Fatalf("código reutilizado: %v", err)
	}
	// Ya completo: login con Google, mismo UUID, código de un solo uso.
	cb3, err := e.callback(sub, email, true, "")
	if err != nil || cb3.Result != ResultadoLogin {
		t.Fatalf("login: %+v %v", cb3, err)
	}
	ses, err := NewGoogleExchangeLogic(e.ctx, e.sc).GoogleExchange(&auth.GoogleExchangeRequest{LoginCode: cb3.LoginCode})
	if err != nil || ses.Result != ResultadoSesion || ses.Session.UserId != reg.UserId {
		t.Fatalf("sesión: %+v %v", ses, err)
	}
	if _, err := NewGoogleExchangeLogic(e.ctx, e.sc).GoogleExchange(&auth.GoogleExchangeRequest{LoginCode: cb3.LoginCode}); clave(err) != "codigo_invalido" {
		t.Fatalf("código de login reutilizado: %v", err)
	}
	// La renovación tradicional funciona con la sesión de Google.
	ren, err := NewRefreshTokenLogic(e.ctx, e.sc).RefreshToken(&auth.RefreshTokenRequest{RefreshToken: ses.Session.RefreshToken})
	if err != nil || ren.AccessToken == "" {
		t.Fatalf("refresh: %v", err)
	}
	if e.contar(`SELECT count(*) FROM auth_eventos WHERE usuario_id=$1 AND resultado='OK'`, reg.UserId) < 2 {
		t.Fatal("auditoría de eventos")
	}
}

// El DNI escrito en el registro con Google solo se DECLARA: usar el DNI de
// una cuenta existente no se rechaza (eso permitiría enumerar DNI), pero
// tampoco toma ni modifica nada. La cuenta existente queda intacta y el
// conflicto solo lo ve el personal al verificar.
func TestGoogle_DNIExistenteNoSeReservaNiSeToma(t *testing.T) {
	e := nuevoEntorno(t)
	dni, correoInst := e.dniNuevo(), e.unico("inst")+"@colegio.example"
	inst := e.crearUsuario(dni, correoInst, "ClaveInstitucional1", "SOLICITANTE", true)
	var hashAntes string
	_ = e.admin.QueryRow(e.ctx, `SELECT password_hash FROM usuarios WHERE id=$1`, inst).Scan(&hashAntes)

	email := e.unico("g") + "@gmail.example"
	cb, err := e.callback(e.unico("sub-"), email, true, "")
	if err != nil {
		t.Fatal(err)
	}
	reg, err := NewGoogleRegisterLogic(e.ctx, e.sc).GoogleRegister(&auth.GoogleRegisterRequest{LoginCode: cb.LoginCode, Dni: dni})
	if err != nil {
		t.Fatalf("el registro no revela que el DNI existe: %v", err)
	}
	if e.contar(`SELECT count(*) FROM usuarios WHERE id=$1 AND dni IS NULL`, reg.UserId) != 1 ||
		e.contar(`SELECT count(*) FROM usuarios WHERE dni=$1`, dni) != 1 ||
		e.contar(`SELECT count(*) FROM identidad_solicitantes WHERE usuario_id=$1 AND dni_declarado=$2 AND estado='PENDIENTE'`, reg.UserId, dni) != 1 {
		t.Fatal("el DNI solo queda declarado: la cuenta nueva no lo obtiene")
	}
	if i, err := e.sc.IdentidadRepository.Obtener(e.ctx, reg.UserId); err != nil || !i.Conflicto {
		t.Fatalf("el personal debe ver el conflicto: %+v %v", i, err)
	}
	var hashDespues, correo string
	var activo bool
	_ = e.admin.QueryRow(e.ctx, `SELECT password_hash, email::text, estado FROM usuarios WHERE id=$1`, inst).Scan(&hashDespues, &correo, &activo)
	if hashDespues != hashAntes || correo != correoInst || !activo || e.contar(`SELECT count(*) FROM user_auth_providers WHERE usuario_id=$1`, inst) != 0 {
		t.Fatal("la cuenta institucional no debe cambiar ni vincularse")
	}
	// Con el DNI de una cuenta ADMIN: misma respuesta y el ADMIN no cambia.
	dniAdmin := e.dniNuevo()
	admin := e.crearUsuario(dniAdmin, e.unico("adm")+"@colegio.example", "ClaveAdmin123", "ADMIN", true)
	cb2, _ := e.callback(e.unico("sub-"), e.unico("g")+"@gmail.example", true, "")
	reg2, err2 := NewGoogleRegisterLogic(e.ctx, e.sc).GoogleRegister(&auth.GoogleRegisterRequest{LoginCode: cb2.LoginCode, Dni: dniAdmin})
	if err2 != nil || e.contar(`SELECT count(*) FROM usuarios WHERE id=$1 AND dni IS NULL`, reg2.UserId) != 1 {
		t.Fatalf("DNI de un ADMIN: %v", err2)
	}
	if e.contar(`SELECT count(*) FROM usuarios WHERE id=$1 AND dni=$2`, admin, dniAdmin) != 1 ||
		e.contar(`SELECT count(*) FROM user_auth_providers WHERE usuario_id=$1`, admin) != 0 {
		t.Fatal("la cuenta ADMIN no debe cambiar")
	}
	if ses, err := NewLoginLogic(e.ctx, e.sc).Login(&auth.LoginRequest{Dni: dniAdmin, Password: "ClaveAdmin123"}); err != nil || ses.UserId != admin || ses.Role != "ADMIN" {
		t.Fatalf("login del ADMIN: %v", err)
	}
}

func TestGoogle_CorreoExistenteYEmailNoVerificado(t *testing.T) {
	e := nuevoEntorno(t)
	correo := e.unico("Persona") + "@Gmail.example"
	inst := e.crearUsuario(e.dniNuevo(), strings.ToLower(correo), "ClaveInstitucional1", "SOLICITANTE", true)
	_, err := e.callback(e.unico("sub-"), strings.ToUpper(correo), true, "")
	if status.Code(err) != codes.AlreadyExists || clave(err) != "correo_existente" {
		t.Fatalf("correo existente: %v", err)
	}
	if e.contar(`SELECT count(*) FROM user_auth_providers WHERE usuario_id=$1`, inst) != 0 {
		t.Fatal("nunca se vincula por correo")
	}
	if _, err := e.callback(e.unico("sub-"), e.unico("x")+"@gmail.example", false, ""); status.Code(err) != codes.FailedPrecondition || clave(err) != "email_no_verificado" {
		t.Fatalf("email no verificado: %v", err)
	}
}

func TestGoogle_CodigoVencidoEInvalido(t *testing.T) {
	e := nuevoEntorno(t)
	cb, _ := e.callback(e.unico("sub-"), e.unico("v")+"@gmail.example", true, "")
	if _, err := e.admin.Exec(e.ctx, `UPDATE oauth_login_codes SET created_at = now() - interval '2 hours', expires_at = now() - interval '1 hour' WHERE code_hash=$1`,
		security.HashRefreshToken(cb.LoginCode)); err != nil {
		t.Fatal(err)
	}
	if _, err := NewGoogleExchangeLogic(e.ctx, e.sc).GoogleExchange(&auth.GoogleExchangeRequest{LoginCode: cb.LoginCode}); clave(err) != "codigo_invalido" {
		t.Fatalf("código vencido (canje): %v", err)
	}
	if _, err := NewGoogleRegisterLogic(e.ctx, e.sc).GoogleRegister(&auth.GoogleRegisterRequest{LoginCode: cb.LoginCode, Dni: e.dniNuevo()}); clave(err) != "codigo_invalido" {
		t.Fatalf("código vencido (registro): %v", err)
	}
	if _, err := NewGoogleExchangeLogic(e.ctx, e.sc).GoogleExchange(&auth.GoogleExchangeRequest{LoginCode: "inventado"}); clave(err) != "codigo_invalido" {
		t.Fatalf("código inventado: %v", err)
	}
}

func TestGoogle_ConcurrenciaSinDuplicados(t *testing.T) {
	e := nuevoEntorno(t)
	// Doble envío del mismo formulario (mismo código): una sola cuenta.
	email, dni := e.unico("doble")+"@gmail.example", e.dniNuevo()
	cb, _ := e.callback(e.unico("sub-"), email, true, "")
	var wg sync.WaitGroup
	ids := make([]string, 5)
	for i := range ids {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if r, err := NewGoogleRegisterLogic(e.ctx, e.sc).GoogleRegister(&auth.GoogleRegisterRequest{LoginCode: cb.LoginCode, Dni: dni}); err == nil {
				ids[i] = r.UserId
			}
		}(i)
	}
	wg.Wait()
	if e.contar(`SELECT count(*) FROM identidad_solicitantes WHERE dni_declarado=$1`, dni) != 1 {
		t.Fatal("doble envío creó más de una cuenta")
	}
	for _, id := range ids {
		if id != "" && id != ids[0] && ids[0] != "" {
			t.Fatalf("reintentos con distinto usuario: %v", ids)
		}
	}
	// Varias personas (identidades Google distintas) declaran el mismo DNI a la
	// vez: todas se registran, ninguna lo reserva (la unicidad se aplica al
	// verificar la identidad; ver TestIdentidad_VerificacionesConcurrentes).
	dni2 := e.dniNuevo()
	codigos := make([]string, 4)
	for i := range codigos {
		c, _ := e.callback(e.unico("sub-"), e.unico("p")+"@gmail.example", true, "")
		codigos[i] = c.LoginCode
	}
	var ok, rechazados int
	var mu sync.Mutex
	for _, c := range codigos {
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			_, err := NewGoogleRegisterLogic(e.ctx, e.sc).GoogleRegister(&auth.GoogleRegisterRequest{LoginCode: c, Dni: dni2})
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				ok++
			} else if clave(err) == "dni_registrado" {
				rechazados++
			}
		}(c)
	}
	wg.Wait()
	if ok != 4 || rechazados != 0 || e.contar(`SELECT count(*) FROM usuarios WHERE dni=$1`, dni2) != 0 ||
		e.contar(`SELECT count(*) FROM identidad_solicitantes WHERE dni_declarado=$1 AND estado='PENDIENTE'`, dni2) != 4 {
		t.Fatalf("declaraciones simultáneas del mismo DNI: ok=%d rechazados=%d", ok, rechazados)
	}
}

func TestGoogle_CompensacionPorPerfilRechazado(t *testing.T) {
	e := nuevoEntorno(t)
	email := e.unico("comp") + "@gmail.example"
	cb, _ := e.callback(e.unico("sub-"), email, true, "")
	reg, err := NewGoogleRegisterLogic(e.ctx, e.sc).GoogleRegister(&auth.GoogleRegisterRequest{LoginCode: cb.LoginCode, Dni: e.dniNuevo()})
	if err != nil {
		t.Fatal(err)
	}
	// Usuarios Service rechazó el perfil (DNI ya registrado allí): se compensa.
	if r, err := NewGoogleCancelRegistrationLogic(e.ctx, e.sc).GoogleCancelRegistration(&auth.GoogleRegistrationRequest{LoginCode: cb.LoginCode}); err != nil || !r.Cancelled {
		t.Fatalf("cancelar: %v", err)
	}
	if e.contar(`SELECT count(*) FROM usuarios WHERE id=$1`, reg.UserId) != 0 || e.contar(`SELECT count(*) FROM user_auth_providers WHERE provider_email=$1`, email) != 0 {
		t.Fatal("la cuenta pendiente debe eliminarse")
	}
	if e.contar(`SELECT count(*) FROM auth_eventos WHERE usuario_id=$1 AND evento='google_registro_cancelado'`, reg.UserId) != 1 {
		t.Fatal("la compensación queda auditada")
	}
	// Una cuenta ya completa (con sesión) nunca se elimina por compensación.
	cb2, _ := e.callback(e.unico("sub-"), e.unico("ok")+"@gmail.example", true, "")
	reg2, _ := NewGoogleRegisterLogic(e.ctx, e.sc).GoogleRegister(&auth.GoogleRegisterRequest{LoginCode: cb2.LoginCode, Dni: e.dniNuevo()})
	_, _ = NewGoogleCompleteRegistrationLogic(e.ctx, e.sc).GoogleCompleteRegistration(&auth.GoogleRegistrationRequest{LoginCode: cb2.LoginCode})
	if _, err := NewGoogleCancelRegistrationLogic(e.ctx, e.sc).GoogleCancelRegistration(&auth.GoogleRegistrationRequest{LoginCode: cb2.LoginCode}); err == nil ||
		e.contar(`SELECT count(*) FROM usuarios WHERE id=$1`, reg2.UserId) != 1 {
		t.Fatalf("no debe cancelarse un registro completo: %v", err)
	}
}

func TestGoogle_VinculacionCuentaInstitucional(t *testing.T) {
	e := nuevoEntorno(t)
	dni, correo := e.dniNuevo(), e.unico("inst")+"@colegio.example"
	inst := e.crearUsuario(dni, correo, "ClaveInstitucional1", "SOLICITANTE", true)
	var hashAntes string
	_ = e.admin.QueryRow(e.ctx, `SELECT password_hash FROM usuarios WHERE id=$1`, inst).Scan(&hashAntes)
	iniciar := func(usuario, rol, clave string) (*auth.GoogleStartLinkResponse, error) {
		r, err := e.conSesion(usuario, rol, "/auth.Auth/GoogleStartLink", func(ctx context.Context) (any, error) {
			return NewGoogleStartLinkLogic(ctx, e.sc).GoogleStartLink(&auth.GoogleStartLinkRequest{Password: clave})
		})
		if err != nil {
			return nil, err
		}
		return r.(*auth.GoogleStartLinkResponse), nil
	}
	// Sin sesión no se puede iniciar.
	if _, err := NewGoogleStartLinkLogic(e.ctx, e.sc).GoogleStartLink(&auth.GoogleStartLinkRequest{Password: "x"}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("sin sesión: %v", err)
	}
	// Reautenticación: clave incorrecta -> 400 y cuenta el intento fallido.
	if _, err := iniciar(inst, "SOLICITANTE", "otra-clave"); clave(err) != "clave_incorrecta" || status.Code(err) != codes.InvalidArgument {
		t.Fatalf("clave incorrecta: %v", err)
	}
	if e.contar(`SELECT failed_login_attempts FROM usuarios WHERE id=$1`, inst) != 1 {
		t.Fatal("el intento fallido debe contarse")
	}
	inicio, err := iniciar(inst, "SOLICITANTE", "ClaveInstitucional1")
	if err != nil || inicio.LinkCode == "" {
		t.Fatalf("iniciar vinculación: %v", err)
	}
	sub := e.unico("sub-")
	cb, err := e.callback(sub, e.unico("personal")+"@gmail.example", true, inicio.LinkCode)
	if err != nil || cb.Result != ResultadoVinculado {
		t.Fatalf("vincular: %+v %v", cb, err)
	}
	// Nada de la cuenta cambió; mismo UUID y mismo rol.
	if e.contar(`SELECT count(*) FROM usuario_roles ur JOIN roles r ON r.id = ur.rol_id WHERE ur.usuario_id=$1 AND r.nombre='SOLICITANTE'`, inst) != 1 ||
		e.contar(`SELECT count(*) FROM usuario_roles WHERE usuario_id=$1`, inst) != 1 {
		t.Fatal("la vinculación no debe cambiar el rol")
	}
	if e.contar(`SELECT count(*) FROM user_auth_providers WHERE usuario_id=$1 AND registro_completo`, inst) != 1 {
		t.Fatal("la identidad vinculada a una cuenta institucional queda como registro completo")
	}
	var hashDespues, correoDespues, dniDespues string
	_ = e.admin.QueryRow(e.ctx, `SELECT password_hash, email::text, dni FROM usuarios WHERE id=$1`, inst).Scan(&hashDespues, &correoDespues, &dniDespues)
	if hashDespues != hashAntes || correoDespues != correo || dniDespues != dni {
		t.Fatal("la vinculación no debe cambiar clave, correo ni DNI")
	}
	// El código de vinculación es de un solo uso.
	if _, err := e.callback(e.unico("sub-"), e.unico("z")+"@gmail.example", true, inicio.LinkCode); clave(err) != "codigo_invalido" {
		t.Fatalf("código de vinculación reutilizado: %v", err)
	}
	estado, _ := e.conSesion(inst, "SOLICITANTE", "/auth.Auth/GoogleLinkStatus", func(ctx context.Context) (any, error) {
		return NewGoogleLinkStatusLogic(ctx, e.sc).GoogleLinkStatus(&auth.GoogleLinkStatusRequest{})
	})
	if !estado.(*auth.GoogleLinkStatusResponse).Linked {
		t.Fatal("estado de vinculación")
	}
	// Ahora entra con Google a SU cuenta (mismo UUID).
	cbLogin, _ := e.callback(sub, "cualquiera@gmail.example", true, "")
	ses, err := NewGoogleExchangeLogic(e.ctx, e.sc).GoogleExchange(&auth.GoogleExchangeRequest{LoginCode: cbLogin.LoginCode})
	if err != nil || ses.Session.UserId != inst {
		t.Fatalf("login tras vincular: %v", err)
	}
	if _, err := iniciar(inst, "SOLICITANTE", "ClaveInstitucional1"); clave(err) != "cuenta_ya_vinculada" {
		t.Fatalf("segunda vinculación: %v", err)
	}
	// El login tradicional sigue funcionando para la misma cuenta.
	if r, err := NewLoginLogic(e.ctx, e.sc).Login(&auth.LoginRequest{Dni: dni, Password: "ClaveInstitucional1"}); err != nil || r.UserId != inst {
		t.Fatalf("login tradicional: %v", err)
	}

	// Un Google ya vinculado a otra cuenta no se vincula de nuevo.
	otra := e.crearUsuario(e.dniNuevo(), e.unico("otra")+"@colegio.example", "ClaveInstitucional2", "SOLICITANTE", true)
	inicio2, err := iniciar(otra, "SOLICITANTE", "ClaveInstitucional2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.callback(sub, "x@gmail.example", true, inicio2.LinkCode); clave(err) != "ya_vinculado" {
		t.Fatalf("Google vinculado a otra cuenta: %v", err)
	}
}

func TestGoogle_CuentasAdministrativasInactivasYBloqueadas(t *testing.T) {
	e := nuevoEntorno(t)
	admin := e.crearUsuario(e.dniNuevo(), e.unico("adm")+"@colegio.example", "ClaveAdmin123", "ADMIN", true)
	// ADMIN no puede vincular Google.
	_, err := e.conSesion(admin, "ADMIN", "/auth.Auth/GoogleStartLink", func(ctx context.Context) (any, error) {
		return NewGoogleStartLinkLogic(ctx, e.sc).GoogleStartLink(&auth.GoogleStartLinkRequest{Password: "ClaveAdmin123"})
	})
	if status.Code(err) != codes.PermissionDenied || clave(err) != "cuenta_no_disponible" {
		t.Fatalf("vinculación de ADMIN: %v", err)
	}
	// PSICOLOGO tampoco: Google es solo para SOLICITANTES.
	psico := e.crearUsuario(e.dniNuevo(), e.unico("psi")+"@colegio.example", "ClavePsico123", "PSICOLOGO", true)
	if _, err := e.conSesion(psico, "PSICOLOGO", "/auth.Auth/GoogleStartLink", func(ctx context.Context) (any, error) {
		return NewGoogleStartLinkLogic(ctx, e.sc).GoogleStartLink(&auth.GoogleStartLinkRequest{Password: "ClavePsico123"})
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("vinculación de PSICOLOGO: %v", err)
	}
	subPsico := e.unico("sub-")
	_, _ = e.admin.Exec(e.ctx, `INSERT INTO user_auth_providers (usuario_id, provider, provider_user_id, provider_email) VALUES ($1,'google',$2,'p@x.example')`, psico, subPsico)
	if _, err := e.callback(subPsico, "p@x.example", true, ""); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("login Google de PSICOLOGO: %v", err)
	}
	// Aunque existiera una identidad Google en una cuenta ADMIN, Google no la autentica.
	subAdmin := e.unico("sub-")
	_, _ = e.admin.Exec(e.ctx, `INSERT INTO user_auth_providers (usuario_id, provider, provider_user_id, provider_email) VALUES ($1,'google',$2,'a@x.example')`, admin, subAdmin)
	if _, err := e.callback(subAdmin, "a@x.example", true, ""); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("login Google de ADMIN: %v", err)
	}
	// Cuenta inactiva: rechazada y sin reactivarse.
	inactivo := e.crearUsuario(e.dniNuevo(), e.unico("ina")+"@colegio.example", "Clave1234", "SOLICITANTE", false)
	subIna := e.unico("sub-")
	_, _ = e.admin.Exec(e.ctx, `INSERT INTO user_auth_providers (usuario_id, provider, provider_user_id, provider_email) VALUES ($1,'google',$2,'i@x.example')`, inactivo, subIna)
	if _, err := e.callback(subIna, "i@x.example", true, ""); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("cuenta inactiva: %v", err)
	}
	if e.contar(`SELECT count(*) FROM usuarios WHERE id=$1 AND estado`, inactivo) != 0 {
		t.Fatal("Google no reactiva cuentas")
	}
	// Cuenta bloqueada temporalmente: se respeta el bloqueo.
	bloqueado := e.crearUsuario(e.dniNuevo(), e.unico("blo")+"@colegio.example", "Clave1234", "SOLICITANTE", true)
	_, _ = e.admin.Exec(e.ctx, `UPDATE usuarios SET locked_until = now() + interval '10 minutes' WHERE id=$1`, bloqueado)
	subBlo := e.unico("sub-")
	_, _ = e.admin.Exec(e.ctx, `INSERT INTO user_auth_providers (usuario_id, provider, provider_user_id, provider_email) VALUES ($1,'google',$2,'b@x.example')`, bloqueado, subBlo)
	if _, err := e.callback(subBlo, "b@x.example", true, ""); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("cuenta bloqueada: %v", err)
	}
}
