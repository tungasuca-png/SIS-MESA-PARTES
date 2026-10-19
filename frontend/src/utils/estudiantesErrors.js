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
        "Otro usuario modificó este registro mientras trabajabas. Tus cambios NO se guardaron: recarga los datos y vuelve a intentarlo.",
    // Etapas 5 y 6 del maestro: familia, personas, periodos y matrículas.
    persona_not_found: "La persona no existe o ya no está disponible.",
    relacion_not_found: "La relación familiar no existe para este estudiante.",
    periodo_not_found: "El periodo académico no existe.",
    seccion_not_found: "La sección no existe en el periodo de la matrícula.",
    matricula_not_found: "La matrícula no existe.",
    duplicate_persona: "Ya existe una persona registrada con ese tipo y número de documento.",
    duplicate_familiar: "Esa persona ya está vinculada al estudiante.",
    duplicate_progenitor: "El estudiante ya tiene un padre o una madre vigente con ese parentesco.",
    duplicate_apoderado: "El estudiante ya tiene un apoderado vigente. Quita primero el rol al apoderado actual.",
    duplicate_periodo: "Ya existe un periodo para ese año.",
    duplicate_seccion: "Ya existe esa sección (nivel, grado, sección y turno) en el periodo.",
    duplicate_matricula: "El estudiante ya tiene una matrícula en ese periodo.",
    persona_conflict: "La persona se registró al mismo tiempo desde otra sesión. Vuelve a intentarlo.",
    persona_mismatch: "Ya existe una persona con ese documento y otros nombres. Revisa los datos o búscala para vincularla.",
    invalid_transition: "El estado actual no permite esta operación.",
    periodo_cerrado: "El periodo está cerrado y ya no admite cambios.",
    sin_vacantes: "La sección no tiene vacantes disponibles.",
    estudiante_inactivo: "Solo se puede matricular a estudiantes con registro activo.",
    relacion_no_vigente: "La relación ya estaba cerrada.",
    no_changes: "No hay cambios para guardar.",
    invalid_date: "La fecha es anterior a la matrícula, al retiro o al inicio de la relación registrados.",
    invalid_parentesco: "El parentesco no está registrado en el catálogo.",
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
        const notFound = code && MENSAJES[code] ? code : "student_not_found";
        return { kind: "not_found", code: notFound, message: MENSAJES[notFound] };
    }
    if (status === 409) {
        if (code && MENSAJES[code]) return { kind: "conflict", code, message: MENSAJES[code] };
        return { kind: "conflict", code, message: "La operación entra en conflicto con datos existentes. Recarga y vuelve a intentarlo." };
    }
    if (status === 400) {
        if (code && CON_DETALLE[code]) {
            return { kind: "validation", code, message: detail ? `${CON_DETALLE[code]}: ${detail}.` : `${CON_DETALLE[code]}.` };
        }
        // Reglas de integridad del backend (FailedPrecondition -> 400): mensaje fijo por código.
        if (code && MENSAJES[code]) {
            return { kind: "validation", code, message: MENSAJES[code] };
        }
        return { kind: "validation", code, message: "Los datos ingresados no son válidos." };
    }
    return { kind: "server", code: null, message: "Ocurrió un error en el servidor. Intenta nuevamente más tarde." };
}
