package logic

import (
	"estudiantes/estudiantes"
	"estudiantes/internal/repository"
)

func toProto(e *repository.Estudiante) *estudiantes.Estudiante {
	return &estudiantes.Estudiante{
		Id:              e.ID,
		Codigo:          e.Codigo,
		Nombres:         e.Nombres,
		Apellidos:       e.Apellidos,
		Nivel:           e.Nivel,
		Grado:           e.Grado,
		Seccion:         e.Seccion,
		Turno:           e.Turno,
		EstadoMatricula: e.EstadoMatricula,
		Version:         int32(e.Version),
	}
}

func toProtoBasic(e *repository.Estudiante) *estudiantes.EstudianteBasico {
	return &estudiantes.EstudianteBasico{
		Id:              e.ID,
		Codigo:          e.Codigo,
		NombreCompleto:  e.NombreCompleto(),
		Nivel:           e.Nivel,
		Grado:           e.Grado,
		Seccion:         e.Seccion,
		EstadoMatricula: e.EstadoMatricula,
		NumeroDocumento: e.NumeroDocumento,
		// FASE E-04: identificación básica (sin datos personales ni sensibles).
		Nombres:         e.Nombres,
		Apellidos:       e.Apellidos,
		ApellidoPaterno: e.ApellidoPaterno,
		ApellidoMaterno: e.ApellidoMaterno,
		CodigoSiagie:    e.CodigoSiagie,
		TipoDocumento:   e.TipoDocumento,
		Estado:          e.Estado,
		Turno:           e.Turno,
		Version:         int32(e.Version),
	}
}
