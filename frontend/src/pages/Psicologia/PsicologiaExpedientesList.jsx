import { useEffect, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import RoleLayout from "../../components/portal/RoleLayout";
import PsicologiaExpedienteFormModal from "../../components/dashboard/PsicologiaExpedienteFormModal";
import StatusBadge from "../../components/dashboard/StatusBadge";
import SolicitanteNombre from "../../components/dashboard/SolicitanteNombre";
import EstudianteNombre from "../../components/dashboard/EstudianteNombre";
import Icon from "../../components/dashboard/Icon";
import BuscarAlumnoPanel from "../../components/psicologia/BuscarAlumnoPanel";
import { usePermissions } from "../../hooks/usePermissions";
import { useUsuariosBasic } from "../../hooks/useUsuariosBasic";
import { useEstudiantesBasic } from "../../hooks/useEstudiantesBasic";
import { listExpedientesPsicologicos } from "../../services/psicologiaService";
import { friendlyErrorMessage } from "../../utils/apiErrors";
import { formatDate } from "../../utils/format";
import { ESTADOS } from "../../constants/psicologia";
import "../../components/dashboard/RecentExpedientes.css";
import "../../components/dashboard/forms.css";
import "../Expedientes/expedientesList.css";

const PAGE_SIZE = 20;

// Psicología → Expedientes: listado real (GET /api/psicologia/expedientes).
// El backend es la única autoridad de alcance: con "psicologia.expedientes.
// view" devuelve todos los expedientes permitidos; con solo "...view_own",
// únicamente los que tienen como responsable al usuario autenticado. Este
// componente no reproduce esa regla, solo muestra lo que la API entrega.
function PsicologiaExpedientesList() {
    const { can } = usePermissions();
    const [searchParams, setSearchParams] = useSearchParams();

    const [items, setItems] = useState([]);
    const [total, setTotal] = useState(0);
    const [page, setPage] = useState(1);
    const [estado, setEstado] = useState(searchParams.get("estado") || "");
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");
    const [showCreate, setShowCreate] = useState(false);
    const [createEstudiante, setCreateEstudiante] = useState(null); // desde "Buscar alumno"
    const navigate = useNavigate();
    const [refreshKey, setRefreshKey] = useState(0);

    const canCreate = can("psicologia.expedientes.create");
    // El buscador consulta Estudiantes Service ("estudiantes.view").
    const canBuscarAlumno = can("estudiantes.view");

    useEffect(() => {
        let active = true;

        async function load() {
            setLoading(true);
            setError("");
            try {
                const data = await listExpedientesPsicologicos({ estado, page, pageSize: PAGE_SIZE });
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
    }, [estado, page, refreshKey]);

    const { usuarios, unavailable } = useUsuariosBasic(items.map((item) => item.responsable_id));
    // FASE 5D: estudiante_id ahora se resuelve a un nombre real (Estudiantes
    // Service), igual que responsable_id se resuelve contra Usuarios Service.
    const { estudiantes, unavailable: estudiantesUnavailable } = useEstudiantesBasic(
        items.map((item) => item.estudiante_id)
    );

    const handleEstadoChange = (event) => {
        const value = event.target.value;
        setPage(1);
        setEstado(value);
        setSearchParams(value ? { estado: value } : {}, { replace: true });
    };

    const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

    return (
        <RoleLayout title="Psicología · Expedientes">
            {canBuscarAlumno && (
                <BuscarAlumnoPanel
                    canCreate={canCreate}
                    onAbrirExpediente={(estudiante) => {
                        setCreateEstudiante(estudiante);
                        setShowCreate(true);
                    }}
                />
            )}

            <section className="dp-panel">
                <div className="dp-list-header">
                    <h2 className="dp-panel-title">Expedientes psicológicos</h2>
                    {canCreate && (
                        <button type="button" className="dp-btn-primary" onClick={() => setShowCreate(true)}>
                            Nuevo expediente
                        </button>
                    )}
                </div>

                <div className="dp-filters">
                    <select value={estado} onChange={handleEstadoChange} aria-label="Filtrar por estado">
                        <option value="">Todos los estados</option>
                        {ESTADOS.map((item) => (
                            <option key={item.value} value={item.value}>
                                {item.label}
                            </option>
                        ))}
                    </select>
                </div>

                {loading && <p className="dp-table-empty">Cargando expedientes...</p>}
                {!loading && error && <p className="dp-form-error">{error}</p>}

                {!loading && !error && items.length === 0 && (
                    <p className="dp-table-empty">No hay expedientes psicológicos registrados todavía.</p>
                )}

                {!loading && !error && items.length > 0 && (
                    <>
                        <div className="dp-table-wrapper">
                            <table className="dp-table">
                                <thead>
                                    <tr>
                                        <th>Código</th>
                                        <th>Estudiante</th>
                                        <th>Responsable</th>
                                        <th>Estado</th>
                                        <th>Fecha de apertura</th>
                                        <th>Acciones</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {items.map((item) => (
                                        <tr key={item.id}>
                                            <td className="dp-table-code">{item.codigo}</td>
                                            <td>
                                                <EstudianteNombre
                                                    id={item.estudiante_id}
                                                    estudiantes={estudiantes}
                                                    unavailable={estudiantesUnavailable}
                                                />
                                            </td>
                                            <td>
                                                <SolicitanteNombre
                                                    id={item.responsable_id}
                                                    usuarios={usuarios}
                                                    unavailable={unavailable}
                                                />
                                            </td>
                                            <td>
                                                <StatusBadge status={item.estado} />
                                            </td>
                                            <td>{formatDate(item.fecha_apertura)}</td>
                                            <td>
                                                <Link to={`/psicologia/expedientes/${item.id}`} className="dp-table-action">
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
                <PsicologiaExpedienteFormModal
                    initialEstudiante={createEstudiante}
                    onClose={() => {
                        setShowCreate(false);
                        setCreateEstudiante(null);
                    }}
                    onCreated={(expediente) => {
                        setShowCreate(false);
                        if (createEstudiante && expediente?.id) {
                            navigate(`/psicologia/expedientes/${expediente.id}`);
                            return;
                        }
                        setPage(1);
                        setRefreshKey((current) => current + 1);
                    }}
                />
            )}
        </RoleLayout>
    );
}

export default PsicologiaExpedientesList;
