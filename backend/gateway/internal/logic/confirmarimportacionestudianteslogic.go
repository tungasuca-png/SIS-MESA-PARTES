package logic

import (
	"context"

	"gateway/internal/svc"
	"gateway/internal/types"

	"estudiantes/estudiantesclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConfirmarImportacionEstudiantesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewConfirmarImportacionEstudiantesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfirmarImportacionEstudiantesLogic {
	return &ConfirmarImportacionEstudiantesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ConfirmarImportacionEstudiantesLogic) ConfirmarImportacionEstudiantes(req *types.ExcelEstudiantesRequest, archivo ArchivoExcel) (*types.ImportarEstudiantesExcelResponse, error) {
	resp, err := l.svcCtx.EstudiantesClient.ConfirmarImportacionEstudiantes(withAuthorization(l.ctx, req.Authorization),
		&estudiantesclient.ImportarEstudiantesRequest{NombreArchivo: archivo.Nombre, Contenido: archivo.Contenido, Huella: archivo.Huella})
	if err != nil {
		return nil, err
	}
	return toImportacionDTO(resp), nil
}
