import { useEffect, useState } from "react";
import { getFamiliaresEstudiante } from "../../services/estudiantesService";
import { tipoDocumentoLabel } from "../../constants/estudiantes";
import { estudianteError } from "../../utils/estudiantesErrors";

const PARENTESCOS = {
    PADRE: "Padre",
    MADRE: "Madre",
    TUTOR_LEGAL: "Tutor legal",
    ABUELO_A: "Abuelo(a)",
    HERMANO_A: "Hermano(a)",
    TIO_A: "Tío(a)",
    OTRO: "Otro",
};

const nombrePersona = (p) => [p.nombres, p.apellido_paterno, p.apellido_materno].filter(Boolean).join(" ");

// Familiares (modelo maestro, etapa 4). Se monta SOLO con
// estudiantes.familia.view (ver EstudianteDetail) y pide los datos a su
// propio endpoint (auditado en el backend); nunca vienen del detalle
// básico. Solo lectura en esta etapa: vincular/desvincular desde la
// interfaz queda para una fase posterior (la API ya existe).
function EstudianteFamiliaresSection({ estudianteId }) {
    const [historial, setHistorial] = useState(false);
    const [estado, setEstado] = useState({ key: null, familiares: [], error: null });
    const requestKey = `${estudianteId}|${historial}`;

    useEffect(() => {
        let active = true;
        getFamiliaresEstudiante(estudianteId, { historial })
            .then((data) => {
                if (active) setEstado({ key: requestKey, familiares: data.familiares || [], error: null });
            })
            .catch((err) => {
                if (active) setEstado({ key: requestKey, familiares: [], error: estudianteError(err) });
            });
        return () => {
            active = false;
        };
    }, [estudianteId, historial, requestKey]);

    const loading = estado.key !== requestKey;
    const { familiares, error } = estado;

    return (
        <section className="dp-panel">
            <div className="dp-detail-header">
                <h3 className="dp-panel-title">Familiares</h3>
                <label className="est-form-note">
                    <input type="checkbox" checked={historial} onChange={(e) => setHistorial(e.target.checked)} /> Incluir
                    relaciones cerradas
                </label>
            </div>
            {loading && <p className="dp-table-empty">Cargando familiares...</p>}
            {!loading && error && (
                <p className="dp-form-error">
                    {error.kind === "forbidden" ? "No tienes permiso para ver los familiares." : error.message}
                </p>
            )}
            {!loading && !error && familiares.length === 0 && (
                <p className="dp-table-empty">Sin familiares registrados.</p>
            )}
            {!loading && !error && familiares.length > 0 && (
                <dl className="dp-detail-list">
                    {familiares.map((f) => (
                        <div key={f.relacion_id}>
                            <dt>
                                {PARENTESCOS[f.parentesco] ?? f.parentesco}
                                {f.es_apoderado ? " · Apoderado" : ""}
                                {f.vigente ? "" : ` · Cerrada (${f.hasta})`}
                            </dt>
                            <dd>
                                {nombrePersona(f.persona)} — {tipoDocumentoLabel(f.persona.tipo_documento)}
                                {f.persona.numero_documento ? ` ${f.persona.numero_documento}` : ""}
                            </dd>
                        </div>
                    ))}
                </dl>
            )}
        </section>
    );
}

export default EstudianteFamiliaresSection;
