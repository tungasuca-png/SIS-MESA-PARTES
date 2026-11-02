package reportes

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Fase 2 de Reportes: documentos exportables por pestaña, comparación de
// periodos (con umbral mínimo) y actividad por ROL. Solo da formato y
// combina respuestas agregadas: no consulta datos.

const (
	SinClasificar     = "SIN_CLASIFICAR"
	MinimoComparacion = 10

	ModoAnterior      = "ANTERIOR"
	ModoAnioAnterior  = "ANIO_ANTERIOR"
	ModoPersonalizado = "PERSONALIZADO"

	MensajeInsuficiente = "Datos insuficientes para comparar: se requieren al menos 10 expedientes en cada periodo."
)

var (
	EtiquetasTipoDocumento = map[string]string{
		"ADJUNTO": "Adjunto", "PROVEIDO": "Proveído", "ACTA": "Acta", "INFORME": "Informe", "FORMATO": "Formato", "OTRO": "Otro",
	}
	EtiquetasMovimiento = map[string]string{
		"NOTIFICACION": "Notificación", "ASIGNACION": "Asignación", "RECEPCION": "Recepción", "APROBACION": "Aprobación",
	}
	EtiquetasRol = map[string]string{
		"ADMIN": "Administración", "DIRECTOR": "Dirección", "SUBDIRECTOR": "Subdirección", "SECRETARIA": "Secretaría",
		"DOCENTE": "Docente", "AUXILIAR": "Auxiliar", "PSICOLOGO": "Psicología", "SOLICITANTE": "Solicitante (externo)",
		SinClasificar: "Sin clasificar",
	}
	EtiquetasGranularidad = map[string]string{"DIA": "Por día", "SEMANA": "Por semana (lunes a domingo)", "MES": "Por mes"}
	OrdenRoles            = []string{"ADMIN", "DIRECTOR", "SUBDIRECTOR", "SECRETARIA", "DOCENTE", "AUXILIAR", "PSICOLOGO", "SOLICITANTE", SinClasificar}
	ordenEstados          = []string{"PENDIENTE", "EN_PROCESO", "OBSERVADO", "ATENDIDO"}
)

func etiquetaAreaOSin(clave string) string {
	if clave == SinClasificar {
		return "Sin clasificar"
	}
	return etiqueta(EtiquetasArea, clave)
}

// EtiquetaPeriodo: clave de serie -> texto ("30/10/2026", "Semana del
// 26/10/2026", "Octubre 2026").
func EtiquetaPeriodo(clave, granularidad string) string {
	switch granularidad {
	case "DIA":
		return fechaLegible(clave)
	case "SEMANA":
		return "Semana del " + fechaLegible(clave)
	}
	return EtiquetaMes(clave)
}

// Cruce: celda de tabla cruzada.
type Cruce struct {
	Fila, Columna string
	Cantidad      int
}

// DatosExpedientes: vista "Expedientes y áreas".
type DatosExpedientes struct {
	Base                   Datos
	Serie                  []Conteo
	Granularidad           string
	AreaEstado, TipoEstado []Cruce
	SerieEstado            []Cruce // Fila = clave del periodo, Columna = estado actual
}

// DatosDerivaciones / DatosDocumentos: respuestas agregadas de cada servicio.
type DatosDerivaciones struct {
	Total, SinClasificar                                int
	PorDestino, PorOrigen, PorTipoTramite, Serie, Otros []Conteo
	Flujo                                               []Cruce // Fila = origen, Columna = destino
	Granularidad, Alcance, AreaAlcance                  string
	Generado                                            time.Time
}

type DatosDocumentos struct {
	Total                              int
	PorTipoDocumento, PorTipoTramite   []Conteo
	Serie                              []Conteo
	Granularidad, Alcance, AreaAlcance string
	Generado                           time.Time
}

// FiltrosPeriodo: filtros de las pestañas de derivaciones, documentos y
// actividad (periodo y tipo de trámite).
type FiltrosPeriodo struct {
	Desde, Hasta, Tipo, Granularidad string
}

func textoAlcance(alcance, area string) string {
	if alcance == "AREA" {
		return "Expedientes actualmente en el área " + etiqueta(EtiquetasArea, area)
	}
	return "Toda la institución"
}

