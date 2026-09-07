import { useEffect, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import DashboardLayout from "../../components/dashboard/DashboardLayout";
import ExpedienteFormModal from "../../components/dashboard/ExpedienteFormModal";
import StatusBadge from "../../components/dashboard/StatusBadge";
import SolicitanteNombre from "../../components/dashboard/SolicitanteNombre";
import Icon from "../../components/dashboard/Icon";
import { usePermissions } from "../../hooks/usePermissions";
import { useUsuariosBasic } from "../../hooks/useUsuariosBasic";
import { deleteExpediente, getExpedientes } from "../../services/expedientesService";
import { friendlyErrorMessage } from "../../utils/apiErrors";
import { formatDate } from "../../utils/format";
import { DEFAULT_PAGE_SIZE, ESTADOS, PRIORIDADES, TIPOS } from "../../constants/expedientes";
import "../../components/dashboard/RecentExpedientes.css";
import "../../components/dashboard/forms.css";
import "../../components/dashboard/documentosPanel.css";
import "./expedientesList.css";

function ExpedientesList() {
    const { can } = usePermissions();
    const location = useLocation();
    const navigate = useNavigate();

    const [items, setItems] = useState([]);
    const [total, setTotal] = useState(0);
    const [page, setPage] = useState(1);
    const [estado, setEstado] = useState("");
    const [prioridad, setPrioridad] = useState("");
    const [tipo, setTipo] = useState("");
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");
    const [showCreate, setShowCreate] = useState(Boolean(location.state?.openCreate));
    const [refreshKey, setRefreshKey] = useState(0);

    const [confirmingCodigo, setConfirmingCodigo] = useState(null);
    const [deletingCodigo, setDeletingCodigo] = useState(null);
    const [deleteError, setDeleteError] = useState("");

    const canCreate = can(["expedientes.create", "solicitudes.create"]);
    const canViewAll = can("expedientes.view");
    const canDelete = can("expedientes.delete");

    useEffect(() => {
        let active = true;

        async function load() {
            setLoading(true);
            setError("");
            try {
                const data = await getExpedientes({ estado, prioridad, tipo, page, pageSize: DEFAULT_PAGE_SIZE });
                if (!active) return;
                setItems(data.expedientes || []);
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
    }, [estado, prioridad, tipo, page, refreshKey]);

    useEffect(() => {
        // Limpia el estado de navegación para que un F5 no reabra el modal.
        if (location.state?.openCreate) {
            navigate(location.pathname, { replace: true, state: null });
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const { usuarios, unavailable } = useUsuariosBasic(items.map((item) => item.solicitante_id));

    const totalPages = Math.max(1, Math.ceil(total / DEFAULT_PAGE_SIZE));

    const handleFilterChange = (setter) => (event) => {
        setPage(1);
        setter(event.target.value);
    };

    const handleDelete = async (codigo) => {
        setDeleteError("");
        setDeletingCodigo(codigo);
        try {
            await deleteExpediente(codigo);
            setConfirmingCodigo(null);
            setRefreshKey((current) => current + 1);
        } catch (err) {
            setDeleteError(friendlyErrorMessage(err));
        } finally {
            setDeletingCodigo(null);
        }
    };

    const emptyMessage = canViewAll
        ? "No hay expedientes registrados."
        : "Aún no tienes expedientes registrados.";

    return (
        <DashboardLayout title="Expedientes">
            <section className="dp-panel">
                <div className="dp-list-header">
                    <h2 className="dp-panel-title">Expedientes</h2>
                    {canCreate && (
                        <button type="button" className="dp-btn-primary" onClick={() => setShowCreate(true)}>
                            Nuevo expediente
                        </button>
                    )}
                </div>

                <div className="dp-filters">
                    <select value={estado} onChange={handleFilterChange(setEstado)} aria-label="Filtrar por estado">
                        <option value="">Todos los estados</option>
                        {ESTADOS.map((item) => (
                            <option key={item.value} value={item.value}>
                                {item.label}
                            </option>
                        ))}
                    </select>
                    <select value={prioridad} onChange={handleFilterChange(setPrioridad)} aria-label="Filtrar por prioridad">
                        <option value="">Todas las prioridades</option>
                        {PRIORIDADES.map((item) => (
                            <option key={item.value} value={item.value}>
                                {item.label}
                            </option>
                        ))}
                    </select>
                    <select value={tipo} onChange={handleFilterChange(setTipo)} aria-label="Filtrar por tipo">
                        <option value="">Todos los tipos</option>
                        {TIPOS.map((item) => (
                            <option key={item.value} value={item.value}>
                                {item.label}
                            </option>
                        ))}
                    </select>
                </div>

                {loading && <p className="dp-table-empty">Cargando expedientes...</p>}

                {!loading && error && <p className="dp-form-error">{error}</p>}
                {deleteError && <p className="dp-form-error">{deleteError}</p>}

                {!loading && !error && items.length === 0 && (
                    <p className="dp-table-empty">{emptyMessage}</p>
                )}

                {!loading && !error && items.length > 0 && (
                    <>
                        <div className="dp-table-wrapper">
                            <table className="dp-table">
                                <thead>
                                    <tr>
                                        <th>Expediente</th>
                                        <th>Asunto</th>
                                        <th>Remitente</th>
                                        <th>Estado</th>
                                        <th>Prioridad</th>
                                        <th>Fecha</th>
                                        <th>Acciones</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {items.map((item) => (
                                        <tr key={item.codigo}>
                                            <td className="dp-table-code">{item.codigo}</td>
                                            <td>{item.asunto}</td>
                                            <td>
                                                <SolicitanteNombre
                                                    id={item.solicitante_id}
                                                    usuarios={usuarios}
                                                    unavailable={unavailable}
                                                />
                                            </td>
                                            <td>
                                                <StatusBadge status={item.estado} />
                                            </td>
                                            <td>
                                                {item.prioridad === "URGENTE" ? (
                                                    <span className="dp-priority-urgente">Urgente</span>
                                                ) : (
                                                    "Normal"
                                                )}
                                            </td>
                                            <td>{formatDate(item.fecha_registro)}</td>
                                            <td>
                                                {confirmingCodigo === item.codigo ? (
                                                    <div className="dp-table-actions">
                                                        <span className="dp-table-confirm-text">¿Eliminar?</span>
                                                        <button
                                                            type="button"
                                                            className="dp-table-action"
                                                            onClick={() => setConfirmingCodigo(null)}
                                                            disabled={deletingCodigo === item.codigo}
                                                        >
                                                            No
                                                        </button>
                                                        <button
                                                            type="button"
                                                            className="dp-table-action dp-documentos-delete"
                                                            onClick={() => handleDelete(item.codigo)}
                                                            disabled={deletingCodigo === item.codigo}
                                                        >
                                                            {deletingCodigo === item.codigo ? "Eliminando..." : "Sí"}
                                                        </button>
                                                    </div>
                                                ) : (
                                                    <div className="dp-table-actions">
                                                        <Link to={`/expedientes/${item.codigo}`} className="dp-table-action">
                                                            <Icon name="eye" size={16} />
                                                            Ver
                                                        </Link>
                                                        {canDelete && (
                                                            <button
                                                                type="button"
                                                                className="dp-table-action dp-documentos-delete"
                                                                onClick={() => setConfirmingCodigo(item.codigo)}
                                                            >
                                                                Eliminar
                                                            </button>
                                                        )}
                                                    </div>
                                                )}
                                            </td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>
                        </div>

                        <div className="dp-pagination">
                            <span>
                                Página {page} de {totalPages} · {total} expediente{total === 1 ? "" : "s"}
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

            {showCreate && (
                <ExpedienteFormModal
                    onClose={() => setShowCreate(false)}
                    onCreated={() => {
                        setShowCreate(false);
                        setPage(1);
                        setRefreshKey((current) => current + 1);
                    }}
                />
            )}
        </DashboardLayout>
    );
}

export default ExpedientesList;
