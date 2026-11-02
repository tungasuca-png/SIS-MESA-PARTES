package reportes

import (
	"bytes"
	"compress/zlib"
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"
)

// Las seis pestañas, con datos de prueba (sin personas).
func documentosPestanas() map[string]Documento {
	exp := DatosExpedientes{
		Base: Datos{Total: 7, Pendientes: 6, Generado: generadoPrueba, Alcance: "INSTITUCION",
			PorEstado: []Conteo{{"PENDIENTE", 3}, {"EN_PROCESO", 3}, {"OBSERVADO", 0}, {"ATENDIDO", 1}}},
		Serie: []Conteo{{"2026-09", 0}, {"2026-10", 7}}, Granularidad: "MES",
		SerieEstado: []Cruce{{"2026-10", "PENDIENTE", 3}, {"2026-10", "EN_PROCESO", 3}, {"2026-10", "ATENDIDO", 1}},
		AreaEstado:  []Cruce{{"SECRETARIA", "PENDIENTE", 1}, {"DIRECTOR", "PENDIENTE", 2}, {"DIRECTOR", "EN_PROCESO", 3}, {"DOCENTE", "ATENDIDO", 1}},
		TipoEstado:  []Cruce{{"CERTIFICADO", "PENDIENTE", 1}, {"PERMISO", "PENDIENTE", 2}, {"PERMISO", "EN_PROCESO", 3}, {"PERMISO", "ATENDIDO", 1}},
	}
	der := DatosDerivaciones{Total: 4, SinClasificar: 1, Otros: []Conteo{{"NOTIFICACION", 2}, {"APROBACION", 2}},
		PorDestino: []Conteo{{"DIRECTOR", 3}, {SinClasificar, 1}}, PorOrigen: []Conteo{{"SECRETARIA", 3}, {SinClasificar, 1}},
		Flujo:          []Cruce{{"SECRETARIA", "DIRECTOR", 3}, {SinClasificar, SinClasificar, 1}},
		PorTipoTramite: []Conteo{{"CONSTANCIA", 1}, {"PERMISO", 3}}, Serie: []Conteo{{"2026-10", 4}}, Granularidad: "MES",
		Alcance: "AREA", AreaAlcance: "DIRECTOR", Generado: generadoPrueba}
	doc := DatosDocumentos{Total: 3, PorTipoDocumento: []Conteo{{"ADJUNTO", 2}, {"PROVEIDO", 1}}, PorTipoTramite: []Conteo{{"PERMISO", 3}},
		Serie: []Conteo{{"2026-10-30", 3}}, Granularidad: "DIA", Alcance: "INSTITUCION", Generado: generadoPrueba}
	roles := AgruparActividadPorRol(map[string]int{"a": 5, "b": 3}, map[string]int{"a": 2, "c": 4}, map[string]int{"a": 1, "c": 3}, map[string]int{"c": 1},
		map[string]string{"a": "SECRETARIA", "b": "SOLICITANTE", "c": "DIRECTOR"})
	comp := Comparar(resumen(12, 4, map[string]int{"PENDIENTE": 4}), resumen(10, 6, map[string]int{"PENDIENTE": 6}))
	f := Filtros{Desde: "2026-09-01", Hasta: "2026-10-30", Tipo: "PERMISO"}
	fp := FiltrosPeriodo{Desde: "2026-09-01", Hasta: "2026-10-30", Tipo: "PERMISO", Granularidad: "MES"}
	return map[string]Documento{
		"resumen":      DocumentoResumen(datosPrueba(), f),
		"expedientes":  DocumentoExpedientes(exp, f),
		"derivaciones": DocumentoDerivaciones(der, fp),
		"documentos":   DocumentoDocumentos(doc, fp),
		"actividad":    DocumentoActividad(roles, fp, "INSTITUCION", "", generadoPrueba),
		"comparacion":  DocumentoComparacion(comp, f, "2026-07-01", "2026-08-31", "INSTITUCION", "", generadoPrueba),
	}
}

