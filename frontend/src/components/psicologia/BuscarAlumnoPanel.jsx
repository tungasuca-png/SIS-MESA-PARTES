import { useRef, useState } from "react";
import { Link } from "react-router-dom";
import EstudianteSelector from "../dashboard/EstudianteSelector";
import StatusBadge from "../dashboard/StatusBadge";
import { getExpedientePsicologicoByEstudiante } from "../../services/psicologiaService";
import { friendlyErrorMessage } from "../../utils/apiErrors";
import { formatDate } from "../../utils/format";
import "./hojaAtencion.css";

// Psicología → Buscar alumno (por DNI, nombre o apellidos; nunca por UUID).
// Muestra el expediente psicológico del estudiante (uno por estudiante) o,
// si no tiene, permite abrirlo con el estudiante ya seleccionado. Un 404
// también cubre "fuera de tu alcance": el backend no distingue a propósito.
function BuscarAlumnoPanel({ canCreate, onAbrirExpediente }) {
    const [estudiante, setEstudiante] = useState(null);
    const [status, setStatus] = useState("idle"); // idle | loading | found | none | error
    const [expediente, setExpediente] = useState(null);
    const [error, setError] = useState("");
    const requestRef = useRef(0);

    const handleChange = async (seleccionado) => {
        setEstudiante(seleccionado);
        setExpediente(null);
        setError("");
        const requestId = ++requestRef.current;
        if (!seleccionado) {
            setStatus("idle");
            return;
        }
        setStatus("loading");
        try {
            const data = await getExpedientePsicologicoByEstudiante(seleccionado.id);
            if (requestRef.current !== requestId) return;
            setExpediente(data.expediente);
            setStatus("found");
        } catch (err) {
            if (requestRef.current !== requestId) return;
            if (err?.response?.status === 404) {
                setStatus("none");
                return;
            }
            setError(friendlyErrorMessage(err));
            setStatus("error");
        }
    };

    return (
        <section className="dp-panel" style={{ marginBottom: 16 }}>
            <h2 className="dp-panel-title">Buscar alumno</h2>
            <div className="dp-form">
                <EstudianteSelector id="psi-buscar-alumno" value={estudiante} onChange={handleChange} />
            </div>

            {status === "loading" && <p className="dp-form-hint">Buscando expediente psicológico...</p>}
            {status === "error" && <p className="dp-form-error">{error}</p>}
            {status === "found" && expediente && (
                <div className="psi-buscar-result">
                    <div>
                        <strong>{expediente.codigo}</strong> <StatusBadge status={expediente.estado} />
                        <p className="dp-form-hint" style={{ margin: "4px 0 0" }}>
                            Abierto el {formatDate(expediente.fecha_apertura)}
                        </p>
                    </div>
                    <Link to={`/psicologia/expedientes/${expediente.id}`} className="dp-btn-primary">
                        Ver expediente y atenciones
                    </Link>
                </div>
            )}
            {status === "none" && (
                <div className="psi-buscar-result">
                    <p className="dp-form-hint" style={{ margin: 0 }}>
                        Este estudiante no tiene un expediente psicológico disponible para tu usuario.
                    </p>
                    {canCreate && (
                        <button type="button" className="dp-btn-primary" onClick={() => onAbrirExpediente(estudiante)}>
                            Abrir expediente
                        </button>
                    )}
                </div>
            )}
        </section>
    );
}

export default BuscarAlumnoPanel;
