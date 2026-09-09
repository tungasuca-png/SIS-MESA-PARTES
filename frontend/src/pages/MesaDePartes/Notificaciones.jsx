import { useEffect, useState } from "react";
import { Link, Navigate } from "react-router-dom";
import PortalLayout from "../../components/portal/PortalLayout";
import Icon from "../../components/dashboard/Icon";
import { useAuth } from "../../hooks/useAuth";
import { getExpedientes } from "../../services/expedientesService";
import { getDerivacionesByExpediente } from "../../services/derivacionesService";
import { friendlyErrorMessage } from "../../utils/apiErrors";
import { formatDateTime } from "../../utils/format";
import { TIPOS_DERIVACION } from "../../constants/derivaciones";
import "../../components/dashboard/documentosPanel.css";
import "./mesaDePartes.css";

function tipoDerivacionLabel(value) {
    return TIPOS_DERIVACION.find((item) => item.value === value)?.label ?? value;
}

// No existe un Notificaciones Service (no hay tabla ni eventos push) — en
// vez de inventar datos, este feed se arma con eventos REALES que ya
// existen: cuando se presentó cada solicitud (fecha_registro), cada
// derivación real registrada (Derivaciones Service) y, si ya se resolvió,
// el resultado final (Atendido/Observado, con su fecha_actualizacion).
// Expedientes Service no guarda un historial de CADA cambio de estado
// (solo el vigente), así que no se puede notificar "pasó a en proceso el
// día X" salvo el desenlace final — ver docs/etapa-12-especificacion-
// funcional.md sección 17 (Notificaciones), que ya deja esto documentado
// como pendiente de un canal/servicio real.
async function construirNotificaciones() {
    const { expedientes } = await getExpedientes({ page: 1, pageSize: 50 });
    const items = [];

    for (const exp of expedientes || []) {
        items.push({
            key: `registro-${exp.id}`,
            fecha: exp.fecha_registro,
            tipoVisual: "pendiente",
            titulo: "Solicitud registrada",
            codigo: exp.codigo,
            mensaje: exp.asunto,
        });

        if (exp.estado === "ATENDIDO" || exp.estado === "OBSERVADO") {
            items.push({
                key: `resultado-${exp.id}`,
                fecha: exp.fecha_actualizacion,
                tipoVisual: exp.estado === "ATENDIDO" ? "atendido" : "observado",
                titulo: exp.estado === "ATENDIDO" ? "Solicitud atendida" : "Solicitud observada",
                codigo: exp.codigo,
                mensaje: exp.asunto,
            });
        }
    }

    const derivacionesPorExpediente = await Promise.all(
        (expedientes || []).map((exp) =>
            getDerivacionesByExpediente(exp.id)
                .then((data) => ({ exp, derivaciones: data.derivaciones || [] }))
                .catch(() => ({ exp, derivaciones: [] }))
        )
    );

    for (const { exp, derivaciones } of derivacionesPorExpediente) {
        for (const item of derivaciones) {
            items.push({
                key: `deriv-${item.id}`,
                fecha: item.fecha_registro,
                tipoVisual: "proceso",
                titulo: tipoDerivacionLabel(item.tipo),
                codigo: exp.codigo,
                mensaje: `${item.origen} → ${item.destino}: ${item.motivo}`,
            });
        }
    }

    items.sort((a, b) => new Date(b.fecha) - new Date(a.fecha));
    return items;
}

function Notificaciones() {
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
                const data = await construirNotificaciones();
                if (active) setItems(data);
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
        <PortalLayout title="Notificaciones">
            <section className="dp-panel">
                <h2 className="dp-panel-title">Novedades de mis solicitudes</h2>

                {loading && <p className="dp-table-empty">Cargando notificaciones...</p>}
                {!loading && error && <p className="dp-form-error">{error}</p>}
                {!loading && !error && items.length === 0 && (
                    <p className="dp-table-empty">Todavía no tienes novedades.</p>
                )}

                {!loading && !error && items.length > 0 && (
                    <ul className="dp-documentos-list">
                        {items.map((item) => (
                            <li key={item.key} className="dp-documentos-item">
                                <div className="dp-documentos-info">
                                    <span className={`dp-badge dp-badge--${item.tipoVisual}`}>
                                        <Icon name="bell" size={13} />
                                    </span>
                                    <div>
                                        <p className="dp-documentos-nombre">
                                            {item.titulo} —{" "}
                                            <Link to={`/expedientes/${item.codigo}`} className="dp-table-action">
                                                {item.codigo}
                                            </Link>
                                        </p>
                                        <p className="dp-documentos-meta">
                                            {item.mensaje} · {formatDateTime(item.fecha)}
                                        </p>
                                    </div>
                                </div>
                            </li>
                        ))}
                    </ul>
                )}
            </section>
        </PortalLayout>
    );
}

export default Notificaciones;
