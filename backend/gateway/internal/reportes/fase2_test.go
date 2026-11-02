package reportes

import (
	"strings"
	"testing"
	"time"
)

var generadoPrueba = time.Date(2026, 10, 30, 9, 15, 0, 0, time.FixedZone("Lima", -5*3600))

func TestPeriodoB(t *testing.T) {
	casos := []struct {
		modo, db, hb string
		desde, hasta string
	}{
		{"", "2026-09-01", "2026-09-30", "2026-08-02", "2026-08-31"},              // 30 días antes
		{"ANTERIOR", "2026-10-01", "2026-10-07", "2026-09-24", "2026-09-30"},      // semana anterior
		{"ANIO_ANTERIOR", "2026-03-01", "2026-03-31", "2025-03-01", "2025-03-31"}, // mismo periodo del año anterior
		{"PERSONALIZADO", "2026-10-01", "2026-10-30", "2025-01-01", "2025-01-31"},
	}
	for _, c := range casos {
		desdeB, hastaB := "", ""
		if c.modo == "PERSONALIZADO" {
			desdeB, hastaB = "2025-01-01", "2025-01-31"
		}
		d, h, err := PeriodoB(c.db, c.hb, c.modo, desdeB, hastaB)
		if err != nil || d != c.desde || h != c.hasta {
			t.Errorf("%s: %s..%s %v (esperado %s..%s)", c.modo, d, h, err, c.desde, c.hasta)
		}
	}
	for _, e := range []struct{ d, h, modo, db, hb string }{
		{"", "2026-10-30", "", "", ""}, {"2026-10-01", "", "", "", ""}, {"2026-10-01", "2026-10-30", "X", "", ""}, {"2026-10-01", "2026-10-30", "PERSONALIZADO", "", ""},
	} {
		if _, _, err := PeriodoB(e.d, e.h, e.modo, e.db, e.hb); err == nil {
			t.Errorf("%v: se esperaba error", e)
		}
	}
}

func resumen(total, pend int, estado map[string]int) ResumenPeriodo {
	r := ResumenPeriodo{Total: total, Pendientes: pend}
	for _, k := range []string{"PENDIENTE", "EN_PROCESO", "OBSERVADO", "ATENDIDO"} {
		r.PorEstado = append(r.PorEstado, Conteo{k, estado[k]})
	}
	return r
}

func TestComparar_Umbral(t *testing.T) {
	// 9 frente a 50: NO comparable, sin variaciones.
	c := Comparar(resumen(9, 3, map[string]int{"PENDIENTE": 3, "ATENDIDO": 6}), resumen(50, 10, map[string]int{"PENDIENTE": 10, "ATENDIDO": 40}))
	if c.Comparable || c.Mensaje != MensajeInsuficiente {
		t.Fatal("con menos de 10 en un periodo no es comparable")
	}
	for _, f := range c.Filas {
		if f.TieneVariacion {
			t.Fatalf("sin variaciones si no es comparable: %+v", f)
		}
	}
	// Exactamente 10 y 10: comparable.
	c = Comparar(resumen(12, 4, map[string]int{"PENDIENTE": 4, "ATENDIDO": 8}), resumen(10, 0, map[string]int{"ATENDIDO": 10}))
	if !c.Comparable || c.Mensaje != "" {
		t.Fatal("10 en ambos periodos es comparable")
	}
	por := map[string]FilaComparacion{}
	for _, f := range c.Filas {
		por[f.Grupo+"/"+f.Clave] = f
	}
	if f := por["RESUMEN/TOTAL"]; !f.TieneVariacion || f.Variacion != 20 {
		t.Fatalf("variación total: %+v", f)
	}
	if f := por["ESTADO/ATENDIDO"]; !f.TieneVariacion || f.Variacion != -20 {
		t.Fatalf("variación atendidos: %+v", f)
	}
	// Base 0: sin variación (no se inventa un infinito).
	if f := por["RESUMEN/PENDIENTES"]; f.TieneVariacion {
		t.Fatalf("con base 0 no hay variación: %+v", f)
	}
}

