import { useCallback, useEffect, useState } from "react";
import { Link, useLocation, useParams, useSearchParams } from "react-router-dom";
import RoleLayout from "../../components/portal/RoleLayout";
import EstadoEstudianteBadge from "../../components/estudiantes/EstadoEstudianteBadge";
import EstudianteFormModal from "../../components/estudiantes/EstudianteFormModal";
import CambiarEstadoModal from "../../components/estudiantes/CambiarEstadoModal";
import EstudiantePersonalSection from "../../components/estudiantes/EstudiantePersonalSection";
import FamiliaTab from "../../components/estudiantes/ficha/FamiliaTab";
import MatriculasTab from "../../components/estudiantes/ficha/MatriculasTab";
import HistorialTab from "../../components/estudiantes/ficha/HistorialTab";
import PsicologiaTab from "../../components/estudiantes/ficha/PsicologiaTab";
import ProximamenteTab from "../../components/estudiantes/ficha/ProximamenteTab";
import { usePermissions } from "../../hooks/usePermissions";
import { getEstudiante, getEstudiantesBasic, getMatriculasEstudiante } from "../../services/estudiantesService";
import { estadoMatriculaLabel, tipoDocumentoLabel } from "../../constants/estudiantes";
import { estudianteError } from "../../utils/estudiantesErrors";
import { enmascararDocumento } from "../../utils/privacidad";
import { matriculaVigente, primeraMatricula, seccionLabel } from "../../utils/matricula";
import "../../components/dashboard/RecentExpedientes.css";
import "../../components/dashboard/forms.css";
import "../Expedientes/expedienteDetail.css";
import "./estudiantes.css";

// Carga el estudiante desde los endpoints EXISTENTES (sin inventar ninguno):
//   - GET /api/estudiantes/:id: identidad mínima, datos académicos heredados
//     y version (y el 404 si no existe);
//   - POST /api/estudiantes/basic-batch: el resto de la identidad BÁSICA
//     (SIAGIE, documento, apellidos separados y estado del registro).
// Ninguna devuelve datos personales: esos tienen su propio endpoint
// (EstudiantePersonalSection), solo con estudiantes.personal.view.
async function cargarEstudiante(id) {
    const [detalle, basicos] = await Promise.all([getEstudiante(id), getEstudiantesBasic([id])]);
    const basico = (basicos.estudiantes || []).find((item) => item.id === id) || {};
    return { ...basico, ...detalle.estudiante };
}

const valor = (texto) => texto || "—";

// Pestañas de la ficha con su estado REAL (no se muestra como operativo lo
// que no existe). "parcial" explica qué falta en la propia pestaña.
const PESTANAS = [
    { id: "general", label: "Información general", estado: "disponible" },
    { id: "familia", label: "Padres y apoderados", estado: "parcial" },
    { id: "academico", label: "Historial académico", estado: "disponible" },
    { id: "notas", label: "Notas", estado: "proximamente" },
    { id: "psicologia", label: "Atenciones psicológicas", estado: "parcial" },
    { id: "permisos", label: "Permisos y justificaciones", estado: "proximamente" },
    { id: "documentos", label: "Documentos adjuntos", estado: "proximamente" },
    { id: "asistencia", label: "Asistencia", estado: "proximamente" },
    { id: "historial", label: "Historial del estudiante", estado: "disponible" },
];

const BADGE = { parcial: "Parcial", proximamente: "Próximamente" };

const PROXIMAMENTE = {
    notas: {
        titulo: "Notas",
        descripcion: "Aquí se consultarán las calificaciones por curso, periodo y año académico.",
        requiere: "El sistema todavía no tiene un servicio de calificaciones. Se habilitará cuando exista ese módulo; no se registran notas desde esta ficha.",
    },
    permisos: {
        titulo: "Permisos y justificaciones",
        descripcion: "Aquí se registrarán permisos, tardanzas, inasistencias y sus justificaciones.",
        requiere: "No existe todavía un módulo de permisos y justificaciones en el sistema.",
    },
    documentos: {
        titulo: "Documentos adjuntos",
        descripcion: "Aquí se consultarán los documentos del estudiante (constancias, certificados, fichas).",
        requiere:
            "El servicio de Documentos existe, pero guarda archivos de expedientes de Mesa de Partes y no los asocia a un estudiante. Se necesita esa asociación en el backend.",
    },
    asistencia: {
        titulo: "Asistencia",
        descripcion: "Aquí se mostrará el resumen de asistencias, tardanzas e inasistencias por periodo.",
        requiere: "El sistema todavía no tiene un servicio de asistencia.",
    },
};

