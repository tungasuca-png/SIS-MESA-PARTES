import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import RoleLayout from "../../components/portal/RoleLayout";
import AccionModal from "../../components/estudiantes/AccionModal";
import { usePermissions } from "../../hooks/usePermissions";
import { changeEstadoPeriodo, createPeriodo, createSeccion, listPeriodos, listSecciones } from "../../services/estudiantesService";
import { ESTADOS_PERIODO, NIVELES } from "../../constants/estudiantes";
import { estudianteError } from "../../utils/estudiantesErrors";
import { seccionLabel } from "../../utils/matricula";
import "../../components/dashboard/RecentExpedientes.css";
import "../../components/dashboard/forms.css";
import "../Expedientes/expedienteDetail.css";
import "./estudiantes.css";

// Periodos académicos y secciones (lectura: estudiantes.view; registro y
// cambio de estado: estudiantes.matricula.update). Son la base para
// matricular desde la ficha del estudiante. Nada se borra: un periodo solo
// avanza (Planificado -> En curso -> Cerrado).
function NuevoPeriodoModal({ onClose, onDone }) {
    const [f, setF] = useState({ anio: String(new Date().getFullYear()), nombre: "", fechaInicio: "", fechaFin: "" });
    const set = (k) => (e) => setF((c) => ({ ...c, [k]: e.target.value }));
    return (
        <AccionModal
            title="Nuevo periodo académico"
            submitLabel="Registrar periodo"
            onClose={onClose}
            validate={() => {
                const anio = Number(f.anio);
                if (!Number.isInteger(anio) || anio < 2000 || anio > 2100) return "El año debe estar entre 2000 y 2100.";
                if (!f.nombre.trim()) return "Ingresa el nombre del periodo.";
                if (f.fechaInicio && f.fechaFin && f.fechaFin < f.fechaInicio) return "La fecha de fin no puede ser anterior a la de inicio.";
                return null;
            }}
            onSubmit={async () => {
                const data = await createPeriodo({ ...f, nombre: f.nombre.trim() });
                onDone(data.periodo);
            }}
        >
            <div className="est-form-row">
                <div className="dp-form-group">
                    <label htmlFor="per-anio">Año *</label>
                    <input id="per-anio" type="number" min="2000" max="2100" value={f.anio} onChange={set("anio")} />
                </div>
                <div className="dp-form-group">
                    <label htmlFor="per-nombre">Nombre *</label>
                    <input id="per-nombre" maxLength={60} placeholder={`Año escolar ${f.anio}`} value={f.nombre} onChange={set("nombre")} />
                </div>
            </div>
            <div className="est-form-row">
                <div className="dp-form-group">
                    <label htmlFor="per-inicio">Inicio</label>
                    <input id="per-inicio" type="date" value={f.fechaInicio} onChange={set("fechaInicio")} />
                </div>
                <div className="dp-form-group">
                    <label htmlFor="per-fin">Fin</label>
                    <input id="per-fin" type="date" value={f.fechaFin} onChange={set("fechaFin")} />
                </div>
            </div>
            <p className="est-form-note">El periodo se registra como «Planificado».</p>
        </AccionModal>
    );
}

