import api from "./api";

// Verificación de identidad de solicitantes (Secretaría). El backend exige el
// permiso solicitantes.identidad.verify en cada llamada.
export const listIdentidades = async ({ estado = "PENDIENTE", limit = 50, offset = 0 } = {}) => {
    const response = await api.get("/api/solicitantes/identidades", { params: { estado, limit, offset } });
    return response.data;
};

// metodo: PRESENCIAL (documento a la vista) o DOCUMENTO (copia adjunta a un trámite).
export const verificarIdentidad = async (usuarioId, metodo) => {
    const response = await api.post(`/api/solicitantes/identidades/${encodeURIComponent(usuarioId)}/verificar`, { metodo });
    return response.data;
};

export const rechazarIdentidad = async (usuarioId, motivo) => {
    const response = await api.post(`/api/solicitantes/identidades/${encodeURIComponent(usuarioId)}/rechazar`, { motivo });
    return response.data;
};

// Reintenta las verificaciones cuyo perfil (Usuarios) quedó pendiente.
export const reconciliarIdentidades = async () => {
    const response = await api.post("/api/solicitantes/identidades/reconciliar", {});
    return response.data;
};