function EstudianteDetail() {
    const { id } = useParams();
    const location = useLocation();
    const [searchParams, setSearchParams] = useSearchParams();
    const { can } = usePermissions();
    const canUpdate = can("estudiantes.update");
    const canChangeEstado = can("estudiantes.change_estado");
    const canVerPersonal = can("estudiantes.personal.view");
    const canVerFamilia = can("estudiantes.familia.view");

    const tab = PESTANAS.some((p) => p.id === searchParams.get("tab")) ? searchParams.get("tab") : "general";
    const irA = (destino) => setSearchParams(destino === "general" ? {} : { tab: destino }, { replace: true });

    const [estudiante, setEstudiante] = useState(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState(null);
    const [reloadToken, setReloadToken] = useState(0);
    const [editando, setEditando] = useState(false);
    const [cambiandoEstado, setCambiandoEstado] = useState(false);
    const [mensaje, setMensaje] = useState(location.state?.mensaje || "");
    const [docVisible, setDocVisible] = useState(false);
    const [matToken, setMatToken] = useState(0);
    const [matriculasEstado, setMatriculasEstado] = useState({ key: null, matriculas: [], error: null });
    const matKey = `${id}|${reloadToken}|${matToken}`;

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

    // Matrículas: una sola lectura para la cabecera (periodo vigente), la
    // pestaña académica y el historial.
    useEffect(() => {
        let active = true;
        getMatriculasEstudiante(id)
            .then((data) => active && setMatriculasEstado({ key: matKey, matriculas: data.matriculas || [], error: null }))
            .catch((err) => active && setMatriculasEstado({ key: matKey, matriculas: [], error: estudianteError(err) }));
        return () => {
            active = false;
        };
    }, [id, matKey]);
    const matriculas = { ...matriculasEstado, loading: matriculasEstado.key !== matKey };

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

    // Familia, matrícula y datos personales también cambian la versión del
    // registro: se relee el detalle sin perder la pestaña.
    const refrescarVersion = () => {
        cargarEstudiante(id)
            .then((data) => setEstudiante(data))
            .catch(() => {});
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
        const vigente = matriculaVigente(matriculas.matriculas);
        const ingreso = primeraMatricula(matriculas.matriculas);
        const nombre = [estudiante.nombres, estudiante.apellidos].filter(Boolean).join(" ");
        contenido = (
            <>
                <section className="dp-panel est-ficha-head">
                    <div className="est-avatar" aria-hidden="true">
                        {(estudiante.nombres || "?").charAt(0)}
                        {(estudiante.apellido_paterno || estudiante.apellidos || "").charAt(0)}
                    </div>
                    <div className="est-ficha-main">
                        <div className="est-ficha-title">
                            <h2 className="dp-panel-title dp-detail-asunto">{nombre}</h2>
                            <EstadoEstudianteBadge estado={estudiante.estado} />
                            {vigente && <span className="est-chip est-chip--accent">{estadoMatriculaLabel(vigente.estado_matricula)}</span>}
                        </div>
                        <dl className="est-ficha-datos">
                            <div>
                                <dt>Código</dt>
                                <dd className="dp-detail-codigo">{estudiante.codigo}</dd>
                            </div>
                            <div>
                                <dt>Código SIAGIE</dt>
                                <dd>{valor(estudiante.codigo_siagie)}</dd>
                            </div>
                            <div>
                                <dt>Documento</dt>
                                <dd>
                                    {tipoDocumentoLabel(estudiante.tipo_documento)} {enmascararDocumento(estudiante.numero_documento)}
                                </dd>
                            </div>
                            <div>
                                <dt>{vigente ? `Periodo vigente (${vigente.periodo?.anio})` : "Grado y sección"}</dt>
                                <dd>
                                    {vigente
                                        ? seccionLabel(vigente.seccion)
                                        : estudiante.grado || estudiante.seccion
                                          ? `${valor(estudiante.grado)} · ${valor(estudiante.seccion)} (dato anterior al módulo de matrícula)`
                                          : "Sin matrícula vigente"}
                                </dd>
                            </div>
                            <div>
                                <dt>Primera matrícula registrada</dt>
                                <dd>{ingreso || "—"}</dd>
                            </div>
                        </dl>
                    </div>
                    <div className="est-detail-actions est-ficha-actions">
                        {canUpdate && (
                            <button type="button" className="dp-btn-primary" onClick={() => setEditando(true)}>
                                Editar ficha
                            </button>
                        )}
                        <button type="button" className="dp-btn-secondary" onClick={() => irA("historial")}>
                            Ver historial
                        </button>
                        {canChangeEstado && (
                            <button type="button" className="dp-btn-secondary" onClick={() => setCambiandoEstado(true)}>
                                {estudiante.estado === "ACTIVO" ? "Inactivar" : "Activar"}
                            </button>
                        )}
                    </div>
                    {mensaje && <p className="est-banner-success est-ficha-mensaje">{mensaje}</p>}
                </section>

                <div className="est-tabs" role="tablist" aria-label="Secciones de la ficha">
                    {PESTANAS.map((p) => (
                        <button
                            key={p.id}
                            type="button"
                            role="tab"
                            id={`tab-${p.id}`}
                            aria-selected={tab === p.id}
                            aria-controls={`panel-${p.id}`}
                            className={tab === p.id ? "est-tab is-active" : "est-tab"}
                            onClick={() => irA(p.id)}
                        >
                            {p.label}
                            {BADGE[p.estado] && <span className={`est-tab-badge est-tab-badge--${p.estado}`}>{BADGE[p.estado]}</span>}
                        </button>
                    ))}
                </div>

                <div role="tabpanel" id={`panel-${tab}`} aria-labelledby={`tab-${tab}`} className="est-tabpanel">
                    {tab === "general" && (
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
                                            {estudiante.numero_documento &&
                                                ` ${docVisible ? estudiante.numero_documento : enmascararDocumento(estudiante.numero_documento)}`}
                                            {estudiante.numero_documento && (
                                                <button type="button" className="est-link-button est-reveal" onClick={() => setDocVisible((v) => !v)}>
                                                    {docVisible ? "Ocultar" : "Mostrar"}
                                                </button>
                                            )}
                                        </dd>
                                    </div>
                                    <div>
                                        <dt>Estado del registro</dt>
                                        <dd>
                                            <EstadoEstudianteBadge estado={estudiante.estado} />
                                        </dd>
                                    </div>
                                    <div>
                                        <dt>Fecha de ingreso</dt>
                                        <dd>
                                            {ingreso ? `${ingreso} (primera matrícula registrada)` : "No registrada"}
                                        </dd>
                                    </div>
                                </dl>
                            </section>

                            {canVerPersonal ? (
                                <EstudiantePersonalSection
                                    key={`${estudiante.id}-${reloadToken}`}
                                    estudianteId={estudiante.id}
                                    versionDetalle={estudiante.version}
                                    onReload={recargar}
                                    onSaved={(actualizado) => aplicarRespuesta(actualizado, "Datos personales guardados correctamente.")}
                                />
                            ) : (
                                <section className="dp-panel">
                                    <h3 className="dp-panel-title">Datos personales</h3>
                                    <p className="est-form-note">Requieren el permiso de datos personales.</p>
                                </section>
                            )}

                            {(estudiante.nivel || estudiante.grado || estudiante.seccion || estudiante.turno) && (
                                <section className="dp-panel">
                                    <h3 className="dp-panel-title">Información académica actual</h3>
                                    <p className="est-form-note">
                                        Datos anteriores al módulo de matrícula, solo de lectura. La trayectoria está en «Historial académico».
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
                                    </dl>
                                </section>
                            )}
                        </div>
                    )}

                    {tab === "familia" &&
                        (canVerFamilia ? (
                            <FamiliaTab key={`fam-${estudiante.id}-${reloadToken}`} estudianteId={estudiante.id} onCambio={refrescarVersion} />
                        ) : (
                            <section className="dp-panel">
                                <h3 className="dp-panel-title">Padres y apoderados</h3>
                                <p>No tienes permiso para ver los padres y apoderados de los estudiantes.</p>
                            </section>
                        ))}

                    {tab === "academico" && (
                        <MatriculasTab
                            estudianteId={estudiante.id}
                            estado={matriculas}
                            estudianteActivo={estudiante.estado === "ACTIVO"}
                            onCambio={() => {
                                setMatToken((n) => n + 1);
                                refrescarVersion();
                            }}
                        />
                    )}

                    {tab === "psicologia" && <PsicologiaTab estudianteId={estudiante.id} />}

                    {tab === "historial" && <HistorialTab key={`his-${reloadToken}-${matToken}`} estudianteId={estudiante.id} matriculasEstado={matriculas} />}

                    {PROXIMAMENTE[tab] && <ProximamenteTab {...PROXIMAMENTE[tab]} />}
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
