import api from "./api";

// Todas las llamadas pasan por la misma instancia de axios de ./api, que ya
// adjunta el Authorization Bearer y maneja el refresh/401 de forma
// centralizada. Rutas reales del Gateway (ver backend/gateway/gateway.api,
// bloque "Psicología"), ninguna inventada.

export const listExpedientesPsicologicos = async ({ estado, page, pageSize } = {}) => {
    const response = await api.get("/api/psicologia/expedientes", {
        params: {
            estado: estado || undefined,
            page: page || undefined,
            page_size: pageSize || undefined,
        },
    });
    return response.data;
};

export const getExpedientePsicologico = async (id) => {
    const response = await api.get(`/api/psicologia/expedientes/${encodeURIComponent(id)}`);
    return response.data;
};

// Búsqueda por alumno: el expediente psicológico (único) de un estudiante.
// 404 = el estudiante no tiene expediente (o está fuera del alcance del usuario).
export const getExpedientePsicologicoByEstudiante = async (estudianteId) => {
    const response = await api.get(`/api/psicologia/estudiantes/${encodeURIComponent(estudianteId)}/expediente`);
    return response.data;
};

// responsableId vacío = el backend asigna al usuario autenticado como
// responsable (mismo criterio que OpenExpediente en el servicio real).
export const openExpedientePsicologico = async (estudianteId, { responsableId, resumenCaso } = {}) => {
    const response = await api.post(`/api/psicologia/estudiantes/${encodeURIComponent(estudianteId)}/expediente`, {
        responsable_id: responsableId || undefined,
        resumen_caso: resumenCaso || undefined,
    });
    return response.data;
};

// Campos omitidos (undefined) no se modifican — mismo contrato que
// UpdateExpedienteRequest del backend (campos opcionales).
export const updateExpedientePsicologico = async (id, { resumenCaso, responsableId, estado } = {}) => {
    const response = await api.put(`/api/psicologia/expedientes/${encodeURIComponent(id)}`, {
        resumen_caso: resumenCaso,
        responsable_id: responsableId,
        estado,
    });
    return response.data;
};

// Cierre (ABIERTO/EN_SEGUIMIENTO -> CERRADO): el motivo es obligatorio en el
// backend (CloseExpediente), que valida también el estado actual.
export const closeExpedientePsicologico = async (id, motivo) => {
    const response = await api.put(`/api/psicologia/expedientes/${encodeURIComponent(id)}/cerrar`, {
        motivo,
    });
    return response.data;
};

// Reapertura (CERRADO -> ABIERTO o EN_SEGUIMIENTO): estado vacío = el backend
// reabre como ABIERTO (mismo contrato opcional que ReopenExpediente).
export const reopenExpedientePsicologico = async (id, { estado } = {}) => {
    const response = await api.put(`/api/psicologia/expedientes/${encodeURIComponent(id)}/reabrir`, {
        estado: estado || undefined,
    });
    return response.data;
};

// ── Atenciones psicológicas (Hoja de Atención) ─────────────────────────────
// Rutas del Gateway (bloque "Atenciones psicológicas" de gateway.api). El
// backend decide permisos (psicologia.atenciones.*), alcance, versionado,
// anulación y auditoría. Ningún dato clínico se guarda en el navegador: todo
// vive solo en el estado de React mientras la pantalla está abierta.

export const listAtenciones = async (expedienteId, { estado, page, pageSize } = {}) => {
    const response = await api.get(`/api/psicologia/expedientes/${encodeURIComponent(expedienteId)}/atenciones`, {
        params: {
            estado: estado || undefined,
            page: page || undefined,
            page_size: pageSize || undefined,
        },
    });
    return response.data;
};

export const createAtencionDraft = async (expedienteId, contenido) => {
    const response = await api.post(`/api/psicologia/expedientes/${encodeURIComponent(expedienteId)}/atenciones`, {
        contenido,
    });
    return response.data;
};

export const getAtencion = async (id) => {
    const response = await api.get(`/api/psicologia/atenciones/${encodeURIComponent(id)}`);
    return response.data;
};

// revision: la que el usuario tenía abierta; si otro guardado la cambió, el
// backend responde 409 y no pisa nada.
export const updateAtencionDraft = async (id, revision, contenido) => {
    const response = await api.put(`/api/psicologia/atenciones/${encodeURIComponent(id)}/borrador`, {
        revision,
        contenido,
    });
    return response.data;
};

export const registerAtencion = async (id, revision) => {
    const response = await api.put(`/api/psicologia/atenciones/${encodeURIComponent(id)}/registrar`, { revision });
    return response.data;
};

export const discardAtencionDraft = async (id, revision) => {
    const response = await api.put(`/api/psicologia/atenciones/${encodeURIComponent(id)}/descartar`, { revision });
    return response.data;
};

// Corrección = nueva versión inmutable basada en versionBase (la vigente que
// el usuario estaba viendo); las anteriores nunca se modifican.
export const correctAtencion = async (id, { versionBase, motivoCorreccion, contenido }) => {
    const response = await api.post(`/api/psicologia/atenciones/${encodeURIComponent(id)}/versiones`, {
        version_base: versionBase,
        motivo_correccion: motivoCorreccion,
        contenido,
    });
    return response.data;
};

export const listAtencionVersions = async (id) => {
    const response = await api.get(`/api/psicologia/atenciones/${encodeURIComponent(id)}/versiones`);
    return response.data;
};

export const getAtencionVersion = async (id, numeroVersion) => {
    const response = await api.get(
        `/api/psicologia/atenciones/${encodeURIComponent(id)}/versiones/${encodeURIComponent(numeroVersion)}`
    );
    return response.data;
};

export const annulAtencion = async (id, motivo) => {
    const response = await api.put(`/api/psicologia/atenciones/${encodeURIComponent(id)}/anular`, { motivo });
    return response.data;
};

// PDF generado en Psicología Service: {nombre, contenido_base64}. version
// vacía = versión vigente.
export const exportAtencionPdf = async (id, version) => {
    const response = await api.get(`/api/psicologia/atenciones/${encodeURIComponent(id)}/pdf`, {
        params: { version: version || undefined },
    });
    return response.data;
};
