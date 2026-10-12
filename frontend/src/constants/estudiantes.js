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