func metaBase(titulo string, generado time.Time, periodoCampo, periodo, alcance, area string) [][2]string {
	return [][2]string{
		{"Reporte", titulo},
		{"Institución", Institucion},
		{"Generado", generado.Format("02/01/2006 15:04") + " (hora de Lima)"},
		{periodoCampo, periodo},
		{"Alcance", textoAlcance(alcance, area)},
	}
}

func tablaConteos(titulo, nota, columna string, conteos []Conteo, etiq func(string) string, base int, sinCeros bool) Tabla {
	t := Tabla{Titulo: titulo, Nota: nota, Columnas: []string{columna, "Cantidad", "Porcentaje"}}
	suma := 0
	for _, c := range conteos {
		if sinCeros && c.Cantidad == 0 {
			continue
		}
		t.Filas = append(t.Filas, FilaTabla{Etiqueta: etiq(c.Clave), Valores: []Valor{Entero(c.Cantidad), Porcentaje(pct(c.Cantidad, base))}})
		suma += c.Cantidad
	}
	t.Filas = append(t.Filas, FilaTabla{Etiqueta: "Total", Valores: []Valor{Entero(suma), Porcentaje(pct(suma, base))}, Total: true})
	return t
}

// porCantidad: copia ordenada de mayor a menor cantidad (empates: orden original).
func porCantidad(cs []Conteo) []Conteo {
	out := append([]Conteo{}, cs...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Cantidad > out[j].Cantidad })
	return out
}

// filasPresentes: filas del catálogo con datos y, al final, las que no están
// en el catálogo (orden alfabético).
func filasPresentes(orden []string, presentes map[string]bool) []string {
	out, vistas, extra := []string{}, map[string]bool{}, []string{}
	for _, f := range orden {
		vistas[f] = true
		if presentes[f] {
			out = append(out, f)
		}
	}
	for f, ok := range presentes {
		if ok && !vistas[f] {
			extra = append(extra, f)
		}
	}
	sort.Strings(extra)
	return append(out, extra...)
}

// tablaCruzada: filas x estados, con total por fila y por columna y la
// participación de cada fila en el total. porTotal ordena las filas de mayor
// a menor total (empates: orden del catálogo).
func tablaCruzada(titulo, nota, columnaFila string, orden []string, celdas []Cruce, etiq func(string) string, porTotal bool) Tabla {
	t := Tabla{Titulo: titulo, Nota: nota, Columnas: []string{columnaFila}}
	for _, e := range ordenEstados {
		t.Columnas = append(t.Columnas, etiqueta(EtiquetasEstado, e))
	}
	t.Columnas = append(t.Columnas, "Total", "Participación")
	valor := map[[2]string]int{}
	presentes := map[string]bool{}
	for _, c := range celdas {
		valor[[2]string{c.Fila, c.Columna}] += c.Cantidad
		presentes[c.Fila] = true
	}
	filas := filasPresentes(orden, presentes)
	totalDe := func(fila string) int {
		n := 0
		for _, e := range ordenEstados {
			n += valor[[2]string{fila, e}]
		}
		return n
	}
	if porTotal {
		sort.SliceStable(filas, func(i, j int) bool { return totalDe(filas[i]) > totalDe(filas[j]) })
	}
	general := 0
	for _, fila := range filas {
		general += totalDe(fila)
	}
	totalCol := make([]int, len(ordenEstados))
	for _, fila := range filas {
		vals := []Valor{}
		for i, e := range ordenEstados {
			n := valor[[2]string{fila, e}]
			vals = append(vals, Entero(n))
			totalCol[i] += n
		}
		t.Filas = append(t.Filas, FilaTabla{Etiqueta: etiq(fila), Valores: append(vals, Entero(totalDe(fila)), Porcentaje(pct(totalDe(fila), general)))})
	}
	tot := []Valor{}
	for _, n := range totalCol {
		tot = append(tot, Entero(n))
	}
	t.Filas = append(t.Filas, FilaTabla{Etiqueta: "Total", Valores: append(tot, Entero(general), Porcentaje(pct(general, general))), Total: true})
	return t
}

