// Datos MOCK solo para maquetar el Dashboard visualmente.
// Se reemplazarán al conectar los microservicios de Expedientes/Documentos/Reportes.

export const mockStats = [
    {
        key: "recibidos",
        label: "Expedientes recibidos",
        value: 128,
        delta: "+12% este mes",
        positive: true,
        icon: "inbox",
    },
    {
        key: "pendientes",
        label: "Pendientes",
        value: 34,
        delta: "-4% este mes",
        positive: false,
        icon: "clock",
    },
    {
        key: "proceso",
        label: "En proceso",
        value: 52,
        delta: "+8% este mes",
        positive: true,
        icon: "loader",
    },
    {
        key: "atendidos",
        label: "Atendidos",
        value: 42,
        delta: "+15% este mes",
        positive: true,
        icon: "check",
    },
];

export const mockExpedientes = [
    {
        codigo: "EXP-2026-00125",
        asunto: "Solicitud de certificado de estudios",
        remitente: "Juan Pérez",
        estado: "pendiente",
        fecha: "05/09/2026",
    },
    {
        codigo: "EXP-2026-00124",
        asunto: "Traslado de matrícula",
        remitente: "María Gonzales",
        estado: "proceso",
        fecha: "04/09/2026",
    },
    {
        codigo: "EXP-2026-00123",
        asunto: "Constancia de conducta",
        remitente: "Luis Ramírez",
        estado: "atendido",
        fecha: "03/09/2026",
    },
    {
        codigo: "EXP-2026-00122",
        asunto: "Reclamo de nota",
        remitente: "Ana Torres",
        estado: "observado",
        fecha: "02/09/2026",
    },
    {
        codigo: "EXP-2026-00121",
        asunto: "Solicitud de vacante",
        remitente: "Carlos Medina",
        estado: "pendiente",
        fecha: "01/09/2026",
    },
];

export const mockNotifications = [
    {
        id: 1,
        text: "Nuevo expediente derivado a tu área",
        time: "hace 20 min",
    },
    {
        id: 2,
        text: "El expediente EXP-2026-00121 vence mañana",
        time: "hace 2 h",
    },
];
