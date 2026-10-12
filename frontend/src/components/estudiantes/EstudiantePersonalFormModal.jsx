import { useState } from "react";
import Icon from "../dashboard/Icon";
import { SEXOS } from "../../constants/estudiantes";
import { updateEstudiantePersonal } from "../../services/estudiantesService";
import { buildPersonalPatch, formFromPersonal, validarPersonal } from "../../utils/estudianteForm";
import { estudianteError } from "../../utils/estudiantesErrors";
import "../dashboard/forms.css";

// Edición de DATOS PERSONALES (FASE E-08): PATCH /api/estudiantes/:id/personal
// solo con los campos cambiados + expected_version. Nunca envía identidad,
// estado, foto ni datos académicos. En version_conflict avisa y ofrece
// recargar; no reintenta.
function EstudiantePersonalFormModal({ estudianteId, personal, expectedVersion, onClose, onSaved, onReload }) {
    const [form, setForm] = useState(() => formFromPersonal(personal));
    const [errors, setErrors] = useState({});
    const [submitError, setSubmitError] = useState(null);
    const [info, setInfo] = useState("");
    const [saving, setSaving] = useState(false);

    const update = (key) => (event) => {
        setForm((current) => ({ ...current, [key]: event.target.value }));
        setErrors((current) => ({ ...current, [key]: "" }));
        setSubmitError(null);
        setInfo("");
    };

    const close = () => {
        if (!saving) onClose();
    };

    const handleSubmit = async (event) => {
        event.preventDefault();
        const found = validarPersonal(form);
        setErrors(found);
        if (Object.keys(found).length > 0) return;

        const { changed, body, normalizados } = buildPersonalPatch(personal, form, expectedVersion);
        if (!changed) {
            setInfo("No hay cambios para guardar.");
            return;
        }
        setSaving(true);
        setSubmitError(null);
        try {
            const data = await updateEstudiantePersonal(estudianteId, body);
            onSaved(data.estudiante, normalizados);
        } catch (err) {
            setSubmitError(estudianteError(err));
        } finally {
            setSaving(false);
        }
    };

    const field = (id, label, key, props = {}) => (
        <div className="dp-form-group">
            <label htmlFor={id}>{label}</label>
            <input id={id} type="text" value={form[key]} onChange={update(key)} disabled={saving} autoComplete="off" {...props} />
            {errors[key] && <p className="dp-form-error">{errors[key]}</p>}
        </div>
    );

    return (
        <div className="dp-modal-overlay" onClick={close}>
            <div
                className="dp-modal est-modal"
                role="dialog"
                aria-modal="true"
                aria-label="Editar datos personales"
                onClick={(event) => event.stopPropagation()}
            >
                <div className="dp-modal-header">
                    <h2>Editar datos personales</h2>
                    <button type="button" className="dp-modal-close" onClick={close} aria-label="Cerrar" disabled={saving}>
                        <Icon name="close" size={18} />
                    </button>
                </div>
                <form className="dp-form" onSubmit={handleSubmit} noValidate>
                    <div className="est-form-row">
                        {field("per-fecha", "Fecha de nacimiento", "fecha_nacimiento", { type: "date" })}
                        <div className="dp-form-group">
                            <label htmlFor="per-sexo">Sexo</label>
                            <select id="per-sexo" value={form.sexo} onChange={update("sexo")} disabled={saving}>
                                <option value="">Sin especificar</option>
                                {SEXOS.map((item) => (
                                    <option key={item.value} value={item.value}>
                                        {item.label}
                                    </option>
                                ))}
                            </select>
                            {errors.sexo && <p className="dp-form-error">{errors.sexo}</p>}
                        </div>
                    </div>
                    <div className="est-form-row">
                        {field("per-pais", "País de nacimiento (código)", "pais_nacimiento", { maxLength: 2, placeholder: "PE" })}
                        {field("per-ubigeo", "Ubigeo de nacimiento", "ubigeo_nacimiento", { inputMode: "numeric", maxLength: 6 })}
                    </div>
                    <div className="est-form-row">
                        {field("per-lengua", "Lengua materna (código)", "lengua_materna", { maxLength: 10 })}
                        {field("per-lengua2", "Segunda lengua (código)", "segunda_lengua", { maxLength: 10 })}
                    </div>
                    <h3 className="est-form-section">Domicilio</h3>
                    {field("per-direccion", "Dirección", "domicilio_direccion", { maxLength: 200 })}
                    <div className="est-form-row">
                        {field("per-departamento", "Departamento", "domicilio_departamento", { maxLength: 60 })}
                        {field("per-provincia", "Provincia", "domicilio_provincia", { maxLength: 60 })}
                        {field("per-distrito", "Distrito", "domicilio_distrito", { maxLength: 60 })}
                    </div>
                    {field("per-dom-ubigeo", "Ubigeo del domicilio (si se conoce)", "domicilio_ubigeo", {
                        inputMode: "numeric",
                        maxLength: 6,
                    })}
                    <h3 className="est-form-section">Contacto</h3>
                    <div className="est-form-row">
                        {field("per-correo", "Correo electrónico", "correo", { type: "email", maxLength: 254 })}
                        {field("per-telefono", "Teléfono", "telefono", { inputMode: "tel", maxLength: 20 })}
                    </div>
                    <p className="est-form-note">Deja un campo vacío para quitar el dato registrado.</p>

                    {info && <p className="est-form-note">{info}</p>}
                    {submitError && (
                        <div className="est-submit-error" role="alert">
                            <p className="dp-form-error">{submitError.message}</p>
                            {submitError.code === "version_conflict" && onReload && (
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
                        <button type="submit" className="dp-btn-primary" disabled={saving}>
                            {saving ? "Guardando..." : "Guardar datos personales"}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    );
}

export default EstudiantePersonalFormModal;
