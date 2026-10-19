package logic

import (
	"strings"

	"estudiantes/estudiantes"
	"estudiantes/internal/repository"
	"estudiantes/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
)

// Modelo maestro, etapa 5 — piezas comunes de periodos, secciones,
// matrículas y personas.

const (
	permisoView            = "estudiantes.view"
	permisoMatriculaUpdate = "estudiantes.matricula.update"
)

func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// paginar aplica los mismos valores por defecto que SearchEstudiantes.
func paginar(page, pageSize int32) (int, int) {
	p, s := int(page), int(pageSize)
	if p <= 0 {
		p = 1
	}
	if s <= 0 || s > maxPageSize {
		s = defaultPageSize
	}
	return p, s
}

func periodoToProto(p *repository.Periodo) *estudiantes.PeriodoAcademico {
	return &estudiantes.PeriodoAcademico{
		Id: p.ID, Anio: int32(p.Anio), Nombre: p.Nombre, Estado: p.Estado,
		FechaInicio: strOrEmpty(p.FechaInicio), FechaFin: strOrEmpty(p.FechaFin),
	}
}

func seccionToProto(s *repository.Seccion) *estudiantes.SeccionAcademica {
	out := &estudiantes.SeccionAcademica{
		Id: s.ID, PeriodoId: s.PeriodoID, Nivel: s.Nivel, Grado: s.Grado, Seccion: s.Seccion,
		Turno: strOrEmpty(s.Turno), Aula: strOrEmpty(s.Aula), Matriculados: int32(s.Matriculados),
	}
	if s.Vacantes != nil {
		v := int32(*s.Vacantes)
		out.Vacantes = &v
	}
	return out
}

// matriculaToProto no expone el motivo de retiro (texto libre, se minimiza).
func matriculaToProto(m *repository.Matricula) *estudiantes.Matricula {
	out := &estudiantes.Matricula{
		Id:              m.ID,
		EstudianteId:    m.EstudianteID,
		Periodo:         periodoToProto(&m.Periodo),
		EstadoMatricula: m.EstadoMatricula,
		FechaMatricula:  strOrEmpty(m.FechaMatricula),
		FechaRetiro:     strOrEmpty(m.FechaRetiro),
		Origen:          m.Origen,
		Version:         int32(m.Version),
	}
	if m.Seccion != nil {
		out.Seccion = seccionToProto(m.Seccion)
	}
	for _, mv := range m.Movimientos {
		out.Movimientos = append(out.Movimientos, &estudiantes.MovimientoMatricula{
			Tipo: mv.Tipo, Fecha: mv.Fecha,
			SeccionAnteriorId: strOrEmpty(mv.SeccionAnteriorID), SeccionNuevaId: strOrEmpty(mv.SeccionNuevaID),
		})
	}
	return out
}

func personaToProto(p *repository.Persona) *estudiantes.Persona {
	return &estudiantes.Persona{
		Id:              p.ID,
		TipoDocumento:   p.TipoDocumento,
		NumeroDocumento: strOrEmpty(p.NumeroDocumento),
		Nombres:         p.Nombres,
		ApellidoPaterno: strOrEmpty(p.ApellidoPaterno),
		ApellidoMaterno: strOrEmpty(p.ApellidoMaterno),
		Version:         int32(p.Version),
	}
}

// motivoOpcional: "" = sin motivo; si viene, texto limpio de máx. 200.
func motivoOpcional(v string) (*string, error) {
	m := validation.Optional(validation.CleanText(v))
	if err := validation.OptionalText("motivo", m, validation.MaxMotivo); err != nil {
		return nil, err
	}
	return m, nil
}

// idOpcional: "" = ausente; si viene, UUID válido.
func idOpcional(v string) (*string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	id, err := validarID(v)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// matriculaError: misma traducción segura que la escritura del maestro, con
// un mensaje propio para lo inesperado.
func matriculaError(log logx.Logger, op string, err error) error {
	return errorSeguro(log, op, "no se pudo completar la operación de matrícula", err)
}
