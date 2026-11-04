// Package reportes arma los archivos exportables (XLSX y CSV) del reporte de
// expedientes a partir de la respuesta AGREGADA de Expedientes Service. No
// consulta datos: solo da formato. Cada archivo incluye nombre del reporte,
// fecha de generación, periodo, alcance, filtros aplicados, encabezados,
// datos y totales.
//
// Convenciones heredadas del motor de exportación de Estudiantes
// (backend/pkg/tabular): misma librería XLSX (excelize), mismo estilo de
// encabezado, CSV UTF-8 con BOM y fin de línea CRLF, y protección contra
// fórmulas. No se usa tabular.Exportador porque genera UNA sola rejilla con
// celdas de texto; un reporte necesita varias secciones con números reales.
package reportes

import (
	"time"
)

const (
	FormatoXLSX = "xlsx"
	FormatoCSV  = "csv"
	FormatoPDF  = "pdf"
	// SeparadorCSV: separador de listas de Excel en configuración regional
	// española.
	SeparadorCSV = ';'
	Titulo       = "Reporte de expedientes – Mesa de Partes"
	Institucion  = "Colegio Tungasuca – SIS-MESA-PARTES"
)

// MimeDe: tipo MIME por formato ("" si no es un formato de reporte).
func MimeDe(formato string) string {
	return map[string]string{
		FormatoXLSX: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		FormatoCSV:  "text/csv",
		FormatoPDF:  "application/pdf",
	}[formato]
}

// Etiquetas en español (las mismas que muestra el frontend).
var (
	EtiquetasEstado = map[string]string{
		"PENDIENTE": "Pendiente", "EN_PROCESO": "En proceso", "OBSERVADO": "Observado", "ATENDIDO": "Atendido",
	}
	EtiquetasTipo = map[string]string{
		"CERTIFICADO": "Certificado de Estudios", "CONSTANCIA": "Constancia de Estudios", "PERMISO": "Permiso",
		"JUSTIFICACION_FALTA": "Justificación de Falta", "JUSTIFICACION_TARDANZA": "Justificación de Tardanza",
		"SOLICITUD": "Solicitud", "TRAMITE": "Trámite", "OFICIO": "Oficio", "OTRO": "Otro",
	}
	EtiquetasPrioridad = map[string]string{"NORMAL": "Normal", "URGENTE": "Urgente"}
	EtiquetasArea      = map[string]string{
		"SECRETARIA": "Secretaría", "DIRECTOR": "Dirección", "SUBDIRECTOR": "Subdirección",
		"DOCENTE": "Docente", "AUXILIAR": "Auxiliar", "ADMIN": "Administración",
	}
	EtiquetasAntiguedad = map[string]string{
		"0-3": "0 a 3 días", "4-7": "4 a 7 días", "8-15": "8 a 15 días", "16-30": "16 a 30 días", "31+": "Más de 30 días",
	}
	meses = []string{"Enero", "Febrero", "Marzo", "Abril", "Mayo", "Junio", "Julio", "Agosto", "Septiembre", "Octubre", "Noviembre", "Diciembre"}
)

func etiqueta(m map[string]string, clave string) string {
	if v, ok := m[clave]; ok {
		return v
	}
	return clave
}

// EtiquetaMes: "2026-03" -> "Marzo 2026".
func EtiquetaMes(clave string) string {
	t, err := time.Parse("2006-01", clave)
	if err != nil {
		return clave
	}
	return meses[t.Month()-1] + " " + t.Format("2006")
}

// Conteo y Datos: la respuesta agregada, independiente del tipo gRPC.
type Conteo struct {
	Clave    string
	Cantidad int
}

type Datos struct {
	Total, Pendientes                                 int
	PorEstado, PorTipo, PorPrioridad, PorArea, PorMes []Conteo
	Antiguedad                                        []Conteo
	Alcance, AreaAlcance                              string
	Generado                                          time.Time
}

// Filtros aplicados (ya validados por Expedientes Service).
type Filtros struct {
	Desde, Hasta, Estado, Tipo, Prioridad, Area string
}

// Fila de una sección: Concepto, Cantidad y Porcentaje (nil = sin %).
type fila struct {
	concepto   string
	cantidad   int
	porcentaje *float64
	total      bool
}

type seccion struct {
	titulo string
	nota   string
	filas  []fila
}

func pct(valor, base int) *float64 {
	if base <= 0 {
		return nil
	}
	p := float64(valor) / float64(base)
	return &p
}

func fechaLegible(iso string) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	return t.Format("02/01/2006")
}

// Periodo en texto ("Todo el historial" si no hay fechas).
func Periodo(f Filtros) string {
	switch {
	case f.Desde != "" && f.Hasta != "":
		return "Del " + fechaLegible(f.Desde) + " al " + fechaLegible(f.Hasta)
	case f.Desde != "":
		return "Desde el " + fechaLegible(f.Desde)
	case f.Hasta != "":
		return "Hasta el " + fechaLegible(f.Hasta)
	}
	return "Todo el historial"
}

