import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import Icon from "../dashboard/Icon";
import { listMatriculasPeriodo, listPeriodos, listSecciones } from "../../services/estudiantesService";
import { ESTADOS_MATRICULA, ESTADOS_PERIODO, ESTUDIANTES_PAGE_SIZE, estadoMatriculaLabel, tipoDocumentoLabel } from "../../constants/estudiantes";
import { estudianteError } from "../../utils/estudiantesErrors";
import { enmascararDocumento } from "../../utils/privacidad";
import { seccionLabel } from "../../utils/matricula";

// Vista "Por periodo y sección" del directorio: filtra con la MATRÍCULA real
// (GET /api/estudiantes/periodos/:id/matriculas), no con las columnas
// académicas heredadas. Paginación y orden (apellidos) los hace el backend.
function NominaPeriodo() {
    const [periodos, setPeriodos] = useState({ loading: true, items: [], error: null });
    const [periodoId, setPeriodoId] = useState("");
    const [secciones, setSecciones] = useState([]);
    const [seccionId, setSeccionId] = useState("");
    const [estadoMat, setEstadoMat] = useState("");
    const [page, setPage] = useState(1);
    const [resultado, setResultado] = useState({ key: null, items: [], total: 0, error: null });

    useEffect(() => {
        let active = true;
        listPeriodos()
            .then((d) => {
                if (!active) return;
                const items = d.periodos || [];
                setPeriodos({ loading: false, items, error: null });
                const vigente = items.find((p) => p.estado === "EN_CURSO") || items[0];
                if (vigente) setPeriodoId(vigente.id);
            })
            .catch((err) => active && setPeriodos({ loading: false, items: [], error: estudianteError(err) }));
        return () => {
            active = false;
        };
    }, []);

    useEffect(() => {
        if (!periodoId) return undefined;
        let active = true;
        listSecciones(periodoId)
            .then((d) => active && setSecciones(d.secciones || []))
            .catch(() => active && setSecciones([]));
        return () => {
            active = false;
        };
    }, [periodoId]);

    const key = JSON.stringify({ periodoId, seccionId, estadoMat, page });
    useEffect(() => {
        if (!periodoId) return undefined;
        let active = true;
        listMatriculasPeriodo(periodoId, { seccionId, estadoMatricula: estadoMat, page, pageSize: ESTUDIANTES_PAGE_SIZE })
            .then((d) => active && setResultado({ key, items: d.matriculas || [], total: d.total || 0, error: null }))
            .catch((err) => active && setResultado({ key, items: [], total: 0, error: estudianteError(err) }));
        return () => {
            active = false;
        };
    }, [periodoId, seccionId, estadoMat, page, key]);

    const loading = Boolean(periodoId) && resultado.key !== key;
    const totalPages = Math.max(1, Math.ceil(resultado.total / ESTUDIANTES_PAGE_SIZE));

    if (periodos.loading) return <p className="dp-table-empty">Cargando periodos...</p>;
    if (periodos.error) return <p className="dp-form-error">{periodos.error.message}</p>;
    if (periodos.items.length === 0) {
        return (
            <p className="dp-table-empty">
                Todavía no hay periodos académicos. <Link to="/estudiantes/periodos">Regístralos en Periodos y secciones</Link>.
            </p>
        );
    }

    return (
        <>
            <div className="dp-filters est-filters">
                <select
                    aria-label="Periodo académico"
                    value={periodoId}
                    onChange={(e) => {
                        setPeriodoId(e.target.value);
                        setSeccionId("");
                        setPage(1);
                    }}
                >
                    {periodos.items.map((p) => (
                        <option key={p.id} value={p.id}>
                            {p.nombre} ({ESTADOS_PERIODO[p.estado]?.label ?? p.estado})
                        </option>
                    ))}
                </select>
                <select
                    aria-label="Sección"
                    value={seccionId}
                    onChange={(e) => {
                        setSeccionId(e.target.value);
                        setPage(1);
                    }}
                >
                    <option value="">Todas las secciones</option>
                    {secciones.map((s) => (
                        <option key={s.id} value={s.id}>
                            {seccionLabel(s)}
                        </option>
                    ))}
                </select>
                <select
                    aria-label="Estado de matrícula"
                    value={estadoMat}
                    onChange={(e) => {
                        setEstadoMat(e.target.value);
                        setPage(1);
                    }}
                >
                    <option value="">Todos los estados de matrícula</option>
                    {ESTADOS_MATRICULA.map((e) => (
                        <option key={e.value} value={e.value}>
                            {e.label}
                        </option>
                    ))}
                </select>
            </div>

            {loading && <p className="dp-table-empty">Cargando matrículas...</p>}
            {!loading && resultado.error && <p className="dp-form-error">{resultado.error.message}</p>}
            {!loading && !resultado.error && resultado.items.length === 0 && (
                <p className="dp-table-empty">No hay matrículas con esos criterios en el periodo.</p>
            )}
            {!loading && !resultado.error && resultado.items.length > 0 && (
                <>
                    <div className="dp-table-wrapper">
                        <table className="dp-table">
                            <thead>
                                <tr>
                                    <th>Código</th>
                                    <th>Estudiante</th>
                                    <th>Documento</th>
                                    <th>Sección</th>
                                    <th>Matrícula</th>
                                    <th>Acciones</th>
                                </tr>
                            </thead>
                            <tbody>
                                {resultado.items.map(({ matricula, estudiante }) => (
                                    <tr key={matricula.id}>
                                        <td className="dp-table-code">{estudiante?.codigo ?? "—"}</td>
                                        <td>{estudiante?.nombre_completo ?? "Estudiante no disponible"}</td>
                                        <td>
                                            {estudiante && `${tipoDocumentoLabel(estudiante.tipo_documento)} ${enmascararDocumento(estudiante.numero_documento)}`}
                                        </td>
                                        <td>{seccionLabel(matricula.seccion)}</td>
                                        <td>
                                            <span className={`est-chip est-chip--mat-${matricula.estado_matricula.toLowerCase()}`}>
                                                {estadoMatriculaLabel(matricula.estado_matricula)}
                                            </span>
                                        </td>
                                        <td>
                                            <Link to={`/estudiantes/${matricula.estudiante_id}?tab=academico`} className="dp-table-action">
                                                <Icon name="eye" size={16} />
                                                Ver ficha
                                            </Link>
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                    <div className="dp-pagination">
                        <span>
                            Página {page} de {totalPages} · {resultado.total} matrícula{resultado.total === 1 ? "" : "s"}
                        </span>
                        <div className="dp-pagination-buttons">
                            <button type="button" className="dp-btn-secondary" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
                                Anterior
                            </button>
                            <button type="button" className="dp-btn-secondary" disabled={page >= totalPages} onClick={() => setPage((p) => p + 1)}>
                                Siguiente
                            </button>
                        </div>
                    </div>
                </>
            )}
        </>
    );
}

export default NominaPeriodo;
