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

type DiscardPsicologiaAtencionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDiscardPsicologiaAtencionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DiscardPsicologiaAtencionLogic {
	return &DiscardPsicologiaAtencionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DiscardPsicologiaAtencionLogic) DiscardPsicologiaAtencion(req *types.RevisionPsicologiaAtencionRequest) (resp *types.DiscardPsicologiaAtencionResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	if _, err := l.svcCtx.PsicologiaClient.DiscardAtencionDraft(ctx, &psicologiaclient.DiscardAtencionDraftRequest{Id: req.Id, Revision: req.Revision}); err != nil {
		return nil, err
	}
	return &types.DiscardPsicologiaAtencionResponse{}, nil
}