// registrosEsperados: lo que el CSV DEBE contener, fila por fila, a partir del
// Documento (metadatos, y por tabla: vacía, título, nota, encabezado, filas).
func registrosEsperados(d Documento) [][]string {
	var out [][]string
	for _, m := range d.Meta {
		out = append(out, []string{protegerFormula(m[0]), protegerFormula(m[1])})
	}
	for _, t := range d.Tablas {
		out = append(out, nil, []string{t.Titulo})
		if t.Nota != "" {
			out = append(out, []string{t.Nota})
		}
		out = append(out, append([]string{}, t.Columnas...))
		for _, f := range t.Filas {
			fila := []string{protegerFormula(f.Etiqueta)}
			for _, v := range f.Valores {
				fila = append(fila, protegerFormula(v.texto()))
			}
			out = append(out, fila)
		}
	}
	return out
}

func leerCSV(t *testing.T, b []byte) [][]string {
	t.Helper()
	if !bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}) {
		t.Fatal("CSV sin BOM UTF-8")
	}
	cuerpo := b[3:]
	if bytes.Count(cuerpo, []byte("\n")) != bytes.Count(cuerpo, []byte("\r\n")) {
		t.Fatal("todos los saltos de línea de registro deben ser CRLF")
	}
	r := csv.NewReader(bytes.NewReader(cuerpo))
	r.Comma = ';'
	r.FieldsPerRecord = -1
	filas, err := r.ReadAll()
	if err != nil {
		t.Fatalf("CSV inválido con separador ';': %v", err)
	}
	return filas
}

func TestCSV_SeisPestanasColumnasYCorrespondencia(t *testing.T) {
	for nombre, d := range documentosPestanas() {
		b, err := GenerarDocumento(d, FormatoCSV)
		if err != nil {
			t.Fatal(err)
		}
		filas := leerCSV(t, b)
		esperado := registrosEsperados(d)
		// encoding/csv omite las líneas vacías al leer.
		sinVacias := [][]string{}
		for _, e := range esperado {
			if e != nil {
				sinVacias = append(sinVacias, e)
			}
		}
		if len(filas) != len(sinVacias) {
			t.Fatalf("%s: %d registros, esperados %d", nombre, len(filas), len(sinVacias))
		}
		for i := range filas {
			if strings.Join(filas[i], "\x1f") != strings.Join(sinVacias[i], "\x1f") {
				t.Fatalf("%s, registro %d: %q != %q", nombre, i, filas[i], sinVacias[i])
			}
		}
		// Cada fila de datos tiene tantas columnas como su encabezado.
		for _, tabla := range d.Tablas {
			for _, f := range tabla.Filas {
				if len(f.Valores)+1 != len(tabla.Columnas) {
					t.Fatalf("%s/%s: fila %q con %d valores para %d columnas", nombre, tabla.Titulo, f.Etiqueta, len(f.Valores)+1, len(tabla.Columnas))
				}
			}
		}
		// Los porcentajes con un decimal NO se entrecomillan: su coma es decimal.
		if strings.Contains(string(b), `"100,0 %"`) || (!strings.Contains(string(b), ";100,0 %") && nombre != "comparacion") {
			t.Errorf("%s: porcentaje mal delimitado", nombre)
		}
	}
}

