import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import RoleLayout from "../../components/portal/RoleLayout";
import StatusBadge from "../../components/dashboard/StatusBadge";
import SolicitanteNombre from "../../components/dashboard/SolicitanteNombre";
import EstudianteNombre from "../../components/dashboard/EstudianteNombre";
import ResponsablePsicologoField from "../../components/dashboard/ResponsablePsicologoField";
import Icon from "../../components/dashboard/Icon";
import AtencionesSection from "../../components/psicologia/AtencionesSection";
import { usePermissions } from "../../hooks/usePermissions";
import { useUsuariosBasic } from "../../hooks/useUsuariosBasic";
import { useEstudiantesBasic } from "../../hooks/useEstudiantesBasic";
import {
    closeExpedientePsicologico,
    getExpedientePsicologico,
    reopenExpedientePsicologico,
    updateExpedientePsicologico,
} from "../../services/psicologiaService";
import { friendlyErrorMessage } from "../../utils/apiErrors";
import { formatDateTime } from "../../utils/format";
import { ESTADOS_EDITABLES } from "../../constants/psicologia";
import "../../components/dashboard/forms.css";
import "../Expedientes/expedienteDetail.css";

// Mismo límite que el backend (Psicología internal/validation.MaxMotivo);
// el backend sigue siendo la validación definitiva.
const MAX_MOTIVO_CIERRE = 2000;

