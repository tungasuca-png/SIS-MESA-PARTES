import { useState } from "react";
import { openExpedientePsicologico } from "../../services/psicologiaService";
import { friendlyErrorMessage } from "../../utils/apiErrors";
import ResponsablePsicologoField from "./ResponsablePsicologoField";
import EstudianteSelector from "./EstudianteSelector";
import Icon from "./Icon";
import "./forms.css";

// FASE 5D: el estudiante se busca y selecciona por DNI/nombre/apellido
// (EstudianteSelector, real contra Estudiantes Service) — ya no se pide su
// UUID a mano. El backend (Psicología Service, vía EstudianteVerifier)
// sigue siendo quien valida que el estudiante exista de verdad; el frontend
// no confía únicamente en que el usuario eligió un resultado.
// initialEstudiante (opcional): estudiante ya elegido en "Buscar alumno".
function PsicologiaExpedienteFormModal({ onClose, onCreated, initialEstudiante = null }) {
    const [estudiante, setEstudiante] = useState(initialEstudiante);
    const [responsableId, setResponsableId] = useState("");
    const [resumenCaso, setResumenCaso] = useState("");
    const [error, setError] = useState("");
    const [submitting, setSubmitting] = useState(false);

    const handleSubmit = async (event) => {
        event.preventDefault();
        setError("");

        if (!estudiante) {
            setError("Debes buscar y seleccionar un estudiante.");
            return;
        }

        setSubmitting(true);
        try {
            const data = await openExpedientePsicologico(estudiante.id, {
                responsableId: responsableId || undefined,
                resumenCaso: resumenCaso.trim() || undefined,
            });
            onCreated(data.expediente);
        } catch (err) {
            setError(
                friendlyErrorMessage(
                    err,
                    "El estudiante seleccionado ya no está disponible. Busca de nuevo."
                )
            );
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
                aria-label="Nuevo expediente psicológico"
                onClick={(event) => event.stopPropagation()}
            >
                <div className="dp-modal-header">
                    <h2>Nuevo expediente psicológico</h2>
                    <button type="button" className="dp-modal-close" onClick={onClose} aria-label="Cerrar">
                        <Icon name="close" size={18} />
                    </button>
                </div>

                <form className="dp-form" onSubmit={handleSubmit}>
                    <EstudianteSelector
                        id="psi-estudiante"
                        value={estudiante}
                        onChange={setEstudiante}
                        disabled={submitting}
                    />

                    <ResponsablePsicologoField
                        id="psi-responsable"
                        value={responsableId}
                        onChange={setResponsableId}
                        disabled={submitting}
                    />

                    <div className="dp-form-group">
                        <label htmlFor="psi-resumen">Resumen del caso</label>
                        <textarea
                            id="psi-resumen"
                            value={resumenCaso}
                            onChange={(event) => setResumenCaso(event.target.value)}
                            maxLength={10000}
                            rows={4}
                            disabled={submitting}
                            placeholder="Información clínica del caso (opcional)"
                        />
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

export default PsicologiaExpedienteFormModal;