func TestDocumentoComparacion_InsuficienteSinVariaciones(t *testing.T) {
	c := Comparar(resumen(3, 1, map[string]int{"PENDIENTE": 1}), resumen(2, 0, nil))
	b, err := GenerarDocumento(DocumentoComparacion(c, Filtros{Desde: "2026-10-01", Hasta: "2026-10-30"}, "2026-09-01", "2026-09-30", "INSTITUCION", "", generadoPrueba), FormatoCSV)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, e := range []string{"Datos insuficientes para comparar", "Periodo actual (fecha de registro);Del 01/10/2026 al 30/10/2026", "Periodo de referencia (fecha de registro);Del 01/09/2026 al 30/09/2026", "No comparable"} {
		if !strings.Contains(s, e) {
			t.Errorf("falta %q", e)
		}
	}
	if strings.Contains(s, "%") {
		t.Fatal("no debe haber porcentajes de variación con datos insuficientes")
	}
}

func TestAgruparActividadPorRol_SinIdentidades(t *testing.T) {
	roles := AgruparActividadPorRol(
		map[string]int{"u-sec": 5, "u-sol": 3},
		map[string]int{"u-sec": 2, "u-dir": 4}, // movimientos (todos los tipos)
		map[string]int{"u-sec": 1, "u-dir": 3}, // de ellos, derivaciones
		map[string]int{"u-dir": 1, "u-perdido": 2},
		map[string]string{"u-sec": "SECRETARIA", "u-sol": "SOLICITANTE", "u-dir": "DIRECTOR"},
	)
	por := map[string]ActividadRol{}
	orden := []string{}
	for _, r := range roles {
		por[r.Rol] = r
		orden = append(orden, r.Rol)
	}
	if strings.Join(orden, ",") != "DIRECTOR,SECRETARIA,SOLICITANTE,SIN_CLASIFICAR" {
		t.Fatalf("orden fijo por rol (no ranking): %v", orden)
	}
	// Las derivaciones son parte de los movimientos: no se suman dos veces.
	if s := por["SECRETARIA"]; s.Expedientes != 5 || s.Movimientos != 2 || s.Derivaciones != 1 || s.Total != 7 {
		t.Fatalf("secretaría: %+v", s)
	}
	if d := por["DIRECTOR"]; d.Movimientos != 4 || d.Derivaciones != 3 || d.Documentos != 1 || d.Total != 5 {
		t.Fatalf("dirección: %+v", d)
	}
	if por["SIN_CLASIFICAR"].Documentos != 2 {
		t.Fatal("usuario sin rol conocido -> Sin clasificar")
	}
	doc := DocumentoActividad(roles, FiltrosPeriodo{}, "INSTITUCION", "", generadoPrueba)
	b, _ := GenerarDocumento(doc, FormatoCSV)
	for _, prohibido := range []string{"u-sec", "u-dir", "u-sol", "u-perdido"} {
		if strings.Contains(string(b), prohibido) {
			t.Fatalf("el archivo contiene un identificador: %s", prohibido)
		}
	}
	for _, e := range []string{"Rol;Expedientes registrados;Movimientos registrados;De ellos, derivaciones entre áreas;Documentos subidos;Total de acciones;Participación",
		"Solicitante (externo);3;0;0;0;3;17,6 %", "Total;8;6;4;3;17;100,0 %"} {
		if !strings.Contains(string(b), e) {
			t.Fatalf("falta %q en actividad:\n%s", e, b)
		}
	}
	if strings.Contains(string(b), "Derivaciones registradas") {
		t.Fatal("la columna de movimientos no debe llamarse «Derivaciones registradas»")
	}
}

func TestDocumentoDerivaciones_EtiquetasYSinClasificar(t *testing.T) {
	d := DatosDerivaciones{
		Total: 5, SinClasificar: 1,
		PorDestino:     []Conteo{{"SECRETARIA", 1}, {"DIRECTOR", 3}, {"SUBDIRECTOR", 0}, {SinClasificar, 1}},
		PorOrigen:      []Conteo{{"SECRETARIA", 4}, {SinClasificar, 1}},
		PorTipoTramite: []Conteo{{"CONSTANCIA", 5}},
		Serie:          []Conteo{{"2026-10", 5}},
		Otros:          []Conteo{{"NOTIFICACION", 2}, {"ASIGNACION", 0}},
		Flujo:          []Cruce{{"SECRETARIA", "DIRECTOR", 3}, {SinClasificar, SinClasificar, 1}},
		Granularidad:   "MES", Alcance: "AREA", AreaAlcance: "DIRECTOR", Generado: generadoPrueba,
	}
	b, err := GenerarDocumento(DocumentoDerivaciones(d, FiltrosPeriodo{Desde: "2026-10-01", Tipo: "CONSTANCIA"}), FormatoCSV)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, e := range []string{"Periodo (fecha de derivación);Desde el 01/10/2026", "Expedientes actualmente en el área Dirección", "Filtro tipo de trámite;Constancia de Estudios",
		"Secretaría → Dirección;3", "Sin clasificar → Sin clasificar;1", "Notificación;2", "Octubre 2026;5", "no se registran como rechazo diferenciado"} {
		if !strings.Contains(s, e) {
			t.Errorf("falta %q", e)
		}
	}
	if strings.Contains(s, "Subdirección;0") {
		t.Error("las áreas en cero no se listan")
	}
}

