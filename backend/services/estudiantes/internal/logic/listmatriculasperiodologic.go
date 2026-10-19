package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"
	"estudiantes/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListMatriculasPeriodoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListMatriculasPeriodoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMatriculasPeriodoLogic {
	return &ListMatriculasPeriodoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

var estadosMatricula = map[string]bool{
	repository.MatriculaMatriculado: true, repository.MatriculaRetirado: true,
	repository.MatriculaTrasladado: true, repository.MatriculaConcluido: true,
}

// ListMatriculasPeriodo (etapa 5): nómina paginada de un periodo, filtrable
// por sección y estado, con la identidad básica de cada estudiante
// ("estudiantes.view").
func (l *ListMatriculasPeriodoLogic) ListMatriculasPeriodo(in *estudiantes.ListMatriculasPeriodoRequest) (*estudiantes.ListMatriculasPeriodoResponse, error) {
	if _, err := autorizar(l.ctx, l.svcCtx, permisoView); err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.ListMatriculasPeriodoRequest{}
	}
	f := repository.NominaFilter{Estado: validation.Upper(in.EstadoMatricula)}
	var err error
	if f.PeriodoID, err = validarID(in.PeriodoId); err != nil {
		return nil, matriculaError(l.Logger, "listar nómina", err)
	}
	seccion, err := idOpcional(in.SeccionId)
	if err != nil {
		return nil, matriculaError(l.Logger, "listar nómina", err)
	}
	f.SeccionID = strOrEmpty(seccion)
	if f.Estado != "" && !estadosMatricula[f.Estado] {
		return nil, matriculaError(l.Logger, "listar nómina", validation.Invalid("estado_matricula inválido"))
	}
	f.Page, f.PageSize = paginar(in.Page, in.PageSize)

	matriculas, total, err := l.svcCtx.EstudianteRepository.ListMatriculasPeriodo(l.ctx, f)
	if err != nil {
		return nil, matriculaError(l.Logger, "listar nómina", err)
	}
	ids := make([]string, 0, len(matriculas))
	for _, m := range matriculas {
		ids = append(ids, m.EstudianteID)
	}
	basicos, err := l.svcCtx.EstudianteRepository.GetBasicByIDs(l.ctx, ids)
	if err != nil {
		return nil, matriculaError(l.Logger, "listar nómina", err)
	}
	porID := make(map[string]*estudiantes.EstudianteBasico, len(basicos))
	for _, e := range basicos {
		porID[e.ID] = toProtoBasic(e)
	}
	out := make([]*estudiantes.MatriculaNomina, 0, len(matriculas))
	for i := range matriculas {
		out = append(out, &estudiantes.MatriculaNomina{
			Matricula:  matriculaToProto(&matriculas[i]),
			Estudiante: porID[matriculas[i].EstudianteID],
		})
	}
	return &estudiantes.ListMatriculasPeriodoResponse{Matriculas: out, Total: int32(total)}, nil
}