func TestCSV_EscapaSeparadorComillasSaltosYAcentos(t *testing.T) {
	d := Documento{Titulo: "Prueba", Meta: [][2]string{{"Reporte", "Prueba"}, {"Filtro", `Año "2026"; área ñandú`}},
		Tablas: []Tabla{{Titulo: "Tabla; con separador", Nota: "Línea 1\nLínea 2", Columnas: []string{"Concepto", "Cantidad", "Porcentaje"},
			Filas: []FilaTabla{
				{Etiqueta: `Valor con ; y "comillas"`, Valores: []Valor{Entero(3), Porcentaje(pct(1, 3))}},
				{Etiqueta: "Dos\r\nlíneas", Valores: []Valor{Entero(-2), Texto("=SUMA(A1)")}},
			}}}}
	b, _ := GenerarDocumento(d, FormatoCSV)
	filas := leerCSV(t, b)
	quiere := [][]string{
		{"Reporte", "Prueba"}, {"Filtro", `Año "2026"; área ñandú`},
		{"Tabla; con separador"}, {"Línea 1\nLínea 2"}, {"Concepto", "Cantidad", "Porcentaje"},
		{`Valor con ; y "comillas"`, "3", "33,3 %"}, {"Dos\nlíneas", "-2", "'=SUMA(A1)"},
	}
	if fmt.Sprintf("%q", filas) != fmt.Sprintf("%q", quiere) {
		t.Fatalf("CSV leído:\n%q\nesperado:\n%q", filas, quiere)
	}
	if !strings.Contains(string(b), `"Valor con ; y ""comillas"""`) {
		t.Fatal("las comillas se duplican y el valor con ';' va entre comillas")
	}
}

// ---- PDF ----

var (
	reStream = regexp.MustCompile(`(?s)stream\r?\n(.*?)\r?\nendstream`)
	reTexto  = regexp.MustCompile(`\(((?:\\.|[^\\)])*)\) ?Tj`)
)

