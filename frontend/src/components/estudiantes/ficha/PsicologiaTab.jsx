import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { usePermissions } from "../../../hooks/usePermissions";
import { useUsuariosBasic } from "../../../hooks/useUsuariosBasic";
import { getExpedientePsicologicoByEstudiante } from "../../../services/psicologiaService";
import { estadoLabel } from "../../../constants/psicologia";

const fecha = (iso) => (iso ? String(iso).slice(0, 10) : "—");

// Atenciones psicológicas: SOLO la existencia del expediente y su estado
// (GET /api/psicologia/estudiantes/:id/expediente, con el alcance que decide
// Psicología Service). Nunca se muestra el resumen del caso, el motivo de
// cierre ni las atenciones: el contenido clínico se consulta en el módulo de
// Psicología, que aplica sus propios permisos y su auditoría.
function PsicologiaTab({ estudianteId }) {
    const { can } = usePermissions();
    const autorizado = can(["psicologia.expedientes.view", "psicologia.expedientes.view_own"]);
    const [estado, setEstado] = useState({ loading: autorizado, expediente: null, status: null });

    useEffect(() => {
        if (!autorizado) return undefined;
        let active = true;
        getExpedientePsicologicoByEstudiante(estudianteId)
            .then((data) => active && setEstado({ loading: false, expediente: data.expediente, status: 200 }))
            .catch((err) => active && setEstado({ loading: false, expediente: null, status: err.response?.status ?? 0 }));
        return () => {
            active = false;
        };
    }, [estudianteId, autorizado]);

    const exp = estado.expediente;
    const { usuarios } = useUsuariosBasic(exp ? [exp.responsable_id] : []);

    if (!autorizado) {
        return (
            <section className="dp-panel est-soon">
                <h3 className="dp-panel-title">Atenciones psicológicas</h3>
                <p>Esta información es confidencial y solo la consulta el personal autorizado del área de Psicología.</p>
            </section>
        );
    }

    return (
        <section className="dp-panel">
            <h3 className="dp-panel-title">Atenciones psicológicas</h3>
            <p className="est-form-note">
                Por confidencialidad aquí solo se indica si existe un expediente y su estado. El contenido se consulta en el módulo de Psicología.
            </p>
            {estado.loading && <p className="dp-table-empty">Consultando Psicología...</p>}
            {!estado.loading && estado.status === 404 && (
                <p className="dp-table-empty">No hay un expediente psicológico de este estudiante dentro de tu alcance.</p>
            )}
            {!estado.loading && estado.status === 403 && <p className="dp-form-error">No tienes permiso para consultar expedientes psicológicos.</p>}
            {!estado.loading && estado.status !== 200 && estado.status !== 404 && estado.status !== 403 && (
                <p className="dp-form-error">No se pudo consultar el servicio de Psicología. Intenta nuevamente más tarde.</p>
            )}
            {exp && (
                <>
                    <dl className="dp-detail-list">
                        <div>
                            <dt>Expediente</dt>
                            <dd>{exp.codigo}</dd>
                        </div>
                        <div>
                            <dt>Estado</dt>
                            <dd>{estadoLabel(exp.estado)}</dd>
                        </div>
                        <div>
                            <dt>Apertura</dt>
                            <dd>{fecha(exp.fecha_apertura)}</dd>
                        </div>
                        {exp.fecha_cierre && (
                            <div>
                                <dt>Cierre</dt>
                                <dd>{fecha(exp.fecha_cierre)}</dd>
                            </div>
                        )}
                        <div>
                            <dt>Responsable</dt>
                            <dd>{usuarios.get(exp.responsable_id)?.nombre_completo || "—"}</dd>
                        </div>
                    </dl>
                    <Link to={`/psicologia/expedientes/${exp.id}`} className="dp-btn-secondary est-btn-link">
                        Abrir en el módulo de Psicología
                    </Link>
                </>
            )}
        </section>
    );
}

export default PsicologiaTab;
