import { BANDEJAS_EXPEDIENTES } from "../../constants/bandejas";

// Estructura del menú lateral (Sidebar azul). Es EXCLUSIVO del personal
// interno: un SOLICITANTE nunca renderiza el Sidebar (ver RoleLayout /
// PortalLayout) — su navegación propia vive en
// components/portal/PortalLayout.jsx (PORTAL_NAV), no acá. Por eso no hay
// entradas con permisos "*.view_own" (esos son siempre exclusivos de
// SOLICITANTE) en esta lista.
//
// Agrupado por dominio, no por módulo independiente: "Mesa de Partes" es
// UN solo dominio (Expedientes) — "Expedientes", "Por revisar", "En
// proceso", "Observados", "Derivados" y "Finalizados" son la misma
// ExpedientesList con distinto filtro (ver constants/bandejas.js), no
// módulos separados. Un grupo sin `label` (Dashboard) no dibuja
// encabezado.
//
// `path: null` = módulo todavía no implementado (se muestra en el menú
// visualmente, pero no navega a ningún lado todavía).
export const NAV_GROUPS = [
    {
        label: null,
        items: [
            { key: "dashboard", label: "Dashboard", path: "/dashboard", icon: "dashboard", permission: "dashboard.view" },
        ],
    },
    {
        label: "Mesa de partes",
        items: [
            { key: "expedientes", label: "Expedientes", path: "/expedientes", icon: "folder", permission: "expedientes.view" },
            ...BANDEJAS_EXPEDIENTES.map((bandeja) => ({
                key: bandeja.id,
                label: bandeja.title,
                path: bandeja.path,
                icon: bandeja.icon,
                permission: bandeja.permission,
            })),
        ],
    },
    {
        label: "Seguimiento",
        items: [
            { key: "seguimiento", label: "Seguimiento", path: null, icon: "trending", permission: "seguimiento.view" },
        ],
    },
    {
        label: "Reportes",
        items: [
            { key: "reportes", label: "Reportes", path: null, icon: "chart", permission: "reportes.view" },
        ],
    },
    {
        label: "Administración",
        items: [
            { key: "usuarios", label: "Usuarios", path: null, icon: "users", permission: "usuarios.view" },
            { key: "roles", label: "Roles y permisos", path: null, icon: "settings", permission: "roles.view" },
            { key: "configuracion", label: "Configuración", path: null, icon: "settings", permission: "configuracion.view" },
        ],
    },
];