// tablaResumenEstados: indicadores del periodo, diferenciados. Los estados
// son los ACTUALES de los expedientes registrados en el periodo.
func tablaResumenEstados(d Datos) Tabla {
	pendiente, enProceso := cantidadDe(d.PorEstado, "PENDIENTE"), cantidadDe(d.PorEstado, "EN_PROCESO")
	observados, atendidos := cantidadDe(d.PorEstado, "OBSERVADO"), cantidadDe(d.PorEstado, "ATENDIDO")
	return Tabla{Titulo: "Resumen del periodo",
		Nota:     "Registrados según su fecha de registro. Pendientes, observados y atendidos indican el estado ACTUAL de esos expedientes, no su estado en una fecha pasada.",
		Columnas: []string{"Indicador", "Cantidad", "Porcentaje"}, Filas: []FilaTabla{
			{Etiqueta: "Expedientes registrados", Valores: []Valor{Entero(d.Total), Porcentaje(pct(d.Total, d.Total))}},
			{Etiqueta: "Pendientes (Pendiente + En proceso)", Valores: []Valor{Entero(pendiente + enProceso), Porcentaje(pct(pendiente+enProceso, d.Total))}},
			{Etiqueta: "Observados", Valores: []Valor{Entero(observados), Porcentaje(pct(observados, d.Total))}},
			{Etiqueta: "Atendidos", Valores: []Valor{Entero(atendidos), Porcentaje(pct(atendidos, d.Total))}},
		}}
}

func cantidadDe(cs []Conteo, clave string) int {
	for _, c := range cs {
		if c.Clave == clave {
			return c.Cantidad
		}
	}
	return 0
}

// CargaArea: expedientes Pendientes + En proceso de un área (estado y área
// ACTUALES).
type CargaArea struct {
	Area                        string
	Pendiente, EnProceso, Total int
}

// CargaPendientePorArea: áreas con carga (> 0), de mayor a menor; empates en
// el orden del catálogo de áreas.
func CargaPendientePorArea(areaEstado []Cruce) []CargaArea {
	por := map[string]*CargaArea{}
	for _, c := range areaEstado {
		if c.Columna != "PENDIENTE" && c.Columna != "EN_PROCESO" {
			continue
		}
		if por[c.Fila] == nil {
			por[c.Fila] = &CargaArea{Area: c.Fila}
		}
		if c.Columna == "PENDIENTE" {
			por[c.Fila].Pendiente += c.Cantidad
		} else {
			por[c.Fila].EnProceso += c.Cantidad
		}
		por[c.Fila].Total += c.Cantidad
	}
	presentes := map[string]bool{}
	for k, v := range por {
		presentes[k] = v.Total > 0
	}
	out := []CargaArea{}
	for _, a := range filasPresentes(ordenAreasCruce, presentes) {
		out = append(out, *por[a])
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Total > out[j].Total })
	return out
}

func tablaCargaPendiente(areaEstado []Cruce) Tabla {
	carga := CargaPendientePorArea(areaEstado)
	t := Tabla{Titulo: "Áreas con mayor carga pendiente",
		Nota:     "Expedientes Pendientes y En proceso según el área en la que están HOY, de mayor a menor. No mide plazos ni demoras.",
		Columnas: []string{"Área actual", "Pendiente", "En proceso", "Carga pendiente", "% de la carga"}}
	total, p, e := 0, 0, 0
	for _, c := range carga {
		total += c.Total
	}
	for _, c := range carga {
		t.Filas = append(t.Filas, FilaTabla{Etiqueta: etiqueta(EtiquetasArea, c.Area), Valores: []Valor{Entero(c.Pendiente), Entero(c.EnProceso), Entero(c.Total), Porcentaje(pct(c.Total, total))}})
		p, e = p+c.Pendiente, e+c.EnProceso
	}
	t.Filas = append(t.Filas, FilaTabla{Etiqueta: "Total", Valores: []Valor{Entero(p), Entero(e), Entero(total), Porcentaje(pct(total, total))}, Total: true})
	return t
}

