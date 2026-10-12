import { extractBackendMessage } from "./apiErrors";

// Errores del Maestro de Estudiantes (FASE E-06). Estudiantes Service
// (FASE E-05) responde "<codigo_estable>: <texto>" dentro del "desc =" del
// Gateway. Aquí se traduce a un mensaje en español; nunca se muestra el
// texto crudo del Gateway, SQL ni errores internos (500 = mensaje genérico).
const MENSAJES = {
    permission_denied: "No tienes permiso para realizar esta acción.",
    student_not_found: "El estudiante no existe o ya no está disponible.",
    duplicate_codigo: "Ya existe un estudiante con ese código.",
    duplicate_document: "Ya existe un estudiante con ese tipo y número de documento.",
    duplicate_codigo_siagie: "Ya existe un estudiante con ese código SIAGIE.",
    version_conflict:
        "Otro usuario modificó este estudiante mientras trabajabas. Tus cambios NO se guardaron: recarga los datos y vuelve a intentarlo.",
};

// Estos códigos traen un texto del backend ya pensado para el usuario (sin
// valores ni detalles internos, ver E-05): se muestra tras un prefijo fijo.
const CON_DETALLE = {
    validation_error: "Datos inválidos",
    invalid_document: "Documento inválido",
    invalid_state: "Estado inválido",
};

export function parseCodigoError(rawBody) {
    const desc = extractBackendMessage(rawBody);
    if (!desc) return { code: null, detail: null };
    const match = desc.match(/^([a-z_]+):\s*(.*)$/);
    return match ? { code: match[1], detail: match[2] } : { code: null, detail: null };
}

// estudianteError -> { kind, code, message }
//   kind: "network" | "auth" | "forbidden" | "not_found" | "conflict" |
//         "validation" | "server"
export function estudianteError(error) {
    if (!error?.response) {
        return {
            kind: "network",
            code: null,
            message: "No se pudo conectar con el servidor. Verifica tu conexión e intenta nuevamente.",
        };
    }
    const { status, data } = error.response;
    const { code, detail } = parseCodigoError(data);

    if (status === 401) {
        // El interceptor de api.js ya intentó renovar la sesión; si llegó
        // aquí, AuthContext redirige al login.
        return { kind: "auth", code, message: "Tu sesión no es válida. Inicia sesión nuevamente." };
    }
    if (status === 403) {
        return { kind: "forbidden", code: code ?? "permission_denied", message: MENSAJES.permission_denied };
    }
    if (status === 404) {
        return { kind: "not_found", code: code ?? "student_not_found", message: MENSAJES.student_not_found };
    }
    if (status === 409) {
        if (code && MENSAJES[code]) return { kind: "conflict", code, message: MENSAJES[code] };
        return { kind: "conflict", code, message: "La operación entra en conflicto con datos existentes. Recarga y vuelve a intentarlo." };
    }
    if (status === 400) {
        if (code && CON_DETALLE[code]) {
            return { kind: "validation", code, message: detail ? `${CON_DETALLE[code]}: ${detail}.` : `${CON_DETALLE[code]}.` };
        }
        return { kind: "validation", code, message: "Los datos ingresados no son válidos." };
    }
    return { kind: "server", code: null, message: "Ocurrió un error en el servidor. Intenta nuevamente más tarde." };
}
