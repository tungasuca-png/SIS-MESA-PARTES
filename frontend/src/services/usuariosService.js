import api from "./api";

// Reutiliza la misma instancia de axios de ./api (Authorization + refresh 401).

export const getUsuarioBasic = async (id) => {
    const response = await api.get(`/api/usuarios/${encodeURIComponent(id)}/basic`);
    return response.data;
};

// Resolución por lote: una sola petición para todos los solicitantes de una
// lista de expedientes, en vez de una petición por fila.
export const getUsuariosBasic = async (ids) => {
    const response = await api.post("/api/usuarios/basic-batch", { ids });
    return response.data;
};

export const getUsuario = async (id) => {
    const response = await api.get(`/api/usuarios/${encodeURIComponent(id)}`);
    return response.data;
};
