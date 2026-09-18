import api from "./api";

// Reutiliza la misma instancia de axios de ./api (Authorization + refresh 401).
// Los 4 endpoints ya están implementados y probados en Gateway (Paso 21D):
// GET /api/roles, GET /api/permissions, GET /api/roles/:id/permissions,
// PUT /api/roles/:id/permissions.

export const getRoles = async () => {
    const response = await api.get("/api/roles");
    return response.data.roles || [];
};

export const getPermissions = async () => {
    const response = await api.get("/api/permissions");
    return response.data.permissions || [];
};

export const getRolePermissions = async (roleId) => {
    const response = await api.get(`/api/roles/${encodeURIComponent(roleId)}/permissions`);
    return response.data.permissions || [];
};

// permissionIds representa el estado FINAL deseado (reemplazo completo, no
// un "agregar"). Se envía siempre un array, incluso vacío ([]), nunca
// null/undefined — el backend ya soporta y espera ese caso.
export const updateRolePermissions = async (roleId, permissionIds) => {
    const response = await api.put(`/api/roles/${encodeURIComponent(roleId)}/permissions`, {
        permission_ids: permissionIds || [],
    });
    return response.data;
};
