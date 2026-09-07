// Estructura del menú lateral. `path: null` = módulo todavía no implementado
// (se muestra en el Dashboard visual, pero no navega a ningún lado todavía).
export const NAV_ITEMS = [
    { key: "dashboard", label: "Dashboard", path: "/dashboard", icon: "dashboard", permission: "dashboard.view" },
    { key: "registrar-solicitud", label: "Registrar solicitud", path: null, icon: "plus", permission: "solicitudes.create" },
    { key: "mis-expedientes", label: "Mis expedientes", path: "/expedientes", icon: "folder", permission: "expedientes.view_own" },
    { key: "expedientes", label: "Expedientes", path: "/expedientes", icon: "folder", permission: "expedientes.view" },
    // "Documentos" no tiene una pantalla propia: se gestionan dentro del
    // detalle de cada expediente (DocumentosPanel), por eso no aparece como
    // entrada aparte del menú — mostrarla con "Próximamente" sería engañoso,
    // ya que el módulo sí funciona, solo que desde otro lugar.
    { key: "derivaciones", label: "Derivaciones", path: null, icon: "share", permission: "derivaciones.view" },
    { key: "seguimiento", label: "Seguimiento", path: null, icon: "trending", permission: ["seguimiento.view", "seguimiento.view_own"] },
    { key: "reportes", label: "Reportes", path: null, icon: "chart", permission: "reportes.view" },
    { key: "usuarios", label: "Usuarios", path: null, icon: "users", permission: "usuarios.view" },
    { key: "configuracion", label: "Configuración", path: null, icon: "settings", permission: "configuracion.view" },
];
