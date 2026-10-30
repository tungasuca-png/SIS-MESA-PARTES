import { Link } from "react-router-dom";
import Icon from "./Icon";
import "./StatCard.css";

// Indicador compacto. `to` lo convierte en enlace (p. ej. a la bandeja
// filtrada por ese estado); `tone` reutiliza los colores de los estados.
function StatCard({ icon, label, value, hint, to, tone = "primary" }) {
    const content = (
        <>
            <div className="dp-stat-top">
                <p className="dp-stat-label">{label}</p>
                <span className={`dp-stat-icon dp-stat-icon--${tone}`}>
                    <Icon name={icon} size={16} />
                </span>
            </div>
            <p className="dp-stat-value">{value}</p>
            {hint && <p className="dp-stat-hint">{hint}</p>}
        </>
    );

    return to ? (
        <Link to={to} className="dp-stat-card dp-stat-card--link">
            {content}
        </Link>
    ) : (
        <div className="dp-stat-card">{content}</div>
    );
}

export default StatCard;
