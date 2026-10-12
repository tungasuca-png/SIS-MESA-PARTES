import api from "./api";

// Mismo patrón que psicologiaService.js/usuariosService.js: reutiliza la
// misma instancia de axios (Authorization + refresh 401 centralizados).
// Rutas reales del Gateway (ver backend/gateway/gateway.api, bloque
// "Estudiantes"), ninguna inventada. La autorización real (estudiantes.view,
// .create, .update, .change_estado) la decide Estudiantes Service — el
// frontend no la duplica, solo adapta lo que muestra.

// searchEstudiantes: "q" busca sobre código, código SIAGIE, documento,
// nombres y apellidos (normalizado, ver backend E-04). Filtros opcionales
// (FASE E-06): estado, nivel, grado, seccion, turno. Usado por el selector
// de estudiantes (EstudianteSelector) y por el directorio (/estudiantes).
export const searchEstudiantes = async ({ q, page, pageSize, estado, nivel, grado, seccion, turno } = {}) => {
    const response = await api.get("/api/estudiantes", {
        params: {
            q: q || undefined,
            page: page || undefined,
            page_size: pageSize || undefined,
            estado: estado || undefined,
            nivel: nivel || undefined,
            grado: grado || undefined,
            seccion: seccion || undefined,
            turno: turno || undefined,
        },
    });
    return response.data;
};

export const getEstudiantesBasic = async (ids) => {
    const response = await api.post("/api/estudiantes/basic-batch", { ids });
    return response.data;
};

export const getEstudiante = async (id) => {
    const response = await api.get(`/api/estudiantes/${encodeURIComponent(id)}`);
    return response.data;
};

// ---- FASE E-06: escritura (backend FASE E-05) ----
// Las tres devuelven { estudiante: EstudianteBasico } con la versión nueva.

export const createEstudiante = async (payload) => {
    const response = await api.post("/api/estudiantes", payload);
    return response.data;
};

// body = campos cambiados + expected_version (ver utils/estudianteForm.js).
export const updateEstudiante = async (id, body) => {
    const response = await api.patch(`/api/estudiantes/${encodeURIComponent(id)}`, body);
    return response.data;
};

export const changeEstadoEstudiante = async (id, { estado, expectedVersion }) => {
    const response = await api.patch(`/api/estudiantes/${encodeURIComponent(id)}/estado`, {
        estado,
        expected_version: expectedVersion,
    });
    return response.data;
};

// ---- FASE E-08: datos personales (dominio separado) ----
// GET: exige estudiantes.personal.view (cada lectura se audita en el
// backend). Devuelve { personal: { id, version, fecha_nacimiento, sexo,
// pais_nacimiento, ubigeo_nacimiento, lengua_materna, segunda_lengua } }.
export const getEstudiantePersonal = async (id) => {
    const response = await api.get(`/api/estudiantes/${encodeURIComponent(id)}/personal`);
    return response.data;
};

// PATCH: exige estudiantes.personal.update. body = campos personales
// cambiados + expected_version. Responde { estudiante: EstudianteBasico }
// (sin datos personales) con la versión nueva.
export const updateEstudiantePersonal = async (id, body) => {
    const response = await api.patch(`/api/estudiantes/${encodeURIComponent(id)}/personal`, body);
    return response.data;
};

// ---- Modelo maestro ----
// Historial académico (estudiantes.view): una matrícula por periodo con sus
// movimientos. Devuelve { matriculas: [...] }.
export const getMatriculasEstudiante = async (id) => {
    const response = await api.get(`/api/estudiantes/${encodeURIComponent(id)}/matriculas`);
    return response.data;
};

// Familiares (estudiantes.familia.view; cada consulta se audita en el
// backend). historial=true incluye relaciones cerradas.
export const getFamiliaresEstudiante = async (id, { historial = false } = {}) => {
    const response = await api.get(`/api/estudiantes/${encodeURIComponent(id)}/familiares`, {
        params: { historial: historial || undefined },
    });
    return response.data;
};
