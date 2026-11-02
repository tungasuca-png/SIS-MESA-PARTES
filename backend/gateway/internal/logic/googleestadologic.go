package logic

import (
	"context"

	"gateway/internal/svc"
	"gateway/internal/types"

	"auth/authclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type GoogleEstadoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGoogleEstadoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GoogleEstadoLogic {
	return &GoogleEstadoLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// GoogleEstado: si la cuenta autenticada (la del JWT) tiene Google vinculado.
func (l *GoogleEstadoLogic) GoogleEstado(req *types.GoogleEstadoRequest) (*types.GoogleEstadoResponse, error) {
	r, err := l.svcCtx.AuthClient.GoogleLinkStatus(withAuthorization(l.ctx, req.Authorization), &authclient.GoogleLinkStatusRequest{})
	if err != nil {
		return nil, err
	}
	return &types.GoogleEstadoResponse{Vinculado: r.Linked, Correo: r.Email, Disponible: r.Available}, nil
}
