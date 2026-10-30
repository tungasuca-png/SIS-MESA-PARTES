package logic

import (
	"strings"

	"estudiantes/estudiantes"
	"estudiantes/internal/repository"
	"estudiantes/internal/validation"
)

// Modelo maestro, etapa 4 — piezas comunes del dominio FAMILIAR.

const (
	permisoFamiliaView   = "estudiantes.familia.view"
	permisoFamiliaUpdate = "estudiantes.familia.update"
)

// personaDatos normaliza y valida los datos de una persona relacionada con
// las mismas reglas de documento y nombres que el estudiante. No inventa
// datos: lo ausente queda en NULL.
func personaDatos(in *estudiantes.Persona) (*repository.PersonaDatos, error) {
	if in == nil {
		return nil, validation.Invalid("indique persona_id o los datos de la persona")
	}
	d := &repository.PersonaDatos{
		TipoDocumento:   validation.Upper(in.TipoDocumento),
		NumeroDocumento: optUpper(in.NumeroDocumento),
		Nombres:         validation.CleanText(in.Nombres),
		ApellidoPaterno: optClean(in.ApellidoPaterno),
		ApellidoMaterno: optClean(in.ApellidoMaterno),
	}
	if d.TipoDocumento == "" {
		d.TipoDocumento = validation.TipoSinDocumento
	}
	if err := validarPersona(d); err != nil {
		return nil, err
	}
	return d, nil
}

// validarPersona (etapa 5) valida el estado FINAL de una persona (alta o
// edición) con las reglas de documento y nombres del estudiante.
func validarPersona(d *repository.PersonaDatos) error {
	for _, err := range []error{
		validation.RequiredText("persona.nombres", d.Nombres, validation.MaxNombres),
		validation.OptionalText("persona.apellido_paterno", d.ApellidoPaterno, validation.MaxApellido),
		validation.OptionalText("persona.apellido_materno", d.ApellidoMaterno, validation.MaxApellido),
		validation.Documento(d.TipoDocumento, d.NumeroDocumento),
	} {
		if err != nil {
			return err
		}
	}
	return nil
}

// fechaOpcional: "" = nil; si viene, AAAA-MM-DD válida y no anterior a 1900.
func fechaOpcional(campo, v string) (*string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	f, err := validation.Fecha(campo, v)
	if err != nil {
		return nil, err
	}
	if f.Year() < 1900 {
		return nil, validation.Invalid(campo + " fuera de rango")
	}
	return &v, nil
}

func familiarToProto(f *repository.Familiar) *estudiantes.Familiar {
	return &estudiantes.Familiar{
		RelacionId: f.RelacionID,
		Persona: &estudiantes.Persona{
			Id:              f.Persona.ID,
			TipoDocumento:   f.Persona.TipoDocumento,
			NumeroDocumento: strOrEmpty(f.Persona.NumeroDocumento),
			Nombres:         f.Persona.Nombres,
			ApellidoPaterno: strOrEmpty(f.Persona.ApellidoPaterno),
			ApellidoMaterno: strOrEmpty(f.Persona.ApellidoMaterno),
		},
		Parentesco:  f.Parentesco,
		EsApoderado: f.EsApoderado,
		Vigente:     f.Vigente,
		Desde:       strOrEmpty(f.Desde),
		Hasta:       strOrEmpty(f.Hasta),
	}
}
