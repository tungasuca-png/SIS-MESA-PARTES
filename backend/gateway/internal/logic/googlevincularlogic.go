package logic

import (
	"context"

	"gateway/internal/svc"
	"gateway/internal/types"

	"auth/authclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type GoogleVincularLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGoogleVincularLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GoogleVincularLogic {
	return &GoogleVincularLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// GoogleVincular: el SOLICITANTE autenticado se reautentica con su clave;
// Auth emite un código de vinculación que el handler guarda en una cookie
// HttpOnly (nunca en la URL) y responde la ruta para iniciar el flujo.
func (l *GoogleVincularLogic) GoogleVincular(req *types.GoogleVincularRequest) (*types.GoogleVincularResponse, string, error) {
	r, err := l.svcCtx.AuthClient.GoogleStartLink(withAuthorization(l.ctx, req.Authorization), &authclient.GoogleStartLinkRequest{Password: req.Clave})
	if err != nil {
		return nil, "", err
	}
	return &types.GoogleVincularResponse{Url: RutaCookiesGoogle + "/iniciar?modo=vincular", ExpiraEn: r.ExpiresIn}, r.LinkCode, nil
}
