import api from "./api";

// La credencial de ingreso es DNI + clave (texto, 8 dígitos). Los
// solicitantes del registro público cuya identidad todavía no verificó la
// institución ingresan con su correo: si el identificador tiene "@" se envía
// como correo. El rol y el user_id los decide el backend; el frontend nunca
// los envía.
export const login = async (identificador, password) => {
    const cuerpo = identificador.includes("@") ? { correo: identificador, password } : { dni: identificador, password };
    const response = await api.post("/api/auth/login", cuerpo);
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

// ---- Acceso con Google (solo solicitantes) ----
// iniciar es una navegación completa del navegador (redirección a Google),
// no una llamada de API.
export const GOOGLE_INICIAR_URL = `${import.meta.env.VITE_API_URL || ""}/api/auth/google/iniciar`;

export const googleCanjear = async (codigo) => {
    const response = await api.post("/api/auth/google/canjear", { codigo });
    return response.data;
};

// El rol nunca se envía: el backend registra siempre SOLICITANTE.
export const googleRegistro = async ({ codigo, dni, nombres, apellidoPaterno, apellidoMaterno, telefono, direccion }) => {
    const response = await api.post("/api/auth/google/registro", {
        codigo,
        dni,
        nombres,
        apellido_paterno: apellidoPaterno,
        apellido_materno: apellidoMaterno,
        telefono,
        direccion,
    });
    return response.data;
};

// Reautenticación con la clave actual; responde la ruta para iniciar el
// flujo (el código de vinculación queda en una cookie HttpOnly).
export const googleVincular = async (clave) => {
    const response = await api.post("/api/auth/google/vincular", { clave });
    return response.data;
};

export const googleEstado = async () => {
    const response = await api.get("/api/auth/google/estado");
    return response.data;
};

// ---- Registro manual con verificación del correo (solo solicitantes) ----
// El rol nunca se envía: el backend registra siempre SOLICITANTE. La cuenta
// se crea recién al verificar el código.
export const registroCorreo = async ({ nombres, apellidoPaterno, apellidoMaterno, dni, correo, clave, aceptaTerminos, versionTerminos }) => {
    const response = await api.post("/api/auth/registro", {
        nombres,
        apellido_paterno: apellidoPaterno,
        apellido_materno: apellidoMaterno,
        dni,
        correo,
        clave,
        acepta_terminos: aceptaTerminos,
        version_terminos: versionTerminos,
    });
    return response.data;
};

export const verificarCorreo = async (registroId, codigo) => {
    const response = await api.post("/api/auth/registro/verificar", { registro_id: registroId, codigo });
    return response.data;
};

export const reenviarCodigo = async (registroId) => {
    const response = await api.post("/api/auth/registro/reenviar", { registro_id: registroId });
    return response.data;
};
