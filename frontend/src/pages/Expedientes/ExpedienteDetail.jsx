import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import RoleLayout from "../../components/portal/RoleLayout";
import StatusBadge from "../../components/dashboard/StatusBadge";
import SolicitanteNombre from "../../components/dashboard/SolicitanteNombre";
import DocumentosPanel from "../../components/dashboard/DocumentosPanel";
import DerivacionesPanel from "../../components/dashboard/DerivacionesPanel";
import { usePermissions } from "../../hooks/usePermissions";
import { useUsuariosBasic } from "../../hooks/useUsuariosBasic";
import { changeEstado, getExpediente, updateExpediente } from "../../services/expedientesService";
import { parseDatosSolicitante } from "../../services/futService";
import { roleLabel } from "../../constants/roles";
import { friendlyErrorMessage } from "../../utils/apiErrors";
import { formatDateTime } from "../../utils/format";
import { ESTADOS, PRIORIDADES } from "../../constants/expedientes";
import "../../components/dashboard/forms.css";
import "./expedienteDetail.css";

function ExpedienteDetail() {
    const { id } = useParams();
    const { can } = usePermissions();

    const [expediente, setExpediente] = useState(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");

    const [editing, setEditing] = useState(false);
    const [asunto, setAsunto] = useState("");
    const [descripcion, setDescripcion] = useState("");
    const [prioridad, setPrioridad] = useState("NORMAL");
    const [saving, setSaving] = useState(false);
    const [formError, setFormError] = useState("");

    const [nuevoEstado, setNuevoEstado] = useState("");
    const [changingEstado, setChangingEstado] = useState(false);
    const [estadoError, setEstadoError] = useState("");

    const canUpdate = can("expedientes.update");
    const canChangeEstado = can("expedientes.change_estado");
    // Las derivaciones son un movimiento interno del expediente (que area lo
    // deriva a cual, y por que): es informacion de gestion, no algo que el
    // SOLICITANTE necesite ver en el detalle de su propio tramite.
    const canViewDerivaciones = can("derivaciones.view");
    const { usuarios, unavailable } = useUsuariosBasic(
        expediente ? [expediente.solicitante_id] : []
    );

    useEffect(() => {
        let active = true;

        async function load() {
            setLoading(true);
            setError("");
            try {
                const data = await getExpediente(id);
                if (!active) return;
                setExpediente(data.expediente);
                setAsunto(data.expediente.asunto);
                setDescripcion(data.expediente.descripcion || "");
                setPrioridad(data.expediente.prioridad);
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

    const handleSave = async (event) => {
        event.preventDefault();
        setFormError("");

        const trimmed = asunto.trim();
        if (!trimmed) {
            setFormError("El asunto es obligatorio.");
            return;
        }
        if (trimmed.length > 255) {
            setFormError("El asunto no puede superar los 255 caracteres.");
            return;
        }

        setSaving(true);
        try {
            const data = await updateExpediente(expediente.id, {
                asunto: trimmed,
                descripcion: descripcion.trim(),
                prioridad,
            });
            setExpediente(data.expediente);
            setEditing(false);
        } catch (err) {
            setFormError(friendlyErrorMessage(err));
        } finally {
            setSaving(false);
        }
    };

    const handleChangeEstado = async (event) => {
        event.preventDefault();
        setEstadoError("");
        if (!nuevoEstado) return;

        setChangingEstado(true);
        try {
            const data = await changeEstado(expediente.id, nuevoEstado);
            setExpediente(data.expediente);
            setNuevoEstado("");
        } catch (err) {
            setEstadoError(friendlyErrorMessage(err));
        } finally {
            setChangingEstado(false);
        }
    };

    // Un expediente creado por el FUT Digital guarda los datos del
    // solicitante (nombres/DNI/teléfono/domicilio/correo) y la
    // fundamentación empaquetados dentro de "descripcion" (ver
    // buildDescripcion en futService.js — no hay columnas propias para eso
    // todavía). Si se puede desarmar ese formato, se muestran los datos por
    // separado en vez de un bloque de texto plano; si no (un expediente
    // creado por el personal interno, con descripción libre), se muestra
    // tal cual.
    const datosFut = expediente ? parseDatosSolicitante(expediente.descripcion) : null;

    return (
        <RoleLayout title="Detalle del expediente">
            <Link to="/expedientes" className="dp-back-link">
                ← Volver a expedientes
            </Link>

            {loading && <p className="dp-table-empty">Cargando expediente...</p>}
            {!loading && error && <p className="dp-form-error">{error}</p>}

            {!loading && !error && expediente && (
                <div className="dp-detail-grid">
                    <section className="dp-panel dp-detail-panel">
                        <div className="dp-detail-header">
                            <div>
                                <p className="dp-detail-codigo">{expediente.codigo}</p>
                                <h2 className="dp-panel-title dp-detail-asunto">{expediente.asunto}</h2>
                            </div>
                            <StatusBadge status={expediente.estado} />
                        </div>

                        {!editing ? (
                            <dl className="dp-detail-list">
                                <div>
                                    <dt>Tipo</dt>
                                    <dd>{expediente.tipo}</dd>
                                </div>
                                {canViewDerivaciones && (
                                    <div>
                                        <dt>Área actual</dt>
                                        <dd>{roleLabel(expediente.area_actual)}</dd>
                                    </div>
                                )}
                                {datosFut ? (
                                    <>
                                        <div>
                                            <dt>Nombres y apellidos</dt>
                                            <dd>{datosFut.nombres}</dd>
                                        </div>
                                        <div>
                                            <dt>DNI</dt>
                                            <dd>{datosFut.dni || "—"}</dd>
                                        </div>
                                        {datosFut.telefono && (
                                            <div>
                                                <dt>Teléfono</dt>
                                                <dd>{datosFut.telefono}</dd>
                                            </div>
                                        )}
                                        {(datosFut.domicilio || datosFut.distrito) && (
                                            <div>
                                                <dt>Domicilio</dt>
                                                <dd>
                                                    {datosFut.domicilio || "—"}
                                                    {datosFut.distrito ? ` — ${datosFut.distrito}` : ""}
                                                </dd>
                                            </div>
                                        )}
                                        {datosFut.correo && (
                                            <div>
                                                <dt>Correo</dt>
                                                <dd>{datosFut.correo}</dd>
                                            </div>
                                        )}
                                        <div>
                                            <dt>Fundamentación</dt>
                                            <dd>{datosFut.fundamentacion}</dd>
                                        </div>
                                    </>
                                ) : (
                                    <div>
                                        <dt>Descripción</dt>
                                        <dd>{expediente.descripcion || "—"}</dd>
                                    </div>
                                )}
                                <div>
                                    <dt>Solicitante</dt>
                                    <dd>
                                        <SolicitanteNombre
                                            id={expediente.solicitante_id}
                                            usuarios={usuarios}
                                            unavailable={unavailable}
                                        />
                                    </dd>
                                </div>
                                <div>
                                    <dt>Prioridad</dt>
                                    <dd>
                                        {expediente.prioridad === "URGENTE" ? (
                                            <span className="dp-priority-urgente">Urgente</span>
                                        ) : (
                                            "Normal"
                                        )}
                                    </dd>
                                </div>
                                <div>
                                    <dt>Fecha de registro</dt>
                                    <dd>{formatDateTime(expediente.fecha_registro)}</dd>
                                </div>
                                <div>
                                    <dt>Última actualización</dt>
                                    <dd>{formatDateTime(expediente.fecha_actualizacion)}</dd>
                                </div>
                            </dl>
                        ) : (
                            <form className="dp-form" onSubmit={handleSave}>
                                <div className="dp-form-group">
                                    <label htmlFor="edit-asunto">Asunto *</label>
                                    <input
                                        id="edit-asunto"
                                        value={asunto}
                                        onChange={(event) => setAsunto(event.target.value)}
                                        maxLength={255}
                                        disabled={saving}
                                    />
                                </div>
                                <div className="dp-form-group">
                                    <label htmlFor="edit-descripcion">Descripción</label>
                                    <textarea
                                        id="edit-descripcion"
                                        value={descripcion}
                                        onChange={(event) => setDescripcion(event.target.value)}
                                        rows={3}
                                        disabled={saving}
                                    />
                                </div>
                                <div className="dp-form-group">
                                    <label htmlFor="edit-prioridad">Prioridad</label>
                                    <select
                                        id="edit-prioridad"
                                        value={prioridad}
                                        onChange={(event) => setPrioridad(event.target.value)}
                                        disabled={saving}
                                    >
                                        {PRIORIDADES.map((item) => (
                                            <option key={item.value} value={item.value}>
                                                {item.label}
                                            </option>
                                        ))}
                                    </select>
                                </div>
                                {formError && <p className="dp-form-error">{formError}</p>}
                                <div className="dp-form-actions">
                                    <button
                                        type="button"
                                        className="dp-btn-secondary"
                                        onClick={() => setEditing(false)}
                                        disabled={saving}
                                    >
                                        Cancelar
                                    </button>
                                    <button type="submit" className="dp-btn-primary" disabled={saving}>
                                        {saving ? "Guardando..." : "Guardar cambios"}
                                    </button>
                                </div>
                            </form>
                        )}

                        {!editing && canUpdate && (
                            <button
                                type="button"
                                className="dp-btn-secondary dp-detail-edit-btn"
                                onClick={() => setEditing(true)}
                            >
                                Editar
                            </button>
                        )}
                    </section>

                    {canChangeEstado && (
                        <section className="dp-panel">
                            <h2 className="dp-panel-title">Cambiar estado</h2>
                            <form className="dp-form" onSubmit={handleChangeEstado}>
                                <div className="dp-form-group">
                                    <label htmlFor="nuevo-estado">Nuevo estado</label>
                                    <select
                                        id="nuevo-estado"
                                        value={nuevoEstado}
                                        onChange={(event) => setNuevoEstado(event.target.value)}
                                        disabled={changingEstado}
                                    >
                                        <option value="">Selecciona un estado</option>
                                        {ESTADOS.filter((item) => item.value !== expediente.estado).map((item) => (
                                            <option key={item.value} value={item.value}>
                                                {item.label}
                                            </option>
                                        ))}
                                    </select>
                                </div>
                                {estadoError && <p className="dp-form-error">{estadoError}</p>}
                                <div className="dp-form-actions">
                                    <button
                                        type="submit"
                                        className="dp-btn-primary"
                                        disabled={changingEstado || !nuevoEstado}
                                    >
                                        {changingEstado ? "Cambiando..." : "Cambiar estado"}
                                    </button>
                                </div>
                            </form>
                        </section>
                    )}

                    {canViewDerivaciones && (
                        <div className="dp-detail-full-width">
                            <DerivacionesPanel expedienteId={expediente.id} />
                        </div>
                    )}

                    <div className="dp-detail-full-width">
                        <DocumentosPanel expedienteId={expediente.id} expediente={expediente} />
                    </div>
                </div>
            )}
        </RoleLayout>
    );
}

export default ExpedienteDetail;