// Psicología → Expediente → Detalle (GET /api/psicologia/expedientes/:id) +
// edición (PUT .../:id) + cierre (PUT .../:id/cerrar) y reapertura
// (PUT .../:id/reabrir, siempre a ABIERTO) + sección de Atenciones
// (AtencionesSection, con "psicologia.atenciones.view").
function PsicologiaExpedienteDetail() {
    const { id } = useParams();
    const { can } = usePermissions();

    const [expediente, setExpediente] = useState(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");

    const [editing, setEditing] = useState(false);
    const [resumenCaso, setResumenCaso] = useState("");
    const [responsableId, setResponsableId] = useState("");
    const [estado, setEstado] = useState("ABIERTO");
    const [saving, setSaving] = useState(false);
    const [saveError, setSaveError] = useState("");

    // Cierre / reapertura: null | "cerrar" | "reabrir" (modal abierto).
    const [lifecycleAction, setLifecycleAction] = useState(null);
    const [motivoCierre, setMotivoCierre] = useState("");
    const [lifecycleSubmitting, setLifecycleSubmitting] = useState(false);
    const [lifecycleError, setLifecycleError] = useState("");

    const canUpdate = can("psicologia.expedientes.update");
    const canViewAtenciones = can("psicologia.atenciones.view");

    useEffect(() => {
        let active = true;

        async function load() {
            setLoading(true);
            setError("");
            try {
                const data = await getExpedientePsicologico(id);
                if (!active) return;
                setExpediente(data.expediente);
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
    }, [id]);

    const { usuarios, unavailable } = useUsuariosBasic(
        expediente ? [expediente.responsable_id, expediente.abierto_por] : []
    );
    // FASE 5D: estudiante_id se resuelve a un nombre real (Estudiantes Service).
    const { estudiantes, unavailable: estudianteUnavailable } = useEstudiantesBasic(
        expediente ? [expediente.estudiante_id] : []
    );

    const startEditing = () => {
        setSaveError("");
        setResumenCaso(expediente.resumen_caso || "");
        setResponsableId(""); // vacío = mantener al responsable actual (no se reenvía salvo que se cambie)
        setEstado(expediente.estado === "EN_SEGUIMIENTO" ? "EN_SEGUIMIENTO" : "ABIERTO");
        setEditing(true);
    };

    const handleSave = async (event) => {
        event.preventDefault();
        setSaveError("");
        setSaving(true);
        try {
            const data = await updateExpedientePsicologico(id, {
                // Sin acceso clínico el resumen no se envía (no se sobrescribe a ciegas).
                resumenCaso: canViewAtenciones ? resumenCaso : undefined,
                responsableId: responsableId || undefined,
                estado,
            });
            setExpediente(data.expediente);
            setEditing(false);
        } catch (err) {
            setSaveError(friendlyErrorMessage(err));
        } finally {
            setSaving(false);
        }
    };

    const openLifecycle = (action) => {
        setLifecycleError("");
        setMotivoCierre("");
        setLifecycleAction(action);
    };

    const closeLifecycle = () => {
        if (lifecycleSubmitting) return;
        setLifecycleAction(null);
    };

    const handleConfirmLifecycle = async (event) => {
        event.preventDefault();
        if (lifecycleSubmitting) return;
        setLifecycleError("");

        const motivo = motivoCierre.trim();
        if (lifecycleAction === "cerrar" && !motivo) {
            setLifecycleError("El motivo del cierre es obligatorio.");
            return;
        }

        setLifecycleSubmitting(true);
        try {
            // Mismo criterio que handleSave: el backend devuelve el
            // expediente ya actualizado (estado, fecha y motivo de cierre).
            const data =
                lifecycleAction === "cerrar"
                    ? await closeExpedientePsicologico(id, motivo)
                    : await reopenExpedientePsicologico(id);
            setExpediente(data.expediente);
            setLifecycleAction(null);
        } catch (err) {
            setLifecycleError(friendlyErrorMessage(err, "El expediente psicológico no fue encontrado."));
        } finally {
            setLifecycleSubmitting(false);
        }
    };

    if (loading) {
        return (
            <RoleLayout title="Expediente psicológico">
                <p className="dp-table-empty">Cargando expediente...</p>
            </RoleLayout>
        );
    }

    if (error) {
        return (
            <RoleLayout title="Expediente psicológico">
                <section className="dp-panel">
                    <p className="dp-form-error">{error}</p>
                    <Link to="/psicologia/expedientes" className="dp-btn-secondary" style={{ marginTop: 12, display: "inline-block" }}>
                        Volver al listado
                    </Link>
                </section>
            </RoleLayout>
        );
    }

    if (!expediente) return null;

    const estaCerrado = expediente.estado === "CERRADO";

    return (
        <RoleLayout title={`Expediente ${expediente.codigo}`}>
            <section className="dp-panel">
                <div className="dp-list-header">
                    <h2 className="dp-panel-title">
                        {expediente.codigo} <StatusBadge status={expediente.estado} />
                    </h2>
                    {canUpdate && !editing && (
                        <div className="dp-form-actions" style={{ marginTop: 0 }}>
                            {!estaCerrado && (
                                <>
                                    <button type="button" className="dp-btn-secondary" onClick={() => openLifecycle("cerrar")}>
                                        Cerrar expediente
                                    </button>
                                    <button type="button" className="dp-btn-primary" onClick={startEditing}>
                                        Editar
                                    </button>
                                </>
                            )}
                            {estaCerrado && (
                                <button type="button" className="dp-btn-primary" onClick={() => openLifecycle("reabrir")}>
                                    Reabrir expediente
                                </button>
                            )}
                        </div>
                    )}
                </div>

                {estaCerrado && (
                    <p className="dp-form-hint">Este expediente está cerrado; no admite modificaciones.</p>
                )}

                {!editing && (
                    <dl className="dp-detail-list">
                        <div>
                            <dt>Estudiante</dt>
                            <dd>
                                <EstudianteNombre
                                    id={expediente.estudiante_id}
                                    estudiantes={estudiantes}
                                    unavailable={estudianteUnavailable}
                                />
                            </dd>
                        </div>
                        <div>
                            <dt>Responsable</dt>
                            <dd>
                                <SolicitanteNombre id={expediente.responsable_id} usuarios={usuarios} unavailable={unavailable} />
                            </dd>
                        </div>
                        <div>
                            <dt>Abierto por</dt>
                            <dd>
                                <SolicitanteNombre id={expediente.abierto_por} usuarios={usuarios} unavailable={unavailable} />
                            </dd>
                        </div>
                        <div>
                            <dt>Fecha de apertura</dt>
                            <dd>{formatDateTime(expediente.fecha_apertura)}</dd>
                        </div>
                        {expediente.fecha_cierre && (
                            <div>
                                <dt>Fecha de cierre</dt>
                                <dd>{formatDateTime(expediente.fecha_cierre)}</dd>
                            </div>
                        )}
                        {expediente.motivo_cierre && (
                            <div>
                                <dt>Motivo de cierre</dt>
                                <dd>{expediente.motivo_cierre}</dd>
                            </div>
                        )}
                        {/* Dato clínico: solo en el detalle, nunca en el listado. */}
                        {/* El backend solo envía el resumen con psicologia.atenciones.view. */}
                        <div>
                            <dt>Resumen del caso</dt>
                            <dd>
                                {canViewAtenciones
                                    ? expediente.resumen_caso || "Sin registrar."
                                    : "Requiere la autorización para consultar contenido clínico."}
                            </dd>
                        </div>
                    </dl>
                )}

                {editing && (
                    <form className="dp-form" onSubmit={handleSave}>
                        <div className="dp-form-group">
                            <label htmlFor="psi-edit-estado">Estado</label>
                            <select
                                id="psi-edit-estado"
                                value={estado}
                                onChange={(event) => setEstado(event.target.value)}
                                disabled={saving}
                            >
                                {ESTADOS_EDITABLES.map((item) => (
                                    <option key={item.value} value={item.value}>
                                        {item.label}
                                    </option>
                                ))}
                            </select>
                        </div>

                        <ResponsablePsicologoField
                            id="psi-edit-responsable"
                            value={responsableId}
                            onChange={setResponsableId}
                            disabled={saving}
                        />
                        <p className="dp-form-hint">
                            Deja "Yo mismo" sin cambios para mantener al responsable actual; solo se
                            reasigna si eliges otro psicólogo.
                        </p>

                        {canViewAtenciones && (
                        <div className="dp-form-group">
                            <label htmlFor="psi-edit-resumen">Resumen del caso</label>
                            <textarea
                                id="psi-edit-resumen"
                                value={resumenCaso}
                                onChange={(event) => setResumenCaso(event.target.value)}
                                maxLength={10000}
                                rows={6}
                                disabled={saving}
                            />
                        </div>
                        )}

                        {saveError && <p className="dp-form-error">{saveError}</p>}

                        <div className="dp-form-actions">
                            <button type="button" className="dp-btn-secondary" onClick={() => setEditing(false)} disabled={saving}>
                                Cancelar
                            </button>
                            <button type="submit" className="dp-btn-primary" disabled={saving}>
                                {saving ? "Guardando..." : "Guardar cambios"}
                            </button>
                        </div>
                    </form>
                )}
            </section>

            {canViewAtenciones && (
                <AtencionesSection expediente={expediente} canCreate={can("psicologia.atenciones.create")} />
            )}

            <Link to="/psicologia/expedientes" className="dp-btn-secondary" style={{ marginTop: 16, display: "inline-block" }}>
                ← Volver al listado
            </Link>

            {lifecycleAction && (
                <div className="dp-modal-overlay" onClick={closeLifecycle}>
                    <div
                        className="dp-modal"
                        role="dialog"
                        aria-modal="true"
                        aria-label={lifecycleAction === "cerrar" ? "Cerrar expediente" : "Reabrir expediente"}
                        onClick={(event) => event.stopPropagation()}
                    >
                        <div className="dp-modal-header">
                            <h2>{lifecycleAction === "cerrar" ? "Cerrar expediente" : "Reabrir expediente"}</h2>
                            <button
                                type="button"
                                className="dp-modal-close"
                                onClick={closeLifecycle}
                                disabled={lifecycleSubmitting}
                                aria-label="Cerrar"
                            >
                                <Icon name="close" size={18} />
                            </button>
                        </div>

                        <form className="dp-form" onSubmit={handleConfirmLifecycle}>
                            {lifecycleAction === "cerrar" ? (
                                <>
                                    <p className="dp-form-hint" style={{ margin: 0 }}>
                                        Esta acción cerrará el expediente psicológico. Para continuar, indique el
                                        motivo del cierre.
                                    </p>
                                    <div className="dp-form-group">
                                        <label htmlFor="psi-motivo-cierre">Motivo del cierre *</label>
                                        <textarea
                                            id="psi-motivo-cierre"
                                            value={motivoCierre}
                                            onChange={(event) => setMotivoCierre(event.target.value)}
                                            maxLength={MAX_MOTIVO_CIERRE}
                                            rows={4}
                                            disabled={lifecycleSubmitting}
                                        />
                                    </div>
                                </>
                            ) : (
                                <p className="dp-form-hint" style={{ margin: 0 }}>
                                    ¿Está seguro de que desea reabrir este expediente?
                                </p>
                            )}

                            {lifecycleError && <p className="dp-form-error">{lifecycleError}</p>}

                            <div className="dp-form-actions">
                                <button
                                    type="button"
                                    className="dp-btn-secondary"
                                    onClick={closeLifecycle}
                                    disabled={lifecycleSubmitting}
                                >
                                    Cancelar
                                </button>
                                <button type="submit" className="dp-btn-primary" disabled={lifecycleSubmitting}>
                                    {lifecycleSubmitting
                                        ? lifecycleAction === "cerrar"
                                            ? "Cerrando..."
                                            : "Reabriendo..."
                                        : lifecycleAction === "cerrar"
                                          ? "Cerrar expediente"
                                          : "Reabrir expediente"}
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            )}
        </RoleLayout>
    );
}

export default PsicologiaExpedienteDetail;
