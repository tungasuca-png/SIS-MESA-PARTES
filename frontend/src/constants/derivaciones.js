// Valores REALES del contrato de Derivaciones Service (no son mock: deben
// coincidir exactamente con internal/validation/validation.go del
// microservicio).

export const TIPOS_DERIVACION = [
    { value: "DERIVACION", label: "Derivación (cambia de área)" },
    { value: "NOTIFICACION", label: "Notificación" },
    { value: "ASIGNACION", label: "Asignación" },
    { value: "RECEPCION", label: "Recepción" },
    { value: "APROBACION", label: "Aprobación" },
];
