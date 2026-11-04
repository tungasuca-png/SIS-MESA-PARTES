package logic

import (
	"context"
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

type authRegistroFalso struct {
	authclient.Auth
	registro *authclient.EmailRegisterRequest
	// verificaciones: llamadas a EmailVerify (crean la cuenta en Auth).
	verificaciones int
	verifyErr      error
	completos      int
	cancelados     int
}

func (a *authRegistroFalso) EmailRegister(_ context.Context, in *authclient.EmailRegisterRequest, _ ...grpc.CallOption) (*authclient.EmailRegisterResponse, error) {
	a.registro = in
	return &authclient.EmailRegisterResponse{RegistrationId: "r1", ExpiresIn: 600, ResendIn: 60}, nil
}

func (a *authRegistroFalso) EmailVerify(_ context.Context, in *authclient.EmailVerifyRequest, _ ...grpc.CallOption) (*authclient.EmailVerifyResponse, error) {
	a.verificaciones++
	if a.verifyErr != nil {
		return nil, a.verifyErr
	}
	return &authclient.EmailVerifyResponse{UserId: "u1", Dni: "12345678", Email: "a@b.example", Nombres: "Ana",
		Apellidos: "Quispe Huamán", ProfileToken: "token-perfil"}, nil
}

func (a *authRegistroFalso) EmailCompleteRegistration(_ context.Context, _ *authclient.EmailRegistrationRequest, _ ...grpc.CallOption) (*authclient.EmailCompleteResponse, error) {
	a.completos++
	return &authclient.EmailCompleteResponse{Dni: "12345678"}, nil
}

func (a *authRegistroFalso) EmailCancelRegistration(_ context.Context, _ *authclient.EmailRegistrationRequest, _ ...grpc.CallOption) (*authclient.EmailCancelResponse, error) {
	a.cancelados++
	return &authclient.EmailCancelResponse{Cancelled: true}, nil
}

type usuariosRegistroFalso struct {
	usuariosclient.Usuarios
	// listo: respuesta de PublicRegistrationReady (nil = preparado);
	// listoErr: Usuarios no responde.
	listo     *bool
	listoErr  error
	crearErr  error
	getErr    error
	pedido    *usuariosclient.CreateMyProfileRequest
	tokenUso  string
	consultas int
}

func (u *usuariosRegistroFalso) CreateMyProfile(ctx context.Context, in *usuariosclient.CreateMyProfileRequest, _ ...grpc.CallOption) (*usuariosclient.CreateMyProfileResponse, error) {
	u.pedido = in
	if md, ok := metadata.FromOutgoingContext(ctx); ok && len(md.Get("authorization")) > 0 {
		u.tokenUso = md.Get("authorization")[0]
	}
	if u.crearErr != nil {
		return nil, u.crearErr
	}
	return &usuariosclient.CreateMyProfileResponse{Created: true}, nil
}

func (u *usuariosRegistroFalso) GetUsuario(_ context.Context, _ *usuariosclient.GetUsuarioRequest, _ ...grpc.CallOption) (*usuariosclient.GetUsuarioResponse, error) {
	u.consultas++
	if u.getErr != nil {
		return nil, u.getErr
	}
	return &usuariosclient.GetUsuarioResponse{}, nil
}

func contextoRegistro(a *authRegistroFalso, u *usuariosRegistroFalso) *svc.ServiceContext {
	return &svc.ServiceContext{Config: config.Config{}, AuthClient: a, UsuariosClient: u}
}

func TestRegistroCorreo_ReenviaFormularioSinRol(t *testing.T) {
	a := &authRegistroFalso{}
	r, err := NewRegistroCorreoLogic(context.Background(), contextoRegistro(a, &usuariosRegistroFalso{})).RegistroCorreo(&types.RegistroCorreoRequest{
		Nombres: "Ana", ApellidoPaterno: "Quispe", Dni: "12345678", Correo: "a@b.example", Clave: "ClaveSegura1",
		AceptaTerminos: true, VersionTerminos: "v1",
	})
	if err != nil || r.RegistroId != "r1" || r.ExpiraEn != 600 || r.ReenviarEn != 60 || r.Mensaje != MensajeCodigoEnviado {
		t.Fatalf("registro: %+v %v", r, err)
	}
	if a.registro.Email != "a@b.example" || !a.registro.AcceptTerms || a.registro.TermsVersion != "v1" {
		t.Fatal("datos reenviados a Auth")
	}
}

func TestRegistroVerificar_Orquestacion(t *testing.T) {
	ctx := context.Background()
	req := &types.RegistroVerificarRequest{RegistroId: "r1", Codigo: "123456"}

	// Éxito: perfil con los datos de Auth y el token interno; luego completa.
	a, u := &authRegistroFalso{}, &usuariosRegistroFalso{}
	r, err := NewRegistroVerificarLogic(ctx, contextoRegistro(a, u)).RegistroVerificar(req)
	if err != nil || r.Correo != "a@b.example" || r.Mensaje != MensajeCuentaActivada || a.completos != 1 || a.cancelados != 0 {
		t.Fatalf("éxito: %+v %v", r, err)
	}
	// El DNI va como DECLARADO: nunca se reserva el DNI definitivo del perfil.
	if u.tokenUso != "Bearer token-perfil" || u.pedido.Apellidos != "Quispe Huamán" || u.pedido.Correo != "a@b.example" ||
		u.pedido.DniDeclarado != "12345678" || u.pedido.Dni != "" {
		t.Fatalf("perfil: %+v %s", u.pedido, u.tokenUso)
	}

	// Código incorrecto: no se toca Usuarios.
	a, u = &authRegistroFalso{verifyErr: status.Error(codes.InvalidArgument, "registro_codigo_incorrecto: x")}, &usuariosRegistroFalso{}
	if _, err := NewRegistroVerificarLogic(ctx, contextoRegistro(a, u)).RegistroVerificar(req); status.Code(err) != codes.InvalidArgument || u.pedido != nil {
		t.Fatalf("código incorrecto: %v", err)
	}

	// DNI de OTRO perfil: compensa.
	a, u = &authRegistroFalso{}, &usuariosRegistroFalso{crearErr: status.Error(codes.AlreadyExists, "dni"), getErr: status.Error(codes.NotFound, "no")}
	_, err = NewRegistroVerificarLogic(ctx, contextoRegistro(a, u)).RegistroVerificar(req)
	if status.Code(err) != codes.AlreadyExists || !strings.HasPrefix(status.Convert(err).Message(), "registro_dni_registrado") || a.cancelados != 1 || a.completos != 0 {
		t.Fatalf("DNI de otro perfil: %v (cancelados %d)", err, a.cancelados)
	}

	// El perfil ya es de ESTE usuario (verificaciones simultáneas): NO compensa.
	a, u = &authRegistroFalso{}, &usuariosRegistroFalso{crearErr: status.Error(codes.AlreadyExists, "dni")}
	if _, err := NewRegistroVerificarLogic(ctx, contextoRegistro(a, u)).RegistroVerificar(req); err != nil || a.cancelados != 0 || a.completos != 1 {
		t.Fatalf("perfil propio: %v (cancelados %d)", err, a.cancelados)
	}

	// No se pudo comprobar el perfil: queda pendiente, sin compensar.
	a, u = &authRegistroFalso{}, &usuariosRegistroFalso{crearErr: status.Error(codes.AlreadyExists, "dni"), getErr: status.Error(codes.Unavailable, "caído")}
	if _, err := NewRegistroVerificarLogic(ctx, contextoRegistro(a, u)).RegistroVerificar(req); status.Code(err) != codes.Unavailable || a.cancelados != 0 || a.completos != 0 {
		t.Fatalf("comprobación fallida: %v", err)
	}

	// Usuarios caído: queda pendiente (el mismo código vuelve a servir).
	a, u = &authRegistroFalso{}, &usuariosRegistroFalso{crearErr: status.Error(codes.Unavailable, "caído")}
	if _, err := NewRegistroVerificarLogic(ctx, contextoRegistro(a, u)).RegistroVerificar(req); status.Code(err) != codes.Unavailable || a.cancelados != 0 || a.completos != 0 {
		t.Fatalf("Usuarios caído: %v", err)
	}

	// Datos rechazados por Usuarios: compensa.
	a, u = &authRegistroFalso{}, &usuariosRegistroFalso{crearErr: status.Error(codes.InvalidArgument, "nombres inválidos")}
	if _, err := NewRegistroVerificarLogic(ctx, contextoRegistro(a, u)).RegistroVerificar(req); status.Code(err) != codes.InvalidArgument || a.cancelados != 1 {
		t.Fatalf("datos rechazados: %v", err)
	}
}

func (u *usuariosRegistroFalso) PublicRegistrationReady(context.Context, *usuariosclient.PublicRegistrationReadyRequest, ...grpc.CallOption) (*usuariosclient.PublicRegistrationReadyResponse, error) {
	if u.listoErr != nil {
		return nil, u.listoErr
	}
	return &usuariosclient.PublicRegistrationReadyResponse{Ready: u.listo == nil || *u.listo}, nil
}

// Usuarios sin preparar (sin la migración 003 o sin responder): el Gateway
// responde 503 ANTES de que Auth guarde el registro, envíe el código o cree
// la cuenta (registro manual y Google).
func TestRegistroPublico_UsuariosSinPrepararNoCreaNada(t *testing.T) {
	ctx := context.Background()
	no := false
	casos := map[string]*usuariosRegistroFalso{
		"Usuarios sin la migración 003": {listo: &no},
		"Usuarios no disponible":        {listoErr: status.Error(codes.Unavailable, "caído")},
	}
	for nombre, u := range casos {
		a := &authRegistroFalso{}
		sc := contextoRegistro(a, u)
		_, err := NewRegistroCorreoLogic(ctx, sc).RegistroCorreo(&types.RegistroCorreoRequest{
			Nombres: "Ana", ApellidoPaterno: "Quispe", Dni: "12345678", Correo: "a@b.example", Clave: "ClaveSegura1", AceptaTerminos: true, VersionTerminos: "v1",
		})
		if status.Code(err) != codes.Unavailable || a.registro != nil {
			t.Fatalf("%s, inicio del registro: %v (Auth llamado: %v)", nombre, err, a.registro != nil)
		}
		_, err = NewRegistroVerificarLogic(ctx, sc).RegistroVerificar(&types.RegistroVerificarRequest{RegistroId: "r1", Codigo: "123456"})
		if status.Code(err) != codes.Unavailable || a.verificaciones != 0 || u.pedido != nil || a.completos != 0 {
			t.Fatalf("%s, verificación: %v (cuentas creadas: %d)", nombre, err, a.verificaciones)
		}
		if strings.Contains(status.Convert(err).Message(), "dni_declarado") || strings.Contains(status.Convert(err).Message(), "caído") {
			t.Fatalf("%s: la respuesta no debe revelar detalles internos: %v", nombre, err)
		}
	}
	// Preparado: el flujo sigue (una sola cuenta).
	a, u := &authRegistroFalso{}, &usuariosRegistroFalso{}
	if _, err := NewRegistroVerificarLogic(ctx, contextoRegistro(a, u)).RegistroVerificar(&types.RegistroVerificarRequest{RegistroId: "r1", Codigo: "123456"}); err != nil || a.verificaciones != 1 || a.completos != 1 {
		t.Fatalf("preparado: %v", err)
	}
}

func TestGoogleRegistro_UsuariosSinPrepararNoCreaNada(t *testing.T) {
	a, u := &authGoogleFalso{}, &usuariosPerfilFalso{noListo: true}
	_, err := NewGoogleRegistroLogic(context.Background(), contextoGoogle(a, u)).GoogleRegistro(&types.GoogleRegistroRequest{
		Codigo: "c", Dni: "12345678", Nombres: "Ana", ApellidoPaterno: "Quispe",
	})
	if status.Code(err) != codes.Unavailable || a.registros != 0 || u.pedido != nil || a.completados != 0 {
		t.Fatalf("Google con Usuarios sin preparar: %v (cuentas creadas en Auth: %d)", err, a.registros)
	}
}
