package logic

import (
	"context"
	"testing"
	"time"

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

const secretoPerfil = "secreto-de-prueba-para-jwt-0123456789abcdef"

// llamarComo ejecuta CreateMyProfile a través del interceptor REAL con un JWT
// firmado (el rol sale del token, no de la solicitud).
func llamarComo(t *testing.T, rol string, in *usuarios.CreateMyProfileRequest) error {
	t.Helper()
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "00000000-0000-4000-8000-000000000001", "role": rol, "username": "u", "jti": "j",
		"iat": time.Now().Unix(), "exp": time.Now().Add(time.Minute).Unix(),
	}).SignedString([]byte(secretoPerfil))
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))
	icpt := interceptor.AuthenticationInterceptor(security.NewJWTValidator(secretoPerfil))
	_, err := icpt(ctx, in, &grpc.UnaryServerInfo{FullMethod: "/usuarios.Usuarios/CreateMyProfile"}, func(ctx context.Context, req any) (any, error) {
		return NewCreateMyProfileLogic(ctx, &svc.ServiceContext{}).CreateMyProfile(req.(*usuarios.CreateMyProfileRequest))
	})
	return err
}

func TestCreateMyProfile_SoloSolicitanteYValidaciones(t *testing.T) {
	valido := &usuarios.CreateMyProfileRequest{Nombres: "Ana", Apellidos: "Quispe", Dni: "12345678"}
	// Un rol administrativo no puede crear perfiles por esta vía (ni el suyo).
	for _, rol := range []string{"ADMIN", "SECRETARIA", "DIRECTOR"} {
		if err := llamarComo(t, rol, valido); status.Code(err) != codes.PermissionDenied {
			t.Fatalf("%s: %v", rol, err)
		}
	}
	for _, in := range []*usuarios.CreateMyProfileRequest{
		{Nombres: "", Apellidos: "Quispe", Dni: "12345678"},
		{Nombres: "Ana", Apellidos: "", Dni: "12345678"},
		{Nombres: "Ana", Apellidos: "Quispe", Dni: "1234567"},
		{Nombres: "Ana", Apellidos: "Quispe", Dni: "12345678", Telefono: "123456789012345678901"},
		{Nombres: "Ana", Apellidos: "Quispe", Dni: "12345678", Correo: "no-es-correo"},
	} {
		if err := llamarComo(t, "SOLICITANTE", in); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("validación %+v: %v", in, err)
		}
	}
}
