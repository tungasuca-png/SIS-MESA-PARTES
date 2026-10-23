package excel

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"estudiantes/internal/validation"
)

// Registro es una fila ya normalizada y válida, lista para el maestro.
type Registro struct {
	TipoDocumento   string
	NumeroDocumento *string // nil con SIN_DOCUMENTO
	ApellidoPaterno *string
	ApellidoMaterno *string
	Apellidos       string // paterno + " " + materno (regla del maestro)
	Nombres         string
	Estado          string
	Nivel           string
}

// ClaveDocumento identifica a una persona con documento (índice único
// uq_estudiantes_documento). Vacía con SIN_DOCUMENTO.
func (r Registro) ClaveDocumento() string {
	if r.NumeroDocumento == nil {
		return ""
	}
	return r.TipoDocumento + ":" + *r.NumeroDocumento
}

// NombreCompleto para comparar coincidencias (solo avisos, nunca identidad).
func (r Registro) NombreCompleto() string {
	return r.Apellidos + " " + r.Nombres
}

// Validar normaliza (espacios; mayúsculas solo en códigos) y aplica las
// reglas del maestro. Devuelve todos los errores de la fila, no solo el
// primero, para que el usuario corrija de una vez.
func Validar(f Fila) (Registro, []string) {
	var errs []string
	r := Registro{
		TipoDocumento: tipoDocumento(f.TipoDocumento),
		Nombres:       validation.CleanText(f.Nombres),
		Estado:        validation.Upper(f.Estado),
		Nivel:         validation.Upper(f.Nivel),
	}
	paterno := validation.CleanText(f.ApellidoPaterno)
	materno := validation.CleanText(f.ApellidoMaterno)
	documento := strings.ToUpper(strings.Join(strings.Fields(f.Documento), ""))

	for _, c := range []struct{ campo, valor string }{
		{"TIPO.DOC", r.TipoDocumento}, {"DNI", documento}, {"APELLIDO PATERNO", paterno},
		{"APELLIDO MATERNO", materno}, {"NOMBRES", r.Nombres},
	} {
		// Un texto que empieza con "=" es una fórmula de Excel, no un dato.
		if strings.HasPrefix(c.valor, "=") {
			errs = append(errs, c.campo+": no se admiten fórmulas")
		}
	}

	// TIPO.DOC vacío con DNI vacío = sin documento; con número, se exige el tipo.
	if r.TipoDocumento == "" && documento == "" {
		r.TipoDocumento = validation.TipoSinDocumento
	}
	if !contiene(TiposDocumento, r.TipoDocumento) {
		errs = append(errs, "TIPO.DOC inválido: use "+strings.Join(TiposDocumento, ", "))
	} else {
		r.NumeroDocumento = validation.Optional(documento)
		if err := validation.Documento(r.TipoDocumento, r.NumeroDocumento); err != nil {
			msg := mensaje(err)
			if r.TipoDocumento == validation.TipoDNI && esNumero(documento) && len(documento) < 8 {
				msg += " (si empieza con 0, escríbalo como texto en Excel para no perder los ceros)"
			}
			errs = append(errs, "DNI: "+msg)
		}
	}

	if paterno == "" {
		errs = append(errs, "APELLIDO PATERNO es obligatorio")
	} else if err := validation.OptionalText("APELLIDO PATERNO", &paterno, validation.MaxApellido); err != nil {
		errs = append(errs, mensaje(err))
	}
	r.ApellidoPaterno = validation.Optional(paterno)
	r.ApellidoMaterno = validation.Optional(materno)
	if err := validation.OptionalText("APELLIDO MATERNO", r.ApellidoMaterno, validation.MaxApellido); err != nil {
		errs = append(errs, mensaje(err))
	}
	if r.ApellidoPaterno != nil {
		r.Apellidos = *r.ApellidoPaterno
		if r.ApellidoMaterno != nil {
			r.Apellidos += " " + *r.ApellidoMaterno
		}
	}
	if err := validation.RequiredText("NOMBRES", r.Nombres, validation.MaxNombres); err != nil {
		errs = append(errs, mensaje(err))
	}

	if c := validation.Upper(f.Condicion); c != Condicion {
		errs = append(errs, "CONDICION debe ser "+Condicion)
	}
	if !contiene(Estados, r.Estado) {
		errs = append(errs, "ESTADO inválido: use "+strings.Join(Estados, " o "))
	}
	if !contiene(Niveles, r.Nivel) {
		errs = append(errs, "NIVEL inválido: use "+strings.Join(Niveles, ", "))
	}
	return r, errs
}

// aliasTipoDocumento: nombres con que otros sistemas (p. ej. el reporte de
// usuarios del colegio) escriben un tipo del catálogo. Solo equivalencias
// exactas; cualquier otro valor se rechaza.
var aliasTipoDocumento = map[string]string{
	"CARNE EXT.":            validation.TipoCE,
	"CARNE EXT":             validation.TipoCE,
	"CARNET EXT.":           validation.TipoCE,
	"CARNE DE EXTRANJERIA":  validation.TipoCE,
	"CARNÉ DE EXTRANJERÍA":  validation.TipoCE,
	"CARNET DE EXTRANJERIA": validation.TipoCE,
	"C.E.":                  validation.TipoCE,
	"SIN DOCUMENTO":         validation.TipoSinDocumento,
}

func tipoDocumento(v string) string {
	t := strings.ToUpper(strings.Join(strings.Fields(v), " "))
	if alias, ok := aliasTipoDocumento[t]; ok {
		return alias
	}
	return t
}

// CodigoPara deriva el código del maestro (único, obligatorio, máx. 20):
//   - con documento: tipo + número ("DNI01234567", "CE…", "PAS…"); es
//     determinista, así que reimportar a la misma persona choca con el mismo
//     código y nunca crea un duplicado;
//   - sin documento: "SD" + 12 hex de sha256(huella del archivo + fila).
//     Confirmar dos veces el MISMO archivo produce los mismos códigos (no
//     duplica); otro archivo con la misma persona solo se avisa por nombre.
func CodigoPara(r Registro, huellaArchivo string, numeroFila int) string {
	if r.NumeroDocumento != nil {
		prefijo := r.TipoDocumento
		if prefijo == validation.TipoPasaporte {
			prefijo = "PAS"
		}
		return prefijo + *r.NumeroDocumento
	}
	sum := sha256.Sum256([]byte(huellaArchivo + ":" + strconv.Itoa(numeroFila)))
	return "SD" + strings.ToUpper(hex.EncodeToString(sum[:])[:12])
}

// Huella identifica el contenido exacto del archivo (vista previa = confirmación).
func Huella(contenido []byte) string {
	sum := sha256.Sum256(contenido)
	return hex.EncodeToString(sum[:])
}

// mensaje quita el código técnico ("validation_error: ...") para el usuario.
func mensaje(err error) string {
	if e, ok := err.(*validation.Error); ok {
		return e.Msg
	}
	return fmt.Sprint(err)
}

func esNumero(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
