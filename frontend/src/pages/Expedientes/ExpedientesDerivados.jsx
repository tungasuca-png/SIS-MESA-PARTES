import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import RoleLayout from "../../components/portal/RoleLayout";
import StatusBadge from "../../components/dashboard/StatusBadge";
import Icon from "../../components/dashboard/Icon";
import { getDerivaciones } from "../../services/derivacionesService";
import { getExpediente } from "../../services/expedientesService";
import { friendlyErrorMessage } from "../../utils/apiErrors";
import { formatDateTime } from "../../utils/format";
import "../../components/dashboard/RecentExpedientes.css";
import "../../components/dashboard/documentosPanel.css";
import "./expedientesList.css";

const PAGE_SIZE = 20;

// "Derivados" no es un estado del expediente (ver constants/bandejas.js):
// es el historial real de derivaciones tipo=DERIVACION (las únicas que
// cambian area_actual — ver DerivacionesPanel.jsx). GET /api/derivaciones
// sin expediente_id devuelve TODO el sistema para cualquier rol interno
// (Derivaciones Service no filtra por área, ver internal/authorization.go
// de ese microservicio), así que cada fila se valida contra
// GET /api/expedientes/:id, que SÍ aplica la visibilidad real por área
// (ver authorization.CanViewAll/AreaDelRol de Expedientes Service): si esa
// consulta falla, es porque esta cuenta no debe ver ese expediente y la
// fila simplemente se omite. No se agrega ningún endpoint nuevo.
async function enriquecerConExpediente(derivaciones) {
    const cache = new Map();
    const filas = [];

    for (const derivacion of derivaciones) {
        if (!cache.has(derivacion.expediente_id)) {
            cache.set(
                derivacion.expediente_id,
                getExpediente(derivacion.expediente_id)
                    .then((data) => data.expediente)
                    .catch(() => null)
            );
        }
        const expediente = await cache.get(derivacion.expediente_id);
        if (expediente) filas.push({ derivacion, expediente });
    }

    return filas;
}

function ExpedientesDerivados() {
    const [filas, setFilas] = useState([]);
    const [page, setPage] = useState(1);
    const [total, setTotal] = useState(0);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");

    useEffect(() => {
        let active = true;

        async function load() {
            setLoading(true);
            setError("");
            try {
                const data = await getDerivaciones({ tipo: "DERIVACION", page, pageSize: PAGE_SIZE });
                const enriquecidas = await enriquecerConExpediente(data.derivaciones || []);
                if (!active) return;
                setFilas(enriquecidas);
                setTotal(data.total || 0);
            } catch (err) {
                if (active) setError(friendlyErrorMessage(err));
            } finally {
                if (active) setLoading(false);
            }
        }

        load();
        return () => {
            active = false;
        };
    }, [page]);

    const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

    return (
        <RoleLayout title="Derivados">
            <section className="dp-panel">
                <div className="dp-list-header">
                    <h2 className="dp-panel-title">Derivados</h2>
                </div>
                <p className="dp-table-empty" style={{ textAlign: "left", padding: 0, marginBottom: 14 }}>
                    Expedientes que cambiaron de área mediante una derivación real. Solo se muestran los
                    expedientes a los que esta cuenta tiene acceso.
                </p>

                {loading && <p className="dp-table-empty">Cargando derivados...</p>}
                {!loading && error && <p className="dp-form-error">{error}</p>}
                {!loading && !error && filas.length === 0 && (
                    <p className="dp-table-empty">No hay expedientes derivados visibles para esta cuenta.</p>
                )}

                {!loading && !error && filas.length > 0 && (
                    <>
                        <div className="dp-table-wrapper">
                            <table className="dp-table">
                                <thead>
                                    <tr>
                                        <th>Expediente</th>
                                        <th>Asunto</th>
                                        <th>Origen → Destino</th>
                                        <th>Estado actual</th>
                                        <th>Fecha</th>
                                        <th>Acciones</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {filas.map(({ derivacion, expediente }) => (
                                        <tr key={derivacion.id}>
                                            <td className="dp-table-code">{expediente.codigo}</td>
                                            <td>{expediente.asunto}</td>
                                            <td>
                                                <span className="dp-documentos-nombre">
                                                    {derivacion.origen} → {derivacion.destino}
                                                </span>
                                                {derivacion.motivo && (
                                                    <p className="dp-documentos-meta">{derivacion.motivo}</p>
                                                )}
                                            </td>
                                            <td>
                                                <StatusBadge status={expediente.estado} />
                                            </td>
                                            <td>{formatDateTime(derivacion.fecha_registro)}</td>
                                            <td>
                                                <Link to={`/expedientes/${expediente.codigo}`} className="dp-table-action">
                                                    <Icon name="eye" size={16} />
                                                    Ver
                                                </Link>
                                            </td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>
                        </div>

                        <div className="dp-pagination">
                            <span>
                                Página {page} de {totalPages}
                            </span>
                            <div className="dp-pagination-buttons">
                                <button
                                    type="button"
                                    className="dp-btn-secondary"
                                    disabled={page <= 1}
                                    onClick={() => setPage((current) => current - 1)}
                                >
                                    Anterior
                                </button>
                                <button
                                    type="button"
                                    className="dp-btn-secondary"
                                    disabled={page >= totalPages}
                                    onClick={() => setPage((current) => current + 1)}
                                >
                                    Siguiente
                                </button>
                            </div>
                        </div>
                    </>
                )}
            </section>
        </RoleLayout>
    );
}

export default ExpedientesDerivados;
