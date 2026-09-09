// Valores REALES del contrato de Expedientes Service (no son mock: deben
// coincidir exactamente con internal/estados/estados.go del microservicio).

export const TIPOS = [
    { value: "SOLICITUD", label: "Solicitud" },
    { value: "TRAMITE", label: "Trámite" },
    { value: "OFICIO", label: "Oficio" },
    { value: "OTRO", label: "Otro" },
];

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
