import Icon from "./Icon";
import "./StatCard.css";

function StatCard({ icon, label, value, delta, positive }) {
    return (
        <div className="dp-stat-card">
            <div className="dp-stat-icon">
                <Icon name={icon} size={20} />
            </div>
            <p className="dp-stat-label">{label}</p>
            <p className="dp-stat-value">{value}</p>
            {delta && (
                <p className={`dp-stat-delta ${positive ? "dp-stat-delta--up" : ""}`}>{delta}</p>
            )}
        </div>
    );
}

export default StatCard;
