import { ESTADOS, PRIORIDADES, TIPOS } from "../../constants/expedientes";
import { roleLabel } from "../../constants/roles";
import { extractBackendMessage, friendlyErrorMessage } from "../../utils/apiErrors";

// Utilidades puras del módulo de Reportes (Fase 1): filtros, etiquetas y
// formato. Las cifras siempre vienen del backend (GET /api/reportes/expedientes).

export const FILTROS_VACIOS = { desde: "", hasta: "", estado: "", tipo: "", prioridad: "", area: "" };

// Áreas reales de area_actual (mismos códigos que el backend).
export const AREAS = ["SECRETARIA", "DIRECTOR", "SUBDIRECTOR", "DOCENTE", "AUXILIAR", "ADMIN"].map((value) => ({
    value,
    label: roleLabel(value),
}));

export const RANGOS_ANTIGUEDAD = [
    { clave: "0-3", label: "0 a 3 días" },
    { clave: "4-7", label: "4 a 7 días" },
    { clave: "8-15", label: "8 a 15 días" },
    { clave: "16-30", label: "16 a 30 días" },
    { clave: "31+", label: "Más de 30 días" },
];

const MESES = ["Enero", "Febrero", "Marzo", "Abril", "Mayo", "Junio", "Julio", "Agosto", "Septiembre", "Octubre", "Noviembre", "Diciembre"];

const etiquetaDe = (lista) => (valor) => lista.find((item) => item.value === valor)?.label ?? valor;
export const etiquetaEstado = etiquetaDe(ESTADOS);
export const etiquetaTipo = etiquetaDe(TIPOS);
export const etiquetaPrioridad = etiquetaDe(PRIORIDADES);
export const etiquetaArea = etiquetaDe(AREAS);

// "2026-03" -> { corta: "mar 26", larga: "Marzo 2026" }
export function etiquetaMes(clave) {
    const [anio, mes] = String(clave).split("-").map(Number);
    if (!anio || !mes || mes < 1 || mes > 12) return { corta: clave, larga: clave };
    return { corta: `${MESES[mes - 1].slice(0, 3).toLowerCase()} ${String(anio).slice(2)}`, larga: `${MESES[mes - 1]} ${anio}` };
}

// Fecha local AAAA-MM-DD (la del navegador; el backend usa hora de Lima).
export function fechaISO(fecha) {
    const y = fecha.getFullYear();
    const m = String(fecha.getMonth() + 1).padStart(2, "0");
    const d = String(fecha.getDate()).padStart(2, "0");
    return `${y}-${m}-${d}`;
}

// "2026-03-05" -> "05/03/2026"
export function fechaLegible(iso) {
    const [y, m, d] = String(iso).split("-");
    return y && m && d ? `${d}/${m}/${y}` : iso;
}

// Atajos de periodo (sobre la fecha de registro).
export function atajosPeriodo(hoy = new Date()) {
    const y = hoy.getFullYear();
    const m = hoy.getMonth();
    const hace29 = new Date(y, m, hoy.getDate() - 29);
    return [
        { id: "mes", label: "Este mes", desde: fechaISO(new Date(y, m, 1)), hasta: fechaISO(hoy) },
        { id: "mes-anterior", label: "Mes anterior", desde: fechaISO(new Date(y, m - 1, 1)), hasta: fechaISO(new Date(y, m, 0)) },
        { id: "30", label: "Últimos 30 días", desde: fechaISO(hace29), hasta: fechaISO(hoy) },
        { id: "anio", label: "Este año", desde: fechaISO(new Date(y, 0, 1)), hasta: fechaISO(hoy) },
    ];
}

// Devuelve el mensaje de error o "" si los filtros son válidos (el backend
// vuelve a validar todo).
export function validarFiltros(filtros, hoy = new Date()) {
    const hoyISO = fechaISO(hoy);
    if (filtros.desde && filtros.desde > hoyISO) return "La fecha desde no puede ser posterior a hoy.";
    if (filtros.desde && filtros.hasta && filtros.desde > filtros.hasta) {
        return "La fecha desde no puede ser posterior a la fecha hasta.";
    }
    return "";
}

export function textoPeriodo({ desde, hasta }) {
    if (desde && hasta) return `Del ${fechaLegible(desde)} al ${fechaLegible(hasta)}`;
    if (desde) return `Desde el ${fechaLegible(desde)}`;
    if (hasta) return `Hasta el ${fechaLegible(hasta)}`;
    return "Todo el historial";
}

