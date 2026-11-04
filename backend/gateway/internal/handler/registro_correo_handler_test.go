package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gateway/internal/svc"

	"auth/authclient"
	"usuarios/usuariosclient"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
)

type authRegistroCaptura struct {
	authclient.Auth
	pedido *authclient.EmailRegisterRequest
}

func (a *authRegistroCaptura) EmailRegister(_ context.Context, in *authclient.EmailRegisterRequest, _ ...grpc.CallOption) (*authclient.EmailRegisterResponse, error) {
	a.pedido = in
	return &authclient.EmailRegisterResponse{RegistrationId: "r1", ExpiresIn: 600, ResendIn: 60}, nil
}

type usuariosListo struct{ usuariosclient.Usuarios }

func (usuariosListo) PublicRegistrationReady(context.Context, *usuariosclient.PublicRegistrationReadyRequest, ...grpc.CallOption) (*usuariosclient.PublicRegistrationReadyResponse, error) {
	return &usuariosclient.PublicRegistrationReadyResponse{Ready: true}, nil
}

// Una solicitud manipulada con campos de rol o permisos no puede llevarlos a
// Auth: el contrato no los tiene y Auth fija siempre SOLICITANTE.
func TestRegistroCorreoHandler_IgnoraRolYPermisosManipulados(t *testing.T) {
	a := &authRegistroCaptura{}
	cuerpo := `{"nombres":"Ana","apellido_paterno":"Quispe","dni":"12345678","correo":"a@b.example","clave":"ClaveSegura1",
		"acepta_terminos":true,"version_terminos":"v1",
		"role":"ADMIN","rol":"ADMIN","tipo_usuario":"ADMIN","permissions":["usuarios.change_role"],"user_id":"00000000-0000-4000-8000-000000000001"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/registro", strings.NewReader(cuerpo))
	req.Header.Set("Content-Type", "application/json")
	RegistroCorreoHandler(&svc.ServiceContext{AuthClient: a, UsuariosClient: usuariosListo{}})(rec, req)

	if rec.Code != http.StatusOK || a.pedido == nil {
		t.Fatalf("registro: %d %s", rec.Code, rec.Body.String())
	}
	// Lo que llega a Auth: solo los campos del formulario.
	enviado, _ := protojson.Marshal(a.pedido)
	for _, prohibido := range []string{"ADMIN", "role", "rol\"", "permission", "user_id", "tipo_usuario"} {
		if strings.Contains(string(enviado), prohibido) {
			t.Fatalf("la solicitud a Auth contiene %q: %s", prohibido, enviado)
		}
	}
	if strings.Contains(rec.Body.String(), "ADMIN") {
		t.Fatal("la respuesta no debe reflejar el rol pedido")
	}
}
