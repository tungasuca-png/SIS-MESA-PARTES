import api from "./api";

// Todas las llamadas pasan por la misma instancia de axios de ./api (Bearer +
// refresh/401 centralizados).

export const getDerivacionesByExpediente = async (expedienteId) => {
    const response = await api.get(`/api/expedientes/${encodeURIComponent(expedienteId)}/derivaciones`);
    return response.data;
};

export const getDerivaciones = async ({ expedienteId, tipo, page, pageSize } = {}) => {
    const response = await api.get("/api/derivaciones", {
        params: {
            expediente_id: expedienteId || undefined,
            tipo: tipo || undefined,
            page: page || undefined,
            page_size: pageSize || undefined,
        },
    });
    return response.data;
};

export const crearDerivacion = async (expedienteId, { tipo, origen, destino, motivo, condicion }) => {
    const response = await api.post(`/api/expedientes/${encodeURIComponent(expedienteId)}/derivaciones`, {
        tipo,
        origen,
        destino,
        motivo,
        condicion,
    });
    return response.data;
};
