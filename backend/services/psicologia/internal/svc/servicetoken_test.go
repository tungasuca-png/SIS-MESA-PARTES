package svc

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"auth/authclient"
	"psicologia/internal/interceptor"
	"psicologia/internal/security"

	"estudiantes/estudiantesclient"

	"github.com/golang-jwt/jwt/v5"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Pruebas sin red ni base de datos: Auth y Estudiantes se reemplazan por
// dobles. Credenciales y tokens ficticios.

const (
	clientIDPrueba     = "cli_0123456789abcdef0123456789abcdef"
	clientSecretPrueba = "cs_secreto-ficticio-solo-para-pruebas-0123456789"
	claveUsuarios      = "clave-de-usuarios-solo-para-pruebas-psico"
	usuarioPrueba      = "3f2b1c4d-0000-4000-8000-000000000001"
	estudianteExiste   = "11111111-1111-4111-8111-111111111111"
	estudianteNoExiste = "22222222-2222-4222-8222-222222222222"
)

// authFalso emite tokens "svc-1", "svc-2"… y registra la metadata saliente.
type authFalso struct {
	mu       sync.Mutex
	emitidos int
	err      error
	vida     int64
	mdVista  []metadata.MD
	pedidos  []*authclient.IssueServiceTokenRequest
	demora   time.Duration
}

func (a *authFalso) IssueServiceToken(ctx context.Context, in *authclient.IssueServiceTokenRequest, _ ...grpc.CallOption) (*authclient.IssueServiceTokenResponse, error) {
	if a.demora > 0 {
		time.Sleep(a.demora)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	md, _ := metadata.FromOutgoingContext(ctx)
	a.mdVista = append(a.mdVista, md)
	a.pedidos = append(a.pedidos, in)
	if a.err != nil {
		return nil, a.err
	}
	a.emitidos++
	vida := a.vida
	if vida == 0 {
		vida = 300
	}
	return &authclient.IssueServiceTokenResponse{AccessToken: "svc-" + string(rune('0'+a.emitidos)), ExpiresIn: vida, Scope: "estudiantes.basic.view"}, nil
}

func (a *authFalso) cuenta() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.pedidos)
}

// estudiantesFalso implementa solo las dos operaciones usadas; cualquier
// otra haría panic (no debe llamarse).
type estudiantesFalso struct {
	estudiantesclient.Estudiantes
	mu            sync.Mutex
	rechazar      map[string]bool // tokens que responden Unauthenticated
	rechazarTodo  bool
	err           error
	autorizacion  [][]string
	llamadasBasic int
}

func (e *estudiantesFalso) GetEstudiantesBasic(ctx context.Context, in *estudiantesclient.GetEstudiantesBasicRequest, _ ...grpc.CallOption) (*estudiantesclient.GetEstudiantesBasicResponse, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.llamadasBasic++
	md, _ := metadata.FromOutgoingContext(ctx)
	vals := md.Get("authorization")
	e.autorizacion = append(e.autorizacion, vals)
	token := ""
	if len(vals) == 1 {
		token = strings.TrimPrefix(vals[0], "Bearer ")
	}
	if e.rechazarTodo || e.rechazar[token] {
		return nil, status.Error(codes.Unauthenticated, "token de acceso inválido")
	}
	if e.err != nil {
		return nil, e.err
	}
	out := []*estudiantesclient.EstudianteBasico{}
	for _, id := range in.GetIds() {
		if id == estudianteExiste {
			out = append(out, &estudiantesclient.EstudianteBasico{Id: id, Nombres: "Ana María", Apellidos: "Quispe Huamán"})
		}
	}
	return &estudiantesclient.GetEstudiantesBasicResponse{Estudiantes: out}, nil
}

func (e *estudiantesFalso) GetEstudiante(context.Context, *estudiantesclient.GetEstudianteRequest, ...grpc.CallOption) (*estudiantesclient.GetEstudianteResponse, error) {
	panic("en modo servicio no debe usarse GetEstudiante")
}

type reloj struct{ t time.Time }

func (r *reloj) now() time.Time { return r.t }

