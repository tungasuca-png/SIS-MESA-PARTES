package logic

import (
	"context"

	"gateway/internal/svc"
	"gateway/internal/types"

	"estudiantes/estudiantesclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type PlantillaEstudiantesExcelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPlantillaEstudiantesExcelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlantillaEstudiantesExcelLogic {
	return &PlantillaEstudiantesExcelLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *PlantillaEstudiantesExcelLogic) PlantillaEstudiantesExcel(req *types.ExcelEstudiantesRequest) (*types.ArchivoExcelResponse, error) {
	resp, err := l.svcCtx.EstudiantesClient.PlantillaEstudiantes(withAuthorization(l.ctx, req.Authorization),
		&estudiantesclient.PlantillaEstudiantesRequest{})
	if err != nil {
		return nil, err
	}
	return toArchivoExcelDTO(resp), nil
}
