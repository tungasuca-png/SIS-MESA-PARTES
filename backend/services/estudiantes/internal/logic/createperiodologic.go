package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"
	"estudiantes/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreatePeriodoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreatePeriodoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePeriodoLogic {
	return &CreatePeriodoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreatePeriodo (etapa 5): registra el periodo académico de un año, en estado
// PLANIFICADO, con "estudiantes.matricula.update".
func (l *CreatePeriodoLogic) CreatePeriodo(in *estudiantes.CreatePeriodoRequest) (*estudiantes.PeriodoResponse, error) {
	if _, err := autorizar(l.ctx, l.svcCtx, permisoMatriculaUpdate); err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.CreatePeriodoRequest{}
	}
	p := repository.PeriodoInput{Anio: int(in.Anio), Nombre: validation.CleanText(in.Nombre)}
	var err error
	if err = validation.Anio(in.Anio); err != nil {
		return nil, matriculaError(l.Logger, "crear periodo", err)
	}
	if err = validation.RequiredText("nombre", p.Nombre, validation.MaxNombrePeriodo); err != nil {
		return nil, matriculaError(l.Logger, "crear periodo", err)
	}
	if p.FechaInicio, err = fechaOpcional("fecha_inicio", in.FechaInicio); err != nil {
		return nil, matriculaError(l.Logger, "crear periodo", err)
	}
	if p.FechaFin, err = fechaOpcional("fecha_fin", in.FechaFin); err != nil {
		return nil, matriculaError(l.Logger, "crear periodo", err)
	}
	if err = validation.RangoFechas(p.FechaInicio, p.FechaFin); err != nil {
		return nil, matriculaError(l.Logger, "crear periodo", err)
	}
	periodo, err := l.svcCtx.EstudianteRepository.CreatePeriodo(l.ctx, p)
	if err != nil {
		return nil, matriculaError(l.Logger, "crear periodo", err)
	}
	return &estudiantes.PeriodoResponse{Periodo: periodoToProto(periodo)}, nil
}
