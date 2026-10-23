package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type PreviewImportacionEstudiantesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPreviewImportacionEstudiantesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PreviewImportacionEstudiantesLogic {
	return &PreviewImportacionEstudiantesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PreviewImportacionEstudiantes valida el Excel y clasifica cada fila sin
// escribir nada. Exige estudiantes.create: solo quien puede dar de alta importa.
func (l *PreviewImportacionEstudiantesLogic) PreviewImportacionEstudiantes(in *estudiantes.ImportarEstudiantesRequest) (*estudiantes.ImportarEstudiantesResponse, error) {
	if _, err := autorizar(l.ctx, l.svcCtx, "estudiantes.create"); err != nil {
		return nil, err
	}
	a, err := analizarImportacion(l.ctx, l.svcCtx, in)
	if err != nil {
		return nil, errorSeguro(l.Logger, "previsualizar importación", "no se pudo analizar el archivo", err)
	}
	return respuestaImportacion(a, false), nil
}
