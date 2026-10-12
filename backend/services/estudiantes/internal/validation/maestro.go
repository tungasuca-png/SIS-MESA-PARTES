package validation

import (
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// FASE E-05 — reglas de escritura del maestro. Replican los CHECK de las
// migraciones 001/002 para responder con un error claro ANTES de llegar a la
// base (que sigue siendo la última defensa). Los mensajes nunca incluyen el
// valor recibido: pueden ser un DNI o un nombre.

// Códigos de error estables (prefijo del mensaje gRPC).
const (
	CodeValidation = "validation_error"
	CodeDocumento  = "invalid_document"
	CodeEstado     = "invalid_state"
)

type Error struct {
	Code string
	Msg  string
}

func (e *Error) Error() string { return e.Code + ": " + e.Msg }

func invalid(msg string) error   { return &Error{Code: CodeValidation, Msg: msg} }
func documento(msg string) error { return &Error{Code: CodeDocumento, Msg: msg} }
func EstadoInvalido() error {
	return &Error{Code: CodeEstado, Msg: "estado inválido: use ACTIVO o INACTIVO"}
}
func Invalid(msg string) error { return invalid(msg) }

// Tipos de documento (catálogo cat_tipos_documento, migración 002).
const (
	TipoDNI          = "DNI"
	TipoCE           = "CE"
	TipoPasaporte    = "PASAPORTE"
	TipoSinDocumento = "SIN_DOCUMENTO"
)

// Longitudes de columna (VARCHAR de las migraciones 001/002).
const (
	MaxCodigo    = 20
	MaxNombres   = 100
	MaxApellidos = 100
	MaxApellido  = 60
	MaxFotoRef   = 200
)

var (
	dniPattern       = regexp.MustCompile(`^[0-9]{8}$`)
	cePasPattern     = regexp.MustCompile(`^[A-Z0-9]{4,12}$`)
	siagiePattern    = regexp.MustCompile(`^[0-9]{14}$`)
	paisPattern      = regexp.MustCompile(`^[A-Z]{2}$`)
	ubigeoPattern    = regexp.MustCompile(`^[0-9]{6}$`)
	lenguaPattern    = regexp.MustCompile(`^[A-Z0-9_]{1,10}$`)
	urlSchemePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*://`)
)

// CleanText recorta y colapsa espacios internos ("  José   Luis " -> "José
// Luis"). NO cambia mayúsculas/minúsculas ni separa palabras: "DE LA CRUZ"
// se conserva tal cual.
func CleanText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// Optional convierte un texto ya normalizado en *string: "" = NULL.
func Optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Upper recorta y pasa a mayúsculas (códigos: tipo y número de documento,
// sexo, país, lenguas, estado).
func Upper(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

// Texto obligatorio (nombres, apellidos, código): no vacío, largo máximo y
// sin caracteres de control.
func RequiredText(campo, v string, max int) error {
	if v == "" {
		return invalid(campo + " es obligatorio")
	}
	return OptionalText(campo, &v, max)
}

func OptionalText(campo string, v *string, max int) error {
	if v == nil {
		return nil
	}
	if utf8.RuneCountInString(*v) > max {
		return invalid(campo + " es demasiado largo")
	}
	if hasControl(*v) {
		return invalid(campo + " contiene caracteres no permitidos")
	}
	return nil
}

// Documento aplica las reglas de E-03: SIN_DOCUMENTO sin número; DNI con 8
// dígitos; CE/PASAPORTE alfanumérico en mayúsculas de 4 a 12 caracteres.
func Documento(tipo string, numero *string) error {
	switch tipo {
	case TipoSinDocumento:
		if numero != nil {
			return documento("SIN_DOCUMENTO no admite número de documento")
		}
	case TipoDNI:
		if numero == nil || !dniPattern.MatchString(*numero) {
			return documento("el DNI debe tener 8 dígitos")
		}
	case TipoCE, TipoPasaporte:
		if numero == nil || !cePasPattern.MatchString(*numero) {
			return documento("el número de " + tipo + " debe tener de 4 a 12 letras o dígitos")
		}
	default:
		return documento("tipo de documento inválido: use DNI, CE, PASAPORTE o SIN_DOCUMENTO")
	}
	return nil
}

// CodigoSiagie: opcional; si viene, 14 dígitos (formato provisional de E-03,
// pendiente de confirmación institucional).
func CodigoSiagie(v *string) error {
	if v != nil && !siagiePattern.MatchString(*v) {
		return invalid("codigo_siagie debe tener 14 dígitos")
	}
	return nil
}

// FechaNacimiento: AAAA-MM-DD, fecha real, no futura y no anterior a 1900.
func FechaNacimiento(v *string, hoy time.Time) error {
	if v == nil {
		return nil
	}
	f, err := time.Parse("2006-01-02", *v)
	if err != nil {
		return invalid("fecha_nacimiento debe tener el formato AAAA-MM-DD")
	}
	if f.Year() < 1900 || f.After(hoy) {
		return invalid("fecha_nacimiento fuera de rango")
	}
	return nil
}

func Sexo(v *string) error {
	if v != nil && *v != "M" && *v != "F" {
		return invalid("sexo debe ser M o F")
	}
	return nil
}

// Nacimiento: país ISO alfa-2; el ubigeo (6 dígitos) solo con país PE.
func Nacimiento(pais, ubigeo *string) error {
	if pais != nil && !paisPattern.MatchString(*pais) {
		return invalid("pais_nacimiento debe ser un código de 2 letras")
	}
	if ubigeo != nil {
		if !ubigeoPattern.MatchString(*ubigeo) {
			return invalid("ubigeo_nacimiento debe tener 6 dígitos")
		}
		if pais == nil || *pais != "PE" {
			return invalid("ubigeo_nacimiento solo aplica con pais_nacimiento PE")
		}
	}
	return nil
}

func Lenguas(materna, segunda *string) error {
	if materna != nil && !lenguaPattern.MatchString(*materna) {
		return invalid("lengua_materna tiene un formato inválido")
	}
	if segunda != nil && !lenguaPattern.MatchString(*segunda) {
		return invalid("segunda_lengua tiene un formato inválido")
	}
	if materna != nil && segunda != nil && *materna == *segunda {
		return invalid("segunda_lengua debe ser distinta de lengua_materna")
	}
	return nil
}

// FotoRef: solo una referencia interna (sin esquema de URL). E-05 no sube ni
// guarda archivos.
func FotoRef(v *string) error {
	if v == nil {
		return nil
	}
	if err := OptionalText("foto_ref", v, MaxFotoRef); err != nil {
		return err
	}
	if urlSchemePattern.MatchString(*v) {
		return invalid("foto_ref debe ser una referencia interna, no una URL")
	}
	return nil
}

// ---- Modelo maestro: domicilio, contacto y edad ----

const (
	MaxDireccion  = 200
	MaxUbicacion  = 60
	MaxCorreo     = 254
	MaxParentesco = 20
)

var (
	correoPattern     = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	telefonoPattern   = regexp.MustCompile(`^\+?[0-9][0-9 -]{5,18}$`)
	parentescoPattern = regexp.MustCompile(`^[A-Z_]{2,20}$`)
)

// Domicilio: dirección y nombres territoriales como texto (sin catálogo
// todavía); el ubigeo es opcional y nunca se deduce de los nombres.
func Domicilio(direccion, departamento, provincia, distrito, ubigeo *string) error {
	for _, c := range []struct {
		campo string
		v     *string
		max   int
	}{
		{"domicilio_direccion", direccion, MaxDireccion},
		{"domicilio_departamento", departamento, MaxUbicacion},
		{"domicilio_provincia", provincia, MaxUbicacion},
		{"domicilio_distrito", distrito, MaxUbicacion},
	} {
		if err := OptionalText(c.campo, c.v, c.max); err != nil {
			return err
		}
	}
	if ubigeo != nil && !ubigeoPattern.MatchString(*ubigeo) {
		return invalid("domicilio_ubigeo debe tener 6 dígitos")
	}
	return nil
}

// Contacto: se valida el formato sin reescribir el valor (solo se recorta).
func Contacto(correo, telefono *string) error {
	if correo != nil && (utf8.RuneCountInString(*correo) > MaxCorreo || !correoPattern.MatchString(*correo)) {
		return invalid("correo no tiene un formato válido")
	}
	if telefono != nil && !telefonoPattern.MatchString(*telefono) {
		return invalid("telefono debe tener de 7 a 20 dígitos (se admiten +, espacios y guiones)")
	}
	return nil
}

// Fecha valida AAAA-MM-DD (fecha real) y la devuelve.
func Fecha(campo, v string) (time.Time, error) {
	f, err := time.Parse("2006-01-02", v)
	if err != nil {
		return time.Time{}, invalid(campo + " debe tener el formato AAAA-MM-DD")
	}
	return f, nil
}

// Edad calcula los años cumplidos a la fecha de referencia. ok=false si la
// referencia es anterior al nacimiento (no se inventa una edad). La edad
// nunca se almacena.
func Edad(nacimiento, referencia time.Time) (int, bool) {
	if referencia.Before(nacimiento) {
		return 0, false
	}
	edad := referencia.Year() - nacimiento.Year()
	if referencia.Month() < nacimiento.Month() ||
		(referencia.Month() == nacimiento.Month() && referencia.Day() < nacimiento.Day()) {
		edad--
	}
	return edad, true
}

// HoyLima: fecha de referencia por defecto (zona horaria de la institución).
func HoyLima() time.Time {
	loc, err := time.LoadLocation("America/Lima")
	if err != nil {
		loc = time.FixedZone("PET", -5*3600)
	}
	y, m, d := time.Now().In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// Parentesco: código del catálogo cat_parentescos (la existencia la
// garantiza la FK; aquí solo el formato).
func Parentesco(v string) error {
	if !parentescoPattern.MatchString(v) {
		return invalid("parentesco inválido")
	}
	return nil
}
