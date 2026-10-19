package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"
	"estudiantes/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChangeEstadoPeriodoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChangeEstadoPeriodoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangeEstadoPeriodoLogic {
	return &ChangeEstadoPeriodoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ChangeEstadoPeriodo (etapa 5): PLANIFICADO -> EN_CURSO -> CERRADO, solo
// hacia adelante ("estudiantes.matricula.update"). Un periodo CERRADO ya no
// admite secciones ni cambios de matrícula (salvo su conclusión).
func (l *ChangeEstadoPeriodoLogic) ChangeEstadoPeriodo(in *estudiantes.ChangeEstadoPeriodoRequest) (*estudiantes.PeriodoResponse, error) {
	if _, err := autorizar(l.ctx, l.svcCtx, permisoMatriculaUpdate); err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.ChangeEstadoPeriodoRequest{}
	}
	id, err := validarID(in.Id)
	if err != nil {
		return nil, matriculaError(l.Logger, "cambiar estado de periodo", err)
	}
	estado := validation.Upper(in.Estado)
	if estado != repository.PeriodoEnCurso && estado != repository.PeriodoCerrado {
		return nil, matriculaError(l.Logger, "cambiar estado de periodo", validation.Invalid("estado inválido: use EN_CURSO o CERRADO"))
	}
	p, err := l.svcCtx.EstudianteRepository.ChangeEstadoPeriodo(l.ctx, id, estado)
	if err != nil {
		return nil, matriculaError(l.Logger, "cambiar estado de periodo", err)
	}
	return &estudiantes.PeriodoResponse{Periodo: periodoToProto(p)}, nil
}
