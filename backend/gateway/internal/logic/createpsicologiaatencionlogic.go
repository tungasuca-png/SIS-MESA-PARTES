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

type CreatePsicologiaAtencionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreatePsicologiaAtencionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePsicologiaAtencionLogic {
	return &CreatePsicologiaAtencionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreatePsicologiaAtencionLogic) CreatePsicologiaAtencion(req *types.CreatePsicologiaAtencionRequest) (resp *types.PsicologiaAtencionResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	out, err := l.svcCtx.PsicologiaClient.CreateAtencionDraft(ctx, &psicologiaclient.CreateAtencionDraftRequest{
		ExpedienteId: req.Id, Contenido: toAtencionContenidoProto(req.Contenido),
	})
	if err != nil {
		return nil, err
	}
	return toAtencionResponse(out.Atencion), nil
}
