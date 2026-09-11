import { Link, useLocation } from "react-router-dom";
import { useAuth } from "../../hooks/useAuth";
import { usePermissions } from "../../hooks/usePermissions";
import { NAV_GROUPS } from "./navItems";
import Icon from "./Icon";
import "./Sidebar.css";

// react-router's NavLink solo compara el pathname (ignora el query string),
// así que con varias bandejas apuntando a "/expedientes" con distinto
// "?estado=" todas quedarían resaltadas a la vez. Se compara a mano
// pathname + search contra cada item.path.
function isItemActive(item, location) {
    const [itemPath, itemSearch] = item.path.split("?");
    if (location.pathname !== itemPath) return false;
    if (!itemSearch) return true;
    return new URLSearchParams(location.search).toString() === new URLSearchParams(itemSearch).toString();
}

function Sidebar({ open, onClose }) {
    const { logout } = useAuth();
    const { can } = usePermissions();
    const location = useLocation();

    const groups = NAV_GROUPS.map((group) => ({
        ...group,
        items: group.items.filter((item) => can(item.permission)),
    })).filter((group) => group.items.length > 0);

    return (
        <aside className={`dp-sidebar ${open ? "dp-sidebar--open" : ""}`}>
            <div className="dp-sidebar-brand">
                <img src="/logo-tungasuca.png" alt="" className="dp-sidebar-logo" />
                <div>
                    <p className="dp-sidebar-title">I.E. TUNGASUCA</p>
                    <p className="dp-sidebar-subtitle">Mesa de Partes</p>
                </div>
                <button type="button" className="dp-sidebar-close" onClick={onClose} aria-label="Cerrar menú">
                    <Icon name="close" size={18} />
                </button>
            </div>

            <nav className="dp-nav">
                {groups.map((group) => (
                    <div
                        key={group.label ?? "top"}
                        className={`dp-nav-group ${group.label ? "dp-nav-group--nested" : ""}`}
                    >
                        {group.label && <p className="dp-nav-group-label">{group.label}</p>}
                        {group.items.map((item) =>
                            item.path ? (
                                <Link
                                    key={item.key}
                                    to={item.path}
                                    className={`dp-nav-item ${isItemActive(item, location) ? "dp-nav-item--active" : ""}`}
                                    onClick={onClose}
                                >
                                    <Icon name={item.icon} size={18} />
                                    <span>{item.label}</span>
                                </Link>
                            ) : (
                                <button
                                    key={item.key}
                                    type="button"
                                    className="dp-nav-item dp-nav-item--soon"
                                    aria-disabled="true"
                                >
                                    <Icon name={item.icon} size={18} />
                                    <span>{item.label}</span>
                                    <span className="dp-nav-soon">Próximamente</span>
                                </button>
                            )
                        )}
                    </div>
                ))}
            </nav>

            <div className="dp-sidebar-footer">
                <button type="button" className="dp-nav-item dp-nav-item--logout" onClick={logout}>
                    <Icon name="logout" size={18} />
                    <span>Cerrar sesión</span>
                </button>
            </div>
        </aside>
    );
}

export default Sidebar;
