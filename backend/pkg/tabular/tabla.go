// Package tabular es el motor reutilizable de importación y exportación de
// datos de SIS-MESA-PARTES. No conoce ningún módulo ni base de datos:
//
//   - detecta el formato real de un archivo (firma, no solo la extensión);
//   - lo lee a una representación común (Tabla: encabezados + filas de texto);
//   - normaliza encabezados y sugiere el mapeo hacia un Esquema de destino;
//   - escribe registros en los formatos de exportación.
//
// Cada módulo (hoy, Estudiantes) define su Esquema y decide qué hacer con las
// filas: validar, comparar contra su base y guardar.
package tabular

import (
	"errors"
	"fmt"
)

// Formatos que el motor sabe LEER (verificados con pruebas).
const (
	FormatoXLSX  = "xlsx"  // también .xlsm/.xltx: se leen los datos, nunca las macros
	FormatoODS   = "ods"   // OpenDocument (LibreOffice)
	FormatoHTML  = "html"  // tablas HTML; incluye los ".xls" que exportan muchos sistemas web
	FormatoCSV   = "csv"   // texto delimitado (coma, punto y coma, tabulador, barra)
	FormatoJSON  = "json"  // arreglo de objetos, u objeto con un arreglo en una ruta
	FormatoJSONL = "jsonl" // un objeto JSON por línea (.jsonl/.ndjson)
	FormatoXML   = "xml"   // elementos repetidos en una ruta
)

// Tabla es la representación común de cualquier archivo leído.
type Tabla struct {
	Encabezados []string // tal como vinieron (sin normalizar)
	Filas       []Fila
	Origen      Origen
}

// Fila conserva su número en el archivo original (fila de la hoja, línea del
// texto o posición del registro) para ubicar cada incidencia.
type Fila struct {
	Numero  int
	Valores []string // alineados con Encabezados
}

// Valor devuelve la celda i o "" si la fila es más corta.
func (f Fila) Valor(i int) string {
	if i >= 0 && i < len(f.Valores) {
		return f.Valores[i]
	}
	return ""
}

// Origen describe cómo se leyó el archivo (lo que el usuario puede ajustar).
type Origen struct {
	Formato          string
	ExtensionDeclarada string
	Hojas            []string // hojas (xlsx/ods) o tablas (html) disponibles
	Hoja             string
	Rutas            []string // rutas de registros posibles (json/xml)
	Ruta             string
	Separador        string // csv
	Codificacion     string // csv/html
	FilaEncabezado   int    // 1-based dentro de la hoja; 0 = sin encabezado (json/xml)
	Avisos           []string
}

// Opciones de lectura. Los campos vacíos se detectan automáticamente.
type Opciones struct {
	Hoja           string
	Ruta           string
	Separador      string
	Codificacion   string
	FilaEncabezado int // 0 = detectar
	Limites        Limites
	// PuntuarEncabezado (opcional) ayuda a detectar la fila de encabezados:
	// cuántas celdas reconoce el esquema de destino.
	PuntuarEncabezado func(celdas []string) int
}

// Limites protegen memoria y CPU frente a archivos desmedidos.
type Limites struct {
	MaxBytes       int64
	MaxFilas       int
	MaxColumnas    int
	MaxProfundidad int // JSON/XML
	MaxRunesCelda  int
}

// LimitesPorDefecto: generosos para un colegio, acotados para el servidor.
func LimitesPorDefecto() Limites {
	return Limites{MaxBytes: 3 << 20, MaxFilas: 5000, MaxColumnas: 200, MaxProfundidad: 32, MaxRunesCelda: 2000}
}

func (l Limites) conDefectos() Limites {
	d := LimitesPorDefecto()
	if l.MaxBytes <= 0 {
		l.MaxBytes = d.MaxBytes
	}
	if l.MaxFilas <= 0 {
		l.MaxFilas = d.MaxFilas
	}
	if l.MaxColumnas <= 0 {
		l.MaxColumnas = d.MaxColumnas
	}
	if l.MaxProfundidad <= 0 {
		l.MaxProfundidad = d.MaxProfundidad
	}
	if l.MaxRunesCelda <= 0 {
		l.MaxRunesCelda = d.MaxRunesCelda
	}
	return l
}

// Error de lectura con código estable (para traducirlo en la interfaz).
type Error struct {
	Code string
	Msg  string
}

func (e *Error) Error() string { return e.Code + ": " + e.Msg }

func errorf(code, format string, args ...any) error {
	return &Error{Code: code, Msg: fmt.Sprintf(format, args...)}
}

// Códigos de error del motor.
const (
	ErrVacio          = "archivo_vacio"
	ErrGrande         = "archivo_grande"
	ErrNoSoportado    = "formato_no_soportado"
	ErrDanado         = "archivo_invalido"
	ErrEstructura     = "estructura_invalida"
	ErrDemasiadas     = "demasiadas_filas"
	ErrDemasiadasCols = "demasiadas_columnas"
	ErrOpcion         = "opcion_invalida"
)

// CodigoDe devuelve el código de un error del motor ("" si no lo es).
func CodigoDe(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}
