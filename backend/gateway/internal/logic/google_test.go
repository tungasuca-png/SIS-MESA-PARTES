package logic

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"gateway/internal/config"
	"gateway/internal/svc"
	"gateway/internal/types"

	"auth/authclient"
	"usuarios/usuariosclient"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Fakes: solo los métodos usados; el resto queda en la interfaz embebida.
type authGoogleFalso struct {
	authclient.Auth
	callback    *authclient.GoogleCallbackRequest
	callbackErr error
	resultado   string
	registroErr error
	cancelados  int
	completados int
	vinculoAuth string
}

func (a *authGoogleFalso) GoogleStart(context.Context, *authclient.GoogleStartRequest, ...grpc.CallOption) (*authclient.GoogleStartResponse, error) {
	return &authclient.GoogleStartResponse{Url: "https://accounts.example/auth?state=s1", State: "s1", Nonce: "n1", Verifier: "v1"}, nil
}

func (a *authGoogleFalso) GoogleCallback(_ context.Context, in *authclient.GoogleCallbackRequest, _ ...grpc.CallOption) (*authclient.GoogleCallbackResponse, error) {
	a.callback = in
	if a.callbackErr != nil {
		return nil, a.callbackErr
	}
	return &authclient.GoogleCallbackResponse{Result: a.resultado, LoginCode: "codigo-un-uso"}, nil
}

func (a *authGoogleFalso) GoogleRegister(_ context.Context, in *authclient.GoogleRegisterRequest, _ ...grpc.CallOption) (*authclient.GoogleRegisterResponse, error) {
	if a.registroErr != nil {
		return nil, a.registroErr
	}
	return &authclient.GoogleRegisterResponse{UserId: "u-nuevo", Dni: in.Dni, Email: "verificado@gmail.example", ProfileToken: "token-de-perfil"}, nil
}

func (a *authGoogleFalso) GoogleCompleteRegistration(context.Context, *authclient.GoogleRegistrationRequest, ...grpc.CallOption) (*authclient.GoogleCompleteResponse, error) {
	a.completados++
	return &authclient.GoogleCompleteResponse{Session: &authclient.LoginResponse{AccessToken: "at", RefreshToken: "rt", UserId: "u-nuevo", Role: "SOLICITANTE"}}, nil
}

func (a *authGoogleFalso) GoogleCancelRegistration(context.Context, *authclient.GoogleRegistrationRequest, ...grpc.CallOption) (*authclient.GoogleCancelResponse, error) {
	a.cancelados++
	return &authclient.GoogleCancelResponse{Cancelled: true}, nil
}

func (a *authGoogleFalso) GoogleStartLink(ctx context.Context, in *authclient.GoogleStartLinkRequest, _ ...grpc.CallOption) (*authclient.GoogleStartLinkResponse, error) {
	md, _ := metadata.FromOutgoingContext(ctx)
	if v := md.Get("authorization"); len(v) > 0 {
		a.vinculoAuth = v[0]
	}
	if in.Password != "ClaveCorrecta1" {
		return nil, status.Error(codes.InvalidArgument, "google_clave_incorrecta: La clave no es correcta.")
	}
	return &authclient.GoogleStartLinkResponse{LinkCode: "codigo-vinculo", ExpiresIn: 600}, nil
}

type usuariosPerfilFalso struct {
	usuariosclient.Usuarios
	err      error
	pedido   *usuariosclient.CreateMyProfileRequest
	tokenUso string
}

func (u *usuariosPerfilFalso) CreateMyProfile(ctx context.Context, in *usuariosclient.CreateMyProfileRequest, _ ...grpc.CallOption) (*usuariosclient.CreateMyProfileResponse, error) {
	u.pedido = in
	md, _ := metadata.FromOutgoingContext(ctx)
	if v := md.Get("authorization"); len(v) > 0 {
		u.tokenUso = v[0]
	}
	if u.err != nil {
		return nil, u.err
	}
	return &usuariosclient.CreateMyProfileResponse{Created: true}, nil
}

func contextoGoogle(a *authGoogleFalso, u *usuariosPerfilFalso) *svc.ServiceContext {
	return &svc.ServiceContext{Config: config.Config{}, AuthClient: a, UsuariosClient: u}
}