func TestDocumentoExpedientes_CrucesConTotales(t *testing.T) {
	d := DatosExpedientes{
		Base:         Datos{Total: 6, Generado: generadoPrueba},
		Serie:        []Conteo{{"2026-10-26", 6}},
		Granularidad: "SEMANA",
		AreaEstado:   []Cruce{{"SECRETARIA", "PENDIENTE", 2}, {"DIRECTOR", "EN_PROCESO", 3}, {"DIRECTOR", "ATENDIDO", 1}},
		TipoEstado:   []Cruce{{"CONSTANCIA", "PENDIENTE", 2}, {"PERMISO", "EN_PROCESO", 3}, {"PERMISO", "ATENDIDO", 1}},
	}
	b, _ := GenerarDocumento(DocumentoExpedientes(d, Filtros{}), FormatoCSV)
	s := string(b)
	for _, e := range []string{"Agrupación;Por semana (lunes a domingo)", "Semana del 26/10/2026;6", "Área;Pendiente;En proceso;Observado;Atendido;Total",
		"Secretaría;2;0;0;0;2", "Dirección;0;3;0;1;4", "Total;2;3;0;1;6", "Permiso;0;3;0;1;4"} {
		if !strings.Contains(s, e) {
			t.Errorf("falta %q en:\n%s", e, s)
		}
	}
}

func TestDocumentoDocumentos(t *testing.T) {
	d := DatosDocumentos{Total: 3, PorTipoDocumento: []Conteo{{"ADJUNTO", 2}, {"PROVEIDO", 1}}, PorTipoTramite: []Conteo{{"PERMISO", 3}},
		Serie: []Conteo{{"2026-10-30", 3}}, Granularidad: "DIA", Alcance: "INSTITUCION", Generado: generadoPrueba}
	b, _ := GenerarDocumento(DocumentoDocumentos(d, FiltrosPeriodo{}), FormatoXLSX)
	if len(b) < 100 || string(b[:2]) != "PK" {
		t.Fatal("XLSX inválido")
	}
	c, _ := GenerarDocumento(DocumentoDocumentos(d, FiltrosPeriodo{}), FormatoCSV)
	for _, e := range []string{"Proveído;1", "30/10/2026;3", "Periodo (fecha de registro del documento);Todo el historial"} {
		if !strings.Contains(string(c), e) {
			t.Errorf("falta %q", e)
		}
	}
	if NombreArchivoDe("documentos", generadoPrueba, "csv") != "reporte-documentos-20261030-0915.csv" {
		t.Fatal("nombre de archivo")
	}
}

