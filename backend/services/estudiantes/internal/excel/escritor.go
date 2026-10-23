package excel

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Anchos de columna (en caracteres), en el orden de Encabezados.
var anchos = []float64{6, 14, 14, 24, 24, 30, 14, 12, 14}

// formatoTexto: "@" (Texto). Así Excel no convierte un DNI en número ni
// pierde sus ceros iniciales al escribirlo en la plantilla.
const formatoTexto = 49

// FilaExport es un estudiante del maestro con los valores legibles del
// formato oficial.
type FilaExport struct {
	TipoDocumento   string
	Documento       string
	ApellidoPaterno string
	ApellidoMaterno string
	Nombres         string
	Estado          string
	Nivel           string
}

func ultimaColumna() string {
	col, _ := excelize.ColumnNumberToName(len(Encabezados))
	return col
}

// prepararHoja crea la hoja "Estudiantes" con encabezados en negrita, filtro,
// panel fijo, anchos y formato de texto en las columnas de datos.
func prepararHoja(f *excelize.File) error {
	if err := f.SetSheetName("Sheet1", HojaEstudiantes); err != nil {
		return err
	}
	texto, err := f.NewStyle(&excelize.Style{NumFmt: formatoTexto})
	if err != nil {
		return err
	}
	for i, ancho := range anchos {
		col, _ := excelize.ColumnNumberToName(i + 1)
		if err := f.SetColWidth(HojaEstudiantes, col, col, ancho); err != nil {
			return err
		}
		if i > colNro {
			if err := f.SetColStyle(HojaEstudiantes, col, texto); err != nil {
				return err
			}
		}
	}
	if err := f.SetPanes(HojaEstudiantes, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return err
	}
	return f.AutoFilter(HojaEstudiantes, "A1:"+ultimaColumna()+"1", nil)
}

func estiloEncabezado(f *excelize.File) (int, error) {
	return f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"164194"}},
		Alignment: &excelize.Alignment{Vertical: "center"},
	})
}

// Plantilla genera la plantilla oficial vacía: hoja "Estudiantes" con los
// nueve encabezados, listas desplegables con los valores admitidos y una
// hoja "Instrucciones".
func Plantilla() ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	if err := prepararHoja(f); err != nil {
		return nil, err
	}
	encabezado, err := estiloEncabezado(f)
	if err != nil {
		return nil, err
	}
	if err := f.SetSheetRow(HojaEstudiantes, "A1", &Encabezados); err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(HojaEstudiantes, "A1", ultimaColumna()+"1", encabezado); err != nil {
		return nil, err
	}

	fin := strconv.Itoa(MaxFilasPlantilla + 1)
	for _, v := range []struct {
		col     int
		valores []string
		titulo  string
	}{
		{colTipoDoc, TiposDocumento, "TIPO.DOC"},
		{colCondicion, []string{Condicion}, "CONDICION"},
		{colEstado, Estados, "ESTADO"},
		{colNivel, Niveles, "NIVEL"},
	} {
		col, _ := excelize.ColumnNumberToName(v.col + 1)
		dv := excelize.NewDataValidation(true)
		dv.Sqref = col + "2:" + col + fin
		if err := dv.SetDropList(v.valores); err != nil {
			return nil, err
		}
		dv.SetError(excelize.DataValidationErrorStyleStop, v.titulo, "Valor no admitido. Use: "+strings.Join(v.valores, ", "))
		if err := f.AddDataValidation(HojaEstudiantes, dv); err != nil {
			return nil, err
		}
	}

	if err := hojaInstrucciones(f, encabezado); err != nil {
		return nil, err
	}
	f.SetActiveSheet(0)
	return escribir(f)
}

