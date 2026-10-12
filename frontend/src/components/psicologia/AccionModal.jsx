import { useState } from "react";
import Icon from "../dashboard/Icon";
import { MAX_MOTIVO_ATENCION } from "../../constants/psicologia";
import "../dashboard/forms.css";

// Diálogo de confirmación para acciones de atenciones (registrar, descartar,
// anular). Con motivoLabel pide un motivo obligatorio. onConfirm(motivo) es
// async y devuelve un mensaje de error (string) o nada si todo salió bien.
// Mientras se envía, todo queda deshabilitado (sin dobles clics).
function AccionModal({ title, message, motivoLabel, confirmLabel, submittingLabel, danger, onConfirm, onClose }) {
    const [motivo, setMotivo] = useState("");
    const [submitting, setSubmitting] = useState(false);
    const [error, setError] = useState("");

    const close = () => {
        if (!submitting) onClose();
    };

    const handleSubmit = async (event) => {
        event.preventDefault();
        if (submitting) return;
        setError("");
        if (motivoLabel && !motivo.trim()) {
            setError(`${motivoLabel.replace(" *", "")} es obligatorio.`);
            return;
        }
        setSubmitting(true);
        const result = await onConfirm(motivo.trim());
        if (result) {
            setError(result);
            setSubmitting(false);
        }
    };

    return (
        <div className="dp-modal-overlay" onClick={close}>
            <div className="dp-modal" role="dialog" aria-modal="true" aria-label={title} onClick={(event) => event.stopPropagation()}>
                <div className="dp                                                                                                                                                          -modal-header">
                    <h2>{title}</h2>
                    <button type="button" className="dp-modal-close" onClick={close} disabled={submitting} aria-label="Cerrar">
                        <Icon name="close" size={18} />
                    </button>
                </div>
                <form className="dp-form" onSubmit={handleSubmit}>
                    <p className="dp-form-hint" style={{ margin: 0 }}>
                        {message}
                    </p>
                    {motivoLabel && (
                        <div className="dp-form-group">
                            <label htmlFor="psi-accion-motivo">{motivoLabel}</label>
                            <textarea
                                id="psi-accion-motivo"
                                value={motivo}
                                onChange={(event) => setMotivo(event.target.value)}
                                maxLength={MAX_MOTIVO_ATENCION}
                                rows={4}
                                disabled={submitting}
                            />
                        </div>
                    )}
                    {error && <p className="dp-form-error">{error}</p>}
                    <div className="dp-form-actions">
                        <button type="button" className="dp-btn-secondary" onClick={close} disabled={submitting}>
                            Cancelar
                        </button>
                        <button type="submit" className={danger ? "psi-btn-danger" : "dp-btn-primary"} disabled={submitting}>
                            {submitting ? submittingLabel : confirmLabel}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    );
}

export default AccionModal;
