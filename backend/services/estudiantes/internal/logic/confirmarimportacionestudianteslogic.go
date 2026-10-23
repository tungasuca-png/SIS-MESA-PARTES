package logic

import (
	"context"
	"strings"

	"estudiantes/estudiantes"
	"estudiantes/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ConfirmarImportacionEstudiantesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewConfirmarImportacionEstudiantesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfirmarImportacionEstudiantesLogic {
	return &ConfirmarImportacionEstudiantesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ConfirmarImportacionEstudiantes vuelve a leer y validar el MISMO archivo de
// la vista previa (huella) e inserta solo las filas válidas. Nunca actualiza
// ni reemplaza estudiantes existentes.
func (l *ConfirmarImportacionEstudiantesLogic) ConfirmarImportacionEstudiantes(in *estudiantes.ImportarEstudiantesRequest) (*estudiantes.ImportarEstudiantesResponse, error) {
	userID, err := autorizar(l.ctx, l.svcCtx, "estudiantes.create")
	if err != nil {
		return nil, err
	}
	if in == nil || strings.TrimSpace(in.Huella) == "" {
		return nil, status.Error(codes.InvalidArgument, "importacion_sin_vista_previa: previsualice el archivo antes de confirmar")
	}
	a, err := analizarImportacion(l.ctx, l.svcCtx, in)
	if err != nil {
		return nil, errorSeguro(l.Logger, "confirmar importación", "no se pudo analizar el archivo", err)
	}
	if a.huella != strings.TrimSpace(in.Huella) {
		return nil, status.Error(codes.FailedPrecondition, "importacion_cambiada: el archivo no es el mismo de la vista previa; vuelva a previsualizarlo")
	}
	if err := confirmarFilas(l.ctx, l.svcCtx, l.Logger, userID, a); err != nil {
		return nil, err
	}
	resp := respuestaImportacion(a, true)
	// Registro de la operación, sin datos personales: quién, qué archivo
	// (huella) y el resultado. Cada alta además queda en auditoria_estudiantes
	// (CREATE, con el usuario), por la misma vía que el formulario individual.
	r := resp.Resumen
	l.Infof("importación Excel de estudiantes: usuario=%s huella=%s filas=%d insertadas=%d omitidas=%d rechazadas=%d",
		userID, a.huella, r.Total, r.Insertadas, r.Omitidas, r.Rechazadas)
	return resp, nil
}
