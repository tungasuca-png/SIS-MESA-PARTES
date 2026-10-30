import { useEffect, useMemo, useState } from "react";
import { listSecciones } from "../../../services/estudiantesService";
import { MOVIMIENTOS_MATRICULA, estadoMatriculaLabel } from "../../../constants/estudiantes";
import { seccionLabel } from "../../../utils/matricula";

// Historial del estudiante: SOLO su trayectoria académica y administrativa,
// a partir de las matrículas y sus movimientos (GET /api/estudiantes/:id/matriculas,
// cargado por la ficha). No muestra auditoría técnica (quién editó qué): esa
// información sigue registrada en el backend, pero no es parte de esta vista.
// La institución de procedencia no se muestra porque el sistema todavía no
// guarda ese dato.

// Etiquetas de las secciones citadas en los movimientos (una consulta por
// periodo del estudiante; solo traduce ids a "nivel · grado · sección · turno").
function useSeccionesDePeriodos(periodoIds) {
    const [mapa, setMapa] = useState({});
    const key = [...new Set(periodoIds.filter(Boolean))].sort().join(",");
    useEffect(() => {
        if (!key) return undefined;
        let active = true;
        Promise.all(key.split(",").map((id) => listSecciones(id).then((d) => d.secciones || []).catch(() => []))).then((listas) => {
            if (!active) return;
            const out = {};
            listas.flat().forEach((s) => {
                out[s.id] = seccionLabel(s);
            });
            setMapa(out);
        });
        return () => {
            active = false;
        };
    }, [key]);
    return mapa;
}

// Detalle de un movimiento con lo que el backend registra (sin el motivo,
// que es texto interno y no se expone).
function detalleMovimiento(mv, m, secciones) {
    const anterior = mv.seccion_anterior_id ? secciones[mv.seccion_anterior_id] : null;
    const nueva = mv.seccion_nueva_id ? secciones[mv.seccion_nueva_id] : null;
    switch (mv.tipo) {
        case "CAMBIO_SECCION":
        case "CAMBIO_TURNO":
            if (anterior && nueva) return `De ${anterior} a ${nueva}`;
            if (nueva) return `Sección asignada: ${nueva}`;
            return "Cambio dentro del periodo";
        case "RETIRO":
            return anterior ? `Se retiró de ${anterior}` : "Retiro del periodo";
        case "TRASLADO_SALIDA":
            return anterior ? `Trasladado(a) a otra institución desde ${anterior}` : "Traslado a otra institución";
        case "REINGRESO":
            return nueva ? `Reingresó a ${nueva}` : "Reingreso al periodo";
        case "CONCLUSION":
            return anterior ? `Concluyó el periodo en ${anterior}` : "Concluyó el periodo";
        default:
            return seccionLabel(m.seccion);
    }
}

// Eventos en orden cronológico (del más antiguo al más reciente). Si una
// matrícula no tiene fecha registrada, se ubica al inicio de su año.
function eventosTrayectoria(matriculas, secciones) {
    const eventos = [];
    matriculas.forEach((m) => {
        const periodo = m.periodo?.nombre ?? (m.periodo?.anio ? `Periodo ${m.periodo.anio}` : "Periodo");
        const respaldo = m.periodo?.anio ? `${m.periodo.anio}-01-01` : "";
        eventos.push({
            key: `m-${m.id}`,
            fecha: m.fecha_matricula || "",
            orden: m.fecha_matricula || respaldo,
            tipo: "MATRICULA",
            titulo: "Matrícula",
            periodo,
            detalle: [seccionLabel(m.seccion), m.seccion?.aula].filter(Boolean).join(" · "),
            nota: `Situación actual de esta matrícula: ${estadoMatriculaLabel(m.estado_matricula)}`,
        });
        (m.movimientos || []).forEach((mv, i) => {
            eventos.push({
                key: `mv-${m.id}-${i}`,
                fecha: mv.fecha,
                orden: mv.fecha || respaldo,
                tipo: mv.tipo,
                titulo: MOVIMIENTOS_MATRICULA[mv.tipo] ?? mv.tipo,
                periodo,
                detalle: detalleMovimiento(mv, m, secciones),
            });
        });
    });
    // sort estable: con la misma fecha, la matrícula va antes que sus movimientos.
    return eventos.sort((a, b) => (a.orden < b.orden ? -1 : a.orden > b.orden ? 1 : 0));
}

function HistorialTab({ matriculasEstado }) {
    const { loading, error, matriculas } = matriculasEstado;
    const secciones = useSeccionesDePeriodos(matriculas.map((m) => m.periodo?.id));
    const eventos = useMemo(() => eventosTrayectoria(matriculas, secciones), [matriculas, secciones]);

    return (
        <section className="dp-panel">
            <h3 className="dp-panel-title">Trayectoria académica</h3>
            <p className="est-form-note">
                Matrículas y movimientos registrados, en orden cronológico. La institución de procedencia aún no se registra en el sistema.
            </p>
            {loading && <p className="dp-table-empty">Cargando trayectoria...</p>}
            {!loading && error && <p className="dp-form-error">{error.message}</p>}
            {!loading && !error && eventos.length === 0 && <p className="dp-table-empty">Sin matrículas ni movimientos registrados.</p>}
            {!loading && !error && eventos.length > 0 && (
                <ol className="est-timeline" aria-label="Trayectoria académica">
                    {eventos.map((e) => (
                        <li key={e.key} className={`est-timeline-item est-timeline-item--${e.tipo.toLowerCase()}`}>
                            <span className="est-timeline-date">{e.fecha || "Sin fecha registrada"}</span>
                            <p className="est-timeline-title">
                                {e.titulo} <span className="est-muted">· {e.periodo}</span>
                            </p>
                            <p className="est-timeline-detail">{e.detalle}</p>
                            {e.nota && <p className="est-timeline-meta">{e.nota}</p>}
                        </li>
                    ))}
                </ol>
            )}
        </section>
    );
}

export default HistorialTab;
