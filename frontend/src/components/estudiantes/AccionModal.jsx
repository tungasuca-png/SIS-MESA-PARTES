import { useState } from "react";
import Icon from "../dashboard/Icon";
import { estudianteError } from "../../utils/estudiantesErrors";
import "../dashboard/forms.css";

// Modal de una operación del maestro (mismo patrón que CambiarEstadoModal):
// el formulario lo pone quien lo usa. onSubmit hace la llamada real a la API;
// si falla, se muestra el mensaje traducido y NO se cierra (nada se da por
// guardado). validate() devuelve un texto de error local o null. Ante un
// version_conflict se ofrece recargar en vez de reintentar a ciegas.
function AccionModal({ title, submitLabel, onClose, onSubmit, validate, onReload, children, size = "sm", danger = false }) {
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState(null);

    const close = () => {
        if (!saving) onClose();
    };

    const submit = async (event) => {
        event.preventDefault();
        const local = validate ? validate() : null;
        if (local) {
            setError({ message: local, code: null });
            return;
        }
        setSaving(true);
        setError(null);
        try {
            await onSubmit();
        } catch (err) {
            setError(estudianteError(err));
            setSaving(false);
        }
    };

    return (
        <div className="dp-modal-overlay" onClick={close}>
            <div
                className={`dp-modal est-modal${size === "sm" ? " est-modal--sm" : ""}`}
                role="dialog"
                aria-modal="true"
                aria-label={title}
                onClick={(event) => event.stopPropagation()}
            >
                <div className="dp-modal-header">
                    <h2>{title}</h2>
                    <button type="button" className="dp-modal-close" onClick={close} aria-label="Cerrar" disabled={saving}>
                        <Icon name="close" size={18} />
                    </button>
                </div>
                <form className="dp-form" onSubmit={submit} noValidate>
                    {children}
                    {error && (
                        <div className="est-submit-error" role="alert">
                            <p className="dp-form-error">{error.message}</p>
                            {error.code === "version_conflict" && onReload && (
                                <button type="button" className="dp-btn-secondary" onClick={onReload}>
                                    Recargar datos
                                </button>
                            )}
                        </div>
                    )}
                    <div className="dp-form-actions">
                        <button type="button" className="dp-btn-secondary" onClick={close} disabled={saving}>
                            Cancelar
                        </button>
                        <button type="submit" className={danger ? "dp-btn-primary est-btn-danger" : "dp-btn-primary"} disabled={saving}>
                            {saving ? "Guardando..." : submitLabel}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    );
}

export default AccionModal;
