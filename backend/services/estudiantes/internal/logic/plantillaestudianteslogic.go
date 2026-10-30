package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/excel"
	"estudiantes/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PlantillaEstudiantesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPlantillaEstudiantesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlantillaEstudiantesLogic {
	return &PlantillaEstudiantesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PlantillaEstudiantes genera la plantilla oficial (no depende de un archivo
// guardado en ninguna computadora). La usa quien importa: estudiantes.create.
func (l *PlantillaEstudiantesLogic) PlantillaEstudiantes(_ *estudiantes.PlantillaEstudiantesRequest) (*estudiantes.ArchivoExcelResponse, error) {
	if _, err := autorizar(l.ctx, l.svcCtx, "estudiantes.create"); err != nil {
		return nil, err
	}
	contenido, err := excel.Plantilla()
	if err != nil {
		l.Errorf("generar plantilla de estudiantes: %v", err)
		return nil, status.Error(codes.Internal, "no se pudo generar la plantilla")
	}
	return &estudiantes.ArchivoExcelResponse{Nombre: "plantilla_estudiantes.xlsx", Contenido: contenido}, nil
}