// Filtros aplicados en texto (para el encabezado impreso).
export function textoFiltros(filtros) {
    const partes = [
        filtros.estado && `Estado: ${etiquetaEstado(filtros.estado)}`,
        filtros.tipo && `Tipo: ${etiquetaTipo(filtros.tipo)}`,
        filtros.prioridad && `Prioridad: ${etiquetaPrioridad(filtros.prioridad)}`,
        filtros.area && `Área: ${etiquetaArea(filtros.area)}`,
    ].filter(Boolean);
    return partes.length ? partes.join(" · ") : "Sin filtros adicionales";
}

export function textoAlcance(reporte) {
    if (reporte?.alcance === "AREA") {
        return `Expedientes que están actualmente en el área ${etiquetaArea(reporte.area_alcance)}.`;
    }
    return "Expedientes de toda la institución.";
}

export function cantidadDe(conteos, clave) {
    return conteos?.find((c) => c.clave === clave)?.cantidad ?? 0;
}

// Conteos -> items de BarList.
export function aItems(conteos = [], etiqueta, { sinCeros = false } = {}) {
    return conteos
        .filter((c) => !sinCeros || c.cantidad > 0)
        .map((c) => ({ key: c.clave, label: etiqueta(c.clave), value: c.cantidad }));
}

export function porcentaje(valor, total) {
    if (!total) return "—";
    return `${((valor * 100) / total).toFixed(1).replace(".", ",")} %`;
}

// Mensajes de error del módulo: 429 propio y sin el prefijo técnico
// "validation_error:" que envía el backend.
export function mensajeError(err) {
    if (err?.response?.status === 429) return "Demasiadas solicitudes seguidas. Espera unos segundos e intenta nuevamente.";
    if (err?.response?.status === 400) {
        const msg = extractBackendMessage(err.response.data);
        if (msg) {
            const limpio = msg.replace(/^validation_error:\s*/, "");
            return limpio.charAt(0).toUpperCase() + limpio.slice(1) + ".";
        }
    }
    return friendlyErrorMessage(err, "No se encontró el reporte.");
}

// ---- Fase 2 ----

// Filtros completos del panel compartido (los de la Fase 1 + Fase 2).
export const FILTROS_INICIALES = { ...FILTROS_VACIOS, granularidad: "MES", modo: "ANTERIOR", desdeB: "", hastaB: "" };

// Solo los filtros de la Fase 1 (el Resumen los usa tal cual).
export function filtrosResumen({ desde, hasta, estado, tipo, prioridad, area }) {
    return { desde, hasta, estado, tipo, prioridad, area };
}

// Pestañas: qué filtros aplica cada una (el resto se desactiva con aviso).
export const VISTAS = [
    { id: "resumen", label: "Resumen", filtros: ["desde", "hasta", "estado", "tipo", "prioridad", "area"] },
    { id: "expedientes", label: "Expedientes y áreas", filtros: ["desde", "hasta", "estado", "tipo", "prioridad", "area", "granularidad"] },
    { id: "derivaciones", label: "Derivaciones", filtros: ["desde", "hasta", "tipo", "granularidad"] },
    { id: "documentos", label: "Documentos", filtros: ["desde", "hasta", "tipo", "granularidad"] },
    { id: "actividad", label: "Actividad por rol", filtros: ["desde", "hasta", "tipo"] },
    { id: "comparacion", label: "Comparación", filtros: ["desde", "hasta", "estado", "tipo", "prioridad", "area", "modo"] },
];

export const vistaPorId = (id) => VISTAS.find((v) => v.id === id) ?? VISTAS[0];

export const GRANULARIDADES = [
    { value: "DIA", label: "Día" },
    { value: "SEMANA", label: "Semana" },
    { value: "MES", label: "Mes" },
];

export const MODOS_COMPARACION = [
    { value: "ANTERIOR", label: "Periodo anterior equivalente" },
    { value: "ANIO_ANTERIOR", label: "Mismo periodo del año anterior" },
    { value: "PERSONALIZADO", label: "Periodo personalizado" },
];

export const MINIMO_COMPARACION = 10;

const ETIQUETAS_AREA_EXT = { SIN_CLASIFICAR: "Sin clasificar" };
export const etiquetaAreaHistorica = (clave) => ETIQUETAS_AREA_EXT[clave] ?? etiquetaArea(clave);

const TIPOS_DOCUMENTO = { ADJUNTO: "Adjunto", PROVEIDO: "Proveído", ACTA: "Acta", INFORME: "Informe", FORMATO: "Formato", OTRO: "Otro" };
export const etiquetaTipoDocumento = (clave) => TIPOS_DOCUMENTO[clave] ?? clave;

