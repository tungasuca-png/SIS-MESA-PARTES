import api from "./api";

// Todas las llamadas pasan por la misma instancia de axios de ./api, que ya
// adjunta el Authorization Bearer y maneja el refresh/401 de forma centralizada.

export const getExpedientes = async ({ estado, prioridad, tipo, page, pageSize } = {}) => {
    const response = await api.get("/api/expedientes", {
        params: {
            estado: estado || undefined,
            prioridad: prioridad || undefined,
            tipo: tipo || undefined,
            page: page || undefined,
            page_size: pageSize || undefined,
        },
    });
    return response.data;
};

export const getExpediente = async (id) => {
    const response = await api.get(`/api/expedientes/${encodeURIComponent(id)}`);
    return response.data;
};

export const createExpediente = async ({ tipo, asunto, descripcion, prioridad }) => {
    const response = await api.post("/api/expedientes", { tipo, asunto, descripcion, prioridad });
    return response.data;
};

export const updateExpediente = async (id, { asunto, descripcion, prioridad }) => {
    const response = await api.put(`/api/expedientes/${encodeURIComponent(id)}`, {
        asunto,
        descripcion,
        prioridad,
    });
    return response.data;
};

// ---------------------------------------------------------------------------
// Operaciones que mueven área o estado (A + B). Cada una viaja con una
// Idempotency-Key: el mismo pedido con la misma clave nunca se registra dos
// veces (el backend devuelve el mismo movimiento con repetido=true). El
// componente conserva la clave mientras reintenta la MISMA intención y la
// descarta tras el éxito o si cambian los datos.

// UUID v4 (crypto.randomUUID en contextos seguros; si no, getRandomValues).
export const nuevaClaveIdempotencia = () => {
    if (globalThis.crypto?.randomUUID) {
        return globalThis.crypto.randomUUID();
    }
    const b = new Uint8Array(16);
    globalThis.crypto.getRandomValues(b);
    b[6] = (b[6] & 0x0f) | 0x40;
    b[8] = (b[8] & 0x3f) | 0x80;
    const h = Array.from(b, (x) => x.toString(16).padStart(2, "0")).join("");
    return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
};

// Un 503/504 o un error de red dejan el resultado INCIERTO (pudo
// confirmarse). Se reintenta UNA vez con la misma clave: si ya se había
// registrado, el backend devuelve ese mismo movimiento; si no, lo registra.
const esIncierto = (error) => !error?.response || [503, 504].includes(error.response.status);

async function patchIdempotente(url, cuerpo, clave) {
    const config = { headers: { "Idempotency-Key": clave } };
    try {
        return (await api.patch(url, cuerpo, config)).data;
    } catch (error) {
        if (!esIncierto(error)) {
            throw error;
        }
        return (await api.patch(url, cuerpo, config)).data;
    }
}

// Cambio de estado: estadoEsperado es el estado que vio el usuario (si otro
// lo cambió entretanto, el backend responde 409 en vez de aplicarlo encima).
export const changeEstado = async (id, nuevoEstado, estadoEsperado, clave = nuevaClaveIdempotencia()) =>
    patchIdempotente(
        `/api/expedientes/${encodeURIComponent(id)}/estado`,
        { nuevo_estado: nuevoEstado, estado_esperado: estadoEsperado },
        clave
    );

// Baja lógica: el backend marca el expediente como inactivo (no lo borra de
// la base de datos), así que deja de aparecer en listados/búsquedas.
export const deleteExpediente = async (id) => {
    const response = await api.delete(`/api/expedientes/${encodeURIComponent(id)}`);
    return response.data;
};

// Derivación real (Etapa 3): una sola llamada que reemplaza el patrón
// anterior (crearDerivacion + updateArea por separado, sin garantía de que
// ambas tuvieran éxito). El backend valida que la transición esté
// confirmada para el tipo de trámite del expediente y, si es válida,
// mueve el área y registra el historial de forma atómica-como-se-puede
// (ver docs/etapa-3-workflow-real.md). Devuelve el expediente actualizado
// y la derivación creada.
//
// A + B: el backend mueve el área y registra el movimiento en UNA transacción.
// areaOrigenEsperada es el área que vio el usuario (409 si ya cambió).
export const derivarExpediente = async (
    id,
    { areaDestino, areaOrigenEsperada, motivo, condicion },
    clave = nuevaClaveIdempotencia()
) =>
    patchIdempotente(
        `/api/expedientes/${encodeURIComponent(id)}/derivar`,
        { area_destino: areaDestino, area_origen_esperada: areaOrigenEsperada, motivo, condicion },
        clave
    );

// Rechazo real (Etapa 4, exclusivo de F4): quien tiene actualmente el
// expediente (Dirección o Subdirección) lo devuelve a Secretaría con
// motivo — el backend mueve estado y área de forma atómica y registra el
// historial (ver docs/etapa-4-resolucion-cierre-f2-f4.md).
export const rechazarExpediente = async (id, motivo, areaOrigenEsperada, clave = nuevaClaveIdempotencia()) =>
    patchIdempotente(
        `/api/expedientes/${encodeURIComponent(id)}/rechazar`,
        { motivo, area_origen_esperada: areaOrigenEsperada },
        clave
    );

// Corrección del solicitante (Etapa 4): solo sobre su propio expediente
// observado — reingresa a revisión de Secretaría, mismo código.
export const corregirExpediente = async (id, descripcion, clave = nuevaClaveIdempotencia()) =>
    patchIdempotente(`/api/expedientes/${encodeURIComponent(id)}/corregir`, { descripcion }, clave);

// Cierre real de F2 (Etapa 4): Secretaría lo marca ATENDIDO una vez que
// existe el documento final — el backend verifica que ya se haya subido
// antes de cerrar.
export const resolverExpediente = async (id, clave = nuevaClaveIdempotencia()) =>
    patchIdempotente(`/api/expedientes/${encodeURIComponent(id)}/resolver`, {}, clave);
