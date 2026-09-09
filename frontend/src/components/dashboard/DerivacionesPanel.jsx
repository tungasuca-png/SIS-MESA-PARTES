import { useEffect, useState } from "react";
import { usePermissions } from "../../hooks/usePermissions";
import { crearDerivacion, getDerivacionesByExpediente } from "../../services/derivacionesService";
import { friendlyErrorMessage } from "../../utils/apiErrors";
import { formatDateTime } from "../../utils/format";
import { TIPOS_DERIVACION } from "../../constants/derivaciones";
import Icon from "./Icon";
import "./forms.css";
import "./documentosPanel.css";

function tipoLabel(value) {
    return TIPOS_DERIVACION.find((item) => item.value === value)?.label ?? value;
}

const CAMPOS_INICIALES = { tipo: "DERIVACION", origen: "", destino: "", motivo: "", condicion: "" };

function DerivacionesPanel({ expedienteId }) {
    const { can } = usePermissions();
    const canCreate = can("derivaciones.create");

    const [derivaciones, setDerivaciones] = useState([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");

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
        setCampos((current) => ({ ...current, [campo]: event.target.value }));
    };

    const handleCrear = async (event) => {
        event.preventDefault();
        setCreateError("");

        if (!campos.origen.trim() || !campos.destino.trim() || !campos.motivo.trim()) {
            setCreateError("Origen, destino y motivo son obligatorios.");
            return;
        }

        setCreando(true);
        try {
            await crearDerivacion(expedienteId, campos);
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
                        <input
                            id="deriv-origen"
                            value={campos.origen}
                            onChange={handleChange("origen")}
                            placeholder="Ej: Secretaría"
                            maxLength={100}
                            disabled={creando}
                        />
                    </div>
                    <div className="dp-form-group">
                        <label htmlFor="deriv-destino">Destino</label>
                        <input
                            id="deriv-destino"
                            value={campos.destino}
                            onChange={handleChange("destino")}
                            placeholder="Ej: Dirección"
                            maxLength={100}
                            disabled={creando}
                        />
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
        </section>
    );
}

export default DerivacionesPanel;
