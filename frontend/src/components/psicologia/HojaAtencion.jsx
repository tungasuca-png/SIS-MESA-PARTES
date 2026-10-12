import {
    DESTINOS_DERIVACION,
    formatFechaAtencion,
    MAX_TEXTO_ATENCION,
    MAX_TEXTO_OTROS,
    MOTIVOS_ATENCION,
    REALIZADA_POR,
} from "../../constants/psicologia";
import "../dashboard/forms.css";
import "./hojaAtencion.css";

// Hoja de Atención: misma estructura que el formato institucional (y que el
// PDF generado en Psicología Service). readOnly = versión oficial o anulada.
// etiquetas: {motivos, derivaciones} congeladas en la versión (lectura), para
// mostrar exactamente lo que se registró aunque el catálogo cambie después.
function HojaAtencion({ value, onChange, readOnly = false, disabled = false, etiquetas }) {
    const set = (field, fieldValue) => onChange?.({ ...value, [field]: fieldValue });
    const toggle = (field, code) => {
        const current = value[field] || [];
        set(field, current.includes(code) ? current.filter((c) => c !== code) : [...current, code]);
    };
    const off = readOnly || disabled;

    const checks = (field, options, frozen) => {
        if (readOnly) {
            const selected = value[field] || [];
            return (
                <ul className="psi-hoja-checks psi-hoja-checks--read">
                    {options.map((option) => {
                        const label = frozen?.find((o) => o.codigo === option.value)?.etiqueta ?? option.label;
                        const marcado = selected.includes(option.value);
                        return (
                            <li key={option.value} className={marcado ? "is-checked" : ""}>
                                <span className="psi-hoja-box" aria-hidden="true">{marcado ? "✓" : ""}</span>
                                {label}
                                <span className="psi-sr-only">{marcado ? " (marcado)" : " (no marcado)"}</span>
                            </li>
                        );
                    })}
                </ul>
            );
        }
        return (
            <div className="psi-hoja-checks">
                {options.map((option) => (
                    <label key={option.value} className="psi-hoja-check">
                        <input
                            type="checkbox"
                            checked={(value[field] || []).includes(option.value)}
                            onChange={() => toggle(field, option.value)}
                            disabled={off}
                        />
                        {option.label}
                    </label>
                ))}
            </div>
        );
    };

    const text = (field, label, { rows = 4, max = MAX_TEXTO_ATENCION, placeholder } = {}) => (
        <div className="psi-hoja-field">
            <label htmlFor={`psi-hoja-${field}`} className="psi-hoja-label">
                {label}
            </label>
            {readOnly ? (
                <p id={`psi-hoja-${field}`} className="psi-hoja-text">
                    {value[field] || <span className="psi-hoja-empty">Sin registrar.</span>}
                </p>
            ) : (
                <textarea
                    id={`psi-hoja-${field}`}
                    value={value[field] || ""}
                    onChange={(event) => set(field, event.target.value)}
                    maxLength={max}
                    rows={rows}
                    disabled={off}
                    placeholder={placeholder}
                />
            )}
        </div>
    );

    const otros = (field, label) =>
        readOnly ? (
            value[field] ? (
                <p className="psi-hoja-otros">
                    {label}: {value[field]}
                </p>
            ) : null
        ) : (
            <input
                type="text"
                className="psi-hoja-otros-input"
                aria-label={label}
                placeholder={label}
                value={value[field] || ""}
                onChange={(event) => set(field, event.target.value)}
                maxLength={MAX_TEXTO_OTROS}
                disabled={off}
            />
        );

    return (
        <div className={`psi-hoja dp-form ${readOnly ? "psi-hoja--read" : ""}`}>
            <div className="psi-hoja-row psi-hoja-row--datos">
                <div className="psi-hoja-field">
                    <label htmlFor="psi-hoja-fecha" className="psi-hoja-label">
                        Fecha de atención{!readOnly && " *"}
                    </label>
                    {readOnly ? (
                        <p id="psi-hoja-fecha" className="psi-hoja-text">
                            {formatFechaAtencion(value.fecha_atencion)}
                        </p>
                    ) : (
                        <input
                            id="psi-hoja-fecha"
                            type="date"
                            value={value.fecha_atencion || ""}
                            onChange={(event) => set("fecha_atencion", event.target.value)}
                            disabled={off}
                        />
                    )}
                </div>
                <div className="psi-hoja-field">
                    <label htmlFor="psi-hoja-hora" className="psi-hoja-label">
                        Hora{!readOnly && " *"}
                    </label>
                    {readOnly ? (
                        <p id="psi-hoja-hora" className="psi-hoja-text">
                            {value.hora_atencion || "—"}
                        </p>
                    ) : (
                        <input
                            id="psi-hoja-hora"
                            type="time"
                            value={value.hora_atencion || ""}
                            onChange={(event) => set("hora_atencion", event.target.value)}
                            disabled={off}
                        />
                    )}
                </div>
                <fieldset className="psi-hoja-field psi-hoja-fieldset">
                    <legend className="psi-hoja-label">Atención realizada por</legend>
                    {checks("realizada_por", REALIZADA_POR)}
                    {otros("realizada_por_otros", "Otros")}
                </fieldset>
            </div>

            <div className="psi-hoja-row psi-hoja-row--motivo">
                <fieldset className="psi-hoja-field psi-hoja-fieldset">
                    <legend className="psi-hoja-label">Motivo de la atención</legend>
                    {checks("motivos", MOTIVOS_ATENCION, etiquetas?.motivos)}
                    {otros("motivo_otros", "Otros motivos")}
                </fieldset>
                {text("descripcion_motivo", "Descripción del motivo", { rows: 6 })}
            </div>

            {text("analisis_integral", "Análisis integral de la situación", { rows: 6 })}

            <div className="psi-hoja-row psi-hoja-row--full">
                {text("estrategias", "Estrategias / acciones realizadas", { rows: 5 })}
                <fieldset className="psi-hoja-field psi-hoja-fieldset">
                    <legend className="psi-hoja-label">Derivación</legend>
                    {checks("derivaciones", DESTINOS_DERIVACION, etiquetas?.derivaciones)}
                    {(readOnly || (value.derivaciones || []).includes("OTRO")) && otros("derivacion_otro", "Otro destino")}
                </fieldset>
            </div>

            {text("seguimiento_resultados", "Seguimiento y resultados", { rows: 4 })}
            {text("acuerdos", "Acuerdos", { rows: 4 })}

            <div className="psi-hoja-row psi-hoja-row--compromisos">
                {text("compromiso_estudiante", "Compromiso del estudiante", { rows: 4 })}
                {text("compromiso_familia", "Compromiso de la familia", { rows: 4 })}
                <div className="psi-hoja-field psi-hoja-firma">
                    <span className="psi-hoja-label">Firma</span>
                    <p className="psi-hoja-empty">Pendiente de definición institucional (firma manuscrita en el PDF impreso).</p>
                </div>
            </div>
        </div>
    );
}

export default HojaAtencion;