// tablaEvolucionEstados: periodo × estado actual. Manda la serie continua:
// los periodos sin registros aparecen en cero.
func tablaEvolucionEstados(serie []Conteo, celdas []Cruce, granularidad string) Tabla {
	t := Tabla{Titulo: "Evolución de expedientes registrados, por estado actual",
		Nota:     "Fecha usada: fecha de registro (hora de Lima). Cada expediente figura en el periodo en que se registró, con su estado ACTUAL: no es un historial de estados.",
		Columnas: []string{"Periodo"}}
	for _, e := range ordenEstados {
		t.Columnas = append(t.Columnas, etiqueta(EtiquetasEstado, e))
	}
	t.Columnas = append(t.Columnas, "Total")
	valor := map[[2]string]int{}
	for _, c := range celdas {
		valor[[2]string{c.Fila, c.Columna}] += c.Cantidad
	}
	totalCol, general := make([]int, len(ordenEstados)), 0
	for _, p := range serie {
		vals := []Valor{}
		for i, e := range ordenEstados {
			n := valor[[2]string{p.Clave, e}]
			vals = append(vals, Entero(n))
			totalCol[i] += n
		}
		general += p.Cantidad
		t.Filas = append(t.Filas, FilaTabla{Etiqueta: EtiquetaPeriodo(p.Clave, granularidad), Valores: append(vals, Entero(p.Cantidad))})
	}
	tot := []Valor{}
	for _, n := range totalCol {
		tot = append(tot, Entero(n))
	}
	t.Filas = append(t.Filas, FilaTabla{Etiqueta: "Total", Valores: append(tot, Entero(general)), Total: true})
	return t
}

var ordenAreasCruce = []string{"SECRETARIA", "DIRECTOR", "SUBDIRECTOR", "DOCENTE", "AUXILIAR", "ADMIN"}
var ordenTiposCruce = []string{"CERTIFICADO", "CONSTANCIA", "PERMISO", "JUSTIFICACION_FALTA", "JUSTIFICACION_TARDANZA", "SOLICITUD", "TRAMITE", "OFICIO", "OTRO"}

// DocumentoExpedientes: pestaña "Expedientes y áreas".
func DocumentoExpedientes(d DatosExpedientes, f Filtros) Documento {
	titulo := "Reporte de expedientes por periodo y área – Mesa de Partes"
	meta := metadatos(d.Base, f)
	meta[0] = [2]string{"Reporte", titulo}
	meta = append(meta, [2]string{"Agrupación", etiqueta(EtiquetasGranularidad, d.Granularidad)})
	con := func(m map[string]string) func(string) string { return func(k string) string { return etiqueta(m, k) } }
	// Una sola tabla por periodo (sin repetir la serie): con el desglose por
	// estado cuando Expedientes lo entrega.
	serie := tablaEvolucionEstados(d.Serie, d.SerieEstado, d.Granularidad)
	if len(d.SerieEstado) == 0 && d.Base.Total > 0 {
		serie = tablaConteos("Expedientes registrados por periodo", "Fecha usada: fecha de registro (hora de Lima).", "Periodo",
			d.Serie, func(k string) string { return EtiquetaPeriodo(k, d.Granularidad) }, d.Base.Total, false)
	}
	return Documento{Titulo: titulo, Meta: meta, Tablas: []Tabla{
		tablaResumenEstados(d.Base),
		serie,
		tablaCargaPendiente(d.AreaEstado),
		tablaCruzada("Área actual × estado", "Situación actual: área en la que está cada expediente y su estado al generar el reporte. Participación: % del total de expedientes.", "Área", ordenAreasCruce, d.AreaEstado, con(EtiquetasArea), false),
		tablaCruzada("Tipo de trámite × estado (de más a menos frecuente)", "Estado actual de cada expediente. Participación: % del total de expedientes.", "Tipo de trámite", ordenTiposCruce, d.TipoEstado, con(EtiquetasTipo), true),
	}}
}

