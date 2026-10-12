import { useCallback, useEffect, useState } from "react";
import { Link, useLocation, useParams } from "react-router-dom";
import RoleLayout from "../../components/portal/RoleLayout";
import EstadoEstudianteBadge from "../../components/estudiantes/EstadoEstudianteBadge";
import EstudianteFormModal from "../../components/estudiantes/EstudianteFormModal";
import CambiarEstadoModal from "../../components/estudiantes/CambiarEstadoModal";
import EstudiantePersonalSection from "../../components/estudiantes/EstudiantePersonalSection";
import EstudianteMatriculasSection from "../../components/estudiantes/EstudianteMatriculasSection";
import EstudianteFamiliaresSection from "../../components/estudiantes/EstudianteFamiliaresSection";
import { usePermissions } from "../../hooks/usePermissions";
import { getEstudiante, getEstudiantesBasic } from "../../services/estudiantesService";
import { tipoDocumentoLabel } from "../../constants/estudiantes";
import { estudianteError } from "../../utils/estudiantesErrors";
import "../../components/dashboard/RecentExpedientes.css";
import "../../components/dashboard/forms.css";
import "../Expedientes/expedienteDetail.css";
import "./estudiantes.css";

// Carga el estudiante desde los endpoints EXISTENTES (sin inventar ninguno):
//   - GET /api/estudiantes/:id: identidad mínima, datos académicos actuales
//     y version (y el 404 si no existe);
//   - POST /api/estudiantes/basic-batch: el resto de la identidad BÁSICA
//     que GET /:id no devuelve (SIAGIE, documento, apellidos separados y
//     estado del registro). Mismo permiso (estudiantes.view).
// Ninguna de estas dos lecturas devuelve datos personales: esos tienen su
// propia sección y endpoint (FASE E-08, EstudiantePersonalSection), solo con
// estudiantes.personal.view.
async function cargarEstudiante(id) {
    const [detalle, basicos] = await Promise.all([getEstudiante(id), getEstudiantesBasic([id])]);
    const basico = (basicos.estudiantes || []).find((item) => item.id === id) || {};
    return { ...basico, ...detalle.estudiante };
}

const valor = (texto) => texto || "—";

