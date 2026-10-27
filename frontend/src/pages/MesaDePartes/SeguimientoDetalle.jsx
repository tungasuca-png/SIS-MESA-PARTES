import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import RoleLayout from "../../components/portal/RoleLayout";
import StatusBadge from "../../components/dashboard/StatusBadge";
import { LineaTiempoTramite, ProgresoTramite } from "../../components/portal/SeguimientoTramite";
import { getExpediente } from "../../services/expedientesService";
import { getDerivacionesByExpediente } from "../../services/derivacionesService";
import { usePermissions } from "../../hooks/usePermissions";
import { useAuth } from "../../hooks/useAuth";
import { friendlyErrorMessage } from "../../utils/apiErrors";
import { formatDateTime } from "../../utils/format";
import { calcularSeguimiento } from "../../utils/progresoTramite";
import { tipoLabel } from "../../constants/expedientes";
import { roleLabel } from "../../constants/roles";
import "../Expedientes/expedienteDetail.css";
import "./mesaDePartes.css";

const TIPO_DERIVACION_LABEL = {
    DERIVACION: "Derivado",
    NOTIFICACION: "Notificación",
    ASIGNACION: "Asignación",
    RECEPCION: "Recepción",
    APROBACION: "Aprobación",
};

// Seguimiento armado SOLO con datos reales:
//  - Etapas y porcentaje: calcularSeguimiento (utils/progresoTramite.js), a
//    partir de expediente.estado, fecha_registro y fecha_actualizacion.
//  - Movimientos: cada Derivación real (Derivaciones Service) — si no hay
//    ninguna registrada, no se inventan pasos.
// Expedientes Service no guarda un historial propio de cambios de estado
// (solo el estado vigente) — si en el futuro existe ese historial, cada
// cambio podría fechar las etapas intermedias.
// El acceso ya no depende de user.role (Paso 8): la ruta está protegida por
// PermissionRoute con expedientes.view/expedientes.view_own en App.jsx, y el
// backend solo entrega expedientes propios con view_own.
function SeguimientoDetalle() {
    const { id } = useParams();
    const { can } = usePermissions();
    const { user } = useAuth();
    // Las derivaciones se consultan solo con permiso (view_own para expedientes
    // propios, view general); sin ninguno no se pide ni se muestra nada.
    const puedeVerDerivaciones = can(["derivaciones.view", "derivaciones.view_own"]);

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

                if (puedeVerDerivaciones) {
                    const derivData = await getDerivacionesByExpediente(expData.expediente.id);
                    if (!active) return;
                    setDerivaciones(derivData.derivaciones || []);
                } else {
                    setDerivaciones([]);
                }
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
    }, [id, puedeVerDerivaciones]);

    // El backend devuelve las derivaciones más recientes primero; para una
    // línea de tiempo se necesita orden cronológico ascendente.
    const derivacionesOrdenadas = [...derivaciones].reverse();
    const seguimiento = calcularSeguimiento(expediente, derivaciones);

    // Misma condición que ExpedienteDetail para ofrecer la corrección: solo
    // el dueño del expediente observado (el backend vuelve a validarlo).
    const puedeCorregir =
        expediente &&
        can("expedientes.view_own") &&
        expediente.estado === "OBSERVADO" &&
        expediente.solicitante_id === user?.id;

    return (
        <RoleLayout title="Seguimiento del expediente">
            <Link to="/mesa-de-partes/seguimiento" className="dp-back-link">
                ← Volver a seguimiento
            </Link>

            {loading && <p className="dp-table-empty">Cargando...</p>}
            {!loading && error && <p className="dp-form-error">{error}</p>}

            {!loading && !error && expediente && (
                <section className="dp-panel">
                    <div className="dp-detail-header">
                        <div>
                            <p className="dp-detail-codigo">
                                {expediente.codigo} · {tipoLabel(expediente.tipo)}
                            </p>
                            <h2 className="dp-panel-title dp-detail-asunto">{expediente.asunto}</h2>
                        </div>
                        <StatusBadge status={expediente.estado} />
                    </div>

                    <dl className="seg-datos">
                        <div>
                            <dt>Presentado</dt>
                            <dd>{formatDateTime(expediente.fecha_registro)}</dd>
                        </div>
                        <div>
                            <dt>Última actualización</dt>
                            <dd>{formatDateTime(expediente.fecha_actualizacion)}</dd>
                        </div>
                        {expediente.area_actual && (
                            <div>
                                <dt>Área actual</dt>
                                <dd>{roleLabel(expediente.area_actual)}</dd>
                            </div>
                        )}
                    </dl>

                    {seguimiento ? (
                        <>
                            <ProgresoTramite seguimiento={seguimiento} />
                            <LineaTiempoTramite
                                seguimiento={seguimiento}
                                areaActual={expediente.area_actual ? roleLabel(expediente.area_actual) : ""}
                                accionObservacion={
                                    puedeCorregir && (
                                        <Link to={`/expedientes/${expediente.id}`} className="dp-btn-primary">
                                            Corregir y volver a presentar
                                        </Link>
                                    )
                                }
                            />
                        </>
                    ) : (
                        // Estado fuera del flujo conocido: se informa sin porcentaje.
                        <p className="dp-alert dp-alert--info">
                            Estado actual: {expediente.estado}. No es posible calcular el avance de este trámite.
                        </p>
                    )}

                    {puedeVerDerivaciones && (
                        <div className="seg-movimientos">
                            <h3 className="seg-movimientos-titulo">Movimientos registrados</h3>
                            <p className="seg-movimientos-nota">
                                Derivaciones de su expediente entre áreas, tal como fueron registradas.
                            </p>
                            {derivacionesOrdenadas.length === 0 ? (
                                <p className="seg-movimientos-nota">Aún no hay movimientos registrados.</p>
                            ) : (
                                <ul className="mdp-timeline">
                                    {derivacionesOrdenadas.map((item) => (
                                        <li key={item.id} className="mdp-timeline-item">
                                            <span className="mdp-timeline-dot" />
                                            <p className="mdp-timeline-title">
                                                {TIPO_DERIVACION_LABEL[item.tipo] ?? item.tipo}: {roleLabel(item.origen)} →{" "}
                                                {roleLabel(item.destino)}
                                            </p>
                                            <p className="mdp-timeline-meta">{formatDateTime(item.fecha_registro)}</p>
                                            {(item.motivo || item.condicion) && (
                                                <p className="mdp-timeline-desc">
                                                    {item.motivo}
                                                    {item.condicion ? ` — ${item.condicion}` : ""}
                                                </p>
                                            )}
                                        </li>
                                    ))}
                                </ul>
                            )}
                        </div>
                    )}
                </section>
            )}
        </RoleLayout>
    );
}

export default SeguimientoDetalle;
