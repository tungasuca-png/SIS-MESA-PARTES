package logic

import (
	"context"
	"time"

	"estudiantes/estudiantes"
	"estudiantes/internal/authorization"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"
	"estudiantes/internal/validation"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// FASE E-08 — piezas comunes del dominio de DATOS PERSONALES.

const (
	permisoPersonalView   = "estudiantes.personal.view"
	permisoPersonalUpdate = "estudiantes.personal.update"
)

// errPersonalFueraDeUpdate: UpdateEstudiante ya no modifica datos personales.
var errPersonalFueraDeUpdate = validation.Invalid(
	"los datos personales se modifican con UpdateEstudiantePersonal (PATCH /api/estudiantes/:id/personal)")

// requierePermisoPersonal exige "estudiantes.personal.update" además del
// permiso de la operación (alta con datos personales). Fail-closed: un
// error de Auth se propaga, nunca se interpreta como permitido.
func requierePermisoPersonal(ctx context.Context, svcCtx *svc.ServiceContext, userID string) error {
	allowed, err := authorization.HasPermission(ctx, svcCtx.AuthClient, userID, permisoPersonalUpdate)
	if err != nil {
		return err
	}
	if !allowed {
		return status.Error(codes.PermissionDenied,
			"permission_denied: los datos personales requieren el permiso estudiantes.personal.update")
	}
	return nil
}

// validarPersonal valida el estado FINAL del dominio (tras combinar la
// edición parcial con lo guardado): reglas de E-03/E-05 más domicilio y
// contacto (modelo maestro, etapa 2).
func validarPersonal(p *repository.Personal) error {
	for _, err := range []error{
		validation.FechaNacimiento(p.FechaNacimiento, time.Now()),
		validation.Sexo(p.Sexo),
		validation.Nacimiento(p.PaisNacimiento, p.UbigeoNacimiento),
		validation.Lenguas(p.LenguaMaterna, p.SegundaLengua),
		validation.Domicilio(p.DomicilioDireccion, p.DomicilioDepartamento, p.DomicilioProvincia,
			p.DomicilioDistrito, p.DomicilioUbigeo),
		validation.Contacto(p.Correo, p.Telefono),
	} {
		if err != nil {
			return err
		}
	}
	return nil
}

// camposPersonalesModificados devuelve SOLO nombres de columna (auditoría).
func camposPersonalesModificados(a, b *repository.Personal) []string {
	eq := func(x, y *string) bool {
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		return *x == *y
	}
	var out []string
	pares := []struct {
		x, y *string
		name string
	}{
		{a.FechaNacimiento, b.FechaNacimiento, "fecha_nacimiento"},
		{a.Sexo, b.Sexo, "sexo"},
		{a.PaisNacimiento, b.PaisNacimiento, "pais_nacimiento"},
		{a.UbigeoNacimiento, b.UbigeoNacimiento, "ubigeo_nacimiento"},
		{a.LenguaMaterna, b.LenguaMaterna, "lengua_materna"},
		{a.SegundaLengua, b.SegundaLengua, "segunda_lengua"},
		{a.DomicilioDireccion, b.DomicilioDireccion, "domicilio_direccion"},
		{a.DomicilioDepartamento, b.DomicilioDepartamento, "domicilio_departamento"},
		{a.DomicilioProvincia, b.DomicilioProvincia, "domicilio_provincia"},
		{a.DomicilioDistrito, b.DomicilioDistrito, "domicilio_distrito"},
		{a.DomicilioUbigeo, b.DomicilioUbigeo, "domicilio_ubigeo"},
		{a.Correo, b.Correo, "correo"},
		{a.Telefono, b.Telefono, "telefono"},
	}
	for _, par := range pares {
		if !eq(par.x, par.y) {
			out = append(out, par.name)
		}
	}
	return out
}

// personalToProto: edad calculada a la fecha de referencia (nunca guardada);
// ausente si falta la fecha de nacimiento o si la referencia es anterior.
func personalToProto(id string, p *repository.Personal, referencia time.Time) *estudiantes.EstudiantePersonal {
	v := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}
	out := &estudiantes.EstudiantePersonal{
		Id:                    id,
		Version:               int32(p.Version),
		FechaNacimiento:       v(p.FechaNacimiento),
		Sexo:                  v(p.Sexo),
		PaisNacimiento:        v(p.PaisNacimiento),
		UbigeoNacimiento:      v(p.UbigeoNacimiento),
		LenguaMaterna:         v(p.LenguaMaterna),
		SegundaLengua:         v(p.SegundaLengua),
		DomicilioDireccion:    v(p.DomicilioDireccion),
		DomicilioDepartamento: v(p.DomicilioDepartamento),
		DomicilioProvincia:    v(p.DomicilioProvincia),
		DomicilioDistrito:     v(p.DomicilioDistrito),
		DomicilioUbigeo:       v(p.DomicilioUbigeo),
		Correo:                v(p.Correo),
		Telefono:              v(p.Telefono),
	}
	if p.FechaNacimiento != nil {
		if nac, err := time.Parse("2006-01-02", *p.FechaNacimiento); err == nil {
			if edad, ok := validation.Edad(nac, referencia); ok {
				e := int32(edad)
				out.Edad = &e
			}
		}
	}
	return out
}