function EstudianteDetail() {
    const { id } = useParams();
    const location = useLocation();
    const { can } = usePermissions();
    const canUpdate = can("estudiantes.update");
    const canChangeEstado = can("estudiantes.change_estado");
    const canVerPersonal = can("estudiantes.personal.view");
    const canVerFamilia = can("estudiantes.familia.view");

    const [estudiante, setEstudiante] = useState(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState(null);
    const [reloadToken, setReloadToken] = useState(0);
    const [editando, setEditando] = useState(false);
    const [cambiandoEstado, setCambiandoEstado] = useState(false);
    const [mensaje, setMensaje] = useState(location.state?.mensaje || "");

    useEffect(() => {
        let active = true;
        cargarEstudiante(id)
            .then((data) => {
                if (active) {
                    setEstudiante(data);
                    setError(null);
                }
            })
            .catch((err) => {
                if (!active) return;
                const parsed = estudianteError(err);
                // Un id con formato inválido (400) es, para el usuario, un
                // estudiante que no existe.
                setError(parsed.kind === "validation" ? { ...parsed, kind: "not_found", message: "El estudiante no existe o el enlace no es válido." } : parsed);
            })
            .finally(() => {
                if (active) setLoading(false);
            });
        return () => {
            active = false;
        };
    }, [id, reloadToken]);

    const recargar = useCallback(() => {
        setEditando(false);
        setCambiandoEstado(false);
        setMensaje("");
        setLoading(true);
        setReloadToken((token) => token + 1);
    }, []);

    const aplicarRespuesta = (actualizado, texto) => {
        setEstudiante((current) => ({ ...current, ...actualizado }));
        setMensaje(texto);
    };

    let contenido;
    if (loading) {
        contenido = <p className="dp-table-empty">Cargando estudiante...</p>;
    } else if (error?.kind === "forbidden") {
        contenido = (
            <section className="dp-panel" style={{ textAlign: "center", padding: "40px 20px" }}>
                <h2 className="dp-panel-title">Acceso denegado</h2>
                <p>No tienes permiso para consultar estudiantes.</p>
            </section>
        );
    } else if (error) {
        contenido = (
            <section className="dp-panel">
                <p className="dp-form-error">{error.message}</p>
            </section>
        );
    } else if (estudiante) {
        contenido = (
            <>
                <section className="dp-panel">
                    <div className="dp-detail-header">
                        <div>
                            <p className="dp-detail-codigo">{estudiante.codigo}</p>
                            <h2 className="dp-panel-title dp-detail-asunto">
                                {[estudiante.nombres, estudiante.apellidos].filter(Boolean).join(" ")}
                            </h2>
                        </div>
                        <div className="est-detail-actions">
                            <EstadoEstudianteBadge estado={estudiante.estado} />
                            {canUpdate && (
                                <button type="button" className="dp-btn-primary" onClick={() => setEditando(true)}>
                                    Editar
                                </button>
                            )}
                            {canChangeEstado && (
                                <button type="button" className="dp-btn-secondary" onClick={() => setCambiandoEstado(true)}>
                                    {estudiante.estado === "ACTIVO" ? "Inactivar" : "Activar"}
                                </button>
                            )}
                        </div>
                    </div>
                    {mensaje && <p className="est-banner-success">{mensaje}</p>}
                </section>

                <div className="est-detail-sections">
                    <section className="dp-panel">
                        <h3 className="dp-panel-title">Identidad básica</h3>
                        <dl className="dp-detail-list">
                            <div>
                                <dt>Código</dt>
                                <dd>{estudiante.codigo}</dd>
                            </div>
                            <div>
                                <dt>Código SIAGIE</dt>
                                <dd>{valor(estudiante.codigo_siagie)}</dd>
                            </div>
                            <div>
                                <dt>Nombres</dt>
                                <dd>{estudiante.nombres}</dd>
                            </div>
                            <div>
                                <dt>Apellidos</dt>
                                <dd>{estudiante.apellidos}</dd>
                            </div>
                            {estudiante.apellido_paterno && (
                                <div>
                                    <dt>Apellido paterno / materno</dt>
                                    <dd>
                                        {estudiante.apellido_paterno} / {valor(estudiante.apellido_materno)}
                                    </dd>
                                </div>
                            )}
                            <div>
                                <dt>Documento</dt>
                                <dd>
                                    {tipoDocumentoLabel(estudiante.tipo_documento)}
                                    {estudiante.numero_documento ? ` ${estudiante.numero_documento}` : ""}
                                </dd>
                            </div>
                            <div>
                                <dt>Estado del registro</dt>
                                <dd>
                                    <EstadoEstudianteBadge estado={estudiante.estado} />
                                </dd>
                            </div>
                            <div>
                                <dt>Versión</dt>
                                <dd>{estudiante.version}</dd>
                            </div>
                        </dl>
                    </section>

                    {canVerPersonal && (
                        <EstudiantePersonalSection
                            key={`${estudiante.id}-${reloadToken}`}
                            estudianteId={estudiante.id}
                            versionDetalle={estudiante.version}
                            onReload={recargar}
                            onSaved={(actualizado) => aplicarRespuesta(actualizado, "Datos personales guardados correctamente.")}
                        />
                    )}

                    <section className="dp-panel">
                        <h3 className="dp-panel-title">Información académica actual</h3>
                        <p className="est-form-note">
                            Datos heredados, solo de lectura. El historial por periodo está en "Historial de matrícula".
                        </p>
                        <dl className="dp-detail-list">
                            <div>
                                <dt>Nivel</dt>
                                <dd>{valor(estudiante.nivel)}</dd>
                            </div>
                            <div>
                                <dt>Grado</dt>
                                <dd>{valor(estudiante.grado)}</dd>
                            </div>
                            <div>
                                <dt>Sección</dt>
                                <dd>{valor(estudiante.seccion)}</dd>
                            </div>
                            <div>
                                <dt>Turno</dt>
                                <dd>{valor(estudiante.turno)}</dd>
                            </div>
                            <div>
                                <dt>Estado de matrícula</dt>
                                <dd>{valor(estudiante.estado_matricula)}</dd>
                            </div>
                        </dl>
                    </section>

                    {/* Modelo maestro: historial académico (estudiantes.view,
                        mismo permiso que esta página) y familiares (dominio
                        propio: estudiantes.familia.view). */}
                    <EstudianteMatriculasSection key={`mat-${estudiante.id}-${reloadToken}`} estudianteId={estudiante.id} />
                    {canVerFamilia && (
                        <EstudianteFamiliaresSection key={`fam-${estudiante.id}-${reloadToken}`} estudianteId={estudiante.id} />
                    )}
                </div>
            </>
        );
    }

    return (
        <RoleLayout title="Estudiantes">
            <Link to="/estudiantes" className="dp-back-link">
                ← Volver al directorio
            </Link>
            {contenido}

            {editando && estudiante && (
                <EstudianteFormModal
                    mode="edit"
                    estudiante={estudiante}
                    onClose={() => setEditando(false)}
                    onReload={recargar}
                    onSaved={(actualizado) => {
                        setEditando(false);
                        aplicarRespuesta(actualizado, "Cambios guardados correctamente.");
                    }}
                />
            )}
            {cambiandoEstado && estudiante && (
                <CambiarEstadoModal
                    estudiante={estudiante}
                    onClose={() => setCambiandoEstado(false)}
                    onReload={recargar}
                    onChanged={(actualizado) => {
                        setCambiandoEstado(false);
                        aplicarRespuesta(actualizado, "Estado actualizado correctamente.");
                    }}
                />
            )}
        </RoleLayout>
    );
}

export default EstudianteDetail;
