import { useState } from "react";
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

// Cada página monta su propio DashboardLayout/Sidebar (no hay un layout de
// ruta persistente en App.jsx), así que un simple useState se reinicia con
// cada navegación: se colapsa "Administración" y, al hacer clic en
// cualquier link, vuelve a aparecer abierta. Se guarda en localStorage para
// que la preferencia sobreviva el remount (y la sesión del navegador).
const STORAGE_KEY = "dp_sidebar_closed_groups";

function loadClosedGroups() {
    try {
        const raw = localStorage.getItem(STORAGE_KEY);
        return raw ? new Set(JSON.parse(raw)) : new Set();
    } catch {
        return new Set();
    }
}

function saveClosedGroups(set) {
    try {
        localStorage.setItem(STORAGE_KEY, JSON.stringify([...set]));
    } catch {
        // localStorage no disponible (privado/bloqueado): no es crítico, el
        // desplegable sigue funcionando dentro de la misma página.
    }
}

function Sidebar({ open, onClose }) {
    const { logout } = useAuth();
    const { can } = usePermissions();
    const location = useLocation();

    // Cada categoría (Mesa de partes, Seguimiento, etc.) es desplegable.
    // Empiezan todas abiertas; el usuario colapsa la que no le interese.
    const [closedGroups, setClosedGroups] = useState(loadClosedGroups);
    const toggleGroup = (label) => {
        setClosedGroups((current) => {
            const next = new Set(current);
            if (next.has(label)) next.delete(label);
            else next.add(label);
            saveClosedGroups(next);
            return next;
        });
    };

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
                {groups.map((group) => {
                    const isOpen = !group.label || !closedGroups.has(group.label);
                    return (
                        <div
                            key={group.label ?? "top"}
                            className={`dp-nav-group ${group.label ? "dp-nav-group--nested" : ""}`}
                        >
                            {group.label && (
                                <button
                                    type="button"
                                    className="dp-nav-group-label"
                                    onClick={() => toggleGroup(group.label)}
                                    aria-expanded={isOpen}
                                >
                                    <span>{group.label}</span>
                                    <Icon
                                        name="chevronDown"
                                        size={13}
                                        className={`dp-nav-group-chevron ${isOpen ? "" : "dp-nav-group-chevron--closed"}`}
                                    />
                                </button>
                            )}
                            {isOpen &&
                                group.items.map((item) =>
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
                    );
                })}
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
