package logic

import (
	"context"

	"gateway/internal/svc"
	"gateway/internal/types"

	"auth/authclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type GoogleCanjearLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGoogleCanjearLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GoogleCanjearLogic {
	return &GoogleCanjearLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// GoogleCanjear: el frontend canjea el código de un solo uso. LOGIN -> la
// sesión normal (misma forma que /api/auth/login); REGISTRO -> los datos que
// Google verificó para el formulario obligatorio.
func (l *GoogleCanjearLogic) GoogleCanjear(req *types.GoogleCanjearRequest) (*types.GoogleCanjearResponse, error) {
	r, err := l.svcCtx.AuthClient.GoogleExchange(l.ctx, &authclient.GoogleExchangeRequest{LoginCode: req.Codigo})
	if err != nil {
		return nil, err
	}
	resp := &types.GoogleCanjearResponse{Resultado: r.Result, Correo: r.Email, Nombres: r.Nombres, Apellidos: r.Apellidos}
	if s := r.GetSession(); s != nil {
		resp.Sesion = sesionDTO(s)
	}
	return resp, nil
}

func sesionDTO(s *authclient.LoginResponse) *types.LoginResponse {
	return &types.LoginResponse{
		AccessToken: s.AccessToken, RefreshToken: s.RefreshToken, ExpiresIn: s.ExpiresIn,
		UserId: s.UserId, Username: s.Username, Email: s.Email, Role: s.Role,
	}
}
