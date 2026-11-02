package security

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// Google como proveedor de identidad (OpenID Connect, Authorization Code +
// PKCE). Auth es el único que conoce el Client Secret: canjea el código y
// valida el ID token con go-oidc (firma con las claves publicadas por Google,
// issuer, audience = Client ID y expiración). El nonce y email_verified se
// comprueban aquí; las reglas de cuentas, en la lógica.

const IssuerGoogle = "https://accounts.google.com"

var (
	ErrGoogleNoConfigurado = errors.New("google: proveedor no configurado")
	ErrGoogleNoDisponible  = errors.New("google: proveedor no disponible")
	ErrGoogleCodigo        = errors.New("google: código de autorización rechazado")
	ErrGoogleToken         = errors.New("google: id token inválido")
	ErrGoogleNonce         = errors.New("google: nonce no coincide")
)

// GoogleIdentity: lo que Google confirmó en el ID token.
type GoogleIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
	GivenName     string
	FamilyName    string
}

// GoogleConfig: Issuer vacío = Google. HTTPClient solo para pruebas.
type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Issuer       string
	HTTPClient   *http.Client
}

// GoogleProvider: interfaz que usa la lógica (las pruebas usan un proveedor
// OIDC simulado con el mismo código real de validación).
type GoogleProvider interface {
	Configurado() bool
	AuthCodeURL(ctx context.Context, state, nonce, verifier string) (string, error)
	Exchange(ctx context.Context, code, verifier, nonce string) (*GoogleIdentity, error)
}

type GoogleOIDC struct {
	cfg      GoogleConfig
	cliente  *http.Client
	mu       sync.Mutex
	verifier *oidc.IDTokenVerifier
	oauth    *oauth2.Config
}

func NewGoogleOIDC(cfg GoogleConfig) *GoogleOIDC {
	cfg.ClientID = strings.TrimSpace(cfg.ClientID)
	cfg.ClientSecret = strings.TrimSpace(cfg.ClientSecret)
	cfg.RedirectURL = strings.TrimSpace(cfg.RedirectURL)
	cfg.Issuer = strings.TrimSpace(cfg.Issuer)
	if cfg.Issuer == "" {
		cfg.Issuer = IssuerGoogle
	}
	cliente := cfg.HTTPClient
	if cliente == nil {
		cliente = &http.Client{Timeout: 10 * time.Second}
	}
	return &GoogleOIDC{cfg: cfg, cliente: cliente}
}

func (g *GoogleOIDC) Configurado() bool {
	return g.cfg.ClientID != "" && g.cfg.ClientSecret != "" && g.cfg.RedirectURL != ""
}

// iniciar descubre los endpoints del issuer la primera vez que se usa (Auth
// arranca aunque Google no esté configurado o no responda).
func (g *GoogleOIDC) iniciar() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.oauth != nil {
		return nil
	}
	if !g.Configurado() {
		return ErrGoogleNoConfigurado
	}
	// Contexto base sin cancelación: go-oidc lo conserva para descargar las
	// claves públicas más adelante; el límite de tiempo lo pone el cliente HTTP.
	base := oidc.ClientContext(context.Background(), g.cliente)
	proveedor, err := oidc.NewProvider(base, g.cfg.Issuer)
	if err != nil {
		return errors.Join(ErrGoogleNoDisponible, err)
	}
	g.verifier = proveedor.Verifier(&oidc.Config{ClientID: g.cfg.ClientID})
	g.oauth = &oauth2.Config{
		ClientID:     g.cfg.ClientID,
		ClientSecret: g.cfg.ClientSecret,
		RedirectURL:  g.cfg.RedirectURL,
		Endpoint:     proveedor.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
	}
	return nil
}

// AuthCodeURL: URL de autorización con state, nonce y desafío PKCE (S256).
func (g *GoogleOIDC) AuthCodeURL(_ context.Context, state, nonce, verifier string) (string, error) {
	if err := g.iniciar(); err != nil {
		return "", err
	}
	return g.oauth.AuthCodeURL(state,
		oidc.Nonce(nonce),
		oauth2.S256ChallengeOption(verifier),
		oauth2.SetAuthURLParam("prompt", "select_account"),
	), nil
}

// Exchange canjea el código (con el verificador PKCE) y valida el ID token.
func (g *GoogleOIDC) Exchange(ctx context.Context, code, verifier, nonce string) (*GoogleIdentity, error) {
	if err := g.iniciar(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(code) == "" || verifier == "" || nonce == "" {
		return nil, ErrGoogleCodigo
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, g.cliente)
	ctx = oidc.ClientContext(ctx, g.cliente)
	token, err := g.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		var rechazo *oauth2.RetrieveError
		if errors.As(err, &rechazo) {
			return nil, ErrGoogleCodigo
		}
		return nil, ErrGoogleNoDisponible
	}
	crudo, _ := token.Extra("id_token").(string)
	if crudo == "" {
		return nil, ErrGoogleToken
	}
	// Firma, issuer, audience (Client ID) y expiración.
	idToken, err := g.verifier.Verify(ctx, crudo)
	if err != nil {
		return nil, ErrGoogleToken
	}
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(nonce)) != 1 {
		return nil, ErrGoogleNonce
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified any    `json:"email_verified"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
	}
	if err := idToken.Claims(&claims); err != nil || strings.TrimSpace(idToken.Subject) == "" {
		return nil, ErrGoogleToken
	}
	return &GoogleIdentity{
		Subject:       idToken.Subject,
		Email:         strings.ToLower(strings.TrimSpace(claims.Email)),
		EmailVerified: verdadero(claims.EmailVerified),
		GivenName:     strings.TrimSpace(claims.GivenName),
		FamilyName:    strings.TrimSpace(claims.FamilyName),
	}, nil
}

// verdadero: email_verified llega como booleano (Google) o, en algunos
// proveedores, como texto.
func verdadero(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return strings.EqualFold(x, "true")
	}
	return false
}

// ValorAleatorio: 32 bytes aleatorios en base64url (state, nonce).
func ValorAleatorio() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