func proveedor(a *authFalso, r *reloj) *ServiceTokenProvider {
	p := NewServiceTokenProvider(a, clientIDPrueba, clientSecretPrueba)
	p.now = r.now
	return p
}

// ctxUsuario reproduce una solicitud real de un usuario a Psicología: pasa
// por el interceptor real con un JWT humano y, además, deja ese JWT ya
// agregado como metadata saliente (peor caso).
func ctxUsuario(t *testing.T) (context.Context, string) {
	t.Helper()
	humano, err := jwt.NewWithClaims(jwt.SigningMethodHS256, security.Claims{Username: "psicologa.demo", Role: "PSICOLOGO", RegisteredClaims: jwt.RegisteredClaims{
		Subject: usuarioPrueba, IssuedAt: jwt.NewNumericDate(time.Now()), ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
	}}).SignedString([]byte(claveUsuarios))
	if err != nil {
		t.Fatal(err)
	}
	in := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+humano))
	var capturado context.Context
	_, err = interceptor.AuthenticationInterceptor(security.NewJWTValidator(claveUsuarios))(in, nil, &grpc.UnaryServerInfo{FullMethod: "/psicologia.Psicologia/OpenExpediente"},
		func(ctx context.Context, _ any) (any, error) { capturado = ctx; return nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	if tok, ok := interceptor.TokenFromContext(capturado); !ok || tok != humano {
		t.Fatal("el contexto de prueba debe llevar el JWT humano")
	}
	return metadata.AppendToOutgoingContext(capturado, "authorization", "Bearer "+humano), humano
}

func TestToken_ObtencionYReutilizacion(t *testing.T) {
	a, r := &authFalso{}, &reloj{t: time.Now()}
	p := proveedor(a, r)
	ctx, humano := ctxUsuario(t)
	t1, err := p.Token(ctx)
	if err != nil || t1 != "svc-1" {
		t.Fatalf("obtención: %q %v", t1, err)
	}
	if a.pedidos[0].ClientId != clientIDPrueba || a.pedidos[0].ClientSecret != clientSecretPrueba {
		t.Fatal("credenciales no enviadas a Auth")
	}
	if strings.Contains(strings.Join(a.mdVista[0].Get("authorization"), ""), humano) || len(a.mdVista[0]) != 0 {
		t.Fatal("la emisión no debe llevar el JWT del usuario ni otra metadata")
	}
	r.t = r.t.Add(3 * time.Minute)
	t2, _ := p.Token(ctx)
	if t2 != t1 || a.cuenta() != 1 {
		t.Fatalf("debió reutilizarse el token: %q, emisiones=%d", t2, a.cuenta())
	}
}

func TestToken_RenovacionAnticipada(t *testing.T) {
	a, r := &authFalso{vida: 300}, &reloj{t: time.Now()}
	p := proveedor(a, r)
	inicio := r.t
	_, _ = p.Token(context.Background())
	r.t = inicio.Add(300*time.Second - 61*time.Second)
	if tok, _ := p.Token(context.Background()); tok != "svc-1" {
		t.Fatal("a 61 s del vencimiento aún debe reutilizarse")
	}
	r.t = inicio.Add(300*time.Second - 59*time.Second)
	if tok, _ := p.Token(context.Background()); tok != "svc-2" || a.cuenta() != 2 {
		t.Fatalf("a 59 s del vencimiento debe renovarse: %q", tok)
	}
}

func TestToken_ConcurrenciaUnaSolaEmision(t *testing.T) {
	a, r := &authFalso{demora: 50 * time.Millisecond}, &reloj{t: time.Now()}
	p := proveedor(a, r)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = p.Token(context.Background()) }()
	}
	wg.Wait()
	if a.cuenta() != 1 {
		t.Fatalf("20 llamadas concurrentes deben producir 1 emisión, hubo %d", a.cuenta())
	}
}