// DocumentoDerivaciones: pestaña "Derivaciones".
func DocumentoDerivaciones(d DatosDerivaciones, f FiltrosPeriodo) Documento {
	titulo := "Reporte de derivaciones – Mesa de Partes"
	meta := metaBase(titulo, d.Generado, "Periodo (fecha de derivación)", Periodo(Filtros{Desde: f.Desde, Hasta: f.Hasta}), d.Alcance, d.AreaAlcance)
	meta = append(meta, [2]string{"Filtro tipo de trámite", todos(f.Tipo, EtiquetasTipo)}, [2]string{"Agrupación", etiqueta(EtiquetasGranularidad, d.Granularidad)})
	otros := 0
	for _, c := range d.Otros {
		otros += c.Cantidad
	}
	movimientos, aprobaciones := d.Total+otros, cantidadDe(d.Otros, "APROBACION")
	resumen := Tabla{Titulo: "Resumen",
		Nota: "Movimientos de expedientes activos. Las derivaciones entre áreas incluyen las devoluciones a Secretaría: hoy no se registran como rechazo diferenciado. " +
			"Porcentajes: sobre los movimientos registrados, salvo «sin clasificar», que es sobre las derivaciones.",
		Columnas: []string{"Concepto", "Cantidad", "Porcentaje"}, Filas: []FilaTabla{
			{Etiqueta: "Movimientos registrados (todos los tipos)", Valores: []Valor{Entero(movimientos), Porcentaje(pct(movimientos, movimientos))}},
			{Etiqueta: "Derivaciones entre áreas", Valores: []Valor{Entero(d.Total), Porcentaje(pct(d.Total, movimientos))}},
			{Etiqueta: "De ellas, con origen o destino sin clasificar (registros históricos)", Valores: []Valor{Entero(d.SinClasificar), Porcentaje(pct(d.SinClasificar, d.Total))}},
			{Etiqueta: "Otros movimientos (notificaciones, asignaciones, recepciones, aprobaciones)", Valores: []Valor{Entero(otros), Porcentaje(pct(otros, movimientos))}},
			{Etiqueta: "De ellos, aprobaciones", Valores: []Valor{Entero(aprobaciones), Porcentaje(pct(aprobaciones, movimientos))}},
		}}
	flujo := Tabla{Titulo: "Flujo entre áreas (origen → destino)", Columnas: []string{"Origen → destino", "Cantidad", "Porcentaje"}}
	suma := 0
	for _, c := range d.Flujo {
		flujo.Filas = append(flujo.Filas, FilaTabla{Etiqueta: etiquetaAreaOSin(c.Fila) + " → " + etiquetaAreaOSin(c.Columna), Valores: []Valor{Entero(c.Cantidad), Porcentaje(pct(c.Cantidad, d.Total))}})
		suma += c.Cantidad
	}
	flujo.Filas = append(flujo.Filas, FilaTabla{Etiqueta: "Total", Valores: []Valor{Entero(suma), Porcentaje(pct(suma, d.Total))}, Total: true})
	return Documento{Titulo: titulo, Meta: meta, Tablas: []Tabla{
		resumen,
		tablaConteos("Por área de destino", "", "Área", d.PorDestino, etiquetaAreaOSin, d.Total, true),
		tablaConteos("Por área de origen", "", "Área", d.PorOrigen, etiquetaAreaOSin, d.Total, true),
		flujo,
		tablaConteos("Por tipo de trámite (de más a menos frecuente)", "", "Tipo de trámite", porCantidad(d.PorTipoTramite), func(k string) string { return etiqueta(EtiquetasTipo, k) }, d.Total, true),
		tablaConteos("Derivaciones por periodo", "Fecha usada: fecha de la derivación (hora de Lima).", "Periodo", d.Serie, func(k string) string { return EtiquetaPeriodo(k, d.Granularidad) }, d.Total, false),
		tablaConteos("Otros movimientos (solo cantidad)", "Su origen y destino son texto libre: no se analizan por área.", "Tipo de movimiento", d.Otros, func(k string) string { return etiqueta(EtiquetasMovimiento, k) }, otros, false),
	}}
}

// DocumentoDocumentos: pestaña "Documentos".
func DocumentoDocumentos(d DatosDocumentos, f FiltrosPeriodo) Documento {
	titulo := "Reporte de documentos – Mesa de Partes"
	meta := metaBase(titulo, d.Generado, "Periodo (fecha de registro del documento)", Periodo(Filtros{Desde: f.Desde, Hasta: f.Hasta}), d.Alcance, d.AreaAlcance)
	meta = append(meta, [2]string{"Filtro tipo de trámite", todos(f.Tipo, EtiquetasTipo)}, [2]string{"Agrupación", etiqueta(EtiquetasGranularidad, d.Granularidad)})
	return Documento{Titulo: titulo, Meta: meta, Tablas: []Tabla{
		tablaConteos("Por tipo documental (de más a menos frecuente)", "Documentos vigentes (no eliminados) de expedientes activos.", "Tipo documental", porCantidad(d.PorTipoDocumento), func(k string) string { return etiqueta(EtiquetasTipoDocumento, k) }, d.Total, true),
		tablaConteos("Por tipo de trámite del expediente (de más a menos frecuente)", "", "Tipo de trámite", porCantidad(d.PorTipoTramite), func(k string) string { return etiqueta(EtiquetasTipo, k) }, d.Total, true),
		tablaConteos("Documentos por periodo", "Fecha usada: fecha de registro del documento (hora de Lima).", "Periodo", d.Serie, func(k string) string { return EtiquetaPeriodo(k, d.Granularidad) }, d.Total, false),
	}}
}

