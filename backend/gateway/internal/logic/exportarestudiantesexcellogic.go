package logic

import (
	"context"

	"gateway/internal/svc"
	"gateway/internal/types"

	"estudiantes/estudiantesclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type ExportarEstudiantesExcelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewExportarEstudiantesExcelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExportarEstudiantesExcelLogic {
	return &ExportarEstudiantesExcelLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ExportarEstudiantesExcelLogic) ExportarEstudiantesExcel(req *types.ExportarEstudiantesExcelRequest) (*types.ArchivoExcelResponse, error) {
	resp, err := l.svcCtx.EstudiantesClient.ExportarEstudiantes(withAuthorization(l.ctx, req.Authorization),
		&estudiantesclient.ExportarEstudiantesRequest{Estado: req.Estado, Nivel: req.Nivel})
	if err != nil {
		return nil, err
	}
	return toArchivoExcelDTO(resp), nil
}
