package logic

import (
	"context"
	"crypto/subtle"
	"net/url"

	"gateway/internal/svc"
	"gateway/internal/types"

	"auth/authclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type GoogleCallbackLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGoogleCallbackLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GoogleCallbackLogic {
	return &GoogleCallbackLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// GoogleCallback valida el state contra la cookie del flujo (protección
// CSRF), pide a Auth canjear el código y devuelve SIEMPRE una URL del
// frontend: con un código de un solo uso, con "vinculado" o con la clave del
// error (nunca tokens ni mensajes en la URL).
func (l *GoogleCallbackLogic) GoogleCallback(req *types.GoogleCallbackRequest, cookieFlujo string) string {
	frontend := l.svcCtx.Config.FrontendURL
	flujo, ok := DecodificarFlujo(cookieFlujo)
	fallo := func(clave string) string {
		if ok && flujo.Vinculo != "" {
			return DestinoPerfilGoogle(frontend, url.Values{"google_error": {clave}})
		}
		return DestinoLoginGoogle(frontend, url.Values{"error": {clave}})
	}
	if !ok || req.State == "" || subtle.ConstantTimeCompare([]byte(req.State), []byte(flujo.State)) != 1 {
		return fallo("estado_invalido")
	}
	if req.Error != "" {
		// La persona canceló en Google o Google devolvió un error.
		return fallo("cancelado")
	}
	if req.Code == "" {
		return fallo("estado_invalido")
	}
	r, err := l.svcCtx.AuthClient.GoogleCallback(l.ctx, &authclient.GoogleCallbackRequest{
		Code: req.Code, Verifier: flujo.Verifier, Nonce: flujo.Nonce, LinkCode: flujo.Vinculo,
	})
	if err != nil {
		return fallo(ClaveErrorGoogle(err))
	}
	if r.Result == "VINCULADO" {
		return DestinoPerfilGoogle(frontend, url.Values{"google": {"vinculado"}})
	}
	return DestinoLoginGoogle(frontend, url.Values{"codigo": {r.LoginCode}})
}
