// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"encoding/base64"

	"gateway/internal/svc"
	"gateway/internal/types"

	"psicologia/psicologiaclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type ExportPsicologiaAtencionPdfLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewExportPsicologiaAtencionPdfLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExportPsicologiaAtencionPdfLogic {
	return &ExportPsicologiaAtencionPdfLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ExportPsicologiaAtencionPdfLogic) ExportPsicologiaAtencionPdf(req *types.ExportPsicologiaAtencionPdfRequest) (resp *types.ExportPsicologiaAtencionPdfResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	// El PDF se genera y audita (EXPORT) en Psicología Service; aquí solo se
	// transporta. Nunca se registra el contenido.
	out, err := l.svcCtx.PsicologiaClient.ExportAtencionPdf(ctx, &psicologiaclient.ExportAtencionPdfRequest{Id: req.Id, NumeroVersion: req.Version})
	if err != nil {
		return nil, err
	}
	return &types.ExportPsicologiaAtencionPdfResponse{
		Nombre:          out.Nombre,
		ContenidoBase64: base64.StdEncoding.EncodeToString(out.Contenido),
	}, nil
}
