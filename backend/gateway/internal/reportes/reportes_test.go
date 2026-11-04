package reportes

import (
	"bytes"
	"encoding/csv"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func datosPrueba() Datos {
	return Datos{
		Total: 10, Pendientes: 4,
		PorEstado:    []Conteo{{"PENDIENTE", 3}, {"EN_PROCESO", 1}, {"OBSERVADO", 2}, {"ATENDIDO", 4}},
		PorTipo:      []Conteo{{"CONSTANCIA", 6}, {"PERMISO", 4}},
		PorPrioridad: []Conteo{{"NORMAL", 8}, {"URGENTE", 2}},
		PorArea:      []Conteo{{"SECRETARIA", 7}, {"DIRECTOR", 3}},
		PorMes:       []Conteo{{"2026-09", 4}, {"2026-10", 6}},
		Antiguedad:   []Conteo{{"0-3", 1}, {"4-7", 0}, {"8-15", 2}, {"16-30", 0}, {"31+", 1}},
		Alcance:      "INSTITUCION",
		Generado:     time.Date(2026, 10, 30, 9, 15, 0, 0, time.FixedZone("Lima", -5*3600)),
	}
}

var filtrosPrueba = Filtros{Desde: "2026-09-01", Hasta: "2026-10-30", Tipo: "CONSTANCIA", Prioridad: "URGENTE"}

func TestCSV_EncabezadoFiltrosDatosYTotales(t *testing.T) {
	b, err := Generar(datosPrueba(), filtrosPrueba, FormatoCSV)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}) {
		t.Fatal("el CSV debe empezar con BOM UTF-8")
	}
	if !bytes.Contains(b, []byte("\r\n")) {
		t.Fatal("el CSV debe usar CRLF")
	}
	r := csv.NewReader(bytes.NewReader(b[3:]))
	r.Comma = SeparadorCSV // Excel en español
	r.FieldsPerRecord = -1
	filas, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	texto := func(fila []string) string { return strings.Join(fila, "|") }
	todo := []string{}
	for _, f := range filas {
		todo = append(todo, texto(f))
	}
	contenido := strings.Join(todo, "\n")
	for _, esperado := range []string{
		"Reporte|Reporte de expedientes – Mesa de Partes",
		"Generado|30/10/2026 09:15 (hora de Lima)",
		"Periodo (fecha de registro)|Del 01/09/2026 al 30/10/2026",
		"Alcance|Toda la institución",
		"Filtro estado|Todos",
		"Filtro tipo de trámite|Constancia de Estudios",
		"Filtro prioridad|Urgente",
		"Concepto|Cantidad|Porcentaje",
		"Total de expedientes|10|",
		"Pendientes (Pendiente + En proceso)|4|40,0 %",
		"Atendido|4|40,0 %",
		"Por mes de registro",
		"Septiembre 2026|4|40,0 %",
		"Más de 30 días|1|25,0 %", // % sobre pendientes
		"no representan plazos institucionales",
	} {
		if !strings.Contains(contenido, esperado) {
			t.Errorf("falta %q en el CSV", esperado)
		}
	}
	// Cada distribución termina con su total.
	if strings.Count(contenido, "Total|10|100,0 %") != 5 || !strings.Contains(contenido, "Total|4|100,0 %") {
		t.Errorf("faltan filas de total:\n%s", contenido)
	}
}

func TestCSV_ProtegeFormulas(t *testing.T) {
	if protegerFormula("=HYPERLINK(1)") != "'=HYPERLINK(1)" || protegerFormula("@x") != "'@x" || protegerFormula("-5") != "-5" || protegerFormula("Total") != "Total" {
		t.Fatal("regla de protección de fórmulas")
	}
}

func TestXLSX_ContenidoConNumerosReales(t *testing.T) {
	b, err := Generar(datosPrueba(), Filtros{}, FormatoXLSX)
	if err != nil {
		t.Fatal(err)
	}
	libro, err := excelize.OpenReader(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("el XLSX debe abrirse: %v", err)
	}
	defer libro.Close()
	filas, err := libro.GetRows("Reporte")
	if err != nil {
		t.Fatal(err)
	}
	if filas[0][0] != Titulo {
		t.Fatalf("título: %v", filas[0])
	}
	var periodo, totalFila int
	for i, f := range filas {
		if len(f) >= 2 && f[0] == "Periodo (fecha de registro)" && f[1] == "Todo el historial" {
			periodo = i + 1
		}
		if len(f) >= 2 && f[0] == "Total de expedientes" {
			totalFila = i + 1
		}
	}
	if periodo == 0 || totalFila == 0 {
		t.Fatalf("faltan metadatos o resumen: %v", filas[:12])
	}
	// La cantidad es un número (no texto) para poder sumarla en Excel.
	tipo, err := libro.GetCellType("Reporte", "B"+itoa(totalFila))
	if err != nil || tipo == excelize.CellTypeSharedString || tipo == excelize.CellTypeInlineString {
		t.Fatalf("la cantidad debe ser numérica: tipo=%v err=%v", tipo, err)
	}
	if v, _ := libro.GetCellValue("Reporte", "B"+itoa(totalFila)); v != "10" {
		t.Fatalf("total: %q", v)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func TestAlcancePorAreaYEtiquetas(t *testing.T) {
	d := datosPrueba()
	d.Alcance, d.AreaAlcance = "AREA", "DIRECTOR"
	b, _ := Generar(d, Filtros{}, FormatoCSV)
	if !strings.Contains(string(b), "Expedientes actualmente en el área Dirección") {
		t.Fatal("el alcance por área debe indicarse")
	}
	if EtiquetaMes("2026-03") != "Marzo 2026" || EtiquetaMes("x") != "x" {
		t.Fatal("etiqueta de mes")
	}
	if Periodo(Filtros{Desde: "2026-01-01"}) != "Desde el 01/01/2026" || Periodo(Filtros{Hasta: "2026-01-31"}) != "Hasta el 31/01/2026" {
		t.Fatal("periodo parcial")
	}
}

func TestSinDatosYFormatoInvalido(t *testing.T) {
	d := Datos{Generado: time.Now()}
	for _, f := range []string{FormatoCSV, FormatoXLSX, FormatoPDF} {
		if _, err := Generar(d, Filtros{}, f); err != nil {
			t.Fatalf("%s sin datos: %v", f, err)
		}
	}
	if _, err := Generar(d, Filtros{}, "doc"); err == nil {
		t.Fatal("formato no admitido")
	}
	if MimeDe("doc") != "" || MimeDe(FormatoCSV) != "text/csv" || MimeDe(FormatoPDF) != "application/pdf" {
		t.Fatal("mime")
	}
	if NombreArchivo(datosPrueba(), FormatoXLSX) != "reporte-expedientes-20261030-0915.xlsx" {
		t.Fatalf("nombre: %s", NombreArchivo(datosPrueba(), FormatoXLSX))
	}
}
