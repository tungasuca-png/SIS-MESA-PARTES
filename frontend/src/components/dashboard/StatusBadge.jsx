import "./StatusBadge.css";

// El backend real de Expedientes devuelve PENDIENTE / EN_PROCESO / ATENDIDO /
// OBSERVADO; se mapean a las variantes visuales ya definidas en StatusBadge.css.
const ESTADO_VARIANTS = {
    PENDIENTE: "pendiente",
    EN_PROCESO: "proceso",
    ATENDIDO: "atendido",
    OBSERVADO: "observado",
};

const STATUS_LABELS = {
    pendiente: "Pendiente",
    proceso: "En proceso",
    atendido: "Atendido",
    observado: "Observado",
};

function StatusBadge({ status }) {
    const variant = ESTADO_VARIANTS[status] ?? String(status ?? "").toLowerCase();
    const label = STATUS_LABELS[variant] ?? status;

    return <span className={`dp-badge dp-badge--${variant}`}>{label}</span>;
}

export default StatusBadge;
