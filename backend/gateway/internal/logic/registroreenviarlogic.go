package logic

import (
	"context"

	"gateway/internal/svc"
	"gateway/internal/types"

	"auth/authclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type RegistroReenviarLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegistroReenviarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegistroReenviarLogic {
	return &RegistroReenviarLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// RegistroReenviar: código nuevo para un registro pendiente (Auth aplica la
// espera entre envíos y el máximo de envíos).
func (l *RegistroReenviarLogic) RegistroReenviar(req *types.RegistroReenviarRequest) (*types.RegistroReenviarResponse, error) {
	r, err := l.svcCtx.AuthClient.EmailResendCode(l.ctx, &authclient.EmailResendCodeRequest{RegistrationId: req.RegistroId})
	if err != nil {
		return nil, err
	}
	return &types.RegistroReenviarResponse{
		ExpiraEn:   r.GetExpiresIn(),
		ReenviarEn: r.GetResendIn(),
		Mensaje:    MensajeCodigoEnviado,
	}, nil
}
