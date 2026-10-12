// Valores REALES del contrato de Psicología Service (deben coincidir con
// internal/validation/validation.go del microservicio: EstadoAbierto,
// EstadoEnSeguimiento, EstadoCerrado). No confundir con los estados de
// Mesa de Partes (constants/expedientes.js): son dominios distintos.
export const ESTADOS = [
    { value: "ABIERTO", label: "Abierto" },
    { value: "EN_SEGUIMIENTO", label: "En seguimiento" },
    { value: "CERRADO", label: "Cerrado" },
];

// Estados a los que UpdateExpediente puede llevar un expediente (el backend
// rechaza "CERRADO" ahí: solo se cierra con CloseExpediente, no implementado
// todavía en esta pantalla — ver reporte de la fase).
export const ESTADOS_EDITABLES = ESTADOS.filter((item) => item.value !== "CERRADO");

export function estadoLabel(value) {
    return ESTADOS.find((item) => item.value === value)?.label ?? value ?? "";
}

// ── Atenciones psicológicas (Hoja de Atención) ─────────────────────────────
// Deben coincidir con el backend: realizada_por con
// internal/validation.IsRealizadaPor; motivos y destinos con los catálogos
// sembrados en migrations/002_create_atenciones.sql. El backend rechaza
// cualquier código fuera de ellos (el frontend no es la autoridad).
export const REALIZADA_POR = [
    { value: "DOCENTE", label: "Docente" },
    { value: "AUXILIAR", label: "Auxiliar" },
    { value: "DIRECTIVO", label: "Directivo" },
    { value: "PSICOLOGO", label: "Psicólogo(a)" },
];

export const MOTIVOS_ATENCION = [
    { value: "ACADEMICO", label: "Académico" },
    { value: "CONDUCTUAL", label: "Conductual" },
    { value: "SOCIOEMOCIONAL", label: "Socioemocional" },
    { value: "INCLUSION", label: "Inclusión" },
    { value: "FAMILIAR", label: "Familiar" },
];

export const DESTINOS_DERIVACION = [
    { value: "TUTORIA", label: "Tutoría" },
    { value: "PSICOLOGIA", label: "Psicología" },
    { value: "SAANEE", label: "SAANEE" },
    { value: "DEMUNA", label: "DEMUNA" },
    { value: "CENTRO_SALUD", label: "Centro de Salud" },
    { value: "OTRO", label: "Otro" },
];

export const ESTADOS_ATENCION = [
    { value: "BORRADOR", label: "Borrador" },
    { value: "REGISTRADA", label: "Registrada" },
    { value: "ANULADA", label: "Anulada" },
];

export function estadoAtencionLabel(value) {
    return ESTADOS_ATENCION.find((item) => item.value === value)?.label ?? value ?? "";
}

// Mismos límites que el backend (validation.MaxTextoAtencion / MaxTextoOtros).
export const MAX_TEXTO_ATENCION = 10000;
export const MAX_TEXTO_OTROS = 500;
export const MAX_MOTIVO_ATENCION = 2000;

// "AT01", "AT12"...: número correlativo visible de la atención dentro del
// expediente (nunca el UUID).
export function numeroAtencionLabel(numero) {
    return numero ? `AT${String(numero).padStart(2, "0")}` : "Borrador";
}

// "2026-09-23" -> "23/09/2026" (fecha de atención; sin conversión de zona
// horaria porque es una fecha civil, no un instante).
export function formatFechaAtencion(value) {
    if (!value) return "—";
    const [y, m, d] = value.split("-");
    return d && m && y ? `${d}/${m}/${y}` : value;
}

// Contenido vacío de la Hoja de Atención: mismas claves que
// AtencionContenidoDTO (gateway.api).
export const CONTENIDO_ATENCION_VACIO = {
    fecha_atencion: "",
    hora_atencion: "",
    realizada_por: [],
    realizada_por_otros: "",
    motivos: [],
    motivo_otros: "",
    descripcion_motivo: "",
    analisis_integral: "",
    estrategias: "",
    derivaciones: [],
    derivacion_otro: "",
    seguimiento_resultados: "",
    acuerdos: "",
    compromiso_estudiante: "",
    compromiso_familia: "",
};

export function contenidoAtencionDesdeApi(contenido) {
    return { ...CONTENIDO_ATENCION_VACIO, ...(contenido || {}) };
}
