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

type UpdatePsicologiaAtencionDraftLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdatePsicologiaAtencionDraftLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdatePsicologiaAtencionDraftLogic {
	return &UpdatePsicologiaAtencionDraftLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdatePsicologiaAtencionDraftLogic) UpdatePsicologiaAtencionDraft(req *types.UpdatePsicologiaAtencionDraftRequest) (resp *types.PsicologiaAtencionResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	out, err := l.svcCtx.PsicologiaClient.UpdateAtencionDraft(ctx, &psicologiaclient.UpdateAtencionDraftRequest{
		Id: req.Id, Revision: req.Revision, Contenido: toAtencionContenidoProto(req.Contenido),
	})
	if err != nil {
		return nil, err
	}
	return toAtencionResponse(out.Atencion), nil
}