func TestToken_FalloDeAuth(t *testing.T) {
	a := &authFalso{err: status.Error(codes.Unauthenticated, "credenciales de cliente inválidas")}
	est := &estudiantesFalso{}
	v := NewEstudianteVerifier(est, proveedor(a, &reloj{t: time.Now()}))
	existe, err := v.Exists(context.Background(), estudianteExiste)
	if status.Code(err) != codes.Unavailable || existe {
		t.Fatalf("un fallo de Auth debe ser Unavailable, nunca 'no existe': %v %v", existe, err)
	}
	if est.llamadasBasic != 0 {
		t.Fatal("sin token no debe consultarse Estudiantes")
	}
}

func TestVerificador_ExisteYNoExiste(t *testing.T) {
	est := &estudiantesFalso{}
	v := NewEstudianteVerifier(est, proveedor(&authFalso{}, &reloj{t: time.Now()}))
	ctx, _ := ctxUsuario(t)
	if ok, err := v.Exists(ctx, estudianteExiste); err != nil || !ok {
		t.Fatalf("estudiante existente: %v %v", ok, err)
	}
	if ok, err := v.Exists(ctx, estudianteNoExiste); err != nil || ok {
		t.Fatalf("estudiante inexistente debe ser (false, nil): %v %v", ok, err)
	}
	nombre, err := v.NombreCompleto(ctx, estudianteExiste)
	if err != nil || nombre != "Ana María Quispe Huamán" {
		t.Fatalf("nombre: %q %v", nombre, err)
	}
	if _, err := v.NombreCompleto(ctx, estudianteNoExiste); status.Code(err) != codes.NotFound {
		t.Fatalf("NombreCompleto de inexistente debe ser NotFound: %v", err)
	}
}

func TestVerificador_MetadataSoloTokenDeServicio(t *testing.T) {
	est := &estudiantesFalso{}
	v := NewEstudianteVerifier(est, proveedor(&authFalso{}, &reloj{t: time.Now()}))
	ctx, humano := ctxUsuario(t)
	_, _ = v.Exists(ctx, estudianteExiste)
	_, _ = v.NombreCompleto(ctx, estudianteExiste)
	for _, vals := range est.autorizacion {
		if len(vals) != 1 || vals[0] != "Bearer svc-1" {
			t.Fatalf("authorization debe ser exactamente 'Bearer <token de servicio>': %v", vals)
		}
		if strings.Contains(strings.Join(vals, ""), humano) {
			t.Fatal("el JWT humano llegó a Estudiantes")
		}
	}
}

func TestVerificador_RenovacionTrasUnauthenticatedYUnSoloReintento(t *testing.T) {
	a := &authFalso{}
	est := &estudiantesFalso{rechazar: map[string]bool{"svc-1": true}}
	v := NewEstudianteVerifier(est, proveedor(a, &reloj{t: time.Now()}))
	if ok, err := v.Exists(context.Background(), estudianteExiste); err != nil || !ok {
		t.Fatalf("tras renovar debió funcionar: %v %v", ok, err)
	}
	if a.cuenta() != 2 || est.llamadasBasic != 2 {
		t.Fatalf("se esperaba 1 renovación y 1 reintento: emisiones=%d llamadas=%d", a.cuenta(), est.llamadasBasic)
	}

	// Rechazo persistente: un solo reintento, nunca 'no existe'.
	a2 := &authFalso{}
	est2 := &estudiantesFalso{rechazarTodo: true}
	v2 := NewEstudianteVerifier(est2, proveedor(a2, &reloj{t: time.Now()}))
	ok, err := v2.Exists(context.Background(), estudianteExiste)
	if ok || status.Code(err) != codes.Unavailable {
		t.Fatalf("rechazo persistente debe ser Unavailable: %v %v", ok, err)
	}
	if est2.llamadasBasic != 2 || a2.cuenta() != 2 {
		t.Fatalf("debe haber exactamente un reintento: llamadas=%d emisiones=%d", est2.llamadasBasic, a2.cuenta())
	}
}

