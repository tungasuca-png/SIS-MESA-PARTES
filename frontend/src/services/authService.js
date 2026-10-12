import api from "./api";

// La credencial de ingreso es DNI + clave (texto, 8 dígitos). El rol y el
// user_id los decide el backend; el frontend nunca los envía.
export const login = async (dni, password) => {
    const response = await api.post("/api/auth/login", { dni, password });
    return response.data;
};

export const logout = async (refreshToken) => {
    await api.post("/api/auth/logout", { refresh_token: refreshToken });
};

export const validateSession = async () => {
    const response = await api.get("/api/auth/validate");
    return response.data;
};

// getMyPermissions: permisos efectivos REALES del usuario autenticado
// (Auth Service) — es la fuente de autorización del frontend.
export const getMyPermissions = async () => {
    const response = await api.get("/api/auth/me/permissions");
    return response.data.permissions ?? [];
};
