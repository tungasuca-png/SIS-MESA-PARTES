package security

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

// Proveedor OpenID Connect SIMULADO (httptest): descubrimiento, endpoint de
// token (verifica el desafío PKCE S256) y claves públicas. Firma ID tokens
// con una clave RSA de prueba. go-oidc valida igual que contra Google. No hay
// llamadas reales a Google ni cuentas reales.
type proveedorFalso struct {
	srv      *httptest.Server
	clave    *rsa.PrivateKey
	otra     *rsa.PrivateKey // para firmas inválidas
	mu       sync.Mutex
	codigos  map[string]emision
	clientID string
}

// emision: lo que el proveedor devolverá al canjear un código.
type emision struct {
	desafio string // S256 del verificador correcto
	claims  map[string]any
	firma   *rsa.PrivateKey
}

func nuevoProveedorFalso(t *testing.T) *proveedorFalso {
	t.Helper()
	k1, _ := rsa.GenerateKey(rand.Reader, 2048)
	k2, _ := rsa.GenerateKey(rand.Reader, 2048)
	p := &proveedorFalso{clave: k1, otra: k2, codigos: map[string]emision{}, clientID: "cliente-prueba.apps.example"}
	mux := http.NewServeMux()
	p.srv = httptest.NewServer(mux)
	t.Cleanup(p.srv.Close)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer": p.srv.URL, "authorization_endpoint": p.srv.URL + "/auth", "token_endpoint": p.srv.URL + "/token",
			"jwks_uri": p.srv.URL + "/certs", "id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/certs", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &k1.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		p.mu.Lock()
		e, ok := p.codigos[r.Form.Get("code")]
		delete(p.codigos, r.Form.Get("code")) // un solo uso, como Google
		p.mu.Unlock()
		suma := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if !ok || base64.RawURLEncoding.EncodeToString(suma[:]) != e.desafio {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		firmante, _ := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: e.firma, KeyID: "k1"}}, nil)
		cuerpo, _ := json.Marshal(e.claims)
		jws, _ := firmante.Sign(cuerpo)
		idToken, _ := jws.CompactSerialize()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at-prueba", "token_type": "Bearer", "expires_in": 3600, "id_token": idToken})
	})
	return p
}

// emitir registra un código; mod ajusta los claims (válidos por defecto).
func (p *proveedorFalso) emitir(code, verifier, nonce string, mod func(map[string]any), firma *rsa.PrivateKey) {
	suma := sha256.Sum256([]byte(verifier))
	claims := map[string]any{
		"iss": p.srv.URL, "aud": p.clientID, "sub": "1098765432101234567890", "email": "persona.prueba@example.com",
		"email_verified": true, "given_name": "Ana María", "family_name": "Quispe Ñahui", "nonce": nonce,
		"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(),
	}
	if mod != nil {
		mod(claims)
	}
	if firma == nil {
		firma = p.clave
	}
	p.mu.Lock()
	p.codigos[code] = emision{desafio: base64.RawURLEncoding.EncodeToString(suma[:]), claims: claims, firma: firma}
	p.mu.Unlock()
}

func (p *proveedorFalso) cliente() *GoogleOIDC {
	return NewGoogleOIDC(GoogleConfig{ClientID: p.clientID, ClientSecret: "secreto-prueba", RedirectURL: "http://localhost/cb", Issuer: p.srv.URL})
}

func TestGoogleOIDC_ValidaIdentidad(t *testing.T) {
	p := nuevoProveedorFalso(t)
	g := p.cliente()
	ctx := context.Background()

	// URL de autorización: state, nonce, PKCE S256 y scopes mínimos.
	u, err := g.AuthCodeURL(ctx, "estado-1", "nonce-1", "verificador-con-longitud-suficiente-123456789")
	if err != nil {
		t.Fatal(err)
	}
	q, _ := url.Parse(u)
	v := q.Query()
	if v.Get("state") != "estado-1" || v.Get("nonce") != "nonce-1" || v.Get("code_challenge_method") != "S256" || v.Get("code_challenge") == "" ||
		v.Get("scope") != "openid email profile" || v.Get("response_type") != "code" || strings.Contains(u, "secreto-prueba") {
		t.Fatalf("URL de autorización: %s", u)
	}

	ver := "verificador-correcto-0123456789-0123456789-abc"
	p.emitir("c-ok", ver, "n-ok", nil, nil)
	id, err := g.Exchange(ctx, "c-ok", ver, "n-ok")
	if err != nil {
		t.Fatalf("identidad válida: %v", err)
	}
	if id.Subject != "1098765432101234567890" || id.Email != "persona.prueba@example.com" || !id.EmailVerified || id.GivenName != "Ana María" || id.FamilyName != "Quispe Ñahui" {
		t.Fatalf("identidad: %+v", id)
	}
}

