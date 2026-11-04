package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gateway/internal/config"
	"gateway/internal/logic"
	"gateway/internal/svc"

	"auth/authclient"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type authHandlerFalso struct {
	authclient.Auth
	startErr  error
	callbacks int
}

func (a *authHandlerFalso) GoogleStart(context.Context, *authclient.GoogleStartRequest, ...grpc.CallOption) (*authclient.GoogleStartResponse, error) {
	if a.startErr != nil {
		return nil, a.startErr
	}
	return &authclient.GoogleStartResponse{Url: "https://accounts.example/auth?state=s1", State: "s1", Nonce: "n1", Verifier: "v1"}, nil
}

func (a *authHandlerFalso) GoogleCallback(context.Context, *authclient.GoogleCallbackRequest, ...grpc.CallOption) (*authclient.GoogleCallbackResponse, error) {
	a.callbacks++
	return &authclient.GoogleCallbackResponse{Result: "LOGIN", LoginCode: "codigo-un-uso"}, nil
}

func cookieDe(rec *httptest.ResponseRecorder, nombre string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == nombre {
			return c
		}
	}
	return nil
}

func TestGoogleHandlers_CookiesYRedirecciones(t *testing.T) {
	a := &authHandlerFalso{}
	sc := &svc.ServiceContext{Config: config.Config{GoogleCookieSegura: "true"}, AuthClient: a}

	// Iniciar: cookie HttpOnly, Secure, SameSite=Lax, con path limitado; redirección a Google.
	rec := httptest.NewRecorder()
	GoogleIniciarHandler(sc)(rec, httptest.NewRequest(http.MethodGet, "/api/auth/google/iniciar", nil))
	flujo := cookieDe(rec, logic.CookieFlujoGoogle)
	if rec.Code != http.StatusFound || !strings.HasPrefix(rec.Header().Get("Location"), "https://accounts.example/auth") || flujo == nil ||
		!flujo.HttpOnly || !flujo.Secure || flujo.SameSite != http.SameSiteLaxMode || flujo.Path != "/api/auth/google" || flujo.MaxAge != 600 {
		t.Fatalf("iniciar: %d %v %+v", rec.Code, rec.Header(), flujo)
	}

	// Callback con la cookie: redirección al frontend con el código en el
	// fragmento; la cookie se elimina.
	rec2 := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?code=c&state=s1", nil)
	req.AddCookie(&http.Cookie{Name: logic.CookieFlujoGoogle, Value: flujo.Value})
	GoogleCallbackHandler(sc)(rec2, req)
	if rec2.Code != http.StatusFound || rec2.Header().Get("Location") != "/login/google#codigo=codigo-un-uso" || cookieDe(rec2, logic.CookieFlujoGoogle).MaxAge >= 0 {
		t.Fatalf("callback: %d %s", rec2.Code, rec2.Header().Get("Location"))
	}
	// Sin cookie (otro navegador / CSRF): no se llama a Auth.
	rec3 := httptest.NewRecorder()
	GoogleCallbackHandler(sc)(rec3, httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?code=c&state=s1", nil))
	if rec3.Header().Get("Location") != "/login/google#error=estado_invalido" || a.callbacks != 1 {
		t.Fatalf("sin cookie: %s", rec3.Header().Get("Location"))
	}
	// Vincular sin la cookie de vinculación: vuelve al perfil con error.
	rec4 := httptest.NewRecorder()
	GoogleIniciarHandler(sc)(rec4, httptest.NewRequest(http.MethodGet, "/api/auth/google/iniciar?modo=vincular", nil))
	if rec4.Header().Get("Location") != "/mesa-de-partes/perfil?google_error=codigo_invalido" {
		t.Fatalf("vincular sin cookie: %s", rec4.Header().Get("Location"))
	}
	// Google no configurado: vuelve al login con la clave del error.
	a.startErr = status.Error(codes.Unavailable, "google_no_configurado: ...")
	rec5 := httptest.NewRecorder()
	GoogleIniciarHandler(sc)(rec5, httptest.NewRequest(http.MethodGet, "/api/auth/google/iniciar", nil))
	if rec5.Header().Get("Location") != "/login/google#error=no_configurado" || cookieDe(rec5, logic.CookieFlujoGoogle) != nil {
		t.Fatalf("no configurado: %s", rec5.Header().Get("Location"))
	}
}

func TestRegisterHandler_Gone(t *testing.T) {
	rec := httptest.NewRecorder()
	RegisterHandler(nil)(rec, httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{"username":"x","dni":"12345678","email":"a@b.c","password":"12345678","role":"ADMIN"}`)))
	var cuerpo map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &cuerpo)
	if rec.Code != http.StatusGone || !strings.Contains(cuerpo["message"], "deshabilitado") {
		t.Fatalf("registro antiguo: %d %s", rec.Code, rec.Body.String())
	}
}
