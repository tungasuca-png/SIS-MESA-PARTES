import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import AccionModal from "../AccionModal";
import { usePermissions } from "../../../hooks/usePermissions";
import {
    cambiarSeccionMatricula,
    concluirMatricula,
    listPeriodos,
    listSecciones,
    matricularEstudiante,
    reingresarMatricula,
    retirarMatricula,
} from "../../../services/estudiantesService";
import { ESTADOS_PERIODO, MAX_MOTIVO, MOVIMIENTOS_MATRICULA, estadoMatriculaLabel } from "../../../constants/estudiantes";
import { estudianteError } from "../../../utils/estudiantesErrors";
import { seccionLabel } from "../../../utils/matricula";

const hoy = () => new Date().toISOString().slice(0, 10);
const ocupacion = (s) => (s.vacantes == null ? `${s.matriculados} matriculados` : `${s.matriculados}/${s.vacantes} vacantes ocupadas`);

// Carga las secciones de un periodo para un <select> (estudiantes.view).
function useSecciones(periodoId) {
    const [estado, setEstado] = useState({ key: null, secciones: [], error: null });
    useEffect(() => {
        if (!periodoId) return undefined;
        let active = true;
        listSecciones(periodoId)
            .then((data) => active && setEstado({ key: periodoId, secciones: data.secciones || [], error: null }))
            .catch((err) => active && setEstado({ key: periodoId, secciones: [], error: estudianteError(err) }));
        return () => {
            active = false;
        };
    }, [periodoId]);
    return { ...estado, loading: Boolean(periodoId) && estado.key !== periodoId };
}

function SelectSeccion({ periodoId, value, onChange, excluir, opcional }) {
    const { secciones, loading, error } = useSecciones(periodoId);
    if (!periodoId) return null;
    return (
        <div className="dp-form-group">
            <label htmlFor="mat-seccion">Sección{opcional ? "" : " *"}</label>
            <select id="mat-seccion" value={value} onChange={(e) => onChange(e.target.value)} disabled={loading}>
                <option value="">{loading ? "Cargando secciones..." : opcional ? "Sin sección por ahora" : "Selecciona..."}</option>
                {secciones
                    .filter((s) => s.id !== excluir)
                    .map((s) => (
                        <option key={s.id} value={s.id}>
                            {seccionLabel(s)} — {ocupacion(s)}
                        </option>
                    ))}
            </select>
            {error && <span className="dp-form-error">{error.message}</span>}
            {!loading && !error && secciones.length === 0 && <span className="dp-form-hint">El periodo no tiene secciones registradas.</span>}
        </div>
    );
}

function MatricularModal({ estudianteId, ocupados, onClose, onDone }) {
    const [periodos, setPeriodos] = useState(null);
    const [errorPeriodos, setErrorPeriodos] = useState(null);
    const [periodoId, setPeriodoId] = useState("");
    const [seccionId, setSeccionId] = useState("");
    const [fecha, setFecha] = useState("");

    useEffect(() => {
        let active = true;
        listPeriodos()
            .then((data) => active && setPeriodos((data.periodos || []).filter((p) => p.estado !== "CERRADO" && !ocupados.split(",").includes(p.id))))
            .catch((err) => active && setErrorPeriodos(estudianteError(err)));
        return () => {
            active = false;
        };
    }, [ocupados]);

    return (
        <AccionModal
            title="Matricular en un periodo"
            submitLabel="Matricular"
            onClose={onClose}
            validate={() => (!periodoId ? "Elige el periodo académico." : null)}
            onSubmit={async () => {
                const data = await matricularEstudiante(estudianteId, { periodoId, seccionId, fechaMatricula: fecha });
                onDone(`Matrícula registrada en ${data.matricula.periodo.nombre}.`);
            }}
        >
            {errorPeriodos && <p className="dp-form-error">{errorPeriodos.message}</p>}
            {periodos && periodos.length === 0 && (
                <p className="est-form-note">No hay periodos abiertos disponibles para este estudiante. Regístralos en «Periodos y secciones».</p>
            )}
            <div className="dp-form-group">
                <label htmlFor="mat-periodo">Periodo académico *</label>
                <select
                    id="mat-periodo"
                    value={periodoId}
                    disabled={!periodos}
                    onChange={(e) => {
                        setPeriodoId(e.target.value);
                        setSeccionId("");
                    }}
                >
                    <option value="">{periodos ? "Selecciona..." : "Cargando periodos..."}</option>
                    {(periodos || []).map((p) => (
                        <option key={p.id} value={p.id}>
                            {p.nombre} ({ESTADOS_PERIODO[p.estado]?.label ?? p.estado})
                        </option>
                    ))}
                </select>
            </div>
            <SelectSeccion periodoId={periodoId} value={seccionId} onChange={setSeccionId} opcional />
            <div className="dp-form-group">
                <label htmlFor="mat-fecha">Fecha de matrícula</label>
                <input id="mat-fecha" type="date" max={hoy()} value={fecha} onChange={(e) => setFecha(e.target.value)} />
                <span className="dp-form-hint">Vacío = hoy.</span>
            </div>
        </AccionModal>
    );
}

