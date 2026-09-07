import { Link } from "react-router-dom";
import { useUsuariosBasic } from "../../hooks/useUsuariosBasic";
import { formatDate } from "../../utils/format";
import SolicitanteNombre from "./SolicitanteNombre";
import StatusBadge from "./StatusBadge";
import Icon from "./Icon";
import "./RecentExpedientes.css";

function RecentExpedientes({ items, emptyMessage = "No hay expedientes registrados." }) {
    const { usuarios, unavailable } = useUsuariosBasic((items || []).map((item) => item.solicitante_id));

    return (
        <section className="dp-panel">
            <h2 className="dp-panel-title">Expedientes recientes</h2>

            {(!items || items.length === 0) ? (
                <p className="dp-table-empty">{emptyMessage}</p>
            ) : (
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
                                <th>Acción</th>
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
                                        <Link to={`/expedientes/${item.codigo}`} className="dp-table-action">
                                            <Icon name="eye" size={16} />
                                            Ver
                                        </Link>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}
        </section>
    );
}

export default RecentExpedientes;
