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
// orden (etapa 6): "apellidos" (por defecto), "codigo" o "recientes".
export const searchEstudiantes = async ({ q, page, pageSize, estado, nivel, grado, seccion, turno, orden } = {}) => {
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
            orden: orden || undefined,
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

// ---- Etapas 5 y 6 del maestro (backend/services/estudiantes, gateway.api) ----
// Rutas reales del Gateway; la autorización y las reglas de integridad las
// decide Estudiantes Service. El frontend no las replica.
const enc = encodeURIComponent;

// Resolución EXACTA por un solo criterio: { codigo } | { codigo_siagie } |
// { tipo_documento, numero_documento }. POST: el documento nunca va en la URL.
export const resolveEstudiante = async (criterio) => {
    const response = await api.post("/api/estudiantes/resolver", criterio);
    return response.data;
};

// Historial de CAMBIOS (estudiantes.view): solo nombres de campos, nunca
// valores ni lecturas. { eventos, total }.
export const getHistorialEstudiante = async (id, { page, pageSize } = {}) => {
    const response = await api.get(`/api/estudiantes/${enc(id)}/historial`, {
        params: { page: page || undefined, page_size: pageSize || undefined },
    });
    return response.data;
};

// Familia (estudiantes.familia.update). persona: { tipo_documento,
// numero_documento, nombres, apellido_paterno, apellido_materno }.
export const vincularFamiliar = async (id, { personaId, persona, parentesco, esApoderado, desde }) => {
    const response = await api.post(`/api/estudiantes/${enc(id)}/familiares`, {
        persona_id: personaId || undefined,
        persona: personaId ? undefined : persona,
        parentesco,
        es_apoderado: Boolean(esApoderado),
        desde: desde || undefined,
    });
    return response.data;
};

export const desvincularFamiliar = async (id, relacionId, { hasta } = {}) => {
    const response = await api.patch(`/api/estudiantes/${enc(id)}/familiares/${enc(relacionId)}/desvincular`, {
        hasta: hasta || undefined,
    });
    return response.data;
};

// Solo el rol de apoderado de una relación vigente.
export const actualizarFamiliar = async (id, relacionId, { esApoderado }) => {
    const response = await api.patch(`/api/estudiantes/${enc(id)}/familiares/${enc(relacionId)}`, {
        es_apoderado: Boolean(esApoderado),
    });
    return response.data;
};

// Corrige el parentesco conservando el historial (una transacción en el
// backend). { anterior, familiar }.
export const cambiarParentescoFamiliar = async (id, relacionId, { parentesco, desde }) => {
    const response = await api.post(`/api/estudiantes/${enc(id)}/familiares/${enc(relacionId)}/parentesco`, {
        parentesco,
        desde: desde || undefined,
    });
    return response.data;
};

// Personas (padres, apoderados): búsqueda y lectura con
// estudiantes.familia.view; alta y edición con estudiantes.familia.update.
export const searchPersonas = async ({ q, page, pageSize } = {}) => {
    const response = await api.get("/api/estudiantes/personas", {
        params: { q, page: page || undefined, page_size: pageSize || undefined },
    });
    return response.data;
};

export const createPersona = async (persona) => {
    const response = await api.post("/api/estudiantes/personas", { persona });
    return response.data;
};

export const getPersona = async (personaId, { historial = false } = {}) => {
    const response = await api.get(`/api/estudiantes/personas/${enc(personaId)}`, {
        params: { historial: historial || undefined },
    });
    return response.data;
};

// body = campos cambiados + expected_version.
export const updatePersona = async (personaId, body) => {
    const response = await api.patch(`/api/estudiantes/personas/${enc(personaId)}`, body);
    return response.data;
};

// Periodos y secciones: lectura con estudiantes.view; escritura con
// estudiantes.matricula.update.
export const listPeriodos = async () => {
    const response = await api.get("/api/estudiantes/periodos");
    return response.data;
};

export const createPeriodo = async ({ anio, nombre, fechaInicio, fechaFin }) => {
    const response = await api.post("/api/estudiantes/periodos", {
        anio: Number(anio),
        nombre,
        fecha_inicio: fechaInicio || undefined,
        fecha_fin: fechaFin || undefined,
    });
    return response.data;
};

export const changeEstadoPeriodo = async (periodoId, estado) => {
    const response = await api.patch(`/api/estudiantes/periodos/${enc(periodoId)}/estado`, { estado });
    return response.data;
};

export const listSecciones = async (periodoId) => {
    const response = await api.get(`/api/estudiantes/periodos/${enc(periodoId)}/secciones`);
    return response.data;
};

// vacantes: "" = sin límite registrado (no se envía).
export const createSeccion = async (periodoId, { nivel, grado, seccion, turno, aula, vacantes }) => {
    const response = await api.post(`/api/estudiantes/periodos/${enc(periodoId)}/secciones`, {
        nivel,
        grado,
        seccion,
        turno: turno || undefined,
        aula: aula || undefined,
        vacantes: vacantes === "" || vacantes == null ? undefined : Number(vacantes),
    });
    return response.data;
};

// Nómina del periodo (estudiantes.view): { matriculas: [{ matricula, estudiante }], total }.
export const listMatriculasPeriodo = async (periodoId, { seccionId, estadoMatricula, page, pageSize } = {}) => {
    const response = await api.get(`/api/estudiantes/periodos/${enc(periodoId)}/matriculas`, {
        params: {
            seccion_id: seccionId || undefined,
            estado_matricula: estadoMatricula || undefined,
            page: page || undefined,
            page_size: pageSize || undefined,
        },
    });
    return response.data;
};

// Matrícula (estudiantes.matricula.update). Todas devuelven { matricula }.
export const matricularEstudiante = async (id, { periodoId, seccionId, fechaMatricula }) => {
    const response = await api.post(`/api/estudiantes/${enc(id)}/matriculas`, {
        periodo_id: periodoId,
        seccion_id: seccionId || undefined,
        fecha_matricula: fechaMatricula || undefined,
    });
    return response.data;
};

export const cambiarSeccionMatricula = async (matriculaId, { expectedVersion, seccionId, fecha, motivo }) => {
    const response = await api.patch(`/api/estudiantes/matriculas/${enc(matriculaId)}/seccion`, {
        expected_version: expectedVersion,
        seccion_id: seccionId,
        fecha: fecha || undefined,
        motivo: motivo || undefined,
    });
    return response.data;
};

// tipo: "RETIRO" | "TRASLADO"; motivo obligatorio.
export const retirarMatricula = async (matriculaId, { expectedVersion, tipo, fecha, motivo }) => {
    const response = await api.patch(`/api/estudiantes/matriculas/${enc(matriculaId)}/retiro`, {
        expected_version: expectedVersion,
        tipo,
        fecha: fecha || undefined,
        motivo,
    });
    return response.data;
};

export const reingresarMatricula = async (matriculaId, { expectedVersion, fecha, motivo }) => {
    const response = await api.patch(`/api/estudiantes/matriculas/${enc(matriculaId)}/reingreso`, {
        expected_version: expectedVersion,
        fecha: fecha || undefined,
        motivo: motivo || undefined,
    });
    return response.data;
};

export const concluirMatricula = async (matriculaId, { expectedVersion, fecha }) => {
    const response = await api.patch(`/api/estudiantes/matriculas/${enc(matriculaId)}/conclusion`, {
        expected_version: expectedVersion,
        fecha: fecha || undefined,
    });
    return response.data;
};

// Posibles duplicados para REVISIÓN HUMANA (estudiantes.update): grupos de
// estudiantes con el mismo nombre y fecha de nacimiento, o el mismo nombre
// cuando falta el documento. El backend nunca fusiona nada.
export const listPosiblesDuplicados = async ({ page, pageSize } = {}) => {
    const response = await api.get("/api/estudiantes/duplicados", {
        params: { page: page || undefined, page_size: pageSize || undefined },
    });
    return response.data;
};
