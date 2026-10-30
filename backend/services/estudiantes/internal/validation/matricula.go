package validation

import (
	"time"
)

// Modelo maestro, etapa 5 — reglas de periodos, secciones y matrículas.
// Replican los CHECK de la migración 007 para responder con un error claro
// antes de llegar a la base. No se agregan reglas académicas que el colegio
// no haya definido (aprobación, promoción, edades por grado, etc.).

const (
	MinAnio          = 2000
	MaxAnio          = 2100
	MaxNombrePeriodo = 60
	MaxGrado         = 20
	MaxSeccion       = 10
	MaxTurno         = 20
	MaxAula          = 40
	MaxMotivo        = 200
	MaxVacantes      = 32767
)

// Niveles (CHECK chk_secciones_nivel).
var niveles = map[string]bool{"INICIAL": true, "PRIMARIA": true, "SECUNDARIA": true}

func Nivel(v string) error {
	if !niveles[v] {
		return invalid("nivel inválido: use INICIAL, PRIMARIA o SECUNDARIA")
	}
	return nil
}

func Anio(v int32) error {
	if v < MinAnio || v > MaxAnio {
		return invalid("anio fuera de rango")
	}
	return nil
}

func Vacantes(v *int32) error {
	if v != nil && (*v < 0 || *v > MaxVacantes) {
		return invalid("vacantes fuera de rango")
	}
	return nil
}

// RangoFechas: fin no anterior a inicio (si ambas existen).
func RangoFechas(inicio, fin *string) error {
	if inicio != nil && fin != nil && *fin < *inicio {
		return invalid("fecha_fin no puede ser anterior a fecha_inicio")
	}
	return nil
}

// FechaMovimiento: AAAA-MM-DD válida, no anterior a 1900 y no futura respecto
// de hoy (un movimiento registra un hecho ocurrido). Vacía = hoy.
func FechaMovimiento(campo, v string, hoy time.Time) (string, error) {
	if v == "" {
		return hoy.Format("2006-01-02"), nil
	}
	f, err := Fecha(campo, v)
	if err != nil {
		return "", err
	}
	if f.Year() < 1900 {
		return "", invalid(campo + " fuera de rango")
	}
	if f.After(hoy) {
		return "", invalid(campo + " no puede ser futura")
	}
	return v, nil
}