func TestVerificador_ErroresDeEstudiantes(t *testing.T) {
	casos := map[string]struct {
		err      error
		esperado codes.Code
	}{
		"servicio caído":     {status.Error(codes.Unavailable, "sin conexión"), codes.Unavailable},
		"error interno":      {status.Error(codes.Internal, "falla"), codes.Internal},
		"scope insuficiente": {status.Error(codes.PermissionDenied, "permission_denied"), codes.Unavailable},
		"id inválido":        {status.Error(codes.InvalidArgument, "uuid"), codes.InvalidArgument},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			v := NewEstudianteVerifier(&estudiantesFalso{err: c.err}, proveedor(&authFalso{}, &reloj{t: time.Now()}))
			ok, err := v.Exists(context.Background(), estudianteExiste)
			if ok || status.Code(err) != c.esperado {
				t.Fatalf("se esperaba %v sin 'no existe': %v %v", c.esperado, ok, err)
			}
			if _, err := v.NombreCompleto(context.Background(), estudianteExiste); status.Code(err) != c.esperado {
				t.Fatalf("NombreCompleto: se esperaba %v: %v", c.esperado, err)
			}
		})
	}
}

func TestVerificador_LogsSinSecretosNiTokens(t *testing.T) {
	var buf bytes.Buffer
	logx.SetWriter(logx.NewWriter(&buf))
	defer logx.Reset()

	ctx, humano := ctxUsuario(t)
	// Auth rechaza, Estudiantes rechaza siempre y Estudiantes falla: todos
	// los caminos que registran algo.
	_, _ = NewEstudianteVerifier(&estudiantesFalso{}, proveedor(&authFalso{err: status.Error(codes.Unauthenticated, "x")}, &reloj{t: time.Now()})).Exists(ctx, estudianteExiste)
	_, _ = NewEstudianteVerifier(&estudiantesFalso{rechazarTodo: true}, proveedor(&authFalso{}, &reloj{t: time.Now()})).Exists(ctx, estudianteExiste)
	_, _ = NewEstudianteVerifier(&estudiantesFalso{err: status.Error(codes.PermissionDenied, "x")}, proveedor(&authFalso{}, &reloj{t: time.Now()})).Exists(ctx, estudianteExiste)

	salida := buf.String()
	if !strings.Contains(salida, "credencial de servicio") {
		t.Fatalf("se esperaban registros de los fallos: %q", salida)
	}
	for _, sensible := range []string{clientSecretPrueba, humano, "svc-1", "svc-2", "Bearer"} {
		if strings.Contains(salida, sensible) {
			t.Fatalf("el log contiene un secreto o token (%q)", sensible)
		}
	}
}

// Si la renovación ANTICIPADA falla (Auth caído) y el token vigente aún no
// venció, se sigue usando ese token; recién al vencer se propaga el error.
func TestToken_RenovacionFallidaConservaTokenVigente(t *testing.T) {
	a, r := &authFalso{vida: 300}, &reloj{t: time.Now()}
	p := proveedor(a, r)
	inicio := r.t
	if tok, err := p.Token(context.Background()); err != nil || tok != "svc-1" {
		t.Fatalf("emisión inicial: %q %v", tok, err)
	}
	a.err = status.Error(codes.Unavailable, "auth caído")
	r.t = inicio.Add(300*time.Second - 40*time.Second) // dentro del margen, aún vigente
	if tok, err := p.Token(context.Background()); err != nil || tok != "svc-1" {
		t.Fatalf("con el token aún vigente no debe fallar: %q %v", tok, err)
	}
	r.t = inicio.Add(300*time.Second + time.Second) // vencido
	if _, err := p.Token(context.Background()); status.Code(err) != codes.Unavailable {
		t.Fatalf("vencido y sin Auth debe ser Unavailable: %v", err)
	}
	// Un token rechazado por Estudiantes (Invalidate) nunca se reutiliza,
	// aunque Auth esté caído.
	a.err = nil
	r.t = inicio.Add(400 * time.Second)
	tok, _ := p.Token(context.Background())
	p.Invalidate(tok)
	a.err = status.Error(codes.Unavailable, "auth caído")
	if _, err := p.Token(context.Background()); status.Code(err) != codes.Unavailable {
		t.Fatalf("un token invalidado no debe reutilizarse: %v", err)
	}
}
