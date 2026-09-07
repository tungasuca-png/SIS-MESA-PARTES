import { NavLink } from "react-router-dom";
import { useAuth } from "../../hooks/useAuth";
import { usePermissions } from "../../hooks/usePermissions";
import { NAV_ITEMS } from "./navItems";
import Icon from "./Icon";
import "./Sidebar.css";

function Sidebar({ open, onClose }) {
    const { logout } = useAuth();
    const { can } = usePermissions();

    const items = NAV_ITEMS.filter((item) => can(item.permission));

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
                {items.map((item) =>
                    item.path ? (
                        <NavLink
                            key={item.key}
                            to={item.path}
                            className={({ isActive }) =>
                                `dp-nav-item ${isActive ? "dp-nav-item--active" : ""}`
                            }
                            onClick={onClose}
                        >
                            <Icon name={item.icon} size={18} />
                            <span>{item.label}</span>
                        </NavLink>
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
