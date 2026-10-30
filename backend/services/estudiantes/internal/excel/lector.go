package excel

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Fila es una fila de datos del Excel, tal como vino (sin validar). Numero
// es el número de fila de Excel (la de encabezados es la 1), para que el
// usuario ubique cada incidencia.
type Fila struct {
	Numero          int
	TipoDocumento   string
	Documento       string
	ApellidoPaterno string
	ApellidoMaterno string
	Nombres         string
	Condicion       string
	Estado          string
	Nivel           string
}

// Limites protege al servicio de archivos desmedidos.
type Limites struct {
	MaxBytes int64
	MaxFilas int
}

// Límites de descompresión: un .xlsx es un zip; sin tope, un archivo chico
// podría expandirse a gigas en memoria (zip bomb).
const (
	maxDescomprimido    = 64 << 20
	maxXMLDescomprimido = 32 << 20
)

// Leer valida la estructura del archivo y devuelve sus filas de datos. Las
// filas vacías (incluidas las que solo traen "Nro") se omiten, pero cada
// fila conserva su número real de Excel.
func Leer(contenido []byte, lim Limites) ([]Fila, error) {
	if len(contenido) == 0 {
		return nil, ErrArchivoVacio
	}
	if lim.MaxBytes > 0 && int64(len(contenido)) > lim.MaxBytes {
		return nil, &ErrorArchivo{Code: "archivo_grande", Msg: fmt.Sprintf("el archivo supera el máximo de %d KB", lim.MaxBytes/1024)}
	}
	// Un .xlsx es un zip: un .xls antiguo, un CSV o un archivo renombrado
	// no empiezan así.
	if !bytes.HasPrefix(contenido, []byte("PK\x03\x04")) {
		return nil, ErrArchivoInvalido
	}
	f, err := excelize.OpenReader(bytes.NewReader(contenido), excelize.Options{
		UnzipSizeLimit:    maxDescomprimido,
		UnzipXMLSizeLimit: maxXMLDescomprimido,
	})
	if err != nil {
		return nil, ErrArchivoInvalido
	}
	defer func() { _ = f.Close() }()

	hoja := HojaEstudiantes
	if idx, err := f.GetSheetIndex(hoja); err != nil || idx < 0 {
		hojas := f.GetSheetList()
		if len(hojas) == 0 {
			return nil, ErrSinHoja
		}
		hoja = hojas[0]
	}

	rows, err := f.Rows(hoja)
	if err != nil {
		return nil, ErrArchivoInvalido
	}
	defer func() { _ = rows.Close() }()

	var filas []Fila
	numero := 0
	encabezado := false
	var errEncabezado error
	for rows.Next() {
		numero++
		// RawCellValue: el valor almacenado, sin formato de número; así un DNI
		// nunca sale en notación científica.
		celdas, err := rows.Columns(excelize.Options{RawCellValue: true})
		if err != nil {
			return nil, ErrArchivoInvalido
		}
		// Los encabezados van en la primera fila de la plantilla, pero los
		// reportes exportados de otros sistemas traen un título encima: se
		// buscan en las primeras filasTitulo filas y lo anterior se ignora.
		if !encabezado {
			err := validarEncabezados(celdas)
			if err == nil {
				encabezado = true
				continue
			}
			if errEncabezado == nil && !filaVaciaDeCeldas(celdas) {
				errEncabezado = err
			}
			if numero >= filasTitulo {
				return nil, errEncabezado
			}
			continue
		}
		fila := filaDesde(numero, celdas)
		if fila.vacia() {
			continue
		}
		if lim.MaxFilas > 0 && len(filas) >= lim.MaxFilas {
			return nil, &ErrorArchivo{Code: "demasiadas_filas", Msg: fmt.Sprintf("el archivo supera el máximo de %d estudiantes por importación", lim.MaxFilas)}
		}
		filas = append(filas, fila)
	}
	if err := rows.Error(); err != nil {
		return nil, ErrArchivoInvalido
	}
	if !encabezado {
		if errEncabezado != nil {
			return nil, errEncabezado
		}
		return nil, &ErrorArchivo{Code: "encabezados_invalidos", Msg: "la primera fila debe tener los encabezados: " + strings.Join(Encabezados, " | ")}
	}
	if len(filas) == 0 {
		return nil, ErrArchivoVacio
	}
	return filas, nil
}

// filasTitulo: hasta cuántas filas se buscan los encabezados.
const filasTitulo = 10

func filaVaciaDeCeldas(celdas []string) bool {
	for _, c := range celdas {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

func validarEncabezados(celdas []string) error {
	// Celdas finales vacías no cuentan como columnas.
	for len(celdas) > 0 && strings.TrimSpace(celdas[len(celdas)-1]) == "" {
		celdas = celdas[:len(celdas)-1]
	}
	if len(celdas) != len(Encabezados) {
		return &ErrorArchivo{Code: "encabezados_invalidos", Msg: fmt.Sprintf(
			"se esperaban %d columnas (%s) y el archivo tiene %d", len(Encabezados), strings.Join(Encabezados, " | "), len(celdas))}
	}
	for i, esperado := range Encabezados {
		if normalizarEncabezado(celdas[i]) != normalizarEncabezado(esperado) {
			return &ErrorArchivo{Code: "encabezados_invalidos", Msg: fmt.Sprintf(
				"la columna %d debe llamarse %q (se encontró %q)", i+1, esperado, strings.TrimSpace(celdas[i]))}
		}
	}
	return nil
}

func filaDesde(numero int, celdas []string) Fila {
	celda := func(i int) string {
		if i < len(celdas) {
			return celdas[i]
		}
		return ""
	}
	return Fila{
		Numero:          numero,
		TipoDocumento:   celda(colTipoDoc),
		Documento:       celda(colDocumento),
		ApellidoPaterno: celda(colApellidoPaterno),
		ApellidoMaterno: celda(colApellidoMaterno),
		Nombres:         celda(colNombres),
		Condicion:       celda(colCondicion),
		Estado:          celda(colEstado),
		Nivel:           celda(colNivel),
	}
}

// vacia: sin ningún dato fuera de "Nro" (la numeración no es un dato).
func (f Fila) vacia() bool {
	for _, v := range []string{f.TipoDocumento, f.Documento, f.ApellidoPaterno, f.ApellidoMaterno,
		f.Nombres, f.Condicion, f.Estado, f.Nivel} {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}
