package tabular

import (
	"bytes"

	"github.com/xuri/excelize/v2"
)

// Límites de descompresión del zip (protección contra "zip bombs").
const (
	maxDescomprimido    = 64 << 20
	maxXMLDescomprimido = 32 << 20
)

// leerXLSX lee .xlsx/.xlsm/.xltx con excelize. Se leen los VALORES
// almacenados: las fórmulas no se recalculan y las macros (vbaProject.bin)
// nunca se cargan ni ejecutan.
func leerXLSX(contenido []byte, op Opciones) (*Tabla, error) {
	f, err := excelize.OpenReader(bytes.NewReader(contenido), excelize.Options{
		UnzipSizeLimit:    maxDescomprimido,
		UnzipXMLSizeLimit: maxXMLDescomprimido,
	})
	if err != nil {
		return nil, errorf(ErrDanado, "el archivo Excel está dañado o protegido con contraseña")
	}
	defer func() { _ = f.Close() }()

	hojas := f.GetSheetList()
	if len(hojas) == 0 {
		return nil, errorf(ErrEstructura, "el archivo no tiene hojas")
	}
	hoja, err := elegirHoja(hojas, op.Hoja)
	if err != nil {
		return nil, err
	}
	rows, err := f.Rows(hoja)
	if err != nil {
		return nil, errorf(ErrDanado, "no se pudo leer la hoja %q", hoja)
	}
	defer func() { _ = rows.Close() }()

	var crudas []filaCruda
	numero := 0
	tope := op.Limites.MaxFilas + filasBuscarEncabezado + 1
	for rows.Next() {
		numero++
		celdas, err := rows.Columns(excelize.Options{RawCellValue: true})
		if err != nil {
			return nil, errorf(ErrDanado, "no se pudo leer la fila %d", numero)
		}
		if len(celdas) > op.Limites.MaxColumnas {
			return nil, errorf(ErrDemasiadasCols, "la fila %d tiene más de %d columnas", numero, op.Limites.MaxColumnas)
		}
		crudas = append(crudas, filaCruda{numero: numero, celdas: celdas})
		if !vacia(celdas) {
			tope--
			if tope == 0 {
				break
			}
		}
	}
	return armarTabla(crudas, op, Origen{Hojas: hojas, Hoja: hoja})
}

// elegirHoja: la pedida (debe existir) o la primera.
func elegirHoja(hojas []string, pedida string) (string, error) {
	if pedida == "" {
		return hojas[0], nil
	}
	for _, h := range hojas {
		if h == pedida {
			return h, nil
		}
	}
	return "", errorf(ErrOpcion, "la hoja %q no existe en el archivo", pedida)
}
