import { estadoAtencionLabel } from "../../constants/psicologia";
import "../dashboard/StatusBadge.css";
import "./hojaAtencion.css";

// Estado de una atención psicológica. Reutiliza .dp-badge (mismos tokens de
// color) sin tocar StatusBadge, que comparte Mesa de Partes.
const VARIANTS = {
    BORRADOR: "dp-badge--pendiente",
    REGISTRADA: "dp-badge--atendido",
    ANULADA: "psi-badge--anulada",
};

function AtencionEstadoBadge({ estado }) {
    return <span className={`dp-badge ${VARIANTS[estado] ?? "dp-badge--observado"}`}>{estadoAtencionLabel(estado)}</span>;
}

export default AtencionEstadoBadge;