function CampoFechaMotivo({ fecha, setFecha, motivo, setMotivo, motivoObligatorio }) {
    return (
        <>
            <div className="dp-form-group">
                <label htmlFor="op-fecha">Fecha</label>
                <input id="op-fecha" type="date" max={hoy()} value={fecha} onChange={(e) => setFecha(e.target.value)} />
                <span className="dp-form-hint">Vacío = hoy. No puede ser futura.</span>
            </div>
            {setMotivo && (
                <div className="dp-form-group">
                    <label htmlFor="op-motivo">Motivo{motivoObligatorio ? " *" : ""}</label>
                    <textarea id="op-motivo" rows={3} maxLength={MAX_MOTIVO} value={motivo} onChange={(e) => setMotivo(e.target.value)} />
                    <span className="dp-form-hint">Se guarda en el historial interno; no se muestra en la ficha.</span>
                </div>
            )}
        </>
    );
}

function OperacionModal({ tipo, matricula, onClose, onDone, onReload }) {
    const [fecha, setFecha] = useState("");
    const [motivo, setMotivo] = useState("");
    const [seccionId, setSeccionId] = useState("");
    const [retiro, setRetiro] = useState("RETIRO");
    const base = { expectedVersion: matricula.version, fecha };
    const periodo = matricula.periodo?.nombre;

    const config = {
        seccion: {
            title: matricula.seccion ? "Cambiar sección" : "Asignar sección",
            submit: "Guardar sección",
            validate: () => (!seccionId ? "Elige la sección." : null),
            run: () => cambiarSeccionMatricula(matricula.id, { ...base, seccionId, motivo }),
            ok: "Sección actualizada. El cambio quedó en el historial.",
        },
        retiro: {
            title: "Registrar retiro o traslado",
            submit: "Registrar",
            danger: true,
            validate: () => (!motivo.trim() ? "Indica el motivo." : null),
            run: () => retirarMatricula(matricula.id, { ...base, tipo: retiro, motivo: motivo.trim() }),
            ok: retiro === "RETIRO" ? "Retiro registrado." : "Traslado registrado.",
        },
        reingreso: {
            title: "Registrar reingreso",
            submit: "Registrar reingreso",
            run: () => reingresarMatricula(matricula.id, { ...base, motivo }),
            ok: "Reingreso registrado. El retiro anterior se conserva en el historial.",
        },
        conclusion: {
            title: "Concluir matrícula",
            submit: "Concluir",
            run: () => concluirMatricula(matricula.id, base),
            ok: "Matrícula concluida.",
        },
    }[tipo];

    return (
        <AccionModal
            title={config.title}
            submitLabel={config.submit}
            danger={config.danger}
            onClose={onClose}
            onReload={onReload}
            validate={config.validate}
            onSubmit={async () => {
                await config.run();
                onDone(config.ok);
            }}
        >
            <p className="est-form-note">
                Periodo {periodo} · {seccionLabel(matricula.seccion)} · {estadoMatriculaLabel(matricula.estado_matricula)}
            </p>
            {tipo === "seccion" && (
                <SelectSeccion periodoId={matricula.periodo?.id} value={seccionId} onChange={setSeccionId} excluir={matricula.seccion?.id} />
            )}
            {tipo === "retiro" && (
                <div className="est-segmented" role="radiogroup" aria-label="Tipo">
                    {[
                        ["RETIRO", "Retiro"],
                        ["TRASLADO", "Traslado a otra institución"],
                    ].map(([value, label]) => (
                        <button key={value} type="button" role="radio" aria-checked={retiro === value} className={retiro === value ? "is-active" : ""} onClick={() => setRetiro(value)}>
                            {label}
                        </button>
                    ))}
                </div>
            )}
            {tipo === "conclusion" && (
                <p className="est-form-note">
                    Indica que el estudiante terminó el periodo. No registra aprobación, promoción ni egreso: esas reglas aún no están definidas por la institución.
                </p>
            )}
            <CampoFechaMotivo
                fecha={fecha}
                setFecha={setFecha}
                motivo={motivo}
                setMotivo={tipo === "conclusion" ? null : setMotivo}
                motivoObligatorio={tipo === "retiro"}
            />
        </AccionModal>
    );
}

// Operaciones posibles según el estado (las mismas reglas que aplica el
// backend; aquí solo se evita ofrecer lo que de todos modos rechazaría).
function operaciones(m) {
    const cerrado = m.periodo?.estado === "CERRADO";
    const planificado = m.periodo?.estado === "PLANIFICADO";
    if (m.estado_matricula === "MATRICULADO") {
        if (cerrado) return ["conclusion"];
        return ["seccion", "retiro", ...(planificado ? [] : ["conclusion"])];
    }
    if ((m.estado_matricula === "RETIRADO" || m.estado_matricula === "TRASLADADO") && !cerrado) return ["reingreso"];
    return [];
}

