import api from "./api";

// Motor universal de datos de Estudiantes (importación/exportación
// multiformato). El backend detecta el formato por el CONTENIDO del archivo,
// valida todo y es el único que escribe; aquí solo se envían el archivo y las
// decisiones del usuario.

export const FORMATOS_IMPORTACION = ".xlsx,.xlsm,.ods,.xls,.csv,.tsv,.txt,.json,.jsonl,.ndjson,.xml,.htm,.html";
export const MAX_BYTES_IMPORTACION = 3 * 1024 * 1024;

export const getCamposDatos = async () => (await api.get("/api/estudiantes/datos/campos")).data;

// El archivo viaja como multipart; "solicitud" lleva opciones, mapeo y
// decisiones. El Content-Type se declara: con el "application/json" por
// defecto de la instancia, axios convertiría el FormData en JSON.
const enviar = async (ruta, archivo, solicitud) => {
    const form = new FormData();
    form.append("archivo", archivo);
    form.append("solicitud", JSON.stringify(solicitud || {}));
    const response = await api.post(ruta, form, { headers: { "Content-Type": "multipart/form-data" } });
    return response.data;
};

export const analizarDatos = (archivo, solicitud) => enviar("/api/estudiantes/datos/analizar", archivo, solicitud);
export const confirmarDatos = (archivo, solicitud) => enviar("/api/estudiantes/datos/confirmar", archivo, solicitud);

export const exportarDatos = async (parametros) => (await api.post("/api/estudiantes/datos/exportar", parametros)).data;

export const listPlantillasDatos = async (tipo) =>
    (await api.get("/api/estudiantes/datos/plantillas", { params: { tipo } })).data;

export const guardarPlantillaDatos = async (plantilla) =>
    (await api.post("/api/estudiantes/datos/plantillas", plantilla)).data;

export const eliminarPlantillaDatos = async (id, tipo) =>
    (await api.post(`/api/estudiantes/datos/plantillas/${encodeURIComponent(id)}/eliminar`, { tipo })).data;
