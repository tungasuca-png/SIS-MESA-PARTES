package logic

import (
	"context"
	"errors"
	"strings"
	"time"

	"estudiantes/estudiantes"
	"estudiantes/internal/authorization"
	"estudiantes/internal/interceptor"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"
	"estudiantes/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// FASE E-05 — piezas comunes de la escritura del maestro.

// autorizar (E-05; E-08 también para la lectura personal) exige identidad autenticada (del JWT ya validado por el
// interceptor, nunca de un campo de la solicitud) y el permiso indicado. El
// userID devuelto es el que queda en la auditoría.
func autorizar(ctx context.Context, svcCtx *svc.ServiceContext, permiso string) (string, error) {
	userID, _, ok := interceptor.UserFromContext(ctx)
	if !ok || !validation.IsValidUUID(userID) {
		return "", status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if err := authorization.RequirePermission(ctx, svcCtx.AuthClient, userID, permiso); err != nil {
		return "", err
	}
	return userID, nil
}

// joinApellidos deriva apellidos de los separados (regla de consistencia de
// E-03: apellidos = paterno + " " + materno). Nunca se hace lo inverso.
func joinApellidos(paterno, materno *string) string {
	out := *paterno
	if materno != nil {
		out += " " + *materno
	}
	return out
}

// resolverApellidos fija m.Apellidos y verifica la coherencia:
//   - con apellido_paterno, apellidos se deriva; si el cliente también lo
//     envía (enviado != nil) debe coincidir exactamente — no se corrige en
//     silencio;
//   - sin apellido_paterno no puede haber apellido_materno, y apellidos es el
//     que envía el cliente (o el que ya tenía el registro).
func resolverApellidos(m *repository.Maestro, enviado *string) error {
	if m.ApellidoPaterno != nil {
		derivado := joinApellidos(m.ApellidoPaterno, m.ApellidoMaterno)
		if enviado != nil && validation.CleanText(*enviado) != "" && validation.CleanText(*enviado) != derivado {
			return validation.Invalid("apellidos no coincide con apellido_paterno + apellido_materno")
		}
		m.Apellidos = derivado
		return nil
	}
	if m.ApellidoMaterno != nil {
		return validation.Invalid("apellido_materno requiere apellido_paterno")
	}
	if enviado != nil {
		m.Apellidos = validation.CleanText(*enviado)
	}
	return nil
}

// validarMaestro valida el estado FINAL del registro (tras normalizar en el
// alta o combinar en la edición): todas las reglas a la vez.
func validarMaestro(m *repository.Maestro) error {
	checks := []error{
		validation.RequiredText("codigo", m.Codigo, validation.MaxCodigo),
		validation.RequiredText("nombres", m.Nombres, validation.MaxNombres),
		validation.OptionalText("apellido_paterno", m.ApellidoPaterno, validation.MaxApellido),
		validation.OptionalText("apellido_materno", m.ApellidoMaterno, validation.MaxApellido),
		validation.RequiredText("apellidos", m.Apellidos, validation.MaxApellidos),
		validation.Documento(m.TipoDocumento, m.NumeroDocumento),
		validation.CodigoSiagie(m.CodigoSiagie),
		validation.FechaNacimiento(m.FechaNacimiento, time.Now()),
		validation.Sexo(m.Sexo),
		validation.Nacimiento(m.PaisNacimiento, m.UbigeoNacimiento),
		validation.Lenguas(m.LenguaMaterna, m.SegundaLengua),
		validation.FotoRef(m.FotoRef),
	}
	for _, err := range checks {
		if err != nil {
			return err
		}
	}
	if !validation.IsValidEstadoRegistro(m.Estado) {
		return validation.EstadoInvalido()
	}
	return nil
}

// camposModificados compara antes/después y devuelve SOLO nombres de columna
// (van a la auditoría; nunca los valores).
func camposModificados(a, b *repository.Maestro) []string {
	eq := func(x, y *string) bool {
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		return *x == *y
	}
	var out []string
	add := func(changed bool, name string) {
		if changed {
			out = append(out, name)
		}
	}
	add(a.Codigo != b.Codigo, "codigo")
	add(!eq(a.CodigoSiagie, b.CodigoSiagie), "codigo_siagie")
	add(a.TipoDocumento != b.TipoDocumento, "tipo_documento")
	add(!eq(a.NumeroDocumento, b.NumeroDocumento), "numero_documento")
	add(a.Nombres != b.Nombres, "nombres")
	add(!eq(a.ApellidoPaterno, b.ApellidoPaterno), "apellido_paterno")
	add(!eq(a.ApellidoMaterno, b.ApellidoMaterno), "apellido_materno")
	add(a.Apellidos != b.Apellidos, "apellidos")
	add(!eq(a.FechaNacimiento, b.FechaNacimiento), "fecha_nacimiento")
	add(!eq(a.Sexo, b.Sexo), "sexo")
	add(!eq(a.PaisNacimiento, b.PaisNacimiento), "pais_nacimiento")
	add(!eq(a.UbigeoNacimiento, b.UbigeoNacimiento), "ubigeo_nacimiento")
	add(!eq(a.LenguaMaterna, b.LenguaMaterna), "lengua_materna")
	add(!eq(a.SegundaLengua, b.SegundaLengua), "segunda_lengua")
	add(!eq(a.FotoRef, b.FotoRef), "foto_ref")
	add(a.Estado != b.Estado, "estado")
	return out
}

// Normalizadores por tipo de campo (vacío = NULL en los opcionales).
func optTrim(s string) *string  { return validation.Optional(strings.TrimSpace(s)) }
func optUpper(s string) *string { return validation.Optional(validation.Upper(s)) }
func optClean(s string) *string { return validation.Optional(validation.CleanText(s)) }

func validarVersion(v int32) error {
	if v < 1 {
		return validation.Invalid("expected_version es obligatorio")
	}
	return nil
}

func validarID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if !validation.IsValidUUID(id) {
		return "", validation.Invalid("el identificador no es válido")
	}
	return id, nil
}

// escrituraError traduce los errores de dominio a gRPC con un código estable
// al inicio del mensaje. Lo inesperado se registra resumido (SQLSTATE y
// restricción, nunca valores) y sale como Internal genérico.
func escrituraError(log logx.Logger, op string, err error) error {
	return errorSeguro(log, op, "no se pudo guardar el estudiante", err)
}

// errorSeguro (FASE E-08): misma traducción que escrituraError con un mensaje
// genérico propio para lo inesperado (p. ej. una lectura).
func errorSeguro(log logx.Logger, op, mensajeInterno string, err error) error {
	var vErr *validation.Error
	switch {
	case errors.As(err, &vErr):
		return status.Error(codes.InvalidArgument, vErr.Error())
	case errors.Is(err, repository.ErrEstudianteNotFound):
		return status.Error(codes.NotFound, "student_not_found: estudiante no encontrado")
	case errors.Is(err, repository.ErrCodigoExists):
		return status.Error(codes.AlreadyExists, "duplicate_codigo: ya existe un estudiante con ese código")
	case errors.Is(err, repository.ErrDocumentoExists):
		return status.Error(codes.AlreadyExists, "duplicate_document: ya existe un estudiante con ese tipo y número de documento")
	case errors.Is(err, repository.ErrCodigoSiagieExists):
		return status.Error(codes.AlreadyExists, "duplicate_codigo_siagie: ya existe un estudiante con ese código SIAGIE")
	case errors.Is(err, repository.ErrDocumentoInvalido):
		return status.Error(codes.InvalidArgument, "invalid_document: el tipo y número de documento no son válidos")
	case errors.Is(err, repository.ErrDatosInvalidos):
		return status.Error(codes.InvalidArgument, "validation_error: los datos del estudiante no son válidos")
	case errors.Is(err, repository.ErrVersionConflict):
		return status.Error(codes.Aborted, "version_conflict: el estudiante fue modificado por otro usuario; vuelva a cargarlo")
	// Modelo maestro — dominio familiar.
	case errors.Is(err, repository.ErrPersonaNotFound):
		return status.Error(codes.NotFound, "persona_not_found: la persona no existe")
	case errors.Is(err, repository.ErrPersonaDatosDistintos):
		return status.Error(codes.FailedPrecondition, "persona_mismatch: ya existe una persona con ese documento y otros nombres; revise los datos")
	case errors.Is(err, repository.ErrPersonaDuplicada):
		return status.Error(codes.Aborted, "persona_conflict: la persona se registró al mismo tiempo; vuelva a intentarlo")
	case errors.Is(err, repository.ErrFamiliarYaVinculado):
		return status.Error(codes.AlreadyExists, "duplicate_familiar: esa persona ya está vinculada al estudiante")
	case errors.Is(err, repository.ErrProgenitorDuplicado):
		return status.Error(codes.AlreadyExists, "duplicate_progenitor: el estudiante ya tiene ese parentesco (padre o madre) vigente")
	case errors.Is(err, repository.ErrApoderadoDuplicado):
		return status.Error(codes.AlreadyExists, "duplicate_apoderado: el estudiante ya tiene un apoderado vigente")
	case errors.Is(err, repository.ErrParentescoInvalido):
		return status.Error(codes.InvalidArgument, "invalid_parentesco: parentesco no registrado en el catálogo")
	case errors.Is(err, repository.ErrRelacionNotFound):
		return status.Error(codes.NotFound, "relacion_not_found: la relación no existe para este estudiante")
	case errors.Is(err, repository.ErrRelacionNoVigente):
		return status.Error(codes.FailedPrecondition, "relacion_no_vigente: la relación ya estaba cerrada")
	case errors.Is(err, repository.ErrMismoEstado):
		return status.Error(codes.FailedPrecondition, "invalid_state: el estudiante ya tiene ese estado")
	}
	log.Errorf("%s estudiante: %s", op, repository.SafeErrorSummary(err))
	return status.Error(codes.Internal, mensajeInterno)
}

func respuestaBasica(e *repository.Estudiante) *estudiantes.EstudianteBasico {
	return toProtoBasic(e)
}
