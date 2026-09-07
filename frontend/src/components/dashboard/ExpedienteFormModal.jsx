import { useState } from "react";
import { TIPOS, PRIORIDADES } from "../../constants/expedientes";
import { createExpediente } from "../../services/expedientesService";
import { friendlyErrorMessage } from "../../utils/apiErrors";
import Icon from "./Icon";
import "./forms.css";

function ExpedienteFormModal({ onClose, onCreated }) {
    const [tipo, setTipo] = useState("SOLICITUD");
    const [asunto, setAsunto] = useState("");
    const [descripcion, setDescripcion] = useState("");
    const [prioridad, setPrioridad] = useState("NORMAL");
    const [error, setError] = useState("");
    const [submitting, setSubmitting] = useState(false);

    const handleSubmit = async (event) => {
        event.preventDefault();
        setError("");

        const trimmedAsunto = asunto.trim();
        if (!trimmedAsunto) {
            setError("El asunto es obligatorio.");
            return;
        }
        if (trimmedAsunto.length > 255) {
            setError("El asunto no puede superar los 255 caracteres.");
            return;
        }

        setSubmitting(true);
        try {
            const data = await createExpediente({
                tipo,
                asunto: trimmedAsunto,
                descripcion: descripcion.trim(),
                prioridad,
            });
            onCreated(data.expediente);
        } catch (err) {
            setError(friendlyErrorMessage(err));
        } finally {
            setSubmitting(false);
        }
    };

    return (
        <div className="dp-modal-overlay" onClick={onClose}>
            <div
                className="dp-modal"
                role="dialog"
                aria-modal="true"
                aria-label="Nuevo expediente"
                onClick={(event) => event.stopPropagation()}
            >
                <div className="dp-modal-header">
                    <h2>Nuevo expediente</h2>
                    <button type="button" className="dp-modal-close" onClick={onClose} aria-label="Cerrar">
                        <Icon name="close" size={18} />
                    </button>
                </div>

                <form className="dp-form" onSubmit={handleSubmit}>
                    <div className="dp-form-group">
                        <label htmlFor="exp-tipo">Tipo</label>
                        <select
                            id="exp-tipo"
                            value={tipo}
                            onChange={(event) => setTipo(event.target.value)}
                            disabled={submitting}
                        >
                            {TIPOS.map((item) => (
                                <option key={item.value} value={item.value}>
                                    {item.label}
                                </option>
                            ))}
                        </select>
                    </div>

                    <div className="dp-form-group">
                        <label htmlFor="exp-asunto">Asunto *</label>
                        <input
                            id="exp-asunto"
                            type="text"
                            value={asunto}
                            onChange={(event) => setAsunto(event.target.value)}
                            maxLength={255}
                            disabled={submitting}
                            placeholder="Ej. Solicitud de certificado de estudios"
                        />
                    </div>

                    <div className="dp-form-group">
                        <label htmlFor="exp-descripcion">Descripción</label>
                        <textarea
                            id="exp-descripcion"
                            value={descripcion}
                            onChange={(event) => setDescripcion(event.target.value)}
                            rows={3}
                            disabled={submitting}
                        />
                    </div>

                    <div className="dp-form-group">
                        <label htmlFor="exp-prioridad">Prioridad</label>
                        <select
                            id="exp-prioridad"
                            value={prioridad}
                            onChange={(event) => setPrioridad(event.target.value)}
                            disabled={submitting}
                        >
                            {PRIORIDADES.map((item) => (
                                <option key={item.value} value={item.value}>
                                    {item.label}
                                </option>
                            ))}
                        </select>
                    </div>

                    {error && <p className="dp-form-error">{error}</p>}

                    <div className="dp-form-actions">
                        <button type="button" className="dp-btn-secondary" onClick={onClose} disabled={submitting}>
                            Cancelar
                        </button>
                        <button type="submit" className="dp-btn-primary" disabled={submitting}>
                            {submitting ? "Creando..." : "Crear expediente"}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    );
}

export default ExpedienteFormModal;