function NuevaSeccionModal({ periodo, onClose, onDone }) {
    const [f, setF] = useState({ nivel: "", grado: "", seccion: "", turno: "", aula: "", vacantes: "" });
    const set = (k) => (e) => setF((c) => ({ ...c, [k]: e.target.value }));
    return (
        <AccionModal
            title={`Nueva sección · ${periodo.nombre}`}
            submitLabel="Registrar sección"
            onClose={onClose}
            validate={() => {
                if (!f.nivel) return "Elige el nivel.";
                if (!f.grado.trim() || !f.seccion.trim()) return "Ingresa el grado y la sección.";
                if (f.vacantes !== "" && (!Number.isInteger(Number(f.vacantes)) || Number(f.vacantes) < 0)) return "Las vacantes deben ser un número entero mayor o igual a 0.";
                return null;
            }}
            onSubmit={async () => {
                await createSeccion(periodo.id, { ...f, grado: f.grado.trim(), seccion: f.seccion.trim(), turno: f.turno.trim(), aula: f.aula.trim() });
                onDone();
            }}
        >
            <div className="est-form-row">
                <div className="dp-form-group">
                    <label htmlFor="sec-nivel">Nivel *</label>
                    <select id="sec-nivel" value={f.nivel} onChange={set("nivel")}>
                        <option value="">Selecciona...</option>
                        {NIVELES.map((n) => (
                            <option key={n.value} value={n.value}>
                                {n.label}
                            </option>
                        ))}
                    </select>
                </div>
                <div className="dp-form-group">
                    <label htmlFor="sec-grado">Grado *</label>
                    <input id="sec-grado" maxLength={20} value={f.grado} onChange={set("grado")} />
                </div>
                <div className="dp-form-group">
                    <label htmlFor="sec-seccion">Sección *</label>
                    <input id="sec-seccion" maxLength={10} value={f.seccion} onChange={set("seccion")} />
                </div>
            </div>
            <div className="est-form-row">
                <div className="dp-form-group">
                    <label htmlFor="sec-turno">Turno</label>
                    <input id="sec-turno" maxLength={20} value={f.turno} onChange={set("turno")} />
                </div>
                <div className="dp-form-group">
                    <label htmlFor="sec-aula">Aula</label>
                    <input id="sec-aula" maxLength={40} value={f.aula} onChange={set("aula")} />
                </div>
                <div className="dp-form-group">
                    <label htmlFor="sec-vacantes">Vacantes</label>
                    <input id="sec-vacantes" type="number" min="0" value={f.vacantes} onChange={set("vacantes")} />
                    <span className="dp-form-hint">Vacío = sin límite registrado.</span>
                </div>
            </div>
        </AccionModal>
    );
}