// ---- Actividad por rol ----

// ActividadRol: acciones registradas por usuarios de un ROL (nunca por persona).
// Movimientos incluye TODOS los tipos (derivaciones, notificaciones,
// asignaciones, recepciones y aprobaciones); Derivaciones es la parte de tipo
// DERIVACION y no se suma aparte. Total = expedientes + movimientos + documentos.
type ActividadRol struct {
	Rol                                                       string
	Expedientes, Movimientos, Derivaciones, Documentos, Total int
}

// AgruparActividadPorRol suma las acciones por el rol ACTUAL de cada usuario.
// Los IDs solo existen dentro de esta función: el resultado no los contiene.
func AgruparActividadPorRol(expedientes, movimientos, derivaciones, documentos map[string]int, rolDe map[string]string) []ActividadRol {
	por := map[string]*ActividadRol{}
	sumar := func(m map[string]int, campo func(*ActividadRol) *int, enTotal bool) {
		for id, n := range m {
			rol := rolDe[id]
			if _, ok := EtiquetasRol[rol]; !ok || rol == "" {
				rol = SinClasificar
			}
			if por[rol] == nil {
				por[rol] = &ActividadRol{Rol: rol}
			}
			*campo(por[rol]) += n
			if enTotal {
				por[rol].Total += n
			}
		}
	}
	sumar(expedientes, func(a *ActividadRol) *int { return &a.Expedientes }, true)
	sumar(movimientos, func(a *ActividadRol) *int { return &a.Movimientos }, true)
	sumar(derivaciones, func(a *ActividadRol) *int { return &a.Derivaciones }, false)
	sumar(documentos, func(a *ActividadRol) *int { return &a.Documentos }, true)
	out := []ActividadRol{}
	for _, rol := range OrdenRoles {
		if a := por[rol]; a != nil {
			out = append(out, *a)
		}
	}
	return out
}

func DocumentoActividad(roles []ActividadRol, f FiltrosPeriodo, alcance, area string, generado time.Time) Documento {
	titulo := "Reporte de actividad institucional por rol – Mesa de Partes"
	meta := metaBase(titulo, generado, "Periodo (fecha de cada acción)", Periodo(Filtros{Desde: f.Desde, Hasta: f.Hasta}), alcance, area)
	meta = append(meta, [2]string{"Filtro tipo de trámite", todos(f.Tipo, EtiquetasTipo)})
	t := Tabla{Titulo: "Actividad por rol",
		Nota: "Agrupada por el rol ACTUAL de cada usuario. No se muestran personas. Volúmenes, no productividad: no reflejan complejidad ni tiempos. " +
			"Movimientos registrados = derivaciones, notificaciones, asignaciones, recepciones y aprobaciones; «de ellos, derivaciones» ya está incluido en movimientos. " +
			"Total de acciones = expedientes + movimientos + documentos.",
		Columnas: []string{"Rol", "Expedientes registrados", "Movimientos registrados", "De ellos, derivaciones entre áreas", "Documentos subidos", "Total de acciones", "Participación"}}
	var e, m, d, doc, tot int
	for _, r := range roles {
		tot += r.Total
	}
	for _, r := range roles {
		t.Filas = append(t.Filas, FilaTabla{Etiqueta: etiqueta(EtiquetasRol, r.Rol), Valores: []Valor{Entero(r.Expedientes), Entero(r.Movimientos), Entero(r.Derivaciones), Entero(r.Documentos), Entero(r.Total), Porcentaje(pct(r.Total, tot))}})
		e, m, d, doc = e+r.Expedientes, m+r.Movimientos, d+r.Derivaciones, doc+r.Documentos
	}
	t.Filas = append(t.Filas, FilaTabla{Etiqueta: "Total", Valores: []Valor{Entero(e), Entero(m), Entero(d), Entero(doc), Entero(tot), Porcentaje(pct(tot, tot))}, Total: true})
	return Documento{Titulo: titulo, Meta: meta, Tablas: []Tabla{t}}
}

