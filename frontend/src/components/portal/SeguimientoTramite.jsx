import { useEffect, useState } from "react";
import { formatDateTime } from "../../utils/format";
import "./SeguimientoTramite.css";

const ICONOS = {
    REGISTRO: (
        <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8zM14 3v5h5M9 13h6M9 17h4" />
    ),
    REVISION: <path d="M11 4a7 7 0 1 0 0 14 7 7 0 0 0 0-14zM20 20l-4-4" />,
    ATENCION: <path d="M12 7v5l3 2M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18z" />,
    ATENDIDO: <path d="M5 21V4M5 4h11l-2 4 2 4H5" />,
};

const ICONO_COMPLETADA = <path d="M5 12.5l4.5 4.5L19 7.5" />;
const ICONO_BLOQUEADA = <path d="M12 8v5M12 16.5v.5M10.3 3.9 2.6 17.5A2 2 0 0 0 4.3 20.5h15.4a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z" />;

const ETIQUETA_ESTADO = {
    completada: "Completada",
    actual: "En curso",
    pendiente: "Pendiente",
    bloqueada: "Observada",
};

function Icono({ children }) {
    return (
        <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="2"
            strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            {children}
        </svg>
    );
}

// Tarjeta "Avance del trámite": porcentaje, barra y etapa actual. La barra
// arranca en 0 y crece al montar (la transición se anula con movimiento
// reducido desde el CSS).
export function ProgresoTramite({ seguimiento }) {
    const [ancho, setAncho] = useState(0);

    useEffect(() => {
        const id = requestAnimationFrame(() => setAncho(seguimiento.porcentaje));
        return () => cancelAnimationFrame(id);
    }, [seguimiento.porcentaje]);

    const tono = seguimiento.observado ? "observado" : seguimiento.finalizado ? "finalizado" : "curso";

    return (
        <section className={`seg-progreso seg-progreso--${tono}`} aria-labelledby="seg-progreso-titulo">
            <div className="seg-progreso-cabecera">
                <div>
                    <h3 id="seg-progreso-titulo" className="seg-progreso-titulo">Avance del trámite</h3>
                    <p className="seg-progreso-etapa">
                        {seguimiento.finalizado ? "Trámite finalizado" : `Etapa actual: ${seguimiento.etapaActual}`}
                    </p>
                </div>
                <p className="seg-progreso-porcentaje">
                    {seguimiento.porcentaje}
                    <span>%</span>
                </p>
            </div>

            <div
                className="seg-progreso-barra"
                role="progressbar"
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={seguimiento.porcentaje}
                aria-valuetext={`${seguimiento.porcentaje}% — ${seguimiento.completadas} de ${seguimiento.total} etapas completadas`}
            >
                <span className="seg-progreso-relleno" style={{ width: `${ancho}%` }} />
            </div>

            <p className="seg-progreso-detalle">
                {seguimiento.completadas} de {seguimiento.total} etapas completadas · {seguimiento.mensaje}
            </p>
        </section>
    );
}

// Línea de tiempo vertical de etapas. Las pendientes no llevan fecha y las
// completadas solo la muestran si el backend la registra.
export function LineaTiempoTramite({ seguimiento, areaActual, accionObservacion }) {
    return (
        <ol className="seg-linea">
            {seguimiento.etapas.map((etapa) => {
                const icono =
                    etapa.estado === "completada"
                        ? ICONO_COMPLETADA
                        : etapa.estado === "bloqueada"
                            ? ICONO_BLOQUEADA
                            : ICONOS[etapa.clave];
                const enCurso = etapa.estado === "actual" || etapa.estado === "bloqueada";

                return (
                    <li key={etapa.clave} className={`seg-etapa seg-etapa--${etapa.estado}`}
                        aria-current={enCurso ? "step" : undefined}>
                        <span className="seg-etapa-icono">
                            <Icono>{icono}</Icono>
                        </span>
                        <div className="seg-etapa-cuerpo">
                            <div className="seg-etapa-cabecera">
                                <p className="seg-etapa-titulo">
                                    {etapa.estado === "bloqueada" ? "Trámite observado" : etapa.titulo}
                                </p>
                                <span className="seg-etapa-estado">{ETIQUETA_ESTADO[etapa.estado]}</span>
                            </div>
                            {etapa.fecha && <p className="seg-etapa-fecha">{formatDateTime(etapa.fecha)}</p>}
                            {etapa.estado === "completada" && !etapa.fecha && (
                                <p className="seg-etapa-fecha seg-etapa-fecha--sin">Fecha no registrada en el sistema</p>
                            )}
                            <p className="seg-etapa-desc">
                                {etapa.estado === "bloqueada"
                                    ? "La revisión se detuvo porque su solicitud requiere correcciones."
                                    : etapa.descripcion}
                            </p>
                            {etapa.estado === "actual" && areaActual && (
                                <p className="seg-etapa-area">Área a cargo: {areaActual}</p>
                            )}
                            {etapa.estado === "bloqueada" && (
                                <div className="seg-etapa-observacion">
                                    <p>
                                        <strong>Motivo: </strong>
                                        {seguimiento.motivoObservacion ||
                                            "No se registró un motivo en el sistema. Consulte con Secretaría."}
                                    </p>
                                    {accionObservacion}
                                </div>
                            )}
                        </div>
                    </li>
                );
            })}
        </ol>
    );
}

// Versión compacta para el listado de seguimiento.
export function ProgresoMini({ seguimiento }) {
    if (!seguimiento) return <span className="dp-usuario-fallback">—</span>;
    const tono = seguimiento.observado ? "observado" : seguimiento.finalizado ? "finalizado" : "curso";
    return (
        <span className={`seg-mini seg-mini--${tono}`}>
            <span className="seg-mini-barra" aria-hidden="true">
                <span style={{ width: `${seguimiento.porcentaje}%` }} />
            </span>
            <span className="seg-mini-valor">{seguimiento.porcentaje}%</span>
        </span>
    );
}
