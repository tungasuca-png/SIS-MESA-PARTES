// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"gateway/internal/svc"
	"gateway/internal/types"

	"psicologia/psicologiaclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPsicologiaAtencionVersionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPsicologiaAtencionVersionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPsicologiaAtencionVersionLogic {
	return &GetPsicologiaAtencionVersionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPsicologiaAtencionVersionLogic) GetPsicologiaAtencionVersion(req *types.GetPsicologiaAtencionVersionRequest) (resp *types.PsicologiaAtencionResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	out, err := l.svcCtx.PsicologiaClient.GetAtencionVersion(ctx, &psicologiaclient.GetAtencionVersionRequest{Id: req.Id, NumeroVersion: req.Version})
	if err != nil {
		return nil, err
	}
	return toAtencionResponse(out.Atencion), nil
}
