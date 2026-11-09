package outbox

import (
	"context"
	"errors"
	"sync"
	"time"

	"auth/authclient"
)

// TokensAuth obtiene el token de servicio del publicador (client credentials,
// audiencia derivaciones, scope derivaciones.proyectar) y lo reutiliza hasta
// 60 s antes de su vencimiento. Las credenciales llegan solo por entorno
// (backend/.env), nunca por el repositorio.
type TokensAuth struct {
	auth         authclient.Auth
	clientID     string
	clientSecret string
	timeout      time.Duration

	mu    sync.Mutex
	token string
	vence time.Time
	ahora func() time.Time
}

func NewTokensAuth(auth authclient.Auth, clientID, clientSecret string) *TokensAuth {
	return &TokensAuth{auth: auth, clientID: clientID, clientSecret: clientSecret, timeout: 2 * time.Second, ahora: time.Now}
}

func (t *TokensAuth) Token(ctx context.Context) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.token != "" && t.ahora().Before(t.vence.Add(-60*time.Second)) {
		return t.token, nil
	}
	c, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	resp, err := t.auth.IssueServiceToken(c, &authclient.IssueServiceTokenRequest{ClientId: t.clientID, ClientSecret: t.clientSecret})
	if err != nil {
		return "", err
	}
	if resp.GetAccessToken() == "" || resp.GetExpiresIn() <= 0 {
		return "", errors.New("respuesta de token de servicio vacía")
	}
	t.token = resp.GetAccessToken()
	t.vence = t.ahora().Add(time.Duration(resp.GetExpiresIn()) * time.Second)
	return t.token, nil
}

// Invalidar fuerza la renovación (p. ej. tras Unauthenticated del consumidor).
func (t *TokensAuth) Invalidar() {
	t.mu.Lock()
	t.token = ""
	t.mu.Unlock()
}
