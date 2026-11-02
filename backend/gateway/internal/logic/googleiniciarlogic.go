package logic

import (
	"context"

	"gateway/internal/svc"
	"gateway/internal/types"

	"auth/authclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type GoogleIniciarLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGoogleIniciarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GoogleIniciarLogic {
	return &GoogleIniciarLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// GoogleIniciar pide a Auth la URL de autorización y los valores del flujo
// (state, nonce, verificador PKCE). Devuelve la URL de Google y el valor de
// la cookie del flujo. vinculo: código de la cookie de vinculación ("" en
// login/registro). La redirección y las cookies las escribe el handler.
func (l *GoogleIniciarLogic) GoogleIniciar(_ *types.GoogleIniciarRequest, vinculo string) (destino, cookieFlujo string, err error) {
	r, err := l.svcCtx.AuthClient.GoogleStart(l.ctx, &authclient.GoogleStartRequest{})
	if err != nil {
		return "", "", err
	}
	return r.Url, CodificarFlujo(FlujoGoogle{State: r.State, Nonce: r.Nonce, Verifier: r.Verifier, Vinculo: vinculo}), nil
}
