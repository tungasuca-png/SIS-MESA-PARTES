import api from "./api";

// Todas las llamadas pasan por la misma instancia de axios de ./api, que ya
// adjunta el Authorization Bearer y maneja el refresh/401 de forma centralizada.

export const getExpedientes = async ({ estado, prioridad, tipo, page, pageSize } = {}) => {
    const response = await api.get("/api/expedientes", {
        params: {
            estado: estado || undefined,
            prioridad: prioridad || undefined,
            tipo: tipo || undefined,
            page: page || undefined,
            page_size: pageSize || undefined,
        },
    });
    return response.data;
};

export const getExpediente = async (id) => {
    const response = await api.get(`/api/expedientes/${encodeURIComponent(id)}`);
    return response.data;
};

export const createExpediente = async ({ tipo, asunto, descripcion, prioridad }) => {
    const response = await api.post("/api/expedientes", { tipo, asunto, descripcion, prioridad });
    return response.data;
};

export const updateExpediente = async (id, { asunto, descripcion, prioridad }) => {
    const response = await api.put(`/api/expedientes/${encodeURIComponent(id)}`, {
        asunto,
        descripcion,
        prioridad,
    });
    return response.data;
};

export const changeEstado = async (id, nuevoEstado) => {
    const response = await api.patch(`/api/expedientes/${encodeURIComponent(id)}/estado`, {
        nuevo_estado: nuevoEstado,
    });
    return response.data;
};

// Baja lógica: el backend marca el expediente como inactivo (no lo borra de
// la base de datos), así que deja de aparecer en listados/búsquedas.
export const deleteExpediente = async (id) => {
    const response = await api.delete(`/api/expedientes/${encodeURIComponent(id)}`);
    return response.data;
};

// Cambia el área interna responsable del expediente directamente, sin
// pasar por la validación de transición ni registrar historial (ver
// derivarExpediente más abajo, que es lo que usa DerivacionesPanel.jsx
// para una derivación real desde Etapa 3). Queda disponible como
// primitiva genérica para otros usos administrativos.
export const updateArea = async (id, area) => {
    const response = await api.patch(`/api/expedientes/${encodeURIComponent(id)}/area`, { area });
    return response.data;
};

// Derivación real (Etapa 3): una sola llamada que reemplaza el patrón
// anterior (crearDerivacion + updateArea por separado, sin garantía de que
// ambas tuvieran éxito). El backend valida que la transición esté
// confirmada para el tipo de trámite del expediente y, si es válida,
// mueve el área y registra el historial de forma atómica-como-se-puede
// (ver docs/etapa-3-workflow-real.md). Devuelve el expediente actualizado
// y la derivación creada.
export const derivarExpediente = async (id, { areaDestino, motivo, condicion }) => {
    const response = await api.patch(`/api/expedientes/${encodeURIComponent(id)}/derivar`, {
        area_destino: areaDestino,
        motivo,
        condicion,
    });
    return response.data;
};

// Rechazo real (Etapa 4, exclusivo de F4): quien tiene actualmente el
// expediente (Dirección o Subdirección) lo devuelve a Secretaría con
// motivo — el backend mueve estado y área de forma atómica y registra el
// historial (ver docs/etapa-4-resolucion-cierre-f2-f4.md).
export const rechazarExpediente = async (id, motivo) => {
    const response = await api.patch(`/api/expedientes/${encodeURIComponent(id)}/rechazar`, { motivo });
    return response.data;
};

// Corrección del solicitante (Etapa 4): solo sobre su propio expediente
// observado — reingresa a revisión de Secretaría, mismo código.
export const corregirExpediente = async (id, descripcion) => {
    const response = await api.patch(`/api/expedientes/${encodeURIComponent(id)}/corregir`, { descripcion });
    return response.data;
};

// Cierre real de F2 (Etapa 4): Secretaría lo marca ATENDIDO una vez que
// existe el documento final — el backend verifica que ya se haya subido
// antes de cerrar.
export const resolverExpediente = async (id) => {
    const response = await api.patch(`/api/expedientes/${encodeURIComponent(id)}/resolver`, {});
    return response.data;
};
