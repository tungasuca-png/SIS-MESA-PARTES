import { extractBackendMessage } from "../../utils/apiErrors";

// Mensajes del acceso con Google. Las redirecciones del backend solo traen
// una CLAVE (nunca mensajes ni datos en la URL); aquí se traduce.
export const MENSAJES_GOOGLE = {
    no_configurado: "El acceso con Google no está habilitado en este momento. Ingresa con tu DNI y tu clave.",
    no_disponible: "Google no respondió. Inténtalo nuevamente en unos minutos.",
    cancelado: "Se canceló el acceso con Google.",
    estado_invalido: "La solicitud de acceso no es válida o venció. Vuelve a intentarlo desde el inicio.",
    codigo_rechazado: "Google rechazó la autorización. Vuelve a intentarlo.",
    token_invalido: "No se pudo verificar tu identidad con Google. Vuelve a intentarlo.",
    codigo_invalido: "El enlace de acceso venció o ya fue utilizado. Vuelve a intentarlo.",
    email_no_verificado: "Google no confirmó tu correo. Verifícalo en tu cuenta de Google e inténtalo de nuevo.",
    correo_existente:
        "Ya existe una cuenta con este correo. Inicia sesión con tus credenciales institucionales y vincula Google desde tu perfil.",
    dni_registrado:
        "Este DNI ya tiene una cuenta registrada. Inicia sesión con tus credenciales institucionales. Si deseas utilizar Google, podrás vincularlo desde tu perfil después de iniciar sesión.",
    cuenta_no_disponible: "Esta cuenta no puede iniciar sesión con Google. Comunícate con la institución.",
    cuenta_bloqueada: "La cuenta está bloqueada temporalmente por demasiados intentos fallidos. Inténtalo más tarde.",
    ya_vinculado: "Esta cuenta de Google ya está vinculada a otra cuenta del sistema.",
    cuenta_ya_vinculada: "Tu cuenta ya tiene una cuenta de Google vinculada.",
    clave_incorrecta: "La clave no es correcta.",
    error: "No se pudo completar el acceso con Google. Inténtalo nuevamente.",
};

export const mensajeGoogle = (clave) => MENSAJES_GOOGLE[clave] ?? MENSAJES_GOOGLE.error;

// Errores HTTP del backend: "... desc = google_<clave>: <mensaje>".
export function errorGoogle(err) {
    if (!err?.response) {
        return { clave: "sin_conexion", mensaje: "No se pudo conectar con el servidor. Verifica tu conexión e intenta nuevamente." };
    }
    if (err.response.status === 429) {
        const desc = extractBackendMessage(err.response.data) ?? "";
        if (desc.startsWith("account_locked")) return { clave: "cuenta_bloqueada", mensaje: MENSAJES_GOOGLE.cuenta_bloqueada };
        return { clave: "limite", mensaje: "Demasiados intentos seguidos. Espera un momento e inténtalo nuevamente." };
    }
    const desc = extractBackendMessage(err.response.data) ?? "";
    const m = desc.match(/^google_([a-z_]+):\s*(.*)$/s);
    if (m) return { clave: m[1], mensaje: m[2] || mensajeGoogle(m[1]) };
    return { clave: "error", mensaje: desc || MENSAJES_GOOGLE.error };
}

// Registro pendiente: el código de un solo uso y los datos verificados por
// Google viven solo en esta pestaña (sessionStorage), nunca en la URL.
const CLAVE_PENDIENTE = "mesa_partes_registro_google";

export function guardarRegistroPendiente(datos) {
    try {
        sessionStorage.setItem(CLAVE_PENDIENTE, JSON.stringify(datos));
    } catch {
        // almacenamiento no disponible
    }
}

export function leerRegistroPendiente() {
    try {
        const raw = sessionStorage.getItem(CLAVE_PENDIENTE);
        return raw ? JSON.parse(raw) : null;
    } catch {
        return null;
    }
}

export function borrarRegistroPendiente() {
    try {
        sessionStorage.removeItem(CLAVE_PENDIENTE);
    } catch {
        // almacenamiento no disponible
    }
}
