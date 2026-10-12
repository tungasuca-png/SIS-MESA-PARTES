import { useState } from "react";
import Icon from "../dashboard/Icon";
import { changeEstadoEstudiante } from "../../services/estudiantesService";
import { estudianteError } from "../../utils/estudiantesErrors";
import "../dashboard/forms.css";

const LABEL = { ACTIVO: "Activo", INACTIVO: "Inactivo" };

// Cambio de estado del REGISTRO (FASE E-06, PATCH /api/estudiantes/:id/estado):
// solo ACTIVO <-> INACTIVO, con confirmación explícita y expected_version.
// Inactivar no borra al estudiante (no existe borrado físico).
function CambiarEstadoModal({ estudiante, onClose, onChanged, onReload }) {
    const destino = estudiante.estado === "ACTIVO" ? "INACTIVO" : "ACTIVO";
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState(null);

    const close = () => {
        if (!saving) onClose();
    };

    const confirmar = async () => {
        setSaving(true);
        setError(null);
        try {
            const data = await changeEstadoEstudiante(estudiante.id, {
                estado: destino,
                expectedVersion: estudiante.version,
            });
            onChanged(data.estudiante);
        } catch (err) {
            setError(estudianteError(err));
        } finally {
            setSaving(false);
        }
    };

    const nombre = [estudiante.nombres, estudiante.apellidos].filter(Boolean).join(" ") || estudiante.nombre_completo;

    return (
        <div className="dp-modal-overlay" onClick={close}>
            <div
                className="dp-modal est-modal est-modal--sm"
                role="dialog"
                aria-modal="true"
                aria-label="Cambiar estado"
                onClick={(event) => event.stopPropagation()}
            >
                <div className="dp-modal-header">
                    <h2>Cambiar estado</h2>
                    <button type="button" className="dp-modal-close" onClick={close} aria-label="Cerrar" disabled={saving}>
                        <Icon name="close" size={18} />
                    </button>
                </div>
                <div className="dp-form">
                    <p>
                        ¿Cambiar el estado de <strong>{nombre}</strong> de{" "}
                        <strong>{LABEL[estudiante.estado] ?? estudiante.estado}</strong> a <strong>{LABEL[destino]}</strong>?
                    </p>
                    <p className="est-form-note">
                        {destino === "INACTIVO"
                            ? "El registro no se borra: queda inactivo y se puede reactivar después."
                            : "El registro volverá a estar activo."}
                    </p>
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
                        <button type="button" className="dp-btn-primary" onClick={confirmar} disabled={saving}>
                            {saving ? "Guardando..." : `Cambiar a ${LABEL[destino]}`}
                        </button>
                    </div>
                </div>
            </div>
        </div>
    );
}

export default CambiarEstadoModal;