const ETIQUETA_OPERACION = { seccion: "Cambiar sección", retiro: "Retiro o traslado", reingreso: "Reingreso", conclusion: "Concluir" };

function MatriculasTab({ estudianteId, estado, onCambio, estudianteActivo }) {
    const { can } = usePermissions();
    const canUpdate = can("estudiantes.matricula.update");
    const [modal, setModal] = useState(null);
    const [mensaje, setMensaje] = useState("");
    const { loading, matriculas, error } = estado;

    const listo = (texto) => {
        setModal(null);
        setMensaje(texto);
        onCambio();
    };

    return (
        <section className="dp-panel">
            <div className="dp-detail-header">
                <div>
                    <h3 className="dp-panel-title">Historial académico</h3>
                    <p className="est-form-note">Una matrícula por periodo, más reciente primero. Ningún cambio borra lo anterior.</p>
                </div>
                <div className="est-detail-actions">
                    <Link to="/estudiantes/periodos" className="dp-btn-secondary est-btn-link">
                        Periodos y secciones
                    </Link>
                    {canUpdate && (
                        <button
                            type="button"
                            className="dp-btn-primary"
                            disabled={!estudianteActivo}
                            title={estudianteActivo ? undefined : "Solo se matricula a estudiantes con registro activo"}
                            onClick={() => setModal({ tipo: "matricular" })}
                        >
                            Matricular
                        </button>
                    )}
                </div>
            </div>
            {mensaje && <p className="est-banner-success">{mensaje}</p>}
            {loading && <p className="dp-table-empty">Cargando matrículas...</p>}
            {!loading && error && <p className="dp-form-error">{error.message}</p>}
            {!loading && !error && matriculas.length === 0 && <p className="dp-table-empty">Sin matrículas registradas.</p>}
            {!loading && !error && matriculas.length > 0 && (
                <ol className="est-trayectoria">
                    {matriculas.map((m) => (
                        <li key={m.id} className="est-card">
                            <div className="est-card-head">
                                <div>
                                    <p className="est-card-title">
                                        {m.periodo?.nombre} <span className="est-muted">({ESTADOS_PERIODO[m.periodo?.estado]?.label ?? m.periodo?.estado})</span>
                                    </p>
                                    <p className="est-card-sub">
                                        {seccionLabel(m.seccion)}
                                        {m.seccion?.aula ? ` · ${m.seccion.aula}` : ""}
                                    </p>
                                </div>
                                <span className={`est-chip est-chip--mat-${(m.estado_matricula || "").toLowerCase()}`}>{estadoMatriculaLabel(m.estado_matricula)}</span>
                            </div>
                            <dl className="est-card-data">
                                <div>
                                    <dt>Fecha de matrícula</dt>
                                    <dd>{m.fecha_matricula || "—"}</dd>
                                </div>
                                {m.fecha_retiro && (
                                    <div>
                                        <dt>{m.estado_matricula === "TRASLADADO" ? "Fecha de traslado" : "Fecha de retiro"}</dt>
                                        <dd>{m.fecha_retiro}</dd>
                                    </div>
                                )}
                                {m.origen === "MIGRACION_LEGADO" && (
                                    <div>
                                        <dt>Origen</dt>
                                        <dd>Cargada desde datos anteriores</dd>
                                    </div>
                                )}
                            </dl>
                            {(m.movimientos || []).length > 0 && (
                                <ul className="est-movimientos" aria-label={`Movimientos de ${m.periodo?.nombre}`}>
                                    {m.movimientos.map((mv, i) => (
                                        <li key={i}>
                                            <span>{mv.fecha}</span> {MOVIMIENTOS_MATRICULA[mv.tipo] ?? mv.tipo}
                                        </li>
                                    ))}
                                </ul>
                            )}
                            {canUpdate && operaciones(m).length > 0 && (
                                <div className="est-card-actions">
                                    {operaciones(m).map((op) => (
                                        <button
                                            key={op}
                                            type="button"
                                            className={op === "retiro" ? "dp-btn-secondary est-btn-danger-text" : "dp-btn-secondary"}
                                            onClick={() => setModal({ tipo: op, matricula: m })}
                                        >
                                            {op === "seccion" && !m.seccion ? "Asignar sección" : ETIQUETA_OPERACION[op]}
                                        </button>
                                    ))}
                                </div>
                            )}
                        </li>
                    ))}
                </ol>
            )}

            {modal?.tipo === "matricular" && (
                <MatricularModal
                    estudianteId={estudianteId}
                    ocupados={matriculas.map((m) => m.periodo?.id).join(",")}
                    onClose={() => setModal(null)}
                    onDone={listo}
                />
            )}
            {modal && modal.tipo !== "matricular" && (
                <OperacionModal
                    tipo={modal.tipo}
                    matricula={modal.matricula}
                    onClose={() => setModal(null)}
                    onDone={listo}
                    onReload={() => {
                        setModal(null);
                        onCambio();
                    }}
                />
            )}
        </section>
    );
}

export default MatriculasTab;
