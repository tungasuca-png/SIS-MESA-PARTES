// Seguimiento del trámite para el portal del solicitante: etapas y porcentaje
// derivados SOLO de datos reales del expediente.
//
// Flujo real (internal/estados/estados.go de Expedientes, igual para todos
// los tipos; F2/F4 difieren en áreas, no en estados):
//   PENDIENTE -> EN_PROCESO -> ATENDIDO
//   PENDIENTE u EN_PROCESO (rechazo F4) -> OBSERVADO -> (corregir) -> PENDIENTE
//
// Expedientes no guarda historial de cambios de estado: solo hay fecha de
// registro y fecha de última actualización. Por eso las etapas intermedias
// completadas no llevan fecha (no se inventa) y el historial de movimientos
// se toma de Derivaciones, tal cual fue registrado.
//
// Porcentaje = etapas completadas / total de etapas × 100 (todas con el
// mismo peso: no hay una regla de negocio que pondere distinto). La etapa
// actual no cuenta como completada.

export const ETAPAS_TRAMITE = [
    {
        clave: "REGISTRO",
        titulo: "Solicitud registrada",
        descripcion: "Su solicitud fue recibida en la Mesa de Partes Virtual.",
    },
    {
        clave: "REVISION",
        titulo: "Revisión en Mesa de Partes",
        descripcion: "Secretaría revisa los datos y documentos presentados.",
    },
    {
        clave: "ATENCION",
        titulo: "En atención",
        descripcion: "El área responsable está atendiendo su trámite.",
    },
    {
        clave: "ATENDIDO",
        titulo: "Trámite atendido",
        descripcion: "Su trámite concluyó.",
    },
];

// Cuántas etapas están completas según el estado vigente.
const COMPLETADAS_POR_ESTADO = {
    PENDIENTE: 1,
    EN_PROCESO: 2,
    ATENDIDO: 4,
    // Observado: al corregir vuelve a revisión de Secretaría, así que el
    // avance real es el de una solicitud registrada (revisión bloqueada).
    OBSERVADO: 1,
};

const MENSAJES = {
    PENDIENTE: "Su solicitud está en cola de revisión de Mesa de Partes.",
    EN_PROCESO: "Su trámite fue admitido y está siendo atendido.",
    ATENDIDO: "Su trámite fue atendido. Revise sus notificaciones o acérquese a la institución si corresponde.",
    OBSERVADO: "Su trámite fue observado y está detenido hasta que usted lo corrija.",
};

// Un rechazo F4 se registra como una derivación hacia Secretaría en el mismo
// momento en que el expediente pasa a OBSERVADO; solo en ese caso su motivo
// es el de la observación. Una observación de Secretaría no deja motivo.
const MARGEN_MOTIVO_MS = 2 * 60 * 1000;

function motivoObservacion(expediente, derivaciones) {
    const cambio = Date.parse(expediente.fecha_actualizacion);
    if (Number.isNaN(cambio)) return "";
    const coincidente = derivaciones.find((item) => {
        const fecha = Date.parse(item.fecha_registro);
        return (
            String(item.destino).toUpperCase().startsWith("SECRETAR") &&
            !Number.isNaN(fecha) &&
            Math.abs(fecha - cambio) <= MARGEN_MOTIVO_MS &&
            String(item.motivo ?? "").trim() !== ""
        );
    });
    return coincidente ? coincidente.motivo.trim() : "";
}

// Devuelve null si el estado no pertenece al flujo conocido: en ese caso se
// muestra solo el estado, sin porcentaje.
export function calcularSeguimiento(expediente, derivaciones = []) {
    if (!expediente) return null;
    const completadas = COMPLETADAS_POR_ESTADO[expediente.estado];
    if (completadas === undefined) return null;

    const observado = expediente.estado === "OBSERVADO";
    const total = ETAPAS_TRAMITE.length;

    const etapas = ETAPAS_TRAMITE.map((etapa, indice) => {
        let estado = "pendiente";
        if (indice < completadas) estado = "completada";
        else if (indice === completadas) estado = observado ? "bloqueada" : "actual";

        let fecha = "";
        if (etapa.clave === "REGISTRO") fecha = expediente.fecha_registro;
        // ATENDIDO es final: la última actualización es la del cierre.
        if (etapa.clave === "ATENDIDO" && estado === "completada") fecha = expediente.fecha_actualizacion;

        return { ...etapa, estado, fecha };
    });

    const porcentaje = Math.round((completadas / total) * 100);
    const actual = etapas.find((etapa) => etapa.estado === "actual" || etapa.estado === "bloqueada");

    return {
        porcentaje,
        completadas,
        total,
        etapas,
        etapaActual: actual ? actual.titulo : "Finalizado",
        mensaje: MENSAJES[expediente.estado],
        observado,
        finalizado: expediente.estado === "ATENDIDO",
        motivoObservacion: observado ? motivoObservacion(expediente, derivaciones) : "",
    };
}
