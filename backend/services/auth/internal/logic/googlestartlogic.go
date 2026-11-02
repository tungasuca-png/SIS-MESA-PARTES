package logic

import (
	"context"

	"auth/auth"
	"auth/internal/security"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GoogleStartLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGoogleStartLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GoogleStartLogic {
	return &GoogleStartLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// GoogleStart genera state, nonce y verificador PKCE (aleatorios de 32
// bytes) y la URL de autorización. El Gateway los guarda en una cookie
// HttpOnly de corta duración y los devuelve en el callback.
func (l *GoogleStartLogic) GoogleStart(_ *auth.GoogleStartRequest) (*auth.GoogleStartResponse, error) {
	if !l.svcCtx.Google.Configurado() {
		return nil, errNoConfigurado()
	}
	valores := make([]string, 3)
	for i := range valores {
		v, err := security.ValorAleatorio()
		if err != nil {
			return nil, status.Error(codes.Internal, "no se pudo iniciar el acceso con Google")
		}
		valores[i] = v
	}
	state, nonce, verifier := valores[0], valores[1], valores[2]
	url, err := l.svcCtx.Google.AuthCodeURL(l.ctx, state, nonce, verifier)
	if err != nil {
		return nil, errProveedor(err)
	}
	return &auth.GoogleStartResponse{Url: url, State: state, Nonce: nonce, Verifier: verifier}, nil
}
