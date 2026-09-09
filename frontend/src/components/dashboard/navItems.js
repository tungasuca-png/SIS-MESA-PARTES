// Estructura del menú lateral (Sidebar azul). Es EXCLUSIVO del personal
// interno: un SOLICITANTE nunca renderiza el Sidebar (ver RoleLayout /
// PortalLayout) — su navegación propia vive en
// components/portal/PortalLayout.jsx (PORTAL_NAV), no acá. Por eso no hay
// entradas con permisos "*.view_own" (esos son siempre exclusivos de
// SOLICITANTE) en esta lista.
//
// `path: null` = módulo todavía no implementado (se muestra en el menú
// visualmente, pero no navega a ningún lado todavía).
export const NAV_ITEMS = [
    { key: "dashboard", label: "Dashboard", path: "/dashboard", icon: "dashboard", permission: "dashboard.view" },
    { key: "expedientes", label: "Expedientes", path: "/expedientes", icon: "folder", permission: "expedientes.view" },
    { key: "derivaciones", label: "Derivaciones", path: null, icon: "share", permission: "derivaciones.view" },
    { key: "seguimiento", label: "Seguimiento", path: null, icon: "trending", permission: "seguimiento.view" },
    { key: "reportes", label: "Reportes", path: null, icon: "chart", permission: "reportes.view" },
    { key: "usuarios", label: "Usuarios", path: null, icon: "users", permission: "usuarios.view" },
    { key: "configuracion", label: "Configuración", path: null, icon: "settings", permission: "configuracion.view" },
];