const MOVIMIENTOS = { NOTIFICACION: "Notificación", ASIGNACION: "Asignación", RECEPCION: "Recepción", APROBACION: "Aprobación" };
export const etiquetaMovimiento = (clave) => MOVIMIENTOS[clave] ?? clave;

const ROLES = {
    ADMIN: "Administración", DIRECTOR: "Dirección", SUBDIRECTOR: "Subdirección", SECRETARIA: "Secretaría",
    DOCENTE: "Docente", AUXILIAR: "Auxiliar", PSICOLOGO: "Psicología", SOLICITANTE: "Solicitante (externo)", SIN_CLASIFICAR: "Sin clasificar",
};
export const etiquetaRol = (clave) => ROLES[clave] ?? clave;

// Clave de serie -> etiquetas corta (eje) y larga (tooltip/tabla).
export function etiquetaSerie(clave, granularidad) {
    if (granularidad === "MES") return etiquetaMes(clave);
    const [y, m, d] = String(clave).split("-");
    if (!y || !m || !d) return { corta: clave, larga: clave };
    if (granularidad === "SEMANA") return { corta: `sem ${d}/${m}`, larga: `Semana del ${d}/${m}/${y}` };
    return { corta: `${d}/${m}`, larga: `${d}/${m}/${y}` };
}

export function aPuntos(serie = [], granularidad) {
    return serie.map((p) => {
        const e = etiquetaSerie(p.clave, granularidad);
        return { key: p.clave, label: e.corta, labelLargo: e.larga, value: p.cantidad };
    });
}

// Variación en texto ("+20,0 %", "−5,0 %").
export function textoVariacion(v) {
    const signo = v > 0 ? "+" : v < 0 ? "−" : "";
    return `${signo}${Math.abs(v).toFixed(1).replace(".", ",")} %`;
}

// Valida lo propio de la comparación (además de validarFiltros).
export function validarComparacion(filtros) {
    if (!filtros.desde || !filtros.hasta) return "Para comparar, indica la fecha desde y la fecha hasta del periodo actual.";
    if (filtros.modo === "PERSONALIZADO") {
        if (!filtros.desdeB || !filtros.hastaB) return "Indica las fechas del periodo de referencia.";
        if (filtros.desdeB > filtros.hastaB) return "En el periodo de referencia, la fecha desde no puede ser posterior a la fecha hasta.";
    }
    return "";
}

// ---- Mejora incremental: indicadores derivados (sin datos nuevos) ----

// Mismo orden de estados en tablas, gráficos y archivos exportados.
export const ESTADOS_ORDEN = ["PENDIENTE", "EN_PROCESO", "OBSERVADO", "ATENDIDO"];

// Conteos de mayor a menor (empates: orden original). Para "más frecuentes".
export const ordenarPorCantidad = (conteos = []) => [...conteos].sort((a, b) => b.cantidad - a.cantidad);

// Carga pendiente (Pendiente + En proceso) por área ACTUAL, de mayor a menor;
// solo áreas con carga. Empates: orden del catálogo de áreas.
export function cargaPendientePorArea(areaEstado = []) {
    const por = new Map();
    for (const c of areaEstado) {
        if (c.columna !== "PENDIENTE" && c.columna !== "EN_PROCESO") continue;
        const a = por.get(c.fila) ?? { area: c.fila, pendiente: 0, enProceso: 0, total: 0 };
        if (c.columna === "PENDIENTE") a.pendiente += c.cantidad;
        else a.enProceso += c.cantidad;
        a.total += c.cantidad;
        por.set(c.fila, a);
    }
    const orden = AREAS.map((a) => a.value);
    const indice = (area) => (orden.includes(area) ? orden.indexOf(area) : orden.length);
    return [...por.values()].filter((a) => a.total > 0).sort((a, b) => b.total - a.total || indice(a.area) - indice(b.area));
}

// Diferencia absoluta con signo ("+2", "−3", "0").
export function textoDiferencia(n) {
    if (n > 0) return `+${n}`;
    if (n < 0) return `−${Math.abs(n)}`;
    return "0";
}

// Sentido del cambio entre dos periodos comparables.
export function tendencia(actual, anterior) {
    if (actual > anterior) return { clave: "sube", texto: "Aumento", simbolo: "▲" };
    if (actual < anterior) return { clave: "baja", texto: "Disminución", simbolo: "▼" };
    return { clave: "igual", texto: "Sin cambio", simbolo: "=" };
}