func TestDocumentoExpedientes_EvolucionCargaYFrecuencia(t *testing.T) {
	d := DatosExpedientes{
		Base: Datos{Total: 7, Generado: generadoPrueba,
			PorEstado: []Conteo{{"PENDIENTE", 3}, {"EN_PROCESO", 3}, {"OBSERVADO", 0}, {"ATENDIDO", 1}}},
		Serie:        []Conteo{{"2026-09", 0}, {"2026-10", 7}},
		Granularidad: "MES",
		SerieEstado:  []Cruce{{"2026-10", "PENDIENTE", 3}, {"2026-10", "EN_PROCESO", 3}, {"2026-10", "ATENDIDO", 1}},
		AreaEstado:   []Cruce{{"SECRETARIA", "PENDIENTE", 1}, {"DIRECTOR", "PENDIENTE", 2}, {"DIRECTOR", "EN_PROCESO", 3}, {"DOCENTE", "ATENDIDO", 1}},
		TipoEstado:   []Cruce{{"CERTIFICADO", "PENDIENTE", 1}, {"PERMISO", "PENDIENTE", 2}, {"PERMISO", "EN_PROCESO", 3}, {"PERMISO", "ATENDIDO", 1}},
	}
	b, _ := GenerarDocumento(DocumentoExpedientes(d, Filtros{}), FormatoCSV)
	s := string(b)
	for _, e := range []string{
		// Resumen diferenciado (estado actual).
		"Expedientes registrados;7;100,0 %", "Pendientes (Pendiente + En proceso);6;85,7 %", "Observados;0;0,0 %", "Atendidos;1;14,3 %",
		// Evolución: serie continua (septiembre en cero) por estado.
		"Evolución de expedientes registrados, por estado actual", "Septiembre 2026;0;0;0;0;0", "Octubre 2026;3;3;0;1;7", "Total;3;3;0;1;7",
		// Carga pendiente: de mayor a menor; Docente (solo atendidos) no aparece.
		"Área actual;Pendiente;En proceso;Carga pendiente;% de la carga", "Dirección;2;3;5;83,3 %", "Secretaría;1;0;1;16,7 %",
		// Tipo × estado ordenado por frecuencia, con participación.
		"Permiso;2;3;0;1;6;85,7 %",
	} {
		if !strings.Contains(s, e) {
			t.Errorf("falta %q en:\n%s", e, s)
		}
	}
	if strings.Index(s, "Dirección;2;3;5") > strings.Index(s, "Secretaría;1;0;1;") {
		t.Error("la carga pendiente debe ir de mayor a menor")
	}
	if strings.Index(s, "Permiso;2;3;0;1;6") > strings.Index(s, "Certificado de Estudios;1;") {
		t.Error("tipo × estado debe ir de más a menos frecuente")
	}
	carga := s[strings.Index(s, "Áreas con mayor carga pendiente"):strings.Index(s, "Área actual × estado")]
	if strings.Contains(carga, "Docente") {
		t.Errorf("un área sin carga pendiente no figura en la tabla de carga:\n%s", carga)
	}
	if strings.Contains(s, "Expedientes registrados por periodo") {
		t.Error("con evolución por estado no se repite la serie simple")
	}
}

func TestDocumentoComparacion_DiferenciaYTendencia(t *testing.T) {
	c := Comparar(resumen(12, 4, map[string]int{"PENDIENTE": 4}), resumen(10, 6, map[string]int{"PENDIENTE": 6}))
	b, _ := GenerarDocumento(DocumentoComparacion(c, Filtros{Desde: "2026-10-01", Hasta: "2026-10-30"}, "2026-09-01", "2026-09-30", "INSTITUCION", "", generadoPrueba), FormatoCSV)
	s := string(b)
	for _, e := range []string{"Concepto;Periodo actual;Periodo de referencia;Diferencia;Variación;Tendencia",
		"Total de expedientes;12;10;2;20,0 %;Aumento", "Pendientes (Pendiente + En proceso);4;6;-2;-33,3 %;Disminución"} {
		if !strings.Contains(s, e) {
			t.Errorf("falta %q en:\n%s", e, s)
		}
	}
	insuf := Comparar(resumen(12, 4, nil), resumen(9, 6, nil))
	b2, _ := GenerarDocumento(DocumentoComparacion(insuf, Filtros{Desde: "2026-10-01", Hasta: "2026-10-30"}, "2026-09-01", "2026-09-30", "INSTITUCION", "", generadoPrueba), FormatoCSV)
	if !strings.Contains(string(b2), "Total de expedientes;12;9;No comparable;No comparable;") || strings.Contains(string(b2), "Aumento") {
		t.Fatalf("sin el mínimo no hay diferencia ni tendencia:\n%s", b2)
	}
	if Tendencia(5, 5) != "Sin cambio" {
		t.Fatal("tendencia sin cambio")
	}
}

func TestDocumentoDerivaciones_IndicadoresDiferenciados(t *testing.T) {
	d := DatosDerivaciones{Total: 4, SinClasificar: 1, Otros: []Conteo{{"NOTIFICACION", 2}, {"APROBACION", 2}},
		PorTipoTramite: []Conteo{{"CONSTANCIA", 1}, {"PERMISO", 3}}, Alcance: "INSTITUCION", Generado: generadoPrueba}
	b, _ := GenerarDocumento(DocumentoDerivaciones(d, FiltrosPeriodo{}), FormatoCSV)
	s := string(b)
	for _, e := range []string{"Movimientos registrados (todos los tipos);8;100,0 %", "Derivaciones entre áreas;4;50,0 %",
		"De ellas, con origen o destino sin clasificar (registros históricos);1;25,0 %", "De ellos, aprobaciones;2;25,0 %"} {
		if !strings.Contains(s, e) {
			t.Errorf("falta %q en:\n%s", e, s)
		}
	}
	if strings.Index(s, "Permiso;3") > strings.Index(s, "Constancia") {
		t.Error("tipo de trámite de más a menos frecuente")
	}
}
