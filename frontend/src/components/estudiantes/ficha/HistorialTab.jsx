import { useEffect, useMemo, useState } from "react";
import { useUsuariosBasic } from "../../../hooks/useUsuariosBasic";
import { getHistorialEstudiante, listSecciones } from "../../../services/estudiantesService";
import { ACCIONES_HISTORIAL, CAMPOS_HISTORIAL, MOVIMIENTOS_MATRICULA, estadoMatriculaLabel } from "../../../constants/estudiantes";
import { estudianteError } from "../../../utils/estudiantesErrors";
import { seccionLabel } from "../../../utils/matricula";

const PAGE_SIZE = 20;

const fechaHora = (iso) => {
    const d = new Date(iso);
    return Number.isNaN(d.getTime()) ? iso : d.toLocaleString("es-PE", { dateStyle: "medium", timeStyle: "short" });
};

// Etiquetas de las secciones citadas en los movimientos (una consulta por
// periodo, solo de los periodos del estudiante).
function useSeccionesDePeriodos(periodoIds) {
    const [mapa, setMapa] = useState({});
    const key = [...new Set(periodoIds.filter(Boolean))].sort().join(",");
    useEffect(() => {
        if (!key) return undefined;
        let active = true;
        Promise.all(key.split(",").map((id) => listSecciones(id).then((d) => d.secciones || []).catch(() => [])))
            .then((listas) => {
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

// Hechos académicos (fecha del hecho) a partir de matrículas y movimientos.
function hechosAcademicos(matriculas, secciones) {
    const out = [];
    matriculas.forEach((m) => {
        out.push({
            key: `m-${m.id}`,
            fecha: m.fecha_matricula || "",
            titulo: `Matrícula en ${m.periodo?.nombre ?? "periodo"}`,
            detalle: `${seccionLabel(m.seccion)} · estado actual: ${estadoMatriculaLabel(m.estado_matricula)}`,
            tipo: "MATRICULA",
        });
        (m.movimientos || []).forEach((mv, i) => {
            const desde = mv.seccion_anterior_id ? secciones[mv.seccion_anterior_id] : null;
            const hacia = mv.seccion_nueva_id ? secciones[mv.seccion_nueva_id] : null;
            let detalle = m.periodo?.nombre ?? "";
            if (mv.tipo === "CAMBIO_SECCION" || mv.tipo === "CAMBIO_TURNO") {
                detalle += desde ? ` · de ${desde} a ${hacia ?? "otra sección"}` : ` · asignada ${hacia ?? "una sección"}`;
            }
            out.push({ key: `mv-${m.id}-${i}`, fecha: mv.fecha, titulo: MOVIMIENTOS_MATRICULA[mv.tipo] ?? mv.tipo, detalle, tipo: mv.tipo });
        });
    });
    return out.sort((a, b) => (a.fecha < b.fecha ? 1 : a.fecha > b.fecha ? -1 : 0));
}

function HistorialTab({ estudianteId, matriculasEstado }) {
    const [page, setPage] = useState(1);
    const [estado, setEstado] = useState({ key: null, eventos: [], total: 0, error: null });
    const requestKey = `${estudianteId}|${page}`;

    useEffect(() => {
        let active = true;
        getHistorialEstudiante(estudianteId, { page, pageSize: PAGE_SIZE })
            .then((data) => {
                if (!active) return;
                setEstado((current) => ({
                    key: requestKey,
                    eventos: page === 1 ? data.eventos || [] : [...current.eventos, ...(data.eventos || [])],
                    total: data.total || 0,
                    error: null,
                }));
            })
            .catch((err) => active && setEstado((current) => ({ ...current, key: requestKey, error: estudianteError(err) })));
        return () => {
            active = false;
        };
    }, [estudianteId, page, requestKey]);

    const loading = estado.key !== requestKey;
    const matriculas = matriculasEstado.matriculas;
    const secciones = useSeccionesDePeriodos(matriculas.map((m) => m.periodo?.id));
    const hechos = useMemo(() => hechosAcademicos(matriculas, secciones), [matriculas, secciones]);
    const { usuarios, unavailable } = useUsuariosBasic(estado.eventos.map((e) => e.usuario_id));
    const autor = (id) => usuarios.get(id)?.nombre_completo || (unavailable ? "Usuario del sistema" : usuarios.has(id) ? "Usuario no disponible" : "…");

    return (
        <div className="est-historial">
            <section className="dp-panel">
                <h3 className="dp-panel-title">Trayectoria académica</h3>
                <p className="est-form-note">Hechos registrados en las matrículas, por la fecha en que ocurrieron.</p>
                {matriculasEstado.loading && <p className="dp-table-empty">Cargando trayectoria...</p>}
                {!matriculasEstado.loading && matriculasEstado.error && <p className="dp-form-error">{matriculasEstado.error.message}</p>}
                {!matriculasEstado.loading && !matriculasEstado.error && hechos.length === 0 && (
                    <p className="dp-table-empty">Sin matrículas ni movimientos registrados.</p>
                )}
                {hechos.length > 0 && (
                    <ol className="est-timeline" aria-label="Trayectoria académica">
                        {hechos.map((h) => (
                            <li key={h.key} className={`est-timeline-item est-timeline-item--${h.tipo.toLowerCase()}`}>
                                <span className="est-timeline-date">{h.fecha || "Sin fecha"}</span>
                                <p className="est-timeline-title">{h.titulo}</p>
                                <p className="est-timeline-detail">{h.detalle}</p>
                            </li>
                        ))}
                    </ol>
                )}
            </section>

            <section className="dp-panel">
                <h3 className="dp-panel-title">Cambios en el registro</h3>
                <p className="est-form-note">
                    Quién cambió qué y cuándo, según la auditoría. Solo se indican los campos modificados: los valores anteriores no se muestran ni se
                    guardan en el historial.
                </p>
                {loading && page === 1 && <p className="dp-table-empty">Cargando historial...</p>}
                {estado.error && (
                    <p className="dp-form-error">
                        {estado.error.kind === "forbidden" ? "No tienes permiso para ver el historial." : estado.error.message}
                    </p>
                )}
                {!loading && !estado.error && estado.eventos.length === 0 && <p className="dp-table-empty">Sin cambios registrados.</p>}
                {estado.eventos.length > 0 && (
                    <ol className="est-timeline" aria-label="Cambios en el registro">
                        {estado.eventos.map((e) => (
                            <li key={e.id} className="est-timeline-item est-timeline-item--registro">
                                <span className="est-timeline-date">{fechaHora(e.fecha_hora)}</span>
                                <p className="est-timeline-title">{ACCIONES_HISTORIAL[e.accion] ?? e.accion}</p>
                                <p className="est-timeline-detail">
                                    {e.accion === "CHANGE_ESTADO" && e.estado_anterior
                                        ? `De ${e.estado_anterior.toLowerCase()} a ${e.estado_nuevo.toLowerCase()}`
                                        : e.campos.length > 0
                                          ? `Campos: ${e.campos.map((c) => CAMPOS_HISTORIAL[c] ?? c).join(", ")}`
                                          : ""}
                                </p>
                                <p className="est-timeline-meta">Por {autor(e.usuario_id)}</p>
                            </li>
                        ))}
                    </ol>
                )}
                {estado.eventos.length < estado.total && (
                    <button type="button" className="dp-btn-secondary" disabled={loading} onClick={() => setPage((p) => p + 1)}>
                        {loading ? "Cargando..." : "Ver cambios anteriores"}
                    </button>
                )}
            </section>
        </div>
    );
}

export default HistorialTab;
