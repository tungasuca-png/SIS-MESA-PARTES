import { useState } from "react";
import Icon from "../dashboard/Icon";
import { ESTADOS_ESTUDIANTE, SEXOS, TIPOS_DOCUMENTO } from "../../constants/estudiantes";
import { createEstudiante, updateEstudiante } from "../../services/estudiantesService";
import { usePermissions } from "../../hooks/usePermissions";
import {
    EMPTY_ESTUDIANTE_FORM,
    apellidosResultantes,
    buildCreatePayload,
    buildUpdatePatch,
    cleanText,
    formFromEstudiante,
    validarEstudiante,
} from "../../utils/estudianteForm";
import { estudianteError } from "../../utils/estudiantesErrors";
import "../dashboard/forms.css";

// Alta y edición del maestro (FASE E-06, backend FASE E-05).
//   - "create": POST /api/estudiantes con los campos del maestro.
//   - "edit": PATCH /api/estudiantes/:id solo con lo que cambió +
//     expected_version (la versión recibida del backend). Si otro usuario
//     guardó antes, el backend responde version_conflict: se avisa y se
//     ofrece recargar; NUNCA se reintenta automáticamente.
// Ninguno de los dos toca código (en edición), estado (acción propia) ni
// nivel/grado/sección/turno/estado de matrícula (dominio Matrícula).
function EstudianteFormModal({ mode = "create", estudiante, onClose, onSaved, onReload }) {
    const isEdit = mode === "edit";
    // FASE E-08: los datos personales en el alta requieren además
    // estudiantes.personal.update. Sin ese permiso ni se muestran ni se envían.
    const { can } = usePermissions();
    const conPersonales = !isEdit && can("estudiantes.personal.update");
    const [form, setForm] = useState(() => (isEdit ? formFromEstudiante(estudiante) : EMPTY_ESTUDIANTE_FORM));
    const [errors, setErrors] = useState({});
    const [submitError, setSubmitError] = useState(null);
    const [info, setInfo] = useState("");
    const [saving, setSaving] = useState(false);

    const conPaterno = Boolean(cleanText(form.apellido_paterno));
    const sinDocumento = form.tipo_documento === "SIN_DOCUMENTO";

    const update = (field) => (event) => {
        const value = event.target.value;
        setForm((current) => {
            const next = { ...current, [field]: value };
            // Sin documento no lleva número: el campo se vacía y se bloquea.
            if (field === "tipo_documento" && value === "SIN_DOCUMENTO") next.numero_documento = "";
            return next;
        });
        setErrors((current) => ({ ...current, [field]: "" }));
        setSubmitError(null);
        setInfo("");
    };

    const close = () => {
        if (!saving) onClose();
    };

    const handleSubmit = async (event) => {
        event.preventDefault();
        const found = validarEstudiante(form, mode, { personal: conPersonales });
        setErrors(found);
        if (Object.keys(found).length > 0) return;

        let request;
        if (isEdit) {
            const { changed, body } = buildUpdatePatch(estudiante, form);
            if (!changed) {
                setInfo("No hay cambios para guardar.");
                return;
            }
            request = () => updateEstudiante(estudiante.id, body);
        } else {
            request = () => createEstudiante(buildCreatePayload(form, { personal: conPersonales }));
        }

        setSaving(true);
        setSubmitError(null);
        try {
            const data = await request();
            onSaved(data.estudiante);
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

    const title = isEdit ? "Editar estudiante" : "Nuevo estudiante";

    return (
        <div className="dp-modal-overlay" onClick={close}>
            <div
                className="dp-modal est-modal"
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

                <form className="dp-form" onSubmit={handleSubmit} noValidate>
                    <h3 className="est-form-section">Identificación</h3>
                    {isEdit ? (
                        <p className="est-form-note">
                            Código: <strong>{estudiante.codigo}</strong> (no se modifica)
                        </p>
                    ) : (
                        field("est-codigo", "Código *", "codigo", { maxLength: 20 })
                    )}
                    <div className="est-form-row">
                        {field("est-siagie", "Código SIAGIE", "codigo_siagie", { inputMode: "numeric", maxLength: 14 })}
                        <div className="dp-form-group">
                            <label htmlFor="est-tipo-doc">Tipo de documento</label>
                            <select id="est-tipo-doc" value={form.tipo_documento} onChange={update("tipo_documento")} disabled={saving}>
                                {TIPOS_DOCUMENTO.map((item) => (
                                    <option key={item.value} value={item.value}>
                                        {item.label}
                                    </option>
                                ))}
                            </select>
                            {errors.tipo_documento && <p className="dp-form-error">{errors.tipo_documento}</p>}
                        </div>
                        {field("est-num-doc", "Número de documento", "numero_documento", {
                            maxLength: 12,
                            disabled: saving || sinDocumento,
                        })}
                    </div>

                    <h3 className="est-form-section">Nombres y apellidos</h3>
                    {field("est-nombres", "Nombres *", "nombres", { maxLength: 100 })}
                    <div className="est-form-row">
                        {field("est-paterno", "Apellido paterno", "apellido_paterno", { maxLength: 60 })}
                        {field("est-materno", "Apellido materno", "apellido_materno", { maxLength: 60 })}
                    </div>
                    {conPaterno ? (
                        <p className="est-form-note">
                            Apellidos: <strong>{apellidosResultantes(form)}</strong> (se forman con el paterno y el materno)
                        </p>
                    ) : (
                        field("est-apellidos", "Apellidos * (si no se registran por separado)", "apellidos", { maxLength: 100 })
                    )}

                    {isEdit ? (
                        <p className="est-form-note">
                            Los datos personales se editan en su propia sección del detalle. Nivel, grado, sección, turno
                            y estado de matrícula corresponden a Matrícula. El estado del registro tiene su propia acción.
                        </p>
                    ) : (
                        <>
                            {conPersonales && (
                                <>
                                    <h3 className="est-form-section">Datos personales</h3>
                                    <div className="est-form-row">
                                        {field("est-fecha", "Fecha de nacimiento", "fecha_nacimiento", { type: "date" })}
                                        <div className="dp-form-group">
                                            <label htmlFor="est-sexo">Sexo</label>
                                            <select id="est-sexo" value={form.sexo} onChange={update("sexo")} disabled={saving}>
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
                                        {field("est-pais", "País de nacimiento (código)", "pais_nacimiento", { maxLength: 2, placeholder: "PE" })}
                                        {field("est-ubigeo", "Ubigeo de nacimiento", "ubigeo_nacimiento", { inputMode: "numeric", maxLength: 6 })}
                                    </div>
                                    <div className="est-form-row">
                                        {field("est-lengua", "Lengua materna (código)", "lengua_materna", { maxLength: 10 })}
                                        {field("est-lengua2", "Segunda lengua (código)", "segunda_lengua", { maxLength: 10 })}
                                    </div>
                                </>
                            )}
                            <h3 className="est-form-section">Otros datos</h3>
                            {field("est-foto", "Referencia de foto", "foto_ref", { maxLength: 200 })}
                            <div className="dp-form-group">
                                <label htmlFor="est-estado">Estado del registro</label>
                                <select id="est-estado" value={form.estado} onChange={update("estado")} disabled={saving}>
                                    {ESTADOS_ESTUDIANTE.map((item) => (
                                        <option key={item.value} value={item.value}>
                                            {item.label}
                                        </option>
                                    ))}
                                </select>
                                {errors.estado && <p className="dp-form-error">{errors.estado}</p>}
                            </div>
                            <p className="est-form-note">
                                Nivel, grado, sección y turno se asignarán en Matrícula (todavía no disponible).
                            </p>
                        </>
                    )}

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
                            {saving ? "Guardando..." : isEdit ? "Guardar cambios" : "Registrar estudiante"}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    );
}

export default EstudianteFormModal;
