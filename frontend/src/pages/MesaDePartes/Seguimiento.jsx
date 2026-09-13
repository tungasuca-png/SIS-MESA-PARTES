import { useEffect, useState } from "react";
import { Link, Navigate } from "react-router-dom";
import PortalLayout from "../../components/portal/PortalLayout";
import StatusBadge from "../../components/dashboard/StatusBadge";
import { useAuth } from "../../hooks/useAuth";
import { getExpedientes } from "../../services/expedientesService";
import { friendlyErrorMessage } from "../../utils/apiErrors";
import { formatDate } from "../../utils/format";
import { DEFAULT_PAGE_SIZE, tipoLabel } from "../../constants/expedientes";
import "../../components/dashboard/RecentExpedientes.css";
import "../../components/dashboard/forms.css";
import "./mesaDePartes.css";

// Lista de expedientes propios para elegir cuál seguir (el backend ya acota
// a "los del usuario autenticado" — ver ListExpedientesLogic de Expedientes
// Service). El detalle del seguimiento (línea de tiempo) vive en
// SeguimientoDetalle.jsx.
function Seguimiento() {
    const { user } = useAuth();
    const [items, setItems] = useState([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");

    useEffect(() => {
        let active = true;

        async function load() {
            setLoading(true);
            setError("");
            try {
                const data = await getExpedientes({ page: 1, pageSize: DEFAULT_PAGE_SIZE });
                if (!active) return;
                setItems(data.expedientes || []);
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
    }, []);

    if (user && user.role !== "SOLICITANTE") {
        return <Navigate to="/dashboard" replace />;
    }

    return (
        <PortalLayout title="Seguimiento">
            <section className="dp-panel">
                <h2 className="dp-panel-title">Seguimiento de mis solicitudes</h2>

                {loading && <p className="dp-table-empty">Cargando...</p>}
                {!loading && error && <p className="dp-form-error">{error}</p>}
                {!loading && !error && items.length === 0 && (
                    <p className="dp-table-empty">Aún no tienes solicitudes registradas.</p>
                )}

                {!loading && !error && items.length > 0 && (
                    <div className="dp-table-wrapper">
                        <table className="dp-table">
                            <thead>
                                <tr>
                                    <th>Expediente</th>
                                    <th>Tipo</th>
                                    <th>Asunto</th>
                                    <th>Fecha</th>
                                    <th>Estado</th>
                                    <th>Acción</th>
                                </tr>
                            </thead>
                            <tbody>
                                {items.map((item) => (
                                    <tr key={item.codigo}>
                                        <td className="dp-table-code">{item.codigo}</td>
                                        <td>{tipoLabel(item.tipo)}</td>
                                        <td>{item.asunto}</td>
                                        <td>{formatDate(item.fecha_registro)}</td>
                                        <td>
                                            <StatusBadge status={item.estado} />
                                        </td>
                                        <td>
                                            <Link
                                                to={`/mesa-de-partes/seguimiento/${item.codigo}`}
                                                className="dp-table-action"
                                            >
                                                Ver seguimiento
                                            </Link>
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                )}
            </section>
        </PortalLayout>
    );
}

export default Seguimiento;
