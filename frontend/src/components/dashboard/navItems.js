// Estructura del menú lateral. `path: null` = módulo todavía no implementado
// (se muestra en el Dashboard visual, pero no navega a ningún lado todavía).
export const NAV_ITEMS = [
    { key: "dashboard", label: "Dashboard", path: "/dashboard", icon: "dashboard", permission: "dashboard.view" },
    { key: "registrar-solicitud", label: "Registrar solicitud", path: "/fut", icon: "plus", permission: "solicitudes.create" },
    { key: "mis-expedientes", label: "Mis expedientes", path: "/expedientes", icon: "folder", permission: "expedientes.view_own" },
    { key: "expedientes", label: "Expedientes", path: "/expedientes", icon: "folder", permission: "expedientes.view" },
    // "Documentos" en el menú es solo para el SOLICITANTE ("documentos.view_own"
    // — nadie más tiene ese permiso): su propio listado de documentos
    // subidos. El personal interno ("documentos.view") sigue viendo/subiendo
    // documentos desde el detalle del expediente (panel de Documentos), no
    // desde su propia página del menú — por eso no comparte este ítem.
    { key: "documentos", label: "Documentos", path: "/documentos", icon: "file", permission: "documentos.view_own" },
    { key: "derivaciones", label: "Derivaciones", path: null, icon: "share", permission: "derivaciones.view" },
    { key: "seguimiento", label: "Seguimiento", path: null, icon: "trending", permission: ["seguimiento.view", "seguimiento.view_own"] },
    { key: "reportes", label: "Reportes", path: null, icon: "chart", permission: "reportes.view" },
    { key: "usuarios", label: "Usuarios", path: null, icon: "users", permission: "usuarios.view" },
    { key: "configuracion", label: "Configuración", path: null, icon: "settings", permission: "configuracion.view" },
];
