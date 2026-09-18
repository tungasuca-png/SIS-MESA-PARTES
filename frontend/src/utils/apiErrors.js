// El Gateway, cuando un RPC falla, responde con texto plano tipo
// `rpc error: code = PermissionDenied desc = el rol no puede editar expedientes`
// (no JSON). Esta función nunca muestra ese texto crudo al usuario: solo
// extrae el mensaje humano después de "desc =" cuando existe y lo usa como
// mensaje de validación; para todo lo demás usa mensajes fijos en español.
function extractBackendMessage(rawBody) {
    if (typeof rawBody !== "string") return null;
    // http.Error (Go) agrega un salto de línea final; se recorta antes de
    // anclar "$" o la coincidencia falla porque "." no cruza saltos de línea.
    const match = rawBody.trim().match(/desc = (.+)$/);
    return match ? match[1].trim() : null;
}

export function friendlyErrorMessage(error, notFoundMessage = "El expediente no fue encontrado.") {
    if (!error?.response) {
        return "No se pudo conectar con el servidor. Verifica tu conexión e intenta nuevamente.";
    }

    const { status, data } = error.response;
    const backendMessage = extractBackendMessage(data);

    switch (status) {
        case 400:
            return backendMessage || "Los datos ingresados no son válidos.";
        case 401:
            return "Tu sesión no es válida. Inicia sesión nuevamente.";
        case 403:
            return backendMessage || "No tienes permisos para realizar esta acción.";
        case 404:
            return notFoundMessage;
        default:
            return "Ocurrió un error en el servidor. Intenta nuevamente más tarde.";
    }
}