func todos(v string, m map[string]string) string {
	if v == "" {
		return "Todos"
	}
	return etiqueta(m, v)
}

// metadatos: pares (campo, valor) del encabezado del archivo.
func metadatos(d Datos, f Filtros) [][2]string {
	alcance := "Toda la institución"
	if d.Alcance == "AREA" {
		alcance = "Expedientes actualmente en el área " + etiqueta(EtiquetasArea, d.AreaAlcance)
	}
	return [][2]string{
		{"Reporte", Titulo},
		{"Institución", Institucion},
		{"Generado", d.Generado.Format("02/01/2006 15:04") + " (hora de Lima)"},
		{"Periodo (fecha de registro)", Periodo(f)},
		{"Alcance", alcance},
		{"Filtro estado", todos(f.Estado, EtiquetasEstado)},
		{"Filtro tipo de trámite", todos(f.Tipo, EtiquetasTipo)},
		{"Filtro prioridad", todos(f.Prioridad, EtiquetasPrioridad)},
		{"Filtro área", todos(f.Area, EtiquetasArea)},
	}
}

func distribucion(titulo string, conteos []Conteo, etiquetas func(string) string, base int, nota string) seccion {
	s := seccion{titulo: titulo, nota: nota}
	suma := 0
	for _, c := range conteos {
		s.filas = append(s.filas, fila{concepto: etiquetas(c.Clave), cantidad: c.Cantidad, porcentaje: pct(c.Cantidad, base)})
		suma += c.Cantidad
	}
	s.filas = append(s.filas, fila{concepto: "Total", cantidad: suma, porcentaje: pct(suma, base), total: true})
	return s
}

func secciones(d Datos) []seccion {
	cantidad := func(cs []Conteo, clave string) int {
		for _, c := range cs {
			if c.Clave == clave {
				return c.Cantidad
			}
		}
		return 0
	}
	resumen := seccion{titulo: "Resumen", filas: []fila{
		{concepto: "Total de expedientes", cantidad: d.Total},
		{concepto: "Pendientes (Pendiente + En proceso)", cantidad: d.Pendientes, porcentaje: pct(d.Pendientes, d.Total)},
		{concepto: "Atendidos", cantidad: cantidad(d.PorEstado, "ATENDIDO"), porcentaje: pct(cantidad(d.PorEstado, "ATENDIDO"), d.Total)},
		{concepto: "Observados (esperan corrección del solicitante)", cantidad: cantidad(d.PorEstado, "OBSERVADO"), porcentaje: pct(cantidad(d.PorEstado, "OBSERVADO"), d.Total)},
	}}
	con := func(m map[string]string) func(string) string { return func(k string) string { return etiqueta(m, k) } }
	return []seccion{
		resumen,
		distribucion("Por estado", d.PorEstado, con(EtiquetasEstado), d.Total, ""),
		distribucion("Por tipo de trámite", d.PorTipo, con(EtiquetasTipo), d.Total, ""),
		distribucion("Por prioridad", d.PorPrioridad, con(EtiquetasPrioridad), d.Total, ""),
		distribucion("Por área actual", d.PorArea, con(EtiquetasArea), d.Total, "Área en la que se encuentra cada expediente al generar el reporte."),
		distribucion("Por mes de registro", d.PorMes, EtiquetaMes, d.Total, ""),
		distribucion("Pendientes por antigüedad", d.Antiguedad, con(EtiquetasAntiguedad), d.Pendientes,
			"Días desde la fecha de registro hasta hoy. Rangos descriptivos: no representan plazos institucionales. Porcentaje sobre los pendientes."),
	}
}

// NombreArchivo: reporte-expedientes-AAAAMMDD-HHMM.<formato>.
func NombreArchivo(d Datos, formato string) string {
	return "reporte-expedientes-" + d.Generado.Format("20060102-1504") + "." + formato
}

// DocumentoResumen: el archivo del Resumen (Fase 1), sin cambios de contenido.
func DocumentoResumen(d Datos, f Filtros) Documento {
	doc := Documento{Titulo: Titulo, Meta: metadatos(d, f)}
	for _, s := range secciones(d) {
		t := Tabla{Titulo: s.titulo, Nota: s.nota, Columnas: []string{"Concepto", "Cantidad", "Porcentaje"}}
		for _, fl := range s.filas {
			t.Filas = append(t.Filas, FilaTabla{Etiqueta: fl.concepto, Valores: []Valor{Entero(fl.cantidad), Porcentaje(fl.porcentaje)}, Total: fl.total})
		}
		doc.Tablas = append(doc.Tablas, t)
	}
	return doc
}

// Generar devuelve el archivo del Resumen (Fase 1) en el formato pedido.
func Generar(d Datos, f Filtros, formato string) ([]byte, error) {
	return GenerarDocumento(DocumentoResumen(d, f), formato)
}
