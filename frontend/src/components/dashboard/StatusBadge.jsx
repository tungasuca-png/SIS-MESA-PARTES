import "./StatusBadge.css";

// El backend real de Expedientes devuelve PENDIENTE / EN_PROCESO / ATENDIDO /
// OBSERVADO; Psicología devuelve ABIERTO / EN_SEGUIMIENTO / CERRADO (otro
// dominio, ver constants/psicologia.js) — reutiliza los mismos tokens de
// color en vez de crear variantes/CSS nuevos, con su propia etiqueta (abajo).
const ESTADO_VARIANTS = {
    PENDIENTE: "pendiente",
    EN_PROCESO: "proceso",
    ATENDIDO: "atendido",
    OBSERVADO: "observado",
    ABIERTO: "proceso",
    EN_SEGUIMIENTO: "pendiente",
    CERRADO: "observado",
};

const STATUS_LABELS = {
    pendiente: "Pendiente",
    proceso: "En proceso",
    atendido: "Atendido",
    observado: "Observado",
};

// Etiqueta por el status CRUDO (antes que por el token de color que
// reutiliza): Psicología comparte color con Mesa de Partes pero no su
// significado, así que necesita su propio texto.
const RAW_STATUS_LABELS = {
    ABIERTO: "Abierto",
    EN_SEGUIMIENTO: "En seguimiento",
    CERRADO: "Cerrado",
};

function StatusBadge({ status }) {
    const variant = ESTADO_VARIANTS[status] ?? String(status ?? "").toLowerCase();
    const label = RAW_STATUS_LABELS[status] ?? STATUS_LABELS[variant] ?? status;

    return <span className={`dp-badge dp-badge--${variant}`}>{label}</span>;
}

export default StatusBadge;