func TestGoogleFlujo_CookieYCallback(t *testing.T) {
	f := FlujoGoogle{State: "s1", Nonce: "n1", Verifier: "v1"}
	if g, ok := DecodificarFlujo(CodificarFlujo(f)); !ok || g != f {
		t.Fatal("cookie del flujo")
	}
	if _, ok := DecodificarFlujo("no-es-base64-json"); ok {
		t.Fatal("cookie manipulada")
	}
	a := &authGoogleFalso{resultado: "LOGIN"}
	l := NewGoogleCallbackLogic(context.Background(), contextoGoogle(a, nil))
	cookie := CodificarFlujo(f)

	// state correcto: el código de un solo uso va en el fragmento (#), nunca un token.
	d := l.GoogleCallback(&types.GoogleCallbackRequest{Code: "c", State: "s1"}, cookie)
	if d != "/login/google#codigo=codigo-un-uso" || a.callback.Verifier != "v1" || a.callback.Nonce != "n1" {
		t.Fatalf("destino: %s %+v", d, a.callback)
	}
	// state alterado, cookie ausente o manipulada: no se llama a Auth.
	a.callback = nil
	for _, c := range []struct{ state, cookie string }{{"otro", cookie}, {"s1", ""}, {"s1", "basura"}} {
		if d := l.GoogleCallback(&types.GoogleCallbackRequest{Code: "c", State: c.state}, c.cookie); d != "/login/google#error=estado_invalido" || a.callback != nil {
			t.Fatalf("state inválido: %s", d)
		}
	}
	if d := l.GoogleCallback(&types.GoogleCallbackRequest{State: "s1", Error: "access_denied"}, cookie); d != "/login/google#error=cancelado" {
		t.Fatalf("cancelado: %s", d)
	}
	// Error de Auth -> solo la clave, sin mensajes ni datos en la URL.
	a.callbackErr = status.Error(codes.AlreadyExists, "google_correo_existente: Ya existe una cuenta con este correo.")
	if d := l.GoogleCallback(&types.GoogleCallbackRequest{Code: "c", State: "s1"}, cookie); d != "/login/google#error=correo_existente" {
		t.Fatalf("error de Auth: %s", d)
	}
	a.callbackErr = status.Error(codes.ResourceExhausted, "account_locked: ...")
	if d := l.GoogleCallback(&types.GoogleCallbackRequest{Code: "c", State: "s1"}, cookie); d != "/login/google#error=cuenta_bloqueada" {
		t.Fatalf("bloqueo: %s", d)
	}
	// Vinculación: vuelve al perfil y entrega el código de vinculación a Auth.
	a.callbackErr, a.resultado = nil, "VINCULADO"
	vinc := CodificarFlujo(FlujoGoogle{State: "s1", Nonce: "n1", Verifier: "v1", Vinculo: "codigo-vinculo"})
	if d := l.GoogleCallback(&types.GoogleCallbackRequest{Code: "c", State: "s1"}, vinc); d != "/mesa-de-partes/perfil?google=vinculado" || a.callback.LinkCode != "codigo-vinculo" {
		t.Fatalf("vinculación: %s", d)
	}
	if d := l.GoogleCallback(&types.GoogleCallbackRequest{Code: "c", State: "x"}, vinc); d != "/mesa-de-partes/perfil?google_error=estado_invalido" {
		t.Fatalf("vinculación con state inválido: %s", d)
	}
	// Con FRONTEND_URL configurado se usa como origen.
	conFront := &svc.ServiceContext{Config: config.Config{FrontendURL: "https://portal.example/"}, AuthClient: &authGoogleFalso{resultado: "LOGIN"}}
	if d := NewGoogleCallbackLogic(context.Background(), conFront).GoogleCallback(&types.GoogleCallbackRequest{Code: "c", State: "s1"}, cookie); !strings.HasPrefix(d, "https://portal.example/login/google#") {
		t.Fatalf("origen del frontend: %s", d)
	}
}

