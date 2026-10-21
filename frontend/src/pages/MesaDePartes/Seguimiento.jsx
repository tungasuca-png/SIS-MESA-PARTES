import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import RoleLayout from "../../components/portal/RoleLayout";
import StatusBadge from "../../components/dashboard/StatusBadge";
import { getExpedientes } from "../../services/expedientesService";
import { useActualizacionEnVivo } from "../../hooks/useActualizacionEnVivo";
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
//
// El acceso ya no depende de user.role (Paso 8): la ruta está protegida por
// PermissionRoute con expedientes.view/expedientes.view_own en App.jsx, así
// que este componente ya no necesita (ni debe) repetir esa decisión — usa
// RoleLayout para que un usuario interno vea su sidebar normal en vez del
// portal de SOLICITANTE.
function Seguimiento() {
    const [items, setItems] = useState([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");

    // En vivo: el estado de cada trámite se actualiza solo (sin recargar).
    const [vivo, setVivo] = useState(0);
    useActualizacionEnVivo(() => setVivo((n) => n + 1));

    useEffect(() => {
        let active = true;

        async function load() {
            if (vivo === 0) setLoading(true);
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
    }, [vivo]);

    return (
        <RoleLayout title="Seguimiento">
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
        </RoleLayout>
    );
}

export default Seguimiento;
