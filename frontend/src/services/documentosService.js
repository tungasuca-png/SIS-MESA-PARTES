import api from "./api";

// Todas las llamadas pasan por la misma instancia de axios de ./api (Bearer +
// refresh/401 centralizados). El archivo viaja como base64 dentro del JSON,
// tal como lo espera el Gateway (ver backend/gateway/gateway.api).

const CHUNK_SIZE = 0x8000; // 32 KB, para no reventar el limite de argumentos
// de String.fromCharCode.apply con archivos grandes.

export const fileToBase64 = async (file) => {
    const bytes = new Uint8Array(await file.arrayBuffer());
    let binary = "";
    for (let i = 0; i < bytes.length; i += CHUNK_SIZE) {
        binary += String.fromCharCode.apply(null, bytes.subarray(i, i + CHUNK_SIZE));
    }
    return btoa(binary);
};

export const base64ToBlob = (base64, mimeType) => {
    const binary = atob(base64);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) {
        bytes[i] = binary.charCodeAt(i);
    }
    return new Blob([bytes], { type: mimeType });
};

export const getDocumentosByExpediente = async (expedienteId) => {
    const response = await api.get(`/api/expedientes/${encodeURIComponent(expedienteId)}/documentos`);
    return response.data;
};

export const getDocumentos = async ({ expedienteId, tipoDocumento, page, pageSize } = {}) => {
    const response = await api.get("/api/documentos", {
        params: {
            expediente_id: expedienteId || undefined,
            tipo_documento: tipoDocumento || undefined,
            page: page || undefined,
            page_size: pageSize || undefined,
        },
    });
    return response.data;
};

export const uploadDocumento = async ({ expedienteId, nombre, tipoDocumento, extension, contenidoBase64 }) => {
    const response = await api.post("/api/documentos", {
        expediente_id: expedienteId,
        nombre,
        tipo_documento: tipoDocumento,
        extension,
        contenido: contenidoBase64,
    });
    return response.data;
};

export const downloadDocumento = async (id) => {
    const response = await api.get(`/api/documentos/${encodeURIComponent(id)}/contenido`);
    return response.data;
};

export const deleteDocumento = async (id) => {
    const response = await api.delete(`/api/documentos/${encodeURIComponent(id)}`);
    return response.data;
};