function SeccionesPeriodo({ periodo, canUpdate, reload, onNueva }) {
    const [estado, setEstado] = useState({ key: null, secciones: [], error: null });
    const key = `${periodo.id}|${reload}`;
    useEffect(() => {
        let active = true;
        listSecciones(periodo.id)
            .then((d) => active && setEstado({ key, secciones: d.secciones || [], error: null }))
            .catch((err) => active && setEstado({ key, secciones: [], error: estudianteError(err) }));
        return () => {
            active = false;
        };
    }, [periodo.id, key]);
    const loading = estado.key !== key;

    return (
        <div className="est-secciones">
            {loading && <p className="dp-table-empty">Cargando secciones...</p>}
            {!loading && estado.error && <p className="dp-form-error">{estado.error.message}</p>}
            {!loading && !estado.error && estado.secciones.length === 0 && <p className="dp-table-empty">Sin secciones registradas.</p>}
            {!loading && estado.secciones.length > 0 && (
                <div className="dp-table-wrapper">
                    <table className="dp-table">
                        <thead>
                            <tr>
                                <th>Sección</th>
                                <th>Aula</th>
                                <th>Matriculados</th>
                                <th>Vacantes</th>
                            </tr>
                        </thead>
                        <tbody>
                            {estado.secciones.map((s) => (
                                <tr key={s.id}>
                                    <td>{seccionLabel(s)}</td>
                                    <td>{s.aula || "—"}</td>
                                    <td>{s.matriculados}</td>
                                    <td>{s.vacantes == null ? "Sin límite" : s.vacantes}</td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}
            {canUpdate && periodo.estado !== "CERRADO" && (
                <button type="button" className="dp-btn-secondary" onClick={onNueva}>
                    Nueva sección
                </button>
            )}
        </div>
    );
}

function PeriodosSecciones() {
    const { can } = usePermissions();
    const canUpdate = can("estudiantes.matricula.update");
    const [reload, setReload] = useState(0);
    const [estado, setEstado] = useState({ key: null, periodos: [], error: null });
    const [abierto, setAbierto] = useState(null);
    const [modal, setModal] = useState(null);
    const [mensaje, setMensaje] = useState("");

    useEffect(() => {
        let active = true;
        listPeriodos()
            .then((d) => active && setEstado({ key: reload, periodos: d.periodos || [], error: null }))
            .catch((err) => active && setEstado({ key: reload, periodos: [], error: estudianteError(err) }));
        return () => {
            active = false;
        };
    }, [reload]);
    const loading = estado.key !== reload;
    const listo = (texto) => {
        setModal(null);
        setMensaje(texto);
        setReload((n) => n + 1);
    };

    if (!loading && estado.error?.kind === "forbidden") {
        return (
            <RoleLayout title="Estudiantes">
                <section className="dp-panel" style={{ textAlign: "center", padding: "40px 20px" }}>
                    <h2 className="dp-panel-title">Acceso denegado</h2>
                    <p>No tienes permiso para consultar los periodos académicos.</p>
                </section>
            </RoleLayout>
        );
    }

    return (
        <RoleLayout title="Estudiantes">
            <Link to="/estudiantes" className="dp-back-link">
                ← Volver al directorio
            </Link>
            <section className="dp-panel">
                <div className="dp-list-header">
                    <div>
                        <h2 className="dp-panel-title">Periodos y secciones</h2>
                        <p className="est-subtitle">Base de la matrícula: un periodo por año escolar y sus secciones.</p>
                    </div>
                    {canUpdate && (
                        <button type="button" className="dp-btn-primary" onClick={() => setModal({ tipo: "periodo" })}>
                            Nuevo periodo
                        </button>
                    )}
                </div>
                {mensaje && <p className="est-banner-success">{mensaje}</p>}
                {loading && <p className="dp-table-empty">Cargando periodos...</p>}
                {!loading && estado.error && <p className="dp-form-error">{estado.error.message}</p>}
                {!loading && !estado.error && estado.periodos.length === 0 && (
                    <p className="dp-table-empty">Todavía no hay periodos académicos registrados.</p>
                )}
                {!loading && estado.periodos.length > 0 && (
                    <ul className="est-card-list">
                        {estado.periodos.map((p) => {
                            const info = ESTADOS_PERIODO[p.estado] ?? { label: p.estado };
                            return (
                                <li key={p.id} className="est-card">
                                    <div className="est-card-head">
                                        <div>
                                            <p className="est-card-title">{p.nombre}</p>
                                            <p className="est-card-sub">
                                                {p.anio}
                                                {p.fecha_inicio ? ` · del ${p.fecha_inicio}` : ""}
                                                {p.fecha_fin ? ` al ${p.fecha_fin}` : ""}
                                            </p>
                                        </div>
                                        <span className={`est-chip est-chip--periodo-${p.estado.toLowerCase()}`}>{info.label}</span>
                                    </div>
                                    <div className="est-card-actions">
                                        <button
                                            type="button"
                                            className="dp-btn-secondary"
                                            aria-expanded={abierto === p.id}
                                            onClick={() => setAbierto((c) => (c === p.id ? null : p.id))}
                                        >
                                            {abierto === p.id ? "Ocultar secciones" : "Ver secciones"}
                                        </button>
                                        {canUpdate && info.siguiente && (
                                            <button type="button" className="dp-btn-secondary" onClick={() => setModal({ tipo: "estado", periodo: p })}>
                                                {info.accion}
                                            </button>
                                        )}
                                    </div>
                                    {abierto === p.id && (
                                        <SeccionesPeriodo periodo={p} canUpdate={canUpdate} reload={reload} onNueva={() => setModal({ tipo: "seccion", periodo: p })} />
                                    )}
                                </li>
                            );
                        })}
                    </ul>
                )}
            </section>

            {modal?.tipo === "periodo" && <NuevoPeriodoModal onClose={() => setModal(null)} onDone={(p) => listo(`Periodo ${p.nombre} registrado.`)} />}
            {modal?.tipo === "seccion" && (
                <NuevaSeccionModal
                    periodo={modal.periodo}
                    onClose={() => setModal(null)}
                    onDone={() => {
                        setAbierto(modal.periodo.id);
                        listo("Sección registrada.");
                    }}
                />
            )}
            {modal?.tipo === "estado" && (
                <AccionModal
                    title={ESTADOS_PERIODO[modal.periodo.estado].accion}
                    submitLabel="Confirmar"
                    danger={modal.periodo.estado === "EN_CURSO"}
                    onClose={() => setModal(null)}
                    onSubmit={async () => {
                        await changeEstadoPeriodo(modal.periodo.id, ESTADOS_PERIODO[modal.periodo.estado].siguiente);
                        listo("Estado del periodo actualizado.");
                    }}
                >
                    <p>
                        {modal.periodo.estado === "EN_CURSO"
                            ? `Al cerrar ${modal.periodo.nombre} ya no se podrán registrar secciones ni cambiar matrículas (solo concluirlas). El cierre no se puede deshacer.`
                            : `${modal.periodo.nombre} pasará a «En curso».`}
                    </p>
                </AccionModal>
            )}
        </RoleLayout>
    );
}

export default PeriodosSecciones;