// ---- Comparación de periodos ----

var (
	ErrComparacionFechas = errors.New("indique la fecha desde y la fecha hasta del periodo a comparar")
	ErrModo              = errors.New("el modo de comparación debe ser ANTERIOR, ANIO_ANTERIOR o PERSONALIZADO")
	ErrPersonalizado     = errors.New("indique desde_b y hasta_b para la comparación personalizada")
)

// PeriodoB calcula el periodo de referencia. Las fechas ya validadas por
// formato se vuelven a validar en Expedientes al consultar.
func PeriodoB(desde, hasta, modo, desdeB, hastaB string) (string, string, error) {
	if desde == "" || hasta == "" {
		return "", "", ErrComparacionFechas
	}
	a, err1 := time.Parse("2006-01-02", desde)
	b, err2 := time.Parse("2006-01-02", hasta)
	if err1 != nil || err2 != nil {
		return "", "", ErrComparacionFechas
	}
	switch strings.ToUpper(strings.TrimSpace(modo)) {
	case "", ModoAnterior:
		dias := int(b.Sub(a).Hours()/24) + 1
		fin := a.AddDate(0, 0, -1)
		return fin.AddDate(0, 0, -(dias - 1)).Format("2006-01-02"), fin.Format("2006-01-02"), nil
	case ModoAnioAnterior:
		return a.AddDate(-1, 0, 0).Format("2006-01-02"), b.AddDate(-1, 0, 0).Format("2006-01-02"), nil
	case ModoPersonalizado:
		if desdeB == "" || hastaB == "" {
			return "", "", ErrPersonalizado
		}
		return desdeB, hastaB, nil
	}
	return "", "", ErrModo
}

// ResumenPeriodo: lo necesario de un reporte para comparar.
type ResumenPeriodo struct {
	Total, Pendientes                         int
	PorEstado, PorTipo, PorPrioridad, PorArea []Conteo
}

type FilaComparacion struct {
	Grupo, Clave     string
	Actual, Anterior int
	TieneVariacion   bool
	Variacion        float64 // porcentaje
}

type Comparacion struct {
	Comparable bool
	Mensaje    string
	Filas      []FilaComparacion
}

// Comparar: las variaciones SOLO se calculan si ambos periodos tienen al
// menos MinimoComparacion expedientes; con base 0 no hay variación ("sin
// base"). Nunca se estiman datos faltantes.
func Comparar(a, b ResumenPeriodo) Comparacion {
	c := Comparacion{Comparable: a.Total >= MinimoComparacion && b.Total >= MinimoComparacion}
	if !c.Comparable {
		c.Mensaje = MensajeInsuficiente
	}
	fila := func(grupo, clave string, actual, anterior int) {
		f := FilaComparacion{Grupo: grupo, Clave: clave, Actual: actual, Anterior: anterior}
		if c.Comparable && anterior > 0 {
			f.TieneVariacion = true
			f.Variacion = float64(actual-anterior) / float64(anterior) * 100
		}
		c.Filas = append(c.Filas, f)
	}
	cant := func(cs []Conteo, k string) int {
		for _, x := range cs {
			if x.Clave == k {
				return x.Cantidad
			}
		}
		return 0
	}
	fila("RESUMEN", "TOTAL", a.Total, b.Total)
	fila("RESUMEN", "PENDIENTES", a.Pendientes, b.Pendientes)
	grupo := func(nombre string, ca, cb []Conteo, sinCeros bool) {
		vistos := map[string]bool{}
		for _, lista := range [][]Conteo{ca, cb} {
			for _, x := range lista {
				if vistos[x.Clave] {
					continue
				}
				vistos[x.Clave] = true
				va, vb := cant(ca, x.Clave), cant(cb, x.Clave)
				if sinCeros && va == 0 && vb == 0 {
					continue
				}
				fila(nombre, x.Clave, va, vb)
			}
		}
	}
	grupo("ESTADO", a.PorEstado, b.PorEstado, false)
	grupo("TIPO", a.PorTipo, b.PorTipo, true)
	grupo("PRIORIDAD", a.PorPrioridad, b.PorPrioridad, false)
	grupo("AREA", a.PorArea, b.PorArea, true)
	return c
}

