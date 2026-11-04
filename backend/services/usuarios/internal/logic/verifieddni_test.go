package logic

import (
	"context"
	"testing"
	"time"

	"auth/authclient"
	"usuarios/internal/interceptor"
	"usuarios/internal/security"
	"usuarios/internal/svc"
	"usuarios/usuarios"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// authPermisoFalso: Auth.HasPermission simulado; registra qué se consultó.
type authPermisoFalso struct {
	authclient.Auth
	permitido bool
	consultas []string
}

func (a *authPermisoFalso) HasPermission(_ context.Context, in *authclient.HasPermissionRequest, _ ...grpc.CallOption) (*authclient.HasPermissionResponse, error) {
	a.consultas = append(a.consultas, in.Permission)
	return &authclient.HasPermissionResponse{Allowed: a.permitido}, nil
}

const actorPersonal = "00000000-0000-4000-8000-0000000000aa"

func llamarDNIVerificado(t *testing.T, sub, rol, metodo string, auth *authPermisoFalso, in *usuarios.VerifiedDniRequest) error {
	t.Helper()
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": sub, "role": rol, "username": "u", "jti": "j",
		"iat": time.Now().Unix(), "exp": time.Now().Add(time.Minute).Unix(),
	}).SignedString([]byte(secretoPerfil))
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))
	icpt := interceptor.AuthenticationInterceptor(security.NewJWTValidator(secretoPerfil))
	// Sin repositorio: si la autorización fallara en dejar pasar, la prueba
	// lo notaría (el acceso al repositorio nil haría panic).
	sc := &svc.ServiceContext{AuthClient: auth}
	_, err := icpt(ctx, in, &grpc.UnaryServerInfo{FullMethod: metodo}, func(ctx context.Context, req any) (any, error) {
		if metodo == "/usuarios.Usuarios/AssignVerifiedDni" {
			return NewAssignVerifiedDniLogic(ctx, sc).AssignVerifiedDni(req.(*usuarios.VerifiedDniRequest))
		}
		return NewRevertVerifiedDniLogic(ctx, sc).RevertVerifiedDni(req.(*usuarios.VerifiedDniRequest))
	})
	return err
}

func TestDNIVerificado_ExigePermisoDeVerificacion(t *testing.T) {
	in := &usuarios.VerifiedDniRequest{Id: "00000000-0000-4000-8000-000000000001", Dni: "12345678"}
	for _, metodo := range []string{"/usuarios.Usuarios/AssignVerifiedDni", "/usuarios.Usuarios/RevertVerifiedDni"} {
		// El rol no basta: SECRETARIA, ADMIN o SOLICITANTE sin el permiso real.
		for _, rol := range []string{"SECRETARIA", "ADMIN", "SOLICITANTE"} {
			a := &authPermisoFalso{permitido: false}
			if err := llamarDNIVerificado(t, actorPersonal, rol, metodo, a, in); status.Code(err) != codes.PermissionDenied {
				t.Fatalf("%s como %s sin permiso: %v", metodo, rol, err)
			}
			if len(a.consultas) != 1 || a.consultas[0] != "solicitantes.identidad.verify" {
				t.Fatalf("permiso consultado: %v", a.consultas)
			}
		}
		// Con permiso: nadie verifica su propia identidad.
		a := &authPermisoFalso{permitido: true}
		propio := &usuarios.VerifiedDniRequest{Id: actorPersonal, Dni: "12345678"}
		if err := llamarDNIVerificado(t, actorPersonal, "SECRETARIA", metodo, a, propio); status.Code(err) != codes.PermissionDenied {
			t.Fatalf("%s sobre sí mismo: %v", metodo, err)
		}
		// Validación de entrada.
		for _, malo := range []*usuarios.VerifiedDniRequest{{Id: "no-uuid", Dni: "12345678"}, {Id: in.Id, Dni: "1234"}} {
			if err := llamarDNIVerificado(t, actorPersonal, "SECRETARIA", metodo, a, malo); status.Code(err) != codes.InvalidArgument {
				t.Fatalf("%s entrada inválida %+v: %v", metodo, malo, err)
			}
		}
	}
}

func TestCreateMyProfile_DNIDeclarado(t *testing.T) {
	for _, in := range []*usuarios.CreateMyProfileRequest{
		{Nombres: "Ana", Apellidos: "Quispe", DniDeclarado: "1234"},
		// No se puede enviar a la vez un DNI definitivo y uno declarado.
		{Nombres: "Ana", Apellidos: "Quispe", Dni: "12345678", DniDeclarado: "12345678"},
		{Nombres: "Ana", Apellidos: "Quispe"},
	} {
		if err := llamarComo(t, "SOLICITANTE", in); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("validación %+v: %v", in, err)
		}
	}
}
