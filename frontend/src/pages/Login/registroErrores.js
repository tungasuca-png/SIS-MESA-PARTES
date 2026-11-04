import { extractBackendMessage } from "../../utils/apiErrors";

// Errores del registro manual. Auth responde "registro_<clave>: <mensaje>"
// en los 4xx; los 5xx llegan con un mensaje genérico del Gateway, así que se
// traducen por el código HTTP según el paso.
const MENSAJE_503 = {
    registro:
        "No pudimos enviar el código o el registro con correo no está disponible en este momento. Inténtalo nuevamente en unos minutos.",
    verificar: "No se pudo completar tu registro en este momento. Vuelve a ingresar el código en unos minutos.",
    reenviar: "No pudimos enviar un código nuevo. Inténtalo nuevamente en unos minutos.",
};

export function errorRegistro(err, paso = "registro") {
    if (!err?.response) {
        return { clave: "sin_conexion", mensaje: "No se pudo conectar con el servidor. Verifica tu conexión e intenta nuevamente." };
    }
    const { status, data } = err.response;
    const desc = extractBackendMessage(data) ?? "";
    const m = desc.match(/^registro_([a-z_]+):\s*(.*)$/s);
    if (m) return { clave: m[1], mensaje: m[2] };
    if (status >= 500) return { clave: "no_disponible", mensaje: MENSAJE_503[paso] ?? MENSAJE_503.registro };
    if (status === 429) {
        return { clave: "limite", mensaje: "Demasiados intentos seguidos. Espera un momento e inténtalo nuevamente." };
    }
    if (status === 400) return { clave: "datos_invalidos", mensaje: "Revisa los datos ingresados." };
    return { clave: "error", mensaje: "No se pudo completar el registro. Inténtalo nuevamente." };
}

// Pasos que ya no pueden continuar con este registro.
export const CLAVES_FINALES = ["invalido", "dni_registrado", "correo_registrado", "datos_invalidos", "reenvios_agotados"];
