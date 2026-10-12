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

type ListPsicologiaAtencionVersionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPsicologiaAtencionVersionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPsicologiaAtencionVersionsLogic {
	return &ListPsicologiaAtencionVersionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPsicologiaAtencionVersionsLogic) ListPsicologiaAtencionVersions(req *types.GetPsicologiaAtencionRequest) (resp *types.ListPsicologiaAtencionVersionsResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	out, err := l.svcCtx.PsicologiaClient.ListAtencionVersions(ctx, &psicologiaclient.ListAtencionVersionsRequest{Id: req.Id})
	if err != nil {
		return nil, err
	}
	items := make([]types.AtencionVersionResumenDTO, 0, len(out.Versiones))
	for _, v := range out.Versiones {
		items = append(items, types.AtencionVersionResumenDTO{
			NumeroVersion: v.NumeroVersion, MotivoCorreccion: v.MotivoCorreccion, CreadaPor: v.CreadaPor, CreadaEn: v.CreadaEn,
		})
	}
	return &types.ListPsicologiaAtencionVersionsResponse{Versiones: items}, nil
}
