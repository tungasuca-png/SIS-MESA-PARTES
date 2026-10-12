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

type CorrectPsicologiaAtencionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCorrectPsicologiaAtencionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CorrectPsicologiaAtencionLogic {
	return &CorrectPsicologiaAtencionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CorrectPsicologiaAtencionLogic) CorrectPsicologiaAtencion(req *types.CorrectPsicologiaAtencionRequest) (resp *types.PsicologiaAtencionResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	out, err := l.svcCtx.PsicologiaClient.CorrectAtencion(ctx, &psicologiaclient.CorrectAtencionRequest{
		Id: req.Id, VersionBase: req.VersionBase, MotivoCorreccion: req.MotivoCorreccion, Contenido: toAtencionContenidoProto(req.Contenido),
	})
	if err != nil {
		return nil, err
	}
	return toAtencionResponse(out.Atencion), nil
}
