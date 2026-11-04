package svc

import (
	"context"
	"strings"
	"sync"
	"time"

	"auth/authclient"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Credencial de Psicología como SISTEMA consumidor de Estudiantes (client
// credentials): Auth emite un JWT de servicio de corta duración a cambio de
// client_id + client_secret. El token vive solo en memoria y nunca se
// registra; tampoco el secreto.

// margenRenovacion: el token se renueva este tiempo antes de su vencimiento,
// para no usar uno que expire en vuelo.
const margenRenovacion = 60 * time.Second

// ServiceTokenIssuer es la porción de authclient.Auth que se necesita (se
// reemplaza por un doble en las pruebas).
type ServiceTokenIssuer interface {
	IssueServiceToken(ctx context.Context, in *authclient.IssueServiceTokenRequest, opts ...grpc.CallOption) (*authclient.IssueServiceTokenResponse, error)
}

type ServiceTokenProvider struct {
	issuer       ServiceTokenIssuer
	clientID     string
	clientSecret string
	now          func() time.Time

	// mu protege token/renovarEn/venceEn y serializa la emisión: varias
	// llamadas concurrentes con el token vencido esperan una sola renovación.
	mu        sync.Mutex
	token     string
	renovarEn time.Time // inicio del margen de renovación anticipada
	venceEn   time.Time // vencimiento real del token
}

func NewServiceTokenProvider(issuer ServiceTokenIssuer, clientID, clientSecret string) *ServiceTokenProvider {
	return &ServiceTokenProvider{issuer: issuer, clientID: clientID, clientSecret: clientSecret, now: time.Now}
}

// Token devuelve el token vigente o emite uno nuevo. Cualquier fallo de Auth
// se devuelve como Unavailable con un mensaje propio: un problema de la
// credencial del SERVICIO no debe verse como sesión inválida del usuario
// (un 401 cerraría su sesión en el frontend).
func (p *ServiceTokenProvider) Token(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.token != "" && p.now().Before(p.renovarEn) {
		return p.token, nil
	}

	// Contexto saliente LIMPIO: la emisión nunca lleva el JWT del usuario ni
	// otra metadata de la solicitud en curso.
	out := metadata.NewOutgoingContext(ctx, metadata.MD{})
	resp, err := p.issuer.IssueServiceToken(out, &authclient.IssueServiceTokenRequest{
		ClientId:     p.clientID,
		ClientSecret: p.clientSecret,
	})
	if err != nil {
		logx.WithContext(ctx).Errorf("credencial de servicio de Psicología rechazada o no disponible (Auth: %s)", status.Code(err))
		// La renovación era ANTICIPADA: si el token actual aún no venció, se
		// sigue usando (Estudiantes lo acepta) y se reintentará la renovación
		// en la próxima llamada. Un token invalidado ya no está aquí.
		if p.token != "" && p.now().Before(p.venceEn) {
			return p.token, nil
		}
		p.token = ""
		return "", status.Error(codes.Unavailable, "no se pudo obtener la credencial de servicio para consultar Estudiantes")
	}
	token := strings.TrimSpace(resp.GetAccessToken())
	vida := time.Duration(resp.GetExpiresIn()) * time.Second
	if token == "" || vida <= 0 {
		p.token = ""
		logx.WithContext(ctx).Errorf("Auth emitió una credencial de servicio sin token o sin vencimiento")
		return "", status.Error(codes.Unavailable, "no se pudo obtener la credencial de servicio para consultar Estudiantes")
	}

	margen := margenRenovacion
	if vida <= 2*margen {
		margen = vida / 2
	}
	p.token = token
	p.renovarEn = p.now().Add(vida - margen)
	p.venceEn = p.now().Add(vida)
	return token, nil
}

// Invalidate descarta el token si sigue siendo el indicado (Estudiantes lo
// rechazó). Si otra llamada ya lo renovó, no se toca el nuevo.
func (p *ServiceTokenProvider) Invalidate(token string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token == token {
		p.token = ""
	}
}