func TestGoogleOIDC_Rechazos(t *testing.T) {
	p := nuevoProveedorFalso(t)
	g := p.cliente()
	ctx := context.Background()
	ver := "verificador-correcto-0123456789-0123456789-abc"
	casos := []struct {
		nombre    string
		mod       func(map[string]any)
		firma     *rsa.PrivateKey
		verifier  string
		nonce     string
		esperado  error
		reutiliza bool
	}{
		{nombre: "audience incorrecto", mod: func(c map[string]any) { c["aud"] = "otro-cliente" }, esperado: ErrGoogleToken},
		{nombre: "issuer incorrecto", mod: func(c map[string]any) { c["iss"] = "https://emisor-falso.example" }, esperado: ErrGoogleToken},
		{nombre: "token vencido", mod: func(c map[string]any) { c["exp"] = time.Now().Add(-time.Hour).Unix() }, esperado: ErrGoogleToken},
		{nombre: "firma inválida", firma: p.otra, esperado: ErrGoogleToken},
		{nombre: "nonce incorrecto", nonce: "otro-nonce", esperado: ErrGoogleNonce},
		{nombre: "PKCE incorrecto", verifier: "verificador-equivocado-0123456789-0123456789", esperado: ErrGoogleCodigo},
		{nombre: "código reutilizado", reutiliza: true, esperado: ErrGoogleCodigo},
		{nombre: "sin sub", mod: func(c map[string]any) { delete(c, "sub") }, esperado: ErrGoogleToken},
	}
	for i, c := range casos {
		code := "c-" + string(rune('a'+i))
		if !c.reutiliza {
			p.emitir(code, ver, "n-1", c.mod, c.firma)
		}
		verifier, nonce := ver, "n-1"
		if c.verifier != "" {
			verifier = c.verifier
		}
		if c.nonce != "" {
			nonce = c.nonce
		}
		if _, err := g.Exchange(ctx, code, verifier, nonce); !errors.Is(err, c.esperado) {
			t.Errorf("%s: error %v, esperado %v", c.nombre, err, c.esperado)
		}
	}

	// email_verified=false se informa (la lógica lo rechaza).
	p.emitir("c-nv", ver, "n-1", func(c map[string]any) { c["email_verified"] = false }, nil)
	id, err := g.Exchange(ctx, "c-nv", ver, "n-1")
	if err != nil || id.EmailVerified {
		t.Fatalf("email no verificado: %+v %v", id, err)
	}
}

func TestGoogleOIDC_NoConfiguradoYNoDisponible(t *testing.T) {
	sin := NewGoogleOIDC(GoogleConfig{})
	if sin.Configurado() {
		t.Fatal("sin credenciales no está configurado")
	}
	if _, err := sin.AuthCodeURL(context.Background(), "s", "n", "v"); !errors.Is(err, ErrGoogleNoConfigurado) {
		t.Fatalf("no configurado: %v", err)
	}
	caido := httptest.NewServer(http.NotFoundHandler())
	caido.Close()
	g := NewGoogleOIDC(GoogleConfig{ClientID: "x", ClientSecret: "y", RedirectURL: "http://localhost/cb", Issuer: caido.URL})
	if _, err := g.Exchange(context.Background(), "c", "v", "n"); !errors.Is(err, ErrGoogleNoDisponible) {
		t.Fatalf("Google caído: %v", err)
	}
	if NewGoogleOIDC(GoogleConfig{ClientID: "x", ClientSecret: "y", RedirectURL: "z"}).cfg.Issuer != IssuerGoogle {
		t.Fatal("issuer por defecto")
	}
}
