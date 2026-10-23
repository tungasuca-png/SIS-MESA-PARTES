// Package excel implementa el formato OFICIAL de importación/exportación de
// estudiantes (.xlsx). No toca la base de datos: lee, valida y escribe
// archivos; la lógica del servicio decide qué se guarda.
//
// Formato (primera fila de la hoja "Estudiantes", en este orden exacto):
//
//	Nro | TIPO.DOC | DNI | APELLIDO PATERNO | APELLIDO MATERNO | NOMBRES | CONDICION | ESTADO | NIVEL
package excel

import (
	"errors"
	"strings"
)

const (
	HojaEstudiantes   = "Estudiantes"
	HojaInstrucciones = "Instrucciones"

	// Condicion: única condición importable en este alcance. El maestro no
	// tiene columna de condición: todo registro de "estudiantes" lo es.
	Condicion = "ESTUDIANTE"

	// MaxFilasPlantilla: alcance de las listas desplegables de la plantilla.
	MaxFilasPlantilla = 5000
)

// Encabezados oficiales, en orden. No agregar columnas.
var Encabezados = []string{
	"Nro", "TIPO.DOC", "DNI", "APELLIDO PATERNO", "APELLIDO MATERNO", "NOMBRES", "CONDICION", "ESTADO", "NIVEL",
}

const (
	colNro = iota
	colTipoDoc
	colDocumento
	colApellidoPaterno
	colApellidoMaterno
	colNombres
	colCondicion
	colEstado
	colNivel
)

// Catálogos reales del proyecto (no se inventan valores):
//   - tipos: cat_tipos_documento (migración 002);
//   - estados: chk_estudiantes_estado (ACTIVO/INACTIVO);
//   - niveles: chk_secciones_nivel (migración 007), mismo catálogo de las secciones.
var (
	TiposDocumento = []string{"DNI", "CE", "PASAPORTE", "SIN_DOCUMENTO"}
	Estados        = []string{"ACTIVO", "INACTIVO"}
	Niveles        = []string{"INICIAL", "PRIMARIA", "SECUNDARIA"}
)

// Errores del archivo completo (no de una fila). El código estable va al
// inicio del mensaje, igual que el resto del servicio.
var (
	ErrArchivoInvalido = errors.New("archivo_invalido: el archivo no es un Excel .xlsx válido")
	ErrArchivoVacio    = errors.New("archivo_vacio: el archivo no tiene estudiantes para importar")
	ErrSinHoja         = errors.New("archivo_invalido: el archivo no tiene hojas")
)

// ErrorArchivo describe un problema de estructura con detalle para el usuario.
type ErrorArchivo struct {
	Code string
	Msg  string
}

func (e *ErrorArchivo) Error() string { return e.Code + ": " + e.Msg }

// normalizarEncabezado tolera espacios extra y mayúsculas/minúsculas, sin
// alterar los encabezados que genera la plantilla.
func normalizarEncabezado(s string) string {
	return strings.ToUpper(strings.Join(strings.Fields(s), " "))
}

func contiene(lista []string, v string) bool {
	for _, x := range lista {
		if x == v {
			return true
		}
	}
	return false
}
