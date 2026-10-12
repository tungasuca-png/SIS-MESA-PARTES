import api from "./api";

// Reutiliza la misma instancia de axios de ./api (Authorization + refresh 401).

// Lista real de usuarios (Usuarios Service, vía Gateway). "q" delega en
// búsqueda en el propio backend (GET /api/usuarios?q=...); sin "q" devuelve
// el listado completo paginado. page_size alto porque hoy el catálogo real
// es pequeño y esta pantalla no necesita paginación visible todavía.
// "tipoUsuario" (form:"tipo_usuario,optional" en el Gateway) filtra por rol
// real — lo usa, por ejemplo, el selector de responsable de Psicología para
// listar solo cuentas PSICOLOGO, sin inventar un endpoint nuevo.
export const listUsuarios = async ({ q, tipoUsuario } = {}) => {
    const response = await api.get("/api/usuarios", {
        params: {
            q: q || undefined,
            tipo_usuario: tipoUsuario || undefined,
            page_size: 100,
        },
    });
    return response.data;
};

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

// Reemplaza el ÚNICO rol de un usuario existente (Auth Service, vía
// Gateway). Requiere "usuarios.change_role" en el backend; el frontend no
// duplica esa autorización, solo evita mostrar un botón que fallaría.
export const updateUsuarioRol = async (userId, roleId) => {
    const response = await api.put(`/api/usuarios/${encodeURIComponent(userId)}/rol`, {
        role_id: roleId,
    });
    return response.data;
};

// Crea un usuario real (identidad + credenciales + rol único en Auth
// Service, perfil sincronizado en Usuarios Service) — requiere
// "usuarios.change_role" en el backend, el mismo permiso que ya protege
// updateUsuarioRol (no existe "usuarios.create" en el catálogo real).
export const createUsuario = async ({
    username,
    email,
    password,
    roleId,
    nombres,
    apellidos,
    dni,
    telefono,
    direccion,
}) => {
    const response = await api.post("/api/usuarios", {
        username,
        email,
        password,
        role_id: roleId,
        nombres,
        apellidos,
        dni,
        telefono: telefono || undefined,
        direccion: direccion || undefined,
    });
    return response.data;
};
