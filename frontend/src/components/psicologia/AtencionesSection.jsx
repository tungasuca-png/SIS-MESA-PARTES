import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import Icon from "../dashboard/Icon";
import AtencionEstadoBadge from "./AtencionEstadoBadge";
import { listAtenciones } from "../../services/psicologiaService";
import { friendlyErrorMessage } from "../../utils/apiErrors";
import { formatFechaAtencion, numeroAtencionLabel } from "../../constants/psicologia";
import "../dashboard/RecentExpedientes.css";
import "./hojaAtencion.css";

const PAGE_SIZE = 20;

// Psicología → Expediente → Atenciones. Sin límite de atenciones por
// expediente (los dos bloques del formato impreso son solo diseño de hoja).
// El listado no trae contenido clínico: solo número, fecha, hora, motivos,
// estado y versión. Los borradores que aparecen son solo los propios (lo
// decide el backend).
function AtencionesSection({ expediente, canCreate }) {
    const [items, setItems] = useState([]);
    const [total, setTotal] = useState(0);
    const [page, setPage] = useState(1);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");

    useEffect(() => {
        let active = true;
        async function load() {
            setLoading(true);
            setError("");
            try {
                const data = await listAtenciones(expediente.id, { page, pageSize: PAGE_SIZE });
                if (!active) return;
                setItems(data.atenciones || []);
                setTotal(data.total || 0);
            } catch (err) {
                if (active) setError(friendlyErrorMessage(err, "El expediente psicológico no fue encontrado."));
            } finally {
                if (active) setLoading(false);
            }
        }
        load();
        return () => {
            active = false;
        };
    }, [expediente.id, page]);

    const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
    const cerrado = expediente.estado === "CERRADO";

    return (
        <section className="dp-panel" style={{ marginTop: 16 }}>
            <div className="dp-list-header">
                <h2 className="dp-panel-title">Atenciones</h2>
                {canCreate && !cerrado && (
                    <Link to={`/psicologia/expedientes/${expediente.id}/atenciones/nueva`} className="dp-btn-primary">
                        Nueva atención
                    </Link>
                )}
            </div>
            {canCreate && cerrado && (
                <p className="dp-form-hint">El expediente está cerrado: reábrelo para registrar nuevas atenciones.</p>
            )}

            {loading && <p className="dp-table-empty">Cargando atenciones...</p>}
            {!loading && error && <p className="dp-form-error">{error}</p>}
            {!loading && !error && items.length === 0 && (
                <p className="dp-table-empty">Este expediente todavía no tiene atenciones registradas.</p>
            )}

            {!loading && !error && items.length > 0 && (
                <>
                    <div className="dp-table-wrapper">
                        <table className="dp-table">
                            <thead>
                                <tr>
                                    <th>Atención</th>
                                    <th>Fecha</th>
                                    <th>Hora</th>
                                    <th>Motivo</th>
                                    <th>Estado</th>
                                    <th>Versión</th>
                                    <th>Acciones</th>
                                </tr>
                            </thead>
                            <tbody>
                                {items.map((item) => (
                                    <tr key={item.id}>
                                        <td className="dp-table-code">{numeroAtencionLabel(item.numero)}</td>
                                        <td>{formatFechaAtencion(item.fecha_atencion)}</td>
                                        <td>{item.hora_atencion || "—"}</td>
                                        <td>{(item.motivos || []).map((m) => m.etiqueta).join(", ") || "—"}</td>
                                        <td>
                                            <AtencionEstadoBadge estado={item.estado} />
                                        </td>
                                        <td>{item.version_vigente > 0 ? `v${item.version_vigente}` : "—"}</td>
                                        <td>
                                            <Link to={`/psicologia/atenciones/${item.id}`} className="dp-table-action">
                                                <Icon name="eye" size={16} />
                                                {item.estado === "BORRADOR" ? "Continuar" : "Ver"}
                                            </Link>
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                    {totalPages > 1 && (
                        <div className="dp-pagination">
                            <span>
                                Página {page} de {totalPages} · {total} atenciones
                            </span>
                            <div className="dp-pagination-buttons">
                                <button type="button" className="dp-btn-secondary" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
                                    Anterior
                                </button>
                                <button
                                    type="button"
                                    className="dp-btn-secondary"
                                    disabled={page >= totalPages}
                                    onClick={() => setPage((p) => p + 1)}
                                >
                                    Siguiente
                                </button>
                            </div>
                        </div>
                    )}
                </>
            )}
        </section>
    );
}

export default AtencionesSection;
