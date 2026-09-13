// Valores REALES del contrato de Expedientes Service (no son mock: deben
// coincidir exactamente con internal/estados/estados.go del microservicio).

// Genéricos: compatibilidad con expedientes existentes y con el registro
// interno de "Nuevo expediente" (ExpedienteFormModal.jsx) para casos que no
// corresponden a ninguno de los 5 trámites reales de abajo.
export const TIPOS_GENERICOS = [
    { value: "SOLICITUD", label: "Solicitud" },
    { value: "TRAMITE", label: "Trámite" },
    { value: "OFICIO", label: "Oficio" },
    { value: "OTRO", label: "Otro" },
];

// Trámites reales de Mesa de Partes (Etapa 2, ver docs/etapa-2-tipos-tramite.md
// y docs/analisis-5-flujos-completo.md — FLUJO 2 y FLUJO 4). Son los únicos
// que se ofrecen en el FUT Digital (ver FutDigital.jsx): un solicitante
// externo no debería ver "Trámite"/"Oficio"/"Otro" como opciones, esas son
// para uso administrativo interno.
export const TIPOS_TRAMITE = [
    { value: "CERTIFICADO", label: "Certificado de Estudios" },
    { value: "CONSTANCIA", label: "Constancia de Estudios" },
    { value: "PERMISO", label: "Permiso" },
    { value: "JUSTIFICACION_FALTA", label: "Justificación de Falta" },
    { value: "JUSTIFICACION_TARDANZA", label: "Justificación de Tardanza" },
];

export const TIPOS = [...TIPOS_TRAMITE, ...TIPOS_GENERICOS];

export function tipoLabel(value) {
    return TIPOS.find((item) => item.value === value)?.label ?? value ?? "";
}

// F2 / F4 (Etapa 4, ver docs/etapa-4-resolucion-cierre-f2-f4.md): mismos
// conjuntos que estados.EsF2/EsF4 del backend — se usan para decidir qué
// acciones de cierre (Rechazar/Corregir/Resolver) mostrar en el detalle.
// El backend vuelve a validar todo igual; esto es solo para no ofrecer un
// botón que el backend va a rechazar seguro.
export const TIPOS_F2 = ["CERTIFICADO", "CONSTANCIA"];
export const TIPOS_F4 = ["PERMISO", "JUSTIFICACION_FALTA", "JUSTIFICACION_TARDANZA"];

export const PRIORIDADES = [
    { value: "NORMAL", label: "Normal" },
    { value: "URGENTE", label: "Urgente" },
];

export const ESTADOS = [
    { value: "PENDIENTE", label: "Pendiente" },
    { value: "EN_PROCESO", label: "En proceso" },
    { value: "ATENDIDO", label: "Atendido" },
    { value: "OBSERVADO", label: "Observado" },
];

// Transiciones válidas, mismo mapa que internal/estados/estados.go
// (transiciones): ATENDIDO y OBSERVADO son finales a propósito. El
// formulario de "Cambiar estado" solo debe ofrecer estas opciones — antes
// mostraba cualquier otro estado y dejaba que el backend rechazara la
// transición inválida, lo que se sentía como que "no se podía cambiar el
// estado" sin explicar por qué.
export const TRANSICIONES_ESTADO = {
    PENDIENTE: ["EN_PROCESO", "OBSERVADO"],
    EN_PROCESO: ["ATENDIDO"],
    ATENDIDO: [],
    OBSERVADO: [],
};

export const DEFAULT_PAGE_SIZE = 10;
