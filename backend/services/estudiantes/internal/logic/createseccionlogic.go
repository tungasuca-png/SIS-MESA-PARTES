package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"
	"estudiantes/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateSeccionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateSeccionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateSeccionLogic {
	return &CreateSeccionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateSeccion (etapa 5): sección de un periodo no CERRADO, única por
// nivel, grado, sección y turno ("estudiantes.matricula.update"). grado,
// seccion y turno se guardan en mayúsculas (mismo criterio que los códigos).
func (l *CreateSeccionLogic) CreateSeccion(in *estudiantes.CreateSeccionRequest) (*estudiantes.SeccionResponse, error) {
	if _, err := autorizar(l.ctx, l.svcCtx, permisoMatriculaUpdate); err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.CreateSeccionRequest{}
	}
	periodoID, err := validarID(in.PeriodoId)
	if err != nil {
		return nil, matriculaError(l.Logger, "crear sección", err)
	}
	s := repository.SeccionInput{
		PeriodoID: periodoID,
		Nivel:     validation.Upper(in.Nivel),
		Grado:     validation.Upper(validation.CleanText(in.Grado)),
		Seccion:   validation.Upper(validation.CleanText(in.Seccion)),
		Turno:     optUpper(validation.CleanText(in.Turno)),
		Aula:      optClean(in.Aula),
	}
	for _, check := range []error{
		validation.Nivel(s.Nivel),
		validation.RequiredText("grado", s.Grado, validation.MaxGrado),
		validation.RequiredText("seccion", s.Seccion, validation.MaxSeccion),
		validation.OptionalText("turno", s.Turno, validation.MaxTurno),
		validation.OptionalText("aula", s.Aula, validation.MaxAula),
		validation.Vacantes(in.Vacantes),
	} {
		if check != nil {
			return nil, matriculaError(l.Logger, "crear sección", check)
		}
	}
	if in.Vacantes != nil {
		v := int(*in.Vacantes)
		s.Vacantes = &v
	}
	seccion, err := l.svcCtx.EstudianteRepository.CreateSeccion(l.ctx, s)
	if err != nil {
		return nil, matriculaError(l.Logger, "crear sección", err)
	}
	return &estudiantes.SeccionResponse{Seccion: seccionToProto(seccion)}, nil
}
