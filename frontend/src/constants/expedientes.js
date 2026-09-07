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

export const DEFAULT_PAGE_SIZE = 10;
