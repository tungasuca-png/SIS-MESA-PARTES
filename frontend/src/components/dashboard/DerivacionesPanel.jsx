import { useEffect, useState } from "react";
import { useAuth } from "../../hooks/useAuth";
import { usePermissions } from "../../hooks/usePermissions";
import { crearDerivacion, getDerivacionesByExpediente } from "../../services/derivacionesService";
import { derivarExpediente } from "../../services/expedientesService";
import { friendlyErrorMessage } from "../../utils/apiErrors";
import { formatDateTime } from "../../utils/format";
import { TIPOS_DERIVACION } from "../../constants/derivaciones";
import { ACTORES_DESTINO, AREAS_DESTINO, roleLabel } from "../../constants/roles";
import Icon from "./Icon";
import "./forms.css";
import "./documentosPanel.css";

function tipoLabel(value) {
    return TIPOS_DERIVACION.find((item) => item.value === value)?.label ?? value;
}

const CAMPOS_INICIALES = { tipo: "DERIVACION", destino: "", motivo: "", condicion: "" };

function DerivacionesPanel({ expedienteId }) {
    const { user } = useAuth();
    const { can } = usePermissions();
    const canCreate = can("derivaciones.create");

    // El origen de una derivación YA NO se elige: siempre es quien está
    // logueado registrándola (su usuario y su rol), no una lista de opciones.
    const origenActual = user ? `${user.username} (${roleLabel(user.role)})` : "";

    const [derivaciones, setDerivaciones] = useState([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");
    // El historial ocupa mucho espacio en el detalle del expediente —
    // queda colapsado por defecto, disponible con un clic en vez de
    // siempre visible.
    const [mostrarHistorial, setMostrarHistorial] = useState(false);

    const [campos, setCampos] = useState(CAMPOS_INICIALES);
    const [creando, setCreando] = useState(false);
    const [createError, setCreateError] = useState("");

    const load = async () => {
        setLoading(true);
        setError("");
        try {
            const data = await getDerivacionesByExpediente(expedienteId);
            setDerivaciones(data.derivaciones || []);
        } catch (err) {
            setError(friendlyErrorMessage(err));
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        // eslint-disable-next-line react-hooks/set-state-in-effect
        load();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [expedienteId]);

    const handleChange = (campo) => (event) => {
        setCampos((current) => ({
            ...current,
            [campo]: event.target.value,
            // El "destino" cambia de vocabulario según el tipo (código de
            // área para DERIVACION, texto libre para el resto — ver el
            // select más abajo) — al cambiar de tipo se limpia para no
            // arrastrar un valor que ya no aplica.
            ...(campo === "tipo" ? { destino: "" } : {}),
        }));
    };

    const handleCrear = async (event) => {
        event.preventDefault();
        setCreateError("");

        if (!campos.destino.trim() || !campos.motivo.trim()) {
            setCreateError("Destino y motivo son obligatorios.");
            return;
        }

        setCreando(true);
        try {
            // Una derivación real (tipo DERIVACION) es la única que cambia
            // quién es responsable del expediente — una notificación,
            // asignación, recepción o aprobación no transfieren esa
            // responsabilidad (ver sección 15 del análisis funcional). Para
            // esa, desde la Etapa 3, se usa una sola operación de negocio
            // (derivarExpediente) en vez de crear la derivación y mover el
            // área por separado: el backend valida la transición y hace
            // ambas cosas de forma atómica-como-se-puede.
            if (campos.tipo === "DERIVACION") {
                await derivarExpediente(expedienteId, {
                    areaDestino: campos.destino,
                    motivo: campos.motivo,
                    condicion: campos.condicion,
                });
            } else {
                await crearDerivacion(expedienteId, { ...campos, origen: origenActual });
            }

            setCampos(CAMPOS_INICIALES);
            await load();
        } catch (err) {
            setCreateError(friendlyErrorMessage(err));
        } finally {
            setCreando(false);
        }
    };

    return (
        <section className="dp-panel">
            <h2 className="dp-panel-title">Derivaciones</h2>

            {canCreate && (
                <form className="dp-form dp-documentos-upload" onSubmit={handleCrear}>
                    <div className="dp-form-group">
                        <label htmlFor="deriv-tipo">Tipo</label>
                        <select id="deriv-tipo" value={campos.tipo} onChange={handleChange("tipo")} disabled={creando}>
                            {TIPOS_DERIVACION.map((item) => (
                                <option key={item.value} value={item.value}>
                                    {item.label}
                                </option>
                            ))}
                        </select>
                    </div>
                    <div className="dp-form-group">
                        <label htmlFor="deriv-origen">Origen</label>
                        <input id="deriv-origen" value={origenActual} readOnly disabled />
                    </div>
                    <div className="dp-form-group">
                        <label htmlFor="deriv-destino">Destino</label>
                        <select id="deriv-destino" value={campos.destino} onChange={handleChange("destino")} disabled={creando}>
                            <option value="">Selecciona...</option>
                            {(campos.tipo === "DERIVACION" ? AREAS_DESTINO : ACTORES_DESTINO).map((item) => (
                                <option key={item.value} value={item.value}>
                                    {item.label}
                                </option>
                            ))}
                        </select>
                    </div>
                    <div className="dp-form-group">
                        <label htmlFor="deriv-motivo">Motivo</label>
                        <textarea
                            id="deriv-motivo"
                            value={campos.motivo}
                            onChange={handleChange("motivo")}
                            rows={2}
                            maxLength={500}
                            disabled={creando}
                        />
                    </div>
                    <div className="dp-form-group">
                        <label htmlFor="deriv-condicion">Condición (opcional)</label>
                        <input
                            id="deriv-condicion"
                            value={campos.condicion}
                            onChange={handleChange("condicion")}
                            placeholder="Ej: Requisitos cumplidos"
                            maxLength={255}
                            disabled={creando}
                        />
                    </div>
                    {createError && <p className="dp-form-error">{createError}</p>}
                    <div className="dp-form-actions">
                        <button type="submit" className="dp-btn-primary" disabled={creando}>
                            {creando ? "Registrando..." : "Registrar derivación"}
                        </button>
                    </div>
                </form>
            )}

            {loading && <p className="dp-table-empty">Cargando derivaciones...</p>}
            {!loading && error && <p className="dp-form-error">{error}</p>}
            {!loading && !error && derivaciones.length === 0 && (
                <p className="dp-table-empty">No hay derivaciones registradas.</p>
            )}

            {!loading && !error && derivaciones.length > 0 && (
                <>
                    <button
                        type="button"
                        className="dp-btn-secondary"
                        onClick={() => setMostrarHistorial((current) => !current)}
                    >
                        {mostrarHistorial
                            ? "Ocultar historial"
                            : `Ver historial (${derivaciones.length})`}
                    </button>

                    {mostrarHistorial && (
                        <ul className="dp-documentos-list">
                            {derivaciones.map((item) => (
                                <li key={item.id} className="dp-documentos-item">
                                    <div className="dp-documentos-info">
                                        <Icon name="share" size={18} />
                                        <div>
                                            <p className="dp-documentos-nombre">
                                                {item.origen} → {item.destino}
                                            </p>
                                            <p className="dp-documentos-meta">
                                                {tipoLabel(item.tipo)} · {item.motivo}
                                                {item.condicion ? ` · ${item.condicion}` : ""} ·{" "}
                                                {formatDateTime(item.fecha_registro)}
                                            </p>
                                        </div>
                                    </div>
                                </li>
                            ))}
                        </ul>
                    )}
                </>
            )}
        </section>
    );
}

export default DerivacionesPanel;
