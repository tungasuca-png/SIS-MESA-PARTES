package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"
	"estudiantes/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
)

type MatricularEstudianteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMatricularEstudianteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MatricularEstudianteLogic {
	return &MatricularEstudianteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// MatricularEstudiante (etapa 5): una matrícula por estudiante y periodo
// ("estudiantes.matricula.update"). Exige registro ACTIVO, periodo no
// CERRADO y, si se indica sección, que sea del periodo y tenga vacantes.
// Auditado como MATRICULA_UPDATE en el estudiante.
func (l *MatricularEstudianteLogic) MatricularEstudiante(in *estudiantes.MatricularEstudianteRequest) (*estudiantes.MatriculaResponse, error) {
	userID, err := autorizar(l.ctx, l.svcCtx, permisoMatriculaUpdate)
	if err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.MatricularEstudianteRequest{}
	}
	m := repository.MatriculaInput{}
	if m.EstudianteID, err = validarID(in.EstudianteId); err != nil {
		return nil, matriculaError(l.Logger, "matricular", err)
	}
	if m.PeriodoID, err = validarID(in.PeriodoId); err != nil {
		return nil, matriculaError(l.Logger, "matricular", err)
	}
	if m.SeccionID, err = idOpcional(in.SeccionId); err != nil {
		return nil, matriculaError(l.Logger, "matricular", err)
	}
	if m.FechaMatricula, err = validation.FechaMovimiento("fecha_matricula", in.FechaMatricula, validation.HoyLima()); err != nil {
		return nil, matriculaError(l.Logger, "matricular", err)
	}
	out, err := l.svcCtx.EstudianteRepository.MatricularEstudiante(l.ctx, m, repository.Auditoria{UsuarioID: userID})
	if err != nil {
		return nil, matriculaError(l.Logger, "matricular", err)
	}
	return &estudiantes.MatriculaResponse{Matricula: matriculaToProto(out)}, nil
}
