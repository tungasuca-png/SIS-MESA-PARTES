package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gateway/internal/limite"
	"gateway/internal/svc"

	"auth/authclient"

	"github.com/zeromicro/go-zero/rest/pathvar"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// authSinPermiso: Auth responde 403 (permiso solicitantes.identidad.verify
// ausente), como el interceptor real de autorización.
type authSinPermiso struct {
	authclient.Auth
	llamadas int
}

func (a *authSinPermiso) GetSolicitanteIdentity(context.Context, *authclient.GetSolicitanteIdentityRequest, ...grpc.CallOption) (*authclient.GetSolicitanteIdentityResponse, error) {
	a.llamadas++
	return nil, status.Error(codes.PermissionDenied, "el usuario no tiene el permiso requerido")
}

// Limitador real + handler real: sin permiso es 403 (no 429) mientras no se
// exceda el límite, y al excederlo es 429 sin llegar a Auth.
func TestVerificarIdentidad_SinPermiso403YLuego429(t *testing.T) {
	a := &authSinPermiso{}
	h := limite.Nuevo(limite.RutasPorDefecto(), false).Middleware(VerificarIdentidadHandler(&svc.ServiceContext{AuthClient: a}))
	pedir := func(id string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/solicitantes/identidades/"+id+"/verificar", strings.NewReader(`{"metodo":"PRESENCIAL"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer token-sin-permiso")
		req.RemoteAddr = "203.0.113.50:50000"
		req = pathvar.WithVars(req, map[string]string{"id": id})
		rec := httptest.NewRecorder()
		h(rec, req)
		return rec.Code
	}
	for i := 0; i < 5; i++ {
		if c := pedir("11111111-1111-4111-8111-111111111111"); c != http.StatusForbidden {
			t.Fatalf("intento %d sin permiso: %d", i, c)
		}
	}
	if c := pedir("22222222-2222-4222-8222-222222222222"); c != http.StatusTooManyRequests {
		t.Fatalf("exceso (otro id): %d", c)
	}
	if a.llamadas != 5 {
		t.Fatalf("el 429 no debe llegar a Auth: %d llamadas", a.llamadas)
	}
}
