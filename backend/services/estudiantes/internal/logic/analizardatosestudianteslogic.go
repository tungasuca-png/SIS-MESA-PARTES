package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/datos"
	"estudiantes/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalizarDatosEstudiantesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAnalizarDatosEstudiantesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalizarDatosEstudiantesLogic {
	return &AnalizarDatosEstudiantesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AnalizarDatosEstudiantes: vista previa. Detecta el formato, lee, propone
// (o aplica) el mapeo y clasifica cada registro contra el maestro. NO escribe.
func (l *AnalizarDatosEstudiantesLogic) AnalizarDatosEstudiantes(in *estudiantes.DatosImportacionRequest) (*estudiantes.DatosImportacionResponse, error) {
	if _, err := autorizarImportacion(l.ctx, l.svcCtx); err != nil {
		return nil, err
	}
	lectura, err := leerImportacion(l.ctx, l.svcCtx, in)
	if err != nil {
		return nil, errorDatos(l.Logger, "analizar datos", err)
	}
	res, err := datos.Analizar(l.ctx, lectura.tabla, lectura.mapeo, lectura.huellaArchivo, l.svcCtx.EstudianteRepository)
	if err != nil {
		return nil, errorDatos(l.Logger, "analizar datos", err)
	}
	return respuestaDatos(lectura, res, nil), nil
}