var etiquetasGrupo = map[string]string{"RESUMEN": "Resumen", "ESTADO": "Por estado", "TIPO": "Por tipo de trámite", "PRIORIDAD": "Por prioridad", "AREA": "Por área actual"}

// EtiquetaFilaComparacion: texto de la clave según su grupo.
func EtiquetaFilaComparacion(grupo, clave string) string {
	switch grupo {
	case "RESUMEN":
		return map[string]string{"TOTAL": "Total de expedientes", "PENDIENTES": "Pendientes (Pendiente + En proceso)"}[clave]
	case "ESTADO":
		return etiqueta(EtiquetasEstado, clave)
	case "TIPO":
		return etiqueta(EtiquetasTipo, clave)
	case "PRIORIDAD":
		return etiqueta(EtiquetasPrioridad, clave)
	case "AREA":
		return etiqueta(EtiquetasArea, clave)
	}
	return clave
}

func DocumentoComparacion(c Comparacion, f Filtros, desdeB, hastaB, alcance, area string, generado time.Time) Documento {
	titulo := "Comparación de periodos – Mesa de Partes"
	meta := [][2]string{
		{"Reporte", titulo},
		{"Institución", Institucion},
		{"Generado", generado.Format("02/01/2006 15:04") + " (hora de Lima)"},
		{"Periodo actual (fecha de registro)", Periodo(Filtros{Desde: f.Desde, Hasta: f.Hasta})},
		{"Periodo de referencia (fecha de registro)", Periodo(Filtros{Desde: desdeB, Hasta: hastaB})},
		{"Alcance", textoAlcance(alcance, area)},
		{"Filtro estado", todos(f.Estado, EtiquetasEstado)},
		{"Filtro tipo de trámite", todos(f.Tipo, EtiquetasTipo)},
		{"Filtro prioridad", todos(f.Prioridad, EtiquetasPrioridad)},
		{"Filtro área", todos(f.Area, EtiquetasArea)},
		{"Regla", fmt.Sprintf("Variación solo si ambos periodos tienen al menos %d expedientes", MinimoComparacion)},
	}
	if !c.Comparable {
		meta = append(meta, [2]string{"Resultado", MensajeInsuficiente})
	}
	doc := Documento{Titulo: titulo, Meta: meta}
	var actual *Tabla
	for _, fl := range c.Filas {
		if actual == nil || actual.Titulo != etiquetasGrupo[fl.Grupo] {
			doc.Tablas = append(doc.Tablas, Tabla{Titulo: etiquetasGrupo[fl.Grupo], Columnas: []string{"Concepto", "Periodo actual", "Periodo de referencia", "Diferencia", "Variación", "Tendencia"}})
			actual = &doc.Tablas[len(doc.Tablas)-1]
		}
		variacion := Texto("No comparable")
		switch {
		case fl.TieneVariacion:
			p := fl.Variacion / 100
			variacion = Porcentaje(&p)
		case c.Comparable:
			variacion = Texto("Sin base (0 en el periodo de referencia)")
		}
		diferencia, tendencia := Texto("No comparable"), Texto("")
		if c.Comparable {
			diferencia, tendencia = Entero(fl.Actual-fl.Anterior), Texto(Tendencia(fl.Actual, fl.Anterior))
		}
		actual.Filas = append(actual.Filas, FilaTabla{Etiqueta: EtiquetaFilaComparacion(fl.Grupo, fl.Clave), Valores: []Valor{Entero(fl.Actual), Entero(fl.Anterior), diferencia, variacion, tendencia}})
	}
	return doc
}

// Tendencia: sentido del cambio (solo se informa en periodos comparables).
func Tendencia(actual, anterior int) string {
	switch {
	case actual > anterior:
		return "Aumento"
	case actual < anterior:
		return "Disminución"
	}
	return "Sin cambio"
}

// NombreArchivoDe: reporte-<tipo>-AAAAMMDD-HHMM.<formato>.
func NombreArchivoDe(tipo string, generado time.Time, formato string) string {
	return "reporte-" + tipo + "-" + generado.Format("20060102-1504") + "." + formato
}
