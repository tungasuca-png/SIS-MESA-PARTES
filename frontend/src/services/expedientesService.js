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
