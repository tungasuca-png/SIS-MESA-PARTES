import { useState } from "react";
import { NavLink } from "react-router-dom";
import { useAuth } from "../../hooks/useAuth";
import Icon from "../dashboard/Icon";
import "../dashboard/dashboard-theme.css";
import "./PortalLayout.css";

// Navegación del SOLICITANTE: header institucional + menú horizontal, SIN el
// sidebar azul del administrador (ver PortalLayout vs. DashboardLayout). Las
// rutas ya existen y ya están acotadas al propio usuario por el backend
// (/fut, /expedientes) — acá solo se les da una entrada distinta.
// No hay un ítem "Documentos" aparte: sus documentos se consultan desde el
// detalle de cada expediente (panel de Documentos en ExpedienteDetail).
// "Notificaciones" no tiene ruta: no existe todavía un Notificaciones Service
// ni datos reales que mostrar (ver docs/etapa-12-especificacion-funcional.md);
// se deja marcada como "Próximamente" en vez de inventar datos.
const PORTAL_NAV = [
    { key: "inicio", label: "Inicio", to: "/mesa-de-partes", icon: "dashboard" },
    { key: "registrar", label: "Registrar solicitud", to: "/fut", icon: "plus" },
    { key: "expedientes", label: "Mis expedientes", to: "/expedientes", icon: "folder" },
    { key: "seguimiento", label: "Seguimiento", to: "/mesa-de-partes/seguimiento", icon: "trending" },
    { key: "notificaciones", label: "Notificaciones", to: null, icon: "bell" },
    { key: "perfil", label: "Mi perfil", to: "/mesa-de-partes/perfil", icon: "users" },
];

function PortalLayout({ title, children }) {
    const { user, logout } = useAuth();
    const [menuOpen, setMenuOpen] = useState(false);

    return (
        <div className="dashboard-shell dp-portal-shell">
            <header className="dp-portal-header">
                <div className="dp-portal-brand">
                    <img src="/logo-tungasuca.png" alt="" className="dp-portal-logo" />
                    <div>
                        <p className="dp-portal-title">I.E. TUNGASUCA</p>
                        <p className="dp-portal-subtitle">Mesa de Partes Virtual</p>
                    </div>
                </div>

                <div className="dp-portal-header-actions">
                    <span className="dp-portal-account">
                        <Icon name="users" size={15} />
                        {user?.username}
                    </span>
                    <button type="button" className="dp-portal-logout" onClick={logout}>
                        <Icon name="logout" size={16} />
                        Salir
                    </button>
                    <button
                        type="button"
                        className="dp-portal-menu-btn"
                        onClick={() => setMenuOpen((open) => !open)}
                        aria-label="Abrir menú"
                    >
                        <Icon name={menuOpen ? "close" : "menu"} size={20} />
                    </button>
                </div>
            </header>

            <nav className={`dp-portal-nav ${menuOpen ? "dp-portal-nav--open" : ""}`}>
                {PORTAL_NAV.map((item) =>
                    item.to ? (
                        <NavLink
                            key={item.key}
                            to={item.to}
                            className={({ isActive }) => `dp-portal-nav-item ${isActive ? "dp-portal-nav-item--active" : ""}`}
                            onClick={() => setMenuOpen(false)}
                        >
                            <Icon name={item.icon} size={17} />
                            <span>{item.label}</span>
                        </NavLink>
                    ) : (
                        <span key={item.key} className="dp-portal-nav-item dp-portal-nav-item--soon" aria-disabled="true">
                            <Icon name={item.icon} size={17} />
                            <span>{item.label}</span>
                            <span className="dp-portal-nav-soon">Próximamente</span>
                        </span>
                    )
                )}
            </nav>

            {title && (
                <div className="dp-portal-page-header">
                    <h1>{title}</h1>
                    <p>
                        Bienvenido, <strong>{user?.username}</strong>
                    </p>
                </div>
            )}

            <main className="dp-portal-main">{children}</main>

            <footer className="dp-portal-footer">
                <span>© 2026 I.E. Tungasuca — Mesa de Partes Virtual</span>
                <span>Sesión: {user?.username}</span>
            </footer>
        </div>
    );
}

export default PortalLayout;
