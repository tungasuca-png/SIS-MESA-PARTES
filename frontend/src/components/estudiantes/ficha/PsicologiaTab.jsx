import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { usePermissions } from "../../../hooks/usePermissions";
import { useUsuariosBasic } from "../../../hooks/useUsuariosBasic";
import { getExpedientePsicologicoByEstudiante, listAtenciones } from "../../../services/psicologiaService";
import { estadoAtencionLabel, estadoLabel } from "../../../constants/psicologia";

const fecha = (iso) => (iso ? String(iso).slice(0, 10) : "—");

const MSG_SIN_PERMISO = "No tienes permisos para consultar esta información.";

// Traduce la respuesta del servicio de Psicología a un estado de la pestaña.
// NUNCA convierte un error de autorización, conexión o configuración en
// "sin expedientes": solo un 404 de una consulta autorizada lo es.
function clasificar(err) {
    const status = err?.response?.status;
    if (!err?.response) return "sin_conexion";
    if (status === 401) return "sesion";
    if (status === 403) return "sin_permiso";
    if (status === 404) return "no_encontrado";
    if (status >= 500) return "sin_conexion";
    return "error";
}

// Atenciones del expediente (contenido clínico): solo con
// psicologia.atenciones.view, que Psicología vuelve a validar junto con el
// alcance. Se listan número, fecha y estado; el contenido se abre en el
// módulo de Psicología (que lo audita).
function AtencionesExpediente({ expedienteId }) {
    const [reintento, setReintento] = useState(0);
    const [estado, setEstado] = useState({ key: null, atenciones: [], total: 0, error: null });
    const key = `${expedienteId}|${reintento}`;

    useEffect(() => {
        let active = true;
        listAtenciones(expedienteId, { pageSize: 10 })
            .then((d) => active && setEstado({ key, atenciones: d.atenciones || [], total: d.total || 0, error: null }))
            .catch((err) => active && setEstado({ key, atenciones: [], total: 0, error: clasificar(err) }));
        return () => {
            active = false;
        };
    }, [expedienteId, key]);

    const loading = estado.key !== key;
    return (
        <div className="est-psi-atenciones">
            <h4 className="est-form-section">Atenciones registradas</h4>
            {loading && <p className="dp-table-empty">Consultando atenciones...</p>}
            {!loading && estado.error === "sin_permiso" && <p className="dp-form-error">{MSG_SIN_PERMISO}</p>}
            {!loading && (estado.error === "sin_conexion" || estado.error === "error") && (
                <div className="est-submit-error" role="alert">
                    <p className="dp-form-error">No se pudo conectar con el servicio de Psicología.</p>
                    <button type="button" className="dp-btn-secondary" onClick={() => setReintento((n) => n + 1)}>
                        Reintentar
                    </button>
                </div>
            )}
            {!loading && estado.error === "no_encontrado" && <p className="dp-table-empty">El expediente ya no está disponible en tu alcance.</p>}
            {!loading && !estado.error && estado.atenciones.length === 0 && <p className="dp-table-empty">El expediente no tiene atenciones registradas.</p>}
            {!loading && !estado.error && estado.atenciones.length > 0 && (
                <ul className="est-otros-lista" aria-label="Atenciones registradas">
                    {estado.atenciones.map((a) => (
                        <li key={a.id}>
                            <Link to={`/psicologia/atenciones/${a.id}`}>Atención N.° {a.numero}</Link>
                            <span className="est-muted">
                                {" "}
                                · {fecha(a.fecha_atencion)} · {estadoAtencionLabel(a.estado)}
                            </span>
                        </li>
                    ))}
                    {estado.total > estado.atenciones.length && <li className="est-muted">Y {estado.total - estado.atenciones.length} más en el módulo de Psicología.</li>}
                </ul>
            )}
        </div>
    );
}

// Pestaña "Atenciones psicológicas" de la ficha. El expediente se relaciona
// con el estudiante por su UUID (uno por estudiante) y se consulta con
// GET /api/psicologia/estudiantes/:id/expediente. Psicología decide el
// alcance en el backend (psicologia.expedientes.view = todos; view_own =
// solo donde el usuario es responsable) y audita cada consulta; el frontend
// solo evita llamar cuando el usuario no tiene ninguno de esos permisos.
// Nunca se muestra el resumen del caso ni el motivo de cierre.
function PsicologiaTab({ estudianteId }) {
    const { can } = usePermissions();
    const verTodos = can("psicologia.expedientes.view");
    const autorizado = verTodos || can("psicologia.expedientes.view_own");
    const verAtenciones = can("psicologia.atenciones.view");

    const [reintento, setReintento] = useState(0);
    const [estado, setEstado] = useState({ key: null, expediente: null, resultado: null });
    const key = `${estudianteId}|${reintento}`;

    useEffect(() => {
        if (!autorizado) return undefined;
        let active = true;
        getExpedientePsicologicoByEstudiante(estudianteId)
            .then((data) => active && setEstado({ key, expediente: data.expediente, resultado: "ok" }))
            .catch((err) => active && setEstado({ key, expediente: null, resultado: clasificar(err) }));
        return () => {
            active = false;
        };
    }, [estudianteId, autorizado, key]);

    const exp = estado.resultado === "ok" ? estado.expediente : null;
    const { usuarios } = useUsuariosBasic(exp ? [exp.responsable_id] : []);
    const loading = autorizado && estado.key !== key;

    let contenido;
    if (!autorizado || estado.resultado === "sin_permiso") {
        contenido = <p className="dp-form-error">{MSG_SIN_PERMISO}</p>;
    } else if (loading) {
        contenido = <p className="dp-table-empty">Consultando el servicio de Psicología...</p>;
    } else if (estado.resultado === "sesion") {
        contenido = <p className="dp-form-error">Tu sesión no es válida. Inicia sesión nuevamente.</p>;
    } else if (estado.resultado === "sin_conexion" || estado.resultado === "error") {
        contenido = (
            <div className="est-submit-error" role="alert">
                <p className="dp-form-error">No se pudo conectar con el servicio de Psicología.</p>
                <button type="button" className="dp-btn-secondary" disabled={loading} onClick={() => setReintento((n) => n + 1)}>
                    Reintentar
                </button>
            </div>
        );
    } else if (estado.resultado === "no_encontrado") {
        // Con alcance general, Psicología confirma que no existe. Con solo
        // "view_own", un 404 también cubre expedientes de otros responsables:
        // no se afirma que el estudiante no tenga ninguno.
        contenido = (
            <p className="dp-table-empty">
                {verTodos
                    ? "Este estudiante no tiene expedientes psicológicos registrados."
                    : "No hay expedientes psicológicos de este estudiante bajo tu responsabilidad."}
            </p>
        );
    } else if (exp) {
        contenido = (
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
                {verAtenciones ? (
                    <AtencionesExpediente expedienteId={exp.id} />
                ) : (
                    <p className="est-form-note est-psi-nota">El contenido clínico (atenciones) requiere la autorización específica de Psicología.</p>
                )}
            </>
        );
    }

    return (
        <section className="dp-panel">
            <h3 className="dp-panel-title">Atenciones psicológicas</h3>
            {autorizado && (
                <p className="est-form-note">
                    Información confidencial: cada consulta queda registrada en la bitácora de Psicología. Aquí no se muestra el resumen del caso.
                </p>
            )}
            {contenido}
        </section>
    );
}

export default PsicologiaTab;
