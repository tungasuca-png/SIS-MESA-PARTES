import api from "./api";

// Reportes (Fase 1 y 2). El backend calcula las estadísticas agregadas,
// valida "reportes.view" y aplica el alcance por rol. Los filtros vacíos no
// se envían. Las exportaciones responden { nombre, mime, contenido_base64 }
// con exactamente los mismos filtros que la consulta.

function parametros({ desde, hasta, estado, tipo, prioridad, area, granularidad, vista } = {}) {
    return {
        desde: desde || undefined,
        hasta: hasta || undefined,
        estado: estado || undefined,
        tipo: tipo || undefined,
        prioridad: prioridad || undefined,
        area: area || undefined,
        granularidad: granularidad || undefined,
        vista: vista || undefined,
    };
}

async function obtener(ruta, params) {
    const response = await api.get(ruta, { params });
    return response.data;
}

// Expedientes (Resumen de la Fase 1; con "granularidad" agrega la serie).
export const getReporteExpedientes = (filtros = {}) => obtener("/api/reportes/expedientes", parametros(filtros));

// vista: RESUMEN (por defecto, archivo de la Fase 1) o EXPEDIENTES.
export const exportarReporteExpedientes = (filtros, formato) =>
    obtener("/api/reportes/expedientes/exportar", { ...parametros(filtros), formato });

// Derivaciones, documentos y actividad: periodo + tipo de trámite (+ granularidad).
const periodo = ({ desde, hasta, tipo, granularidad } = {}) => parametros({ desde, hasta, tipo, granularidad });

export const getReporteDerivaciones = (filtros) => obtener("/api/reportes/derivaciones", periodo(filtros));
export const exportarReporteDerivaciones = (filtros, formato) =>
    obtener("/api/reportes/derivaciones/exportar", { ...periodo(filtros), formato });

export const getReporteDocumentos = (filtros) => obtener("/api/reportes/documentos", periodo(filtros));
export const exportarReporteDocumentos = (filtros, formato) =>
    obtener("/api/reportes/documentos/exportar", { ...periodo(filtros), formato });

export const getReporteActividad = (filtros) => obtener("/api/reportes/actividad", periodo({ ...filtros, granularidad: undefined }));
export const exportarReporteActividad = (filtros, formato) =>
    obtener("/api/reportes/actividad/exportar", { ...periodo({ ...filtros, granularidad: undefined }), formato });

// Comparación: periodo actual (desde/hasta obligatorios) frente al de
// referencia (modo ANTERIOR, ANIO_ANTERIOR o PERSONALIZADO con desde_b/hasta_b).
function comparacion({ modo, desdeB, hastaB, ...resto } = {}) {
    return {
        ...parametros({ ...resto, granularidad: undefined }),
        modo: modo || undefined,
        desde_b: modo === "PERSONALIZADO" ? desdeB || undefined : undefined,
        hasta_b: modo === "PERSONALIZADO" ? hastaB || undefined : undefined,
    };
}

export const getReporteComparacion = (filtros) => obtener("/api/reportes/comparacion", comparacion(filtros));
export const exportarReporteComparacion = (filtros, formato) =>
    obtener("/api/reportes/comparacion/exportar", { ...comparacion(filtros), formato });