func hojaInstrucciones(f *excelize.File, encabezado int) error {
	if _, err := f.NewSheet(HojaInstrucciones); err != nil {
		return err
	}
	filas := [][]string{
		{"Columna", "Obligatorio", "Valores admitidos / indicaciones"},
		{"Nro", "No", "Numeración correlativa de referencia. No identifica al estudiante."},
		{"TIPO.DOC", "Sí", strings.Join(TiposDocumento, ", ") + ". Vacío junto con DNI vacío = SIN_DOCUMENTO."},
		{"DNI", "Según tipo", "DNI: 8 dígitos. CE / PASAPORTE: 4 a 12 letras o dígitos. SIN_DOCUMENTO: dejar vacío. La columna está en formato texto para conservar los ceros iniciales."},
		{"APELLIDO PATERNO", "Sí", "Hasta 60 caracteres."},
		{"APELLIDO MATERNO", "No", "Hasta 60 caracteres."},
		{"NOMBRES", "Sí", "Hasta 100 caracteres. Se conservan tildes y ñ."},
		{"CONDICION", "Sí", "Solo " + Condicion + "."},
		{"ESTADO", "Sí", strings.Join(Estados, ", ") + "."},
		{"NIVEL", "Sí", strings.Join(Niveles, ", ") + "."},
		{},
		{"Notas", "", "Escriba los estudiantes desde la fila 2 de la hoja «" + HojaEstudiantes + "», sin cambiar los encabezados. Las filas vacías se ignoran."},
		{"", "", "Un estudiante que ya está registrado (mismo tipo y número de documento) no se modifica: se informa y se omite."},
		{"", "", "Los estudiantes sin documento se importan; si su nombre coincide con uno registrado, se avisa para revisarlo."},
	}
	for i, fila := range filas {
		celda, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := f.SetSheetRow(HojaInstrucciones, celda, &fila); err != nil {
			return err
		}
	}
	if err := f.SetCellStyle(HojaInstrucciones, "A1", "C1", encabezado); err != nil {
		return err
	}
	if err := f.SetColWidth(HojaInstrucciones, "A", "A", 20); err != nil {
		return err
	}
	if err := f.SetColWidth(HojaInstrucciones, "B", "B", 13); err != nil {
		return err
	}
	return f.SetColWidth(HojaInstrucciones, "C", "C", 100)
}

// Exportador escribe el Excel oficial fila por fila (StreamWriter: no arma
// toda la hoja en memoria). Todas las celdas se escriben como TEXTO: un dato
// que empiece con "=" nunca se interpreta como fórmula.
type Exportador struct {
	f      *excelize.File
	sw     *excelize.StreamWriter
	filas  int
	estilo int
}

func NuevoExportador() (*Exportador, error) {
	f := excelize.NewFile()
	if err := f.SetSheetName("Sheet1", HojaEstudiantes); err != nil {
		_ = f.Close()
		return nil, err
	}
	encabezado, err := estiloEncabezado(f)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	texto, err := f.NewStyle(&excelize.Style{NumFmt: formatoTexto})
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	sw, err := f.NewStreamWriter(HojaEstudiantes)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	for i, ancho := range anchos {
		if err := sw.SetColWidth(i+1, i+1, ancho); err != nil {
			_ = f.Close()
			return nil, err
		}
	}
	if err := sw.SetPanes(&excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		_ = f.Close()
		return nil, err
	}
	cab := make([]any, len(Encabezados))
	for i, h := range Encabezados {
		cab[i] = excelize.Cell{StyleID: encabezado, Value: h}
	}
	if err := sw.SetRow("A1", cab); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &Exportador{f: f, sw: sw, estilo: texto}, nil
}

// Agregar escribe un estudiante; Nro es la numeración correlativa.
func (e *Exportador) Agregar(r FilaExport) error {
	e.filas++
	valores := []string{strconv.Itoa(e.filas), r.TipoDocumento, r.Documento, r.ApellidoPaterno,
		r.ApellidoMaterno, r.Nombres, Condicion, r.Estado, r.Nivel}
	celdas := make([]any, len(valores))
	for i, v := range valores {
		celdas[i] = excelize.Cell{StyleID: e.estilo, Value: v}
	}
	celda, _ := excelize.CoordinatesToCellName(1, e.filas+1)
	return e.sw.SetRow(celda, celdas)
}

// Filas exportadas hasta ahora.
func (e *Exportador) Filas() int { return e.filas }

// Bytes cierra la hoja (con filtro en los encabezados) y devuelve el .xlsx.
func (e *Exportador) Bytes() ([]byte, error) {
	defer func() { _ = e.f.Close() }()
	if err := e.sw.Flush(); err != nil {
		return nil, err
	}
	if err := e.f.AutoFilter(HojaEstudiantes, "A1:"+ultimaColumna()+"1", nil); err != nil {
		return nil, err
	}
	return escribir(e.f)
}

func escribir(f *excelize.File) ([]byte, error) {
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
