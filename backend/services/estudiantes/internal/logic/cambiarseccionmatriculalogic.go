package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"
	"estudiantes/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
)

type CambiarSeccionMatriculaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCambiarSeccionMatriculaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CambiarSeccionMatriculaLogic {
	return &CambiarSeccionMatriculaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CambiarSeccionMatricula (etapa 5): asigna o cambia la sección de una
// matrícula MATRICULADO dentro de su periodo; la anterior queda en el
// movimiento ("estudiantes.matricula.update").
func (l *CambiarSeccionMatriculaLogic) CambiarSeccionMatricula(in *estudiantes.CambiarSeccionMatriculaRequest) (*estudiantes.MatriculaResponse, error) {
	userID, err := autorizar(l.ctx, l.svcCtx, permisoMatriculaUpdate)
	if err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.CambiarSeccionMatriculaRequest{}
	}
	id, err := validarID(in.MatriculaId)
	if err != nil {
		return nil, matriculaError(l.Logger, "cambiar sección", err)
	}
	if err := validarVersion(in.ExpectedVersion); err != nil {
		return nil, matriculaError(l.Logger, "cambiar sección", err)
	}
	seccionID, err := validarID(in.SeccionId)
	if err != nil {
		return nil, matriculaError(l.Logger, "cambiar sección", err)
	}
	fecha, err := validation.FechaMovimiento("fecha", in.Fecha, validation.HoyLima())
	if err != nil {
		return nil, matriculaError(l.Logger, "cambiar sección", err)
	}
	motivo, err := motivoOpcional(in.Motivo)
	if err != nil {
		return nil, matriculaError(l.Logger, "cambiar sección", err)
	}
	out, err := l.svcCtx.EstudianteRepository.CambiarSeccion(l.ctx, id, int(in.ExpectedVersion), seccionID, fecha, motivo,
		repository.Auditoria{UsuarioID: userID})
	if err != nil {
		return nil, matriculaError(l.Logger, "cambiar sección", err)
	}
	return &estudiantes.MatriculaResponse{Matricula: matriculaToProto(out)}, nil
}
