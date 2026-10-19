// Maestro de Estudiantes (FASE E-06). Valores tomados del contrato real de
// Estudiantes Service (migraciones 001-004 y FASE E-05); ninguno inventado.

// Estado del REGISTRO del estudiante. RETIRADO/TRASLADADO/CONCLUIDO no son
// estados del maestro: pertenecen a la futura Matrícula.
export const ESTADOS_ESTUDIANTE = [
    { value: "ACTIVO", label: "Activo" },
    { value: "INACTIVO", label: "Inactivo" },
];

// Catálogo cat_tipos_documento (migración 002).
export const TIPOS_DOCUMENTO = [
    { value: "DNI", label: "DNI" },
    { value: "CE", label: "Carné de extranjería" },
    { value: "PASAPORTE", label: "Pasaporte" },
    { value: "SIN_DOCUMENTO", label: "Sin documento" },
];

export const SEXOS = [
    { value: "M", label: "Masculino" },
    { value: "F", label: "Femenino" },
];

export const ESTUDIANTES_PAGE_SIZE = 20;

// Límites del backend (SearchEstudiantes y columnas VARCHAR).
export const BUSQUEDA_MIN = 2;
export const BUSQUEDA_MAX = 100;
export const BUSQUEDA_MAX_PALABRAS = 6;
export const FILTRO_MAX = 30;
export const MAX_CODIGO = 20;
export const MAX_NOMBRES = 100;
export const MAX_APELLIDOS = 100;
export const MAX_APELLIDO = 60;
export const MAX_FOTO_REF = 200;

export function tipoDocumentoLabel(value) {
    return TIPOS_DOCUMENTO.find((item) => item.value === value)?.label ?? value ?? "";
}

// ---- Etapas 5 y 6 del maestro. Valores de los CHECK y catálogos reales
// (migraciones 007–009 y cat_parentescos); ninguno inventado. ----

// Orden del listado (lista blanca del backend).
export const ORDENES_ESTUDIANTES = [
    { value: "", label: "Apellidos (A–Z)" },
    { value: "codigo", label: "Código" },
    { value: "recientes", label: "Registro más reciente" },
];

// cat_parentescos (migración 008). El apoderado es un rol, no un parentesco.
export const PARENTESCOS = [
    { value: "PADRE", label: "Padre" },
    { value: "MADRE", label: "Madre" },
    { value: "TUTOR_LEGAL", label: "Tutor legal" },
    { value: "ABUELO_A", label: "Abuelo(a)" },
    { value: "HERMANO_A", label: "Hermano(a)" },
    { value: "TIO_A", label: "Tío(a)" },
    { value: "OTRO", label: "Otro" },
];

export const parentescoLabel = (value) => PARENTESCOS.find((p) => p.value === value)?.label ?? value ?? "";

// chk_matriculas_estado (estados D-M4).
export const ESTADOS_MATRICULA = [
    { value: "MATRICULADO", label: "Matriculado" },
    { value: "RETIRADO", label: "Retirado" },
    { value: "TRASLADADO", label: "Trasladado" },
    { value: "CONCLUIDO", label: "Concluido" },
];

export const estadoMatriculaLabel = (value) => ESTADOS_MATRICULA.find((e) => e.value === value)?.label ?? value ?? "";

// chk_movimientos_tipo (007 + CONCLUSION de 009).
export const MOVIMIENTOS_MATRICULA = {
    CAMBIO_SECCION: "Cambio de sección",
    CAMBIO_TURNO: "Cambio de turno",
    RETIRO: "Retiro",
    REINGRESO: "Reingreso",
    TRASLADO_SALIDA: "Traslado (salida)",
    CONCLUSION: "Conclusión del periodo",
};

// chk_secciones_nivel.
export const NIVELES = [
    { value: "INICIAL", label: "Inicial" },
    { value: "PRIMARIA", label: "Primaria" },
    { value: "SECUNDARIA", label: "Secundaria" },
];

// chk_periodos_academicos_estado; el ciclo solo avanza.
export const ESTADOS_PERIODO = {
    PLANIFICADO: { label: "Planificado", siguiente: "EN_CURSO", accion: "Iniciar periodo" },
    EN_CURSO: { label: "En curso", siguiente: "CERRADO", accion: "Cerrar periodo" },
    CERRADO: { label: "Cerrado", siguiente: null, accion: null },
};

// Acciones del historial de cambios (auditoría, solo eventos de cambio).
export const ACCIONES_HISTORIAL = {
    CREATE: "Registro del estudiante",
    UPDATE: "Actualización de datos de identidad",
    CHANGE_ESTADO: "Cambio de estado del registro",
    PERSONAL_UPDATE: "Actualización de datos personales",
    FAMILIA_UPDATE: "Cambio en padres y apoderados",
    MATRICULA_UPDATE: "Cambio de matrícula",
};

// Nombres de campos de la auditoría -> etiqueta (nunca valores).
export const CAMPOS_HISTORIAL = {
    codigo: "código",
    codigo_siagie: "código SIAGIE",
    tipo_documento: "tipo de documento",
    numero_documento: "número de documento",
    nombres: "nombres",
    apellido_paterno: "apellido paterno",
    apellido_materno: "apellido materno",
    apellidos: "apellidos",
    fecha_nacimiento: "fecha de nacimiento",
    sexo: "sexo",
    pais_nacimiento: "país de nacimiento",
    ubigeo_nacimiento: "ubigeo de nacimiento",
    lengua_materna: "lengua materna",
    segunda_lengua: "segunda lengua",
    foto_ref: "foto",
    estado: "estado",
    domicilio_direccion: "dirección",
    domicilio_departamento: "departamento",
    domicilio_provincia: "provincia",
    domicilio_distrito: "distrito",
    domicilio_ubigeo: "ubigeo del domicilio",
    correo: "correo",
    telefono: "teléfono",
    familiares: "familiares",
    es_apoderado: "rol de apoderado",
    matricula: "matrícula",
    seccion: "sección",
    estado_matricula: "estado de matrícula",
};

export const MAX_MOTIVO = 200;