func TestGoogleRegistro_Orquestacion(t *testing.T) {
	ctx := context.Background()
	base := types.GoogleRegistroRequest{Codigo: "c", Dni: "12345678", Nombres: " Ana  María ", ApellidoPaterno: "Quispe", ApellidoMaterno: "Ñahui", Telefono: "987654321"}

	// Éxito: perfil con el DNI y el correo confirmados por Auth y el token del nuevo usuario.
	a, u := &authGoogleFalso{}, &usuariosPerfilFalso{}
	r, err := NewGoogleRegistroLogic(ctx, contextoGoogle(a, u)).GoogleRegistro(&base)
	if err != nil || r.AccessToken != "at" || r.Role != "SOLICITANTE" || a.completados != 1 {
		t.Fatalf("registro: %+v %v", r, err)
	}
	if u.pedido.Nombres != "Ana María" || u.pedido.Apellidos != "Quispe Ñahui" || u.pedido.Dni != "12345678" ||
		u.pedido.Correo != "verificado@gmail.example" || u.tokenUso != "Bearer token-de-perfil" {
		t.Fatalf("perfil: %+v %s", u.pedido, u.tokenUso)
	}

	// DNI ya registrado en Usuarios: compensación y sin sesión.
	a, u = &authGoogleFalso{}, &usuariosPerfilFalso{err: status.Error(codes.AlreadyExists, "el DNI ya está registrado para otro usuario")}
	_, err = NewGoogleRegistroLogic(ctx, contextoGoogle(a, u)).GoogleRegistro(&base)
	if status.Code(err) != codes.AlreadyExists || !strings.Contains(err.Error(), "google_dni_registrado") || a.cancelados != 1 || a.completados != 0 {
		t.Fatalf("DNI en Usuarios: %v cancelados=%d", err, a.cancelados)
	}

	// Fallo transitorio de Usuarios: la cuenta queda pendiente (sin compensar ni sesión).
	a, u = &authGoogleFalso{}, &usuariosPerfilFalso{err: status.Error(codes.Unavailable, "caído")}
	_, err = NewGoogleRegistroLogic(ctx, contextoGoogle(a, u)).GoogleRegistro(&base)
	if status.Code(err) != codes.Unavailable || a.cancelados != 0 || a.completados != 0 {
		t.Fatalf("Usuarios no disponible: %v", err)
	}

	// DNI ya registrado en Auth: no se llama a Usuarios.
	a, u = &authGoogleFalso{registroErr: status.Error(codes.AlreadyExists, "google_dni_registrado: ...")}, &usuariosPerfilFalso{}
	if _, err := NewGoogleRegistroLogic(ctx, contextoGoogle(a, u)).GoogleRegistro(&base); status.Code(err) != codes.AlreadyExists || u.pedido != nil {
		t.Fatalf("DNI en Auth: %v", err)
	}

	// Validaciones del formulario (también las repiten Auth y Usuarios).
	for _, m := range []func(*types.GoogleRegistroRequest){
		func(r *types.GoogleRegistroRequest) { r.Dni = "1234567" },
		func(r *types.GoogleRegistroRequest) { r.Dni = "1234567a" },
		func(r *types.GoogleRegistroRequest) { r.Nombres = "  " },
		func(r *types.GoogleRegistroRequest) { r.ApellidoPaterno = "" },
		func(r *types.GoogleRegistroRequest) { r.Telefono = strings.Repeat("9", 21) },
		func(r *types.GoogleRegistroRequest) { r.Codigo = "" },
	} {
		req := base
		m(&req)
		a, u = &authGoogleFalso{}, &usuariosPerfilFalso{}
		if _, err := NewGoogleRegistroLogic(ctx, contextoGoogle(a, u)).GoogleRegistro(&req); status.Code(err) != codes.InvalidArgument || u.pedido != nil {
			t.Fatalf("validación %+v: %v", req, err)
		}
	}
}

func TestGoogleVincular_ReenviaSesionYClave(t *testing.T) {
	a := &authGoogleFalso{}
	l := NewGoogleVincularLogic(context.Background(), contextoGoogle(a, nil))
	resp, codigo, err := l.GoogleVincular(&types.GoogleVincularRequest{Authorization: "Bearer jwt.x", Clave: "ClaveCorrecta1"})
	if err != nil || codigo != "codigo-vinculo" || resp.Url != "/api/auth/google/iniciar?modo=vincular" || a.vinculoAuth != "Bearer jwt.x" ||
		strings.Contains(resp.Url, codigo) {
		t.Fatalf("vincular: %+v %s %v", resp, codigo, err)
	}
	if _, _, err := l.GoogleVincular(&types.GoogleVincularRequest{Authorization: "Bearer jwt.x", Clave: "mala"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("clave incorrecta: %v", err)
	}
	if d := DestinoLoginGoogle("", url.Values{"codigo": {"a b"}}); d != "/login/google#codigo=a+b" {
		t.Fatalf("codificación del destino: %s", d)
	}
}
