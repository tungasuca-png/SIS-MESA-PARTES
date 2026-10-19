import { BANDEJAS_EXPEDIENTES } from "../../constants/bandejas";

// Estructura del menú lateral (Sidebar azul). Es EXCLUSIVO del personal
// interno: un SOLICITANTE nunca renderiza el Sidebar (ver RoleLayout /
// PortalLayout) — su navegación propia vive en
// components/portal/PortalLayout.jsx (PORTAL_NAV), no acá. Por eso no hay
// entradas con permisos "*.view_own" (esos son siempre exclusivos de
// SOLICITANTE) en esta lista.
//
// Organización oficial de módulos (una sola fuente para el Sidebar y para
// las tarjetas del Dashboard):
//
//   Dashboard
//   Mesa de Partes -> Expedientes (Por revisar, En proceso, Observados,
//                     Finalizados), Derivados, Seguimiento
//   Psicología -> Expedientes (Psicología Service, expedientes_psicologicos)
//   Estudiantes -> Directorio, Periodos y secciones (Estudiantes Service)
//   Reportes
//   Administración -> Usuarios, Roles y Permisos, Configuración
//
// "Por revisar", "En proceso", "Observados" y "Finalizados" son la misma
// ExpedientesList con distinto filtro (ver constants/bandejas.js): por eso
// cuelgan de "Expedientes" como `children`. "Derivados" es una página propia.
// Un grupo sin `label` (Dashboard) no dibuja encabezado. Solo se muestra lo
// que el usuario puede ver según sus permisos reales (usePermissions).
//
// `path: null` = módulo todavía no implementado (se muestra en el menú y en
// el Dashboard, pero no navega a ningún lado todavía). No se inventan rutas.
const bandejaItem = (bandeja) => ({
    key: bandeja.id,
    label: bandeja.title,
    path: bandeja.path,
    icon: bandeja.icon,
    permission: bandeja.permission,
});

const bandejasPorEstado = BANDEJAS_EXPEDIENTES.filter((bandeja) => bandeja.estado);
const bandejaDerivados = BANDEJAS_EXPEDIENTES.find((bandeja) => bandeja.id === "derivados");

export const NAV_GROUPS = [
    {
        id: "inicio",
        label: null,
        items: [
            { key: "dashboard", label: "Dashboard", path: "/dashboard", icon: "dashboard", permission: "dashboard.view" },
        ],
    },
    {
        id: "mesa-de-partes",
        label: "Mesa de Partes",
        items: [
            {
                key: "expedientes",
                label: "Expedientes",
                description: "Consulta y gestión de los expedientes registrados.",
                path: "/expedientes",
                icon: "folder",
                permission: "expedientes.view",
                children: bandejasPorEstado.map(bandejaItem),
            },
            {
                ...bandejaItem(bandejaDerivados),
                description: "Historial de derivaciones entre áreas.",
            },
            {
                key: "seguimiento",
                label: "Seguimiento",
                description: "Seguimiento del estado de los trámites.",
                path: null,
                icon: "trending",
                permission: "seguimiento.view",
            },
        ],
    },
    {
        id: "psicologia",
        label: "Psicología",
        items: [
            {
                key: "psicologia-expedientes",
                label: "Expedientes",
                description: "Expedientes psicológicos de los estudiantes.",
                path: "/psicologia/expedientes",
                icon: "file",
                permission: ["psicologia.expedientes.view", "psicologia.expedientes.view_own"],
            },
        ],
    },
    {
        // FASE E-06: maestro institucional de estudiantes.
        id: "estudiantes",
        label: "Estudiantes",
        items: [
            {
                key: "estudiantes-directorio",
                label: "Directorio",
                description: "Maestro institucional de estudiantes.",
                path: "/estudiantes",
                icon: "users",
                permission: "estudiantes.view",
            },
            {
                key: "estudiantes-periodos",
                label: "Periodos y secciones",
                description: "Periodos académicos y secciones para la matrícula.",
                path: "/estudiantes/periodos",
                icon: "clock",
                permission: "estudiantes.view",
            },
        ],
    },
    {
        id: "reportes",
        label: "Reportes",
        items: [
            { key: "reportes", label: "Reportes", description: "Reportes del sistema.", path: null, icon: "chart", permission: "reportes.view" },
        ],
    },
    {
        id: "administracion",
        label: "Administración",
        items: [
            { key: "usuarios", label: "Usuarios", description: "Usuarios del sistema y sus roles.", path: "/configuracion/usuarios", icon: "users", permission: "usuarios.view" },
            { key: "roles", label: "Roles y Permisos", description: "Permisos asignados a cada rol.", path: "/configuracion/roles-permisos", icon: "settings", permission: "roles.view" },
            { key: "configuracion", label: "Configuración", description: "Configuración general del sistema.", path: null, icon: "settings", permission: "configuracion.view" },
        ],
    },
];