// textoPDF: texto de las páginas (descomprime los streams y decodifica cp1252).
func textoPDF(t *testing.T, b []byte) string {
	t.Helper()
	var sb strings.Builder
	for _, m := range reStream.FindAllSubmatch(b, -1) {
		r, err := zlib.NewReader(bytes.NewReader(m[1]))
		if err != nil {
			continue
		}
		crudo, _ := io.ReadAll(r)
		for _, tj := range reTexto.FindAllSubmatch(crudo, -1) {
			s := strings.NewReplacer(`\(`, "(", `\)`, ")", `\\`, `\`).Replace(string(tj[1]))
			sb.WriteString(desdeCP1252(s) + "\n")
		}
	}
	return sb.String()
}

// desdeCP1252: 0xA0-0xFF coinciden con Unicode; del rango 0x80-0x9F el PDF
// solo usa aquí el guion largo, la raya y las comillas tipográficas.
func desdeCP1252(s string) string {
	especiales := map[byte]rune{0x96: '–', 0x97: '—', 0x93: '“', 0x94: '”', 0x91: '‘', 0x92: '’', 0x85: '…', 0x95: '•'}
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		if r, ok := especiales[s[i]]; ok {
			sb.WriteRune(r)
		} else {
			sb.WriteRune(rune(s[i]))
		}
	}
	return sb.String()
}

func TestPDF_SeisPestanas(t *testing.T) {
	contenido := map[string][]string{
		"resumen":      {"Reporte de expedientes – Mesa de Partes", "Total de expedientes", "Pendientes por antigüedad", "Septiembre 2026"},
		"expedientes":  {"Evolución de expedientes registrados, por estado actual", "Áreas con mayor carga pendiente", "Participación", "Octubre 2026", "83,3 %"},
		"derivaciones": {"Movimientos registrados (todos los tipos)", "Derivaciones entre áreas", "De ellos, aprobaciones", "Secretaría -> Dirección", "Sin clasificar", "Expedientes actualmente en el área Dirección"},
		"documentos":   {"Por tipo documental (de más a menos frecuente)", "Proveído", "30/10/2026"},
		"actividad":    {"Movimientos registrados", "De ellos, derivaciones entre áreas", "Total de acciones", "Solicitante (externo)"},
		"comparacion":  {"Comparación de periodos", "Diferencia", "Variación", "Tendencia", "Aumento", "Disminución", "20,0 %", "Del 01/07/2026 al 31/08/2026"},
	}
	for nombre, d := range documentosPestanas() {
		b, err := GenerarDocumento(d, FormatoPDF)
		if err != nil {
			t.Fatalf("%s: %v", nombre, err)
		}
		if !bytes.HasPrefix(b, []byte("%PDF-")) || !bytes.Contains(b[len(b)-16:], []byte("%%EOF")) || len(b) < 1500 {
			t.Fatalf("%s: PDF inválido o vacío (%d bytes)", nombre, len(b))
		}
		// Texto con espacios normalizados: los encabezados y etiquetas largos
		// se ajustan en varias líneas dentro de su celda.
		texto := strings.Join(strings.Fields(textoPDF(t, b)), " ")
		paginas := strings.Count(texto, "Página ")
		comunes := []string{"COLEGIO TUNGASUCA · MESA DE PARTES", "Institución", "Colegio Tungasuca – SIS-MESA-PARTES", "30/10/2026 09:15 (hora de Lima)",
			"Alcance", fmt.Sprintf("Página 1 de %d", paginas), fmt.Sprintf("Página %d de %d", paginas, paginas), "Documento generado por el sistema"}
		if nombre != "actividad" {
			comunes = append(comunes, "Filtro tipo de trámite", "Permiso")
		}
		for _, e := range append(comunes, contenido[nombre]...) {
			if !strings.Contains(texto, e) {
				t.Errorf("%s: falta %q en el PDF", nombre, e)
			}
		}
		// Todas las filas de todas las tablas están en el PDF.
		for _, tabla := range d.Tablas {
			for _, c := range tabla.Columnas {
				if !strings.Contains(texto, strings.Join(strings.Fields(pdfSustituir.Replace(c)), " ")) {
					t.Errorf("%s: falta el encabezado %q", nombre, c)
				}
			}
			for _, f := range tabla.Filas {
				if !strings.Contains(texto, strings.Join(strings.Fields(pdfSustituir.Replace(f.Etiqueta)), " ")) {
					t.Errorf("%s: falta la fila %q", nombre, f.Etiqueta)
				}
			}
		}
		// Orientación: horizontal si alguna tabla tiene más de 5 columnas.
		horizontal := bytes.Contains(b, []byte("/MediaBox [0 0 841.89 595.28]"))
		anchas := false
		for _, tabla := range d.Tablas {
			anchas = anchas || len(tabla.Columnas) > 5
		}
		if horizontal != anchas {
			t.Errorf("%s: orientación horizontal=%v con tablas anchas=%v", nombre, horizontal, anchas)
		}
	}
}

func TestPDF_TablaExtensaRepiteEncabezadoSinPerderFilas(t *testing.T) {
	serie := []Conteo{}
	celdas := []Cruce{}
	for i := 0; i < 120; i++ {
		clave := fmt.Sprintf("2026-%02d-%02d", 1+i/28, 1+i%28)
		serie = append(serie, Conteo{clave, 1})
		celdas = append(celdas, Cruce{clave, "ATENDIDO", 1})
	}
	d := DocumentoExpedientes(DatosExpedientes{Base: Datos{Total: 120, Generado: generadoPrueba,
		PorEstado: []Conteo{{"ATENDIDO", 120}}}, Serie: serie, SerieEstado: celdas, Granularidad: "DIA"}, Filtros{})
	b, err := GenerarDocumento(d, FormatoPDF)
	if err != nil {
		t.Fatal(err)
	}
	texto := textoPDF(t, b)
	paginas := bytes.Count(b, []byte("/Type /Page\n")) + bytes.Count(b, []byte("/Type /Page\r")) + bytes.Count(b, []byte("/Type /Page "))
	if paginas < 3 || !strings.Contains(texto, fmt.Sprintf("Página %d de %d", paginas, paginas)) {
		t.Fatalf("paginación: %d páginas", paginas)
	}
	if n := strings.Count(texto, "Evolución de expedientes registrados, por estado actual (continuación)"); n < 2 {
		t.Fatalf("la tabla larga debe continuar con su título en cada página: %d", n)
	}
	// El encabezado de columnas se repite en cada página de la tabla.
	if n := strings.Count(texto, "Periodo\n"); n < 3 {
		t.Fatalf("encabezado repetido %d veces", n)
	}
	for _, s := range serie {
		if !strings.Contains(texto, EtiquetaPeriodo(s.Clave, "DIA")) {
			t.Fatalf("falta el periodo %s", s.Clave)
		}
	}
}
