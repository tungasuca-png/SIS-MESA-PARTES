// Bandejas = configuraciones de filtro sobre el mismo listado de
// expedientes (ExpedientesList), no módulos ni tablas nuevas. Usan
// únicamente los 4 estados reales de Expedientes Service (ver
// internal/estados/estados.go) — no se inventan estados como
// APROBADO/RECHAZADO/DEVUELTO, que hoy no existen en el backend.
// "Derivados" es la excepción: no es un estado, es el historial real de
// derivaciones tipo=DERIVACION (Derivaciones Service) — vive en su propia
// página (ExpedientesDerivados.jsx) porque muestra otra forma de datos.
export const BANDEJAS_EXPEDIENTES = [
    {
        id: "por-revisar",
        title: "Por revisar",
        estado: "PENDIENTE",
        path: "/expedientes?estado=PENDIENTE",
        icon: "inbox",
        permission: "expedientes.view",
    },
    {
        id: "en-proceso",
        title: "En proceso",
        estado: "EN_PROCESO",
        path: "/expedientes?estado=EN_PROCESO",
        icon: "trending",
        permission: "expedientes.view",
    },
    {
        id: "observados",
        title: "Observados",
        estado: "OBSERVADO",
        path: "/expedientes?estado=OBSERVADO",
        icon: "eye",
        permission: "expedientes.view",
    },
    {
        id: "finalizados",
        title: "Finalizados",
        estado: "ATENDIDO",
        path: "/expedientes?estado=ATENDIDO",
        icon: "check",
        permission: "expedientes.view",
    },
    {
        id: "derivados",
        title: "Derivados",
        path: "/expedientes/derivados",
        icon: "share",
        permission: "derivaciones.view",
    },
];

export function tituloBandeja(estado) {
    const bandeja = BANDEJAS_EXPEDIENTES.find((item) => item.estado === estado);
    return bandeja ? bandeja.title : null;
}
