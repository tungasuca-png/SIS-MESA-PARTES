import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import RoleLayout from "../../components/portal/RoleLayout";
import StatusBadge from "../../components/dashboard/StatusBadge";
import SolicitanteNombre from "../../components/dashboard/SolicitanteNombre";
import DocumentosPanel from "../../components/dashboard/DocumentosPanel";
import DerivacionesPanel from "../../components/dashboard/DerivacionesPanel";
import { useAuth } from "../../hooks/useAuth";
import { usePermissions } from "../../hooks/usePermissions";
import { useUsuariosBasic } from "../../hooks/useUsuariosBasic";
import {
    changeEstado,
    corregirExpediente,
    getExpediente,
    rechazarExpediente,
    resolverExpediente,
    updateExpediente,
} from "../../services/expedientesService";
import { parseDatosSolicitante } from "../../services/futService";
import { roleLabel } from "../../constants/roles";
import { friendlyErrorMessage } from "../../utils/apiErrors";
import { formatDateTime } from "../../utils/format";
import { ESTADOS, PRIORIDADES, TRANSICIONES_ESTADO, TIPOS_F2, TIPOS_F4, tipoLabel } from "../../constants/expedientes";
import "../../components/dashboard/forms.css";
import "./expedienteDetail.css";

function ExpedienteDetail() {
    const { id } = useParams();
    const { user } = useAuth();
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

    const [motivoRechazo, setMotivoRechazo] = useState("");
    const [rechazando, setRechazando] = useState(false);
    const [rechazoError, setRechazoError] = useState("");

    const [resolviendo, setResolviendo] = useState(false);
    const [resolverError, setResolverError] = useState("");

    const [descripcionCorregida, setDescripcionCorregida] = useState("");
    const [corrigiendo, setCorrigiendo] = useState(false);
    const [corregirError, setCorregirError] = useState("");

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

    // Etapa 4: cierre real de F2/F4 (ver docs/etapa-4-resolucion-cierre-f2-f4.md).
    // El backend vuelve a validar todo (tipo, área, estado, dueño) — estas
    // condiciones solo evitan ofrecer un botón que el backend va a
    // rechazar seguro; ocultar un botón no es la seguridad real.
    const handleRechazar = async (event) => {
        event.preventDefault();
        setRechazoError("");
        if (!motivoRechazo.trim()) {
            setRechazoError("El motivo es obligatorio.");
            return;
        }
        setRechazando(true);
        try {
            const data = await rechazarExpediente(expediente.id, motivoRechazo.trim());
            setExpediente(data.expediente);
            setMotivoRechazo("");
        } catch (err) {
            setRechazoError(friendlyErrorMessage(err));
        } finally {
            setRechazando(false);
        }
    };

    const handleResolver = async () => {
        setResolverError("");
        setResolviendo(true);
        try {
            const data = await resolverExpediente(expediente.id);
            setExpediente(data.expediente);
        } catch (err) {
            setResolverError(friendlyErrorMessage(err));
        } finally {
            setResolviendo(false);
        }
    };

    const handleCorregir = async (event) => {
        event.preventDefault();
        setCorregirError("");
        if (!descripcionCorregida.trim()) {
            setCorregirError("La descripción corregida es obligatoria.");
            return;
        }
        setCorrigiendo(true);
        try {
            const data = await corregirExpediente(expediente.id, descripcionCorregida.trim());
            setExpediente(data.expediente);
            setDescripcionCorregida("");
        } catch (err) {
            setCorregirError(friendlyErrorMessage(err));
        } finally {
            setCorrigiendo(false);
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

    // Solo se ofrecen las transiciones que el backend realmente admite
    // desde el estado actual (ver estados.go) — antes se mostraban las 3
    // opciones restantes sin importar cuál, y el backend rechazaba las
    // inválidas (por ejemplo, "Pendiente" directo a "Atendido"), lo que se
    // sentía como que no se podía cambiar el estado sin decir por qué.
    const estadosDisponibles = expediente
        ? ESTADOS.filter((item) => (TRANSICIONES_ESTADO[expediente.estado] || []).includes(item.value))
        : [];

    // Rechazar: exclusivo de F4, lo ejecuta quien tiene el expediente
    // (Dirección o Subdirección) mientras está en trámite.
    const puedeRechazar =
        expediente &&
        TIPOS_F4.includes(expediente.tipo) &&
        expediente.estado === "EN_PROCESO" &&
        (user?.role === "DIRECTOR" || user?.role === "SUBDIRECTOR") &&
        expediente.area_actual === user?.role;

    // Resolver/cerrar: exclusivo de F2, solo Secretaría y solo cuando ya lo
    // tiene de vuelta (Dirección ya lo procesó y devolvió).
    const puedeResolver =
        expediente &&
        TIPOS_F2.includes(expediente.tipo) &&
        expediente.estado === "EN_PROCESO" &&
        expediente.area_actual === "SECRETARIA" &&
        user?.role === "SECRETARIA";

    // Corregir y reenviar: exclusivo del propio solicitante, sobre su
    // propio expediente observado.
    const puedeCorregir =
        expediente &&
        expediente.estado === "OBSERVADO" &&
        user?.role === "SOLICITANTE" &&
        expediente.solicitante_id === user?.id;

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
                                    <dd>{tipoLabel(expediente.tipo)}</dd>
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
                            {estadosDisponibles.length === 0 ? (
                                <p className="dp-table-empty">
                                    Este expediente ya está en un estado final ({expediente.estado.replace("_", " ")})
                                    y no admite más cambios.
                                </p>
                            ) : (
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
                                        {estadosDisponibles.map((item) => (
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
                            )}
                        </section>
                    )}

                    {puedeRechazar && (
                        <section className="dp-panel">
                            <h2 className="dp-panel-title">Rechazar (devolver para corrección)</h2>
                            <p className="dp-table-empty" style={{ textAlign: "left", padding: 0, marginBottom: 12 }}>
                                El expediente vuelve a Secretaría con estado Observado; el solicitante podrá
                                corregir y volver a presentarlo.
                            </p>
                            <form className="dp-form" onSubmit={handleRechazar}>
                                <div className="dp-form-group">
                                    <label htmlFor="motivo-rechazo">Motivo</label>
                                    <textarea
                                        id="motivo-rechazo"
                                        value={motivoRechazo}
                                        onChange={(event) => setMotivoRechazo(event.target.value)}
                                        rows={2}
                                        maxLength={500}
                                        disabled={rechazando}
                                    />
                                </div>
                                {rechazoError && <p className="dp-form-error">{rechazoError}</p>}
                                <div className="dp-form-actions">
                                    <button type="submit" className="dp-btn-secondary" disabled={rechazando}>
                                        {rechazando ? "Rechazando..." : "Rechazar y devolver"}
                                    </button>
                                </div>
                            </form>
                        </section>
                    )}

                    {puedeResolver && (
                        <section className="dp-panel">
                            <h2 className="dp-panel-title">Cerrar expediente</h2>
                            <p className="dp-table-empty" style={{ textAlign: "left", padding: 0, marginBottom: 12 }}>
                                Requiere que el documento final ya esté cargado en Documentos (tipo "Proveído")
                                — verifica el panel de Documentos más abajo.
                            </p>
                            {resolverError && <p className="dp-form-error">{resolverError}</p>}
                            <div className="dp-form-actions">
                                <button
                                    type="button"
                                    className="dp-btn-primary"
                                    onClick={handleResolver}
                                    disabled={resolviendo}
                                >
                                    {resolviendo ? "Cerrando..." : "Marcar como atendido"}
                                </button>
                            </div>
                        </section>
                    )}

                    {puedeCorregir && (
                        <section className="dp-panel">
                            <h2 className="dp-panel-title">Corregir y reenviar</h2>
                            <form className="dp-form" onSubmit={handleCorregir}>
                                <div className="dp-form-group">
                                    <label htmlFor="descripcion-corregida">Descripción corregida</label>
                                    <textarea
                                        id="descripcion-corregida"
                                        value={descripcionCorregida}
                                        onChange={(event) => setDescripcionCorregida(event.target.value)}
                                        rows={4}
                                        placeholder={expediente.descripcion}
                                        disabled={corrigiendo}
                                    />
                                </div>
                                {corregirError && <p className="dp-form-error">{corregirError}</p>}
                                <div className="dp-form-actions">
                                    <button type="submit" className="dp-btn-primary" disabled={corrigiendo}>
                                        {corrigiendo ? "Enviando..." : "Corregir y volver a presentar"}
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
