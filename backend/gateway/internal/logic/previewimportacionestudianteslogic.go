package logic

import (
	"context"

	"gateway/internal/svc"
	"gateway/internal/types"

	"estudiantes/estudiantesclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type PreviewImportacionEstudiantesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPreviewImportacionEstudiantesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PreviewImportacionEstudiantesLogic {
	return &PreviewImportacionEstudiantesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *PreviewImportacionEstudiantesLogic) PreviewImportacionEstudiantes(req *types.ExcelEstudiantesRequest, archivo ArchivoExcel) (*types.ImportarEstudiantesExcelResponse, error) {
	resp, err := l.svcCtx.EstudiantesClient.PreviewImportacionEstudiantes(withAuthorization(l.ctx, req.Authorization),
		&estudiantesclient.ImportarEstudiantesRequest{NombreArchivo: archivo.Nombre, Contenido: archivo.Contenido})
	if err != nil {
		return nil, err
	}
	return toImportacionDTO(resp), nil
}
