import { useEffect, useState } from "react";
import { Link, Navigate, useParams } from "react-router-dom";
import PortalLayout from "../../components/portal/PortalLayout";
import StatusBadge from "../../components/dashboard/StatusBadge";
import { useAuth } from "../../hooks/useAuth";
import { getExpediente } from "../../services/expedientesService";
import { getDerivacionesByExpediente } from "../../services/derivacionesService";
import { friendlyErrorMessage } from "../../utils/apiErrors";
import { formatDateTime } from "../../utils/format";
import "../Expedientes/expedienteDetail.css";
import "./mesaDePartes.css";

const TIPO_DERIVACION_LABEL = {
    DERIVACION: "Derivado",
    NOTIFICACION: "Notificación",
    ASIGNACION: "Asignación",
    RECEPCION: "Recepción",
    APROBACION: "Aprobación",
};

// Línea de tiempo armada SOLO con datos reales:
//  - "Solicitud presentada" viene de expediente.fecha_registro.
//  - Cada paso intermedio viene de una Derivación real (Derivaciones
//    Service) — si no hay ninguna registrada, esa parte de la línea
//    simplemente no aparece (no se inventan pasos).
//  - El estado actual viene de expediente.estado / fecha_actualizacion.
// Expedientes Service no guarda un historial propio de cambios de estado
// (solo el estado vigente) — si en el futuro existe ese historial, cada
// cambio podría agregarse acá como un paso más.
function SeguimientoDetalle() {
    const { user } = useAuth();
    const { id } = useParams();

    const [expediente, setExpediente] = useState(null);
    const [derivaciones, setDerivaciones] = useState([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");

    useEffect(() => {
        let active = true;

        async function load() {
            setLoading(true);
            setError("");
            try {
                const expData = await getExpediente(id);
                if (!active) return;
                setExpediente(expData.expediente);

                const derivData = await getDerivacionesByExpediente(expData.expediente.id);
                if (!active) return;
                setDerivaciones(derivData.derivaciones || []);
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
    }, [id]);

    if (user && user.role !== "SOLICITANTE") {
        return <Navigate to="/dashboard" replace />;
    }

    // El backend devuelve las derivaciones más recientes primero; para una
    // línea de tiempo se necesita orden cronológico ascendente.
    const derivacionesOrdenadas = [...derivaciones].reverse();

    return (
        <PortalLayout title="Seguimiento del expediente">
            <Link to="/mesa-de-partes/seguimiento" className="dp-back-link">
                ← Volver a seguimiento
            </Link>

            {loading && <p className="dp-table-empty">Cargando...</p>}
            {!loading && error && <p className="dp-form-error">{error}</p>}

            {!loading && !error && expediente && (
                <section className="dp-panel">
                    <div className="dp-detail-header">
                        <div>
                            <p className="dp-detail-codigo">{expediente.codigo}</p>
                            <h2 className="dp-panel-title dp-detail-asunto">{expediente.asunto}</h2>
                        </div>
                        <StatusBadge status={expediente.estado} />
                    </div>

                    <ul className="mdp-timeline mdp-timeline--detail">
                        <li className="mdp-timeline-item">
                            <span className="mdp-timeline-dot" />
                            <p className="mdp-timeline-title">Solicitud presentada</p>
                            <p className="mdp-timeline-meta">{formatDateTime(expediente.fecha_registro)}</p>
                            <p className="mdp-timeline-desc">Su solicitud fue registrada correctamente.</p>
                        </li>

                        {derivacionesOrdenadas.map((item) => (
                            <li key={item.id} className="mdp-timeline-item">
                                <span className="mdp-timeline-dot" />
                                <p className="mdp-timeline-title">
                                    {TIPO_DERIVACION_LABEL[item.tipo] ?? item.tipo}: {item.origen} → {item.destino}
                                </p>
                                <p className="mdp-timeline-meta">{formatDateTime(item.fecha_registro)}</p>
                                <p className="mdp-timeline-desc">
                                    {item.motivo}
                                    {item.condicion ? ` — ${item.condicion}` : ""}
                                </p>
                            </li>
                        ))}

                        <li className="mdp-timeline-item mdp-timeline-item--pendiente">
                            <span className="mdp-timeline-dot" />
                            <p className="mdp-timeline-title">Estado actual: {expediente.estado.replace("_", " ")}</p>
                            <p className="mdp-timeline-meta">
                                Última actualización: {formatDateTime(expediente.fecha_actualizacion)}
                            </p>
                        </li>
                    </ul>
                </section>
            )}
        </PortalLayout>
    );
}

export default SeguimientoDetalle;
