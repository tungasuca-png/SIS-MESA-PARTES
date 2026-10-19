import { useEffect, useState } from "react";
import AccionModal from "../AccionModal";
import { usePermissions } from "../../../hooks/usePermissions";
import {
    actualizarFamiliar,
    cambiarParentescoFamiliar,
    desvincularFamiliar,
    getFamiliaresEstudiante,
    searchPersonas,
    updatePersona,
    vincularFamiliar,
} from "../../../services/estudiantesService";
import { MAX_APELLIDO, MAX_NOMBRES, PARENTESCOS, TIPOS_DOCUMENTO, parentescoLabel, tipoDocumentoLabel } from "../../../constants/estudiantes";
import { estudianteError } from "../../../utils/estudiantesErrors";
import { enmascararDocumento } from "../../../utils/privacidad";

const nombrePersona = (p) => [p?.nombres, p?.apellido_paterno, p?.apellido_materno].filter(Boolean).join(" ");
const hoy = () => new Date().toISOString().slice(0, 10);
const PERSONA_VACIA = { tipo_documento: "DNI", numero_documento: "", nombres: "", apellido_paterno: "", apellido_materno: "" };

// Validación de forma (la de verdad la hace el backend: documento único,
// nombres coincidentes, un padre/madre y un apoderado vigentes).
function validarPersona(p) {
    if (!p.nombres.trim()) return "Ingresa los nombres de la persona.";
    if (p.nombres.trim().length > MAX_NOMBRES) return "Los nombres son demasiado largos.";
    if (p.apellido_paterno.length > MAX_APELLIDO || p.apellido_materno.length > MAX_APELLIDO) return "Un apellido es demasiado largo.";
    const numero = p.numero_documento.trim();
    if (p.tipo_documento === "SIN_DOCUMENTO") return numero ? "Sin documento no lleva número." : null;
    if (!numero) return "Ingresa el número de documento o elige «Sin documento».";
    if (p.tipo_documento === "DNI" && !/^[0-9]{8}$/.test(numero)) return "El DNI debe tener 8 dígitos.";
    if (p.tipo_documento !== "DNI" && !/^[A-Za-z0-9]{4,12}$/.test(numero)) return "El documento debe tener de 4 a 12 letras o números.";
    return null;
}

function CamposPersona({ persona, onChange }) {
    const set = (campo) => (event) => onChange({ ...persona, [campo]: event.target.value });
    return (
        <>
            <div className="est-form-row">
                <div className="dp-form-group">
                    <label htmlFor="per-tipo">Tipo de documento</label>
                    <select id="per-tipo" value={persona.tipo_documento} onChange={set("tipo_documento")}>
                        {TIPOS_DOCUMENTO.map((t) => (
                            <option key={t.value} value={t.value}>
                                {t.label}
                            </option>
                        ))}
                    </select>
                </div>
                <div className="dp-form-group">
                    <label htmlFor="per-numero">Número de documento</label>
                    <input
                        id="per-numero"
                        value={persona.numero_documento}
                        disabled={persona.tipo_documento === "SIN_DOCUMENTO"}
                        onChange={set("numero_documento")}
                        autoComplete="off"
                    />
                </div>
            </div>
            <div className="dp-form-group">
                <label htmlFor="per-nombres">Nombres *</label>
                <input id="per-nombres" value={persona.nombres} onChange={set("nombres")} maxLength={MAX_NOMBRES} />
            </div>
            <div className="est-form-row">
                <div className="dp-form-group">
                    <label htmlFor="per-paterno">Apellido paterno</label>
                    <input id="per-paterno" value={persona.apellido_paterno} onChange={set("apellido_paterno")} maxLength={MAX_APELLIDO} />
                </div>
                <div className="dp-form-group">
                    <label htmlFor="per-materno">Apellido materno</label>
                    <input id="per-materno" value={persona.apellido_materno} onChange={set("apellido_materno")} maxLength={MAX_APELLIDO} />
                </div>
            </div>
        </>
    );
}

const personaPayload = (p) => ({
    tipo_documento: p.tipo_documento,
    numero_documento: p.tipo_documento === "SIN_DOCUMENTO" ? "" : p.numero_documento.trim(),
    nombres: p.nombres.trim(),
    apellido_paterno: p.apellido_paterno.trim(),
    apellido_materno: p.apellido_materno.trim(),
});

// Vincular: persona ya registrada (buscada) o persona nueva. Con documento,
// el backend reutiliza a la persona existente si los nombres coinciden.
function VincularModal({ estudianteId, puedeBuscar, onClose, onDone }) {
    const [modo, setModo] = useState(puedeBuscar ? "buscar" : "nueva");
    const [q, setQ] = useState("");
    const [resultados, setResultados] = useState(null);
    const [buscando, setBuscando] = useState(false);
    const [errorBusqueda, setErrorBusqueda] = useState("");
    const [elegida, setElegida] = useState(null);
    const [persona, setPersona] = useState(PERSONA_VACIA);
    const [parentesco, setParentesco] = useState("");
    const [esApoderado, setEsApoderado] = useState(false);
    const [desde, setDesde] = useState("");

    const buscar = async () => {
        if (q.trim().length < 2) {
            setErrorBusqueda("Escribe al menos 2 caracteres.");
            return;
        }
        setBuscando(true);
        setErrorBusqueda("");
        try {
            const data = await searchPersonas({ q: q.trim(), pageSize: 10 });
            setResultados(data.personas || []);
        } catch (err) {
            setErrorBusqueda(estudianteError(err).message);
        } finally {
            setBuscando(false);
        }
    };

    return (
        <AccionModal
            title="Vincular responsable"
            submitLabel="Vincular"
            size="lg"
            onClose={onClose}
            validate={() => {
                if (!parentesco) return "Elige el parentesco.";
                if (modo === "buscar") return elegida ? null : "Busca y selecciona a la persona.";
                return validarPersona(persona);
            }}
            onSubmit={async () => {
                const data = await vincularFamiliar(estudianteId, {
                    personaId: modo === "buscar" ? elegida.id : undefined,
                    persona: modo === "nueva" ? personaPayload(persona) : undefined,
                    parentesco,
                    esApoderado,
                    desde,
                });
                onDone(`${nombrePersona(data.familiar.persona)} quedó vinculado(a) como ${parentescoLabel(data.familiar.parentesco).toLowerCase()}.`);
            }}
        >
            <div className="est-segmented" role="radiogroup" aria-label="Persona">
                {puedeBuscar && (
                    <button type="button" role="radio" aria-checked={modo === "buscar"} className={modo === "buscar" ? "is-active" : ""} onClick={() => setModo("buscar")}>
                        Buscar persona registrada
                    </button>
                )}
                <button type="button" role="radio" aria-checked={modo === "nueva"} className={modo === "nueva" ? "is-active" : ""} onClick={() => setModo("nueva")}>
                    Registrar nueva persona
                </button>
            </div>

            {modo === "buscar" ? (
                <>
                    <div className="est-inline-search">
                        <input
                            type="search"
                            aria-label="Buscar persona por documento o nombre"
                            placeholder="Documento, nombres o apellidos"
                            value={q}
                            onChange={(e) => setQ(e.target.value)}
                            onKeyDown={(e) => {
                                if (e.key === "Enter") {
                                    e.preventDefault();
                                    buscar();
                                }
                            }}
                        />
                        <button type="button" className="dp-btn-secondary" onClick={buscar} disabled={buscando}>
                            {buscando ? "Buscando..." : "Buscar"}
                        </button>
                    </div>
                    {errorBusqueda && <p className="dp-form-error">{errorBusqueda}</p>}
                    {resultados && resultados.length === 0 && (
                        <p className="est-form-note">No hay personas con ese criterio. Puedes registrarla como nueva.</p>
                    )}
                    {resultados && resultados.length > 0 && (
                        <ul className="est-pick-list">
                            {resultados.map((p) => (
                                <li key={p.id}>
                                    <label>
                                        <input type="radio" name="persona" checked={elegida?.id === p.id} onChange={() => setElegida(p)} />
                                        <span>
                                            <strong>{nombrePersona(p)}</strong>
                                            <small>
                                                {tipoDocumentoLabel(p.tipo_documento)} {enmascararDocumento(p.numero_documento)}
                                            </small>
                                        </span>
                                    </label>
                                </li>
                            ))}
                        </ul>
                    )}
                </>
            ) : (
                <CamposPersona persona={persona} onChange={setPersona} />
            )}

            <div className="est-form-row">
                <div className="dp-form-group">
                    <label htmlFor="vin-parentesco">Parentesco *</label>
                    <select id="vin-parentesco" value={parentesco} onChange={(e) => setParentesco(e.target.value)}>
                        <option value="">Selecciona...</option>
                        {PARENTESCOS.map((p) => (
                            <option key={p.value} value={p.value}>
                                {p.label}
                            </option>
                        ))}
                    </select>
                </div>
                <div className="dp-form-group">
                    <label htmlFor="vin-desde">Vigente desde</label>
                    <input id="vin-desde" type="date" max={hoy()} value={desde} onChange={(e) => setDesde(e.target.value)} />
                </div>
            </div>
            <label className="est-check">
                <input type="checkbox" checked={esApoderado} onChange={(e) => setEsApoderado(e.target.checked)} /> Es el apoderado del estudiante
            </label>
            <p className="est-form-note">Solo puede haber un apoderado vigente, y un solo padre y una sola madre vigentes.</p>
        </AccionModal>
    );
}

function FamiliaTab({ estudianteId, onCambio }) {
    const { can } = usePermissions();
    const canUpdate = can("estudiantes.familia.update");
    const canBuscar = can("estudiantes.familia.view");

    const [historial, setHistorial] = useState(false);
    const [reload, setReload] = useState(0);
    const [estado, setEstado] = useState({ key: null, familiares: [], error: null });
    const [modal, setModal] = useState(null);
    const [mensaje, setMensaje] = useState("");
    const [visibles, setVisibles] = useState(() => new Set());
    const requestKey = `${estudianteId}|${historial}|${reload}`;

    useEffect(() => {
        let active = true;
        getFamiliaresEstudiante(estudianteId, { historial })
            .then((data) => active && setEstado({ key: requestKey, familiares: data.familiares || [], error: null }))
            .catch((err) => active && setEstado({ key: requestKey, familiares: [], error: estudianteError(err) }));
        return () => {
            active = false;
        };
    }, [estudianteId, historial, requestKey]);

    const loading = estado.key !== requestKey;
    const listo = (texto) => {
        setModal(null);
        setMensaje(texto);
        setReload((n) => n + 1);
        onCambio?.();
    };
    const recargar = () => {
        setModal(null);
        setReload((n) => n + 1);
    };
    const toggleVisible = (id) =>
        setVisibles((current) => {
            const next = new Set(current);
            if (next.has(id)) next.delete(id);
            else next.add(id);
            return next;
        });

    return (
        <section className="dp-panel">
            <div className="dp-detail-header">
                <div>
                    <h3 className="dp-panel-title">Padres y apoderados</h3>
                    <p className="est-form-note">Cada consulta queda registrada en la auditoría del estudiante.</p>
                </div>
                <div className="est-detail-actions">
                    <label className="est-check">
                        <input type="checkbox" checked={historial} onChange={(e) => setHistorial(e.target.checked)} /> Incluir relaciones cerradas
                    </label>
                    {canUpdate && (
                        <button type="button" className="dp-btn-primary" onClick={() => setModal({ tipo: "vincular" })}>
                            Vincular responsable
                        </button>
                    )}
                </div>
            </div>
            {mensaje && <p className="est-banner-success">{mensaje}</p>}

            {loading && <p className="dp-table-empty">Cargando responsables...</p>}
            {!loading && estado.error && (
                <p className="dp-form-error">
                    {estado.error.kind === "forbidden" ? "No tienes permiso para ver los padres y apoderados." : estado.error.message}
                </p>
            )}
            {!loading && !estado.error && estado.familiares.length === 0 && (
                <p className="dp-table-empty">{historial ? "Sin relaciones registradas." : "Sin padres ni apoderados vigentes."}</p>
            )}
            {!loading && !estado.error && estado.familiares.length > 0 && (
                <ul className="est-card-list">
                    {estado.familiares.map((f) => (
                        <li key={f.relacion_id} className={f.vigente ? "est-card" : "est-card est-card--cerrada"}>
                            <div className="est-card-head">
                                <div>
                                    <p className="est-card-title">{nombrePersona(f.persona)}</p>
                                    <p className="est-card-sub">
                                        {parentescoLabel(f.parentesco)}
                                        {f.desde ? ` · desde ${f.desde}` : ""}
                                        {!f.vigente && ` · cerrada el ${f.hasta}`}
                                    </p>
                                </div>
                                <div className="est-chips">
                                    {f.es_apoderado && <span className="est-chip est-chip--accent">Apoderado</span>}
                                    <span className={f.vigente ? "est-chip est-chip--ok" : "est-chip"}>{f.vigente ? "Vigente" : "Histórica"}</span>
                                </div>
                            </div>
                            <dl className="est-card-data">
                                <div>
                                    <dt>Documento</dt>
                                    <dd>
                                        {tipoDocumentoLabel(f.persona.tipo_documento)}{" "}
                                        {f.persona.numero_documento &&
                                            (visibles.has(f.relacion_id) ? f.persona.numero_documento : enmascararDocumento(f.persona.numero_documento))}
                                        {f.persona.numero_documento && (
                                            <button type="button" className="est-link-button est-reveal" onClick={() => toggleVisible(f.relacion_id)}>
                                                {visibles.has(f.relacion_id) ? "Ocultar" : "Mostrar"}
                                            </button>
                                        )}
                                    </dd>
                                </div>
                                <div>
                                    <dt>Teléfono y correo</dt>
                                    <dd className="est-muted">No se registran todavía en el sistema</dd>
                                </div>
                            </dl>
                            {canUpdate && f.vigente && (
                                <div className="est-card-actions">
                                    <button
                                        type="button"
                                        className="dp-btn-secondary"
                                        onClick={() => setModal({ tipo: "apoderado", familiar: f })}
                                    >
                                        {f.es_apoderado ? "Quitar rol de apoderado" : "Designar apoderado"}
                                    </button>
                                    <button type="button" className="dp-btn-secondary" onClick={() => setModal({ tipo: "parentesco", familiar: f })}>
                                        Cambiar parentesco
                                    </button>
                                    <button type="button" className="dp-btn-secondary" onClick={() => setModal({ tipo: "editar", familiar: f })}>
                                        Corregir datos
                                    </button>
                                    <button type="button" className="dp-btn-secondary est-btn-danger-text" onClick={() => setModal({ tipo: "desvincular", familiar: f })}>
                                        Desvincular
                                    </button>
                                </div>
                            )}
                        </li>
                    ))}
                </ul>
            )}

            {modal?.tipo === "vincular" && (
                <VincularModal estudianteId={estudianteId} puedeBuscar={canBuscar} onClose={() => setModal(null)} onDone={listo} />
            )}
            {modal?.tipo === "apoderado" && (
                <AccionModal
                    title={modal.familiar.es_apoderado ? "Quitar rol de apoderado" : "Designar apoderado"}
                    submitLabel="Confirmar"
                    onClose={() => setModal(null)}
                    onSubmit={async () => {
                        await actualizarFamiliar(estudianteId, modal.familiar.relacion_id, { esApoderado: !modal.familiar.es_apoderado });
                        listo(modal.familiar.es_apoderado ? "Se quitó el rol de apoderado." : `${nombrePersona(modal.familiar.persona)} es ahora el apoderado.`);
                    }}
                >
                    <p>
                        {modal.familiar.es_apoderado
                            ? `${nombrePersona(modal.familiar.persona)} dejará de ser el apoderado. La relación familiar se mantiene.`
                            : `${nombrePersona(modal.familiar.persona)} será el apoderado del estudiante. Si ya hay otro apoderado vigente, primero quítale el rol.`}
                    </p>
                </AccionModal>
            )}
            {modal?.tipo === "parentesco" && <ParentescoModal estudianteId={estudianteId} familiar={modal.familiar} onClose={() => setModal(null)} onDone={listo} />}
            {modal?.tipo === "editar" && <EditarPersonaModal familiar={modal.familiar} onClose={() => setModal(null)} onDone={listo} onReload={recargar} />}
            {modal?.tipo === "desvincular" && <DesvincularModal estudianteId={estudianteId} familiar={modal.familiar} onClose={() => setModal(null)} onDone={listo} />}
        </section>
    );
}

function ParentescoModal({ estudianteId, familiar, onClose, onDone }) {
    const [parentesco, setParentesco] = useState("");
    const [desde, setDesde] = useState("");
    return (
        <AccionModal
            title="Cambiar parentesco"
            submitLabel="Cambiar parentesco"
            onClose={onClose}
            validate={() => (!parentesco ? "Elige el nuevo parentesco." : parentesco === familiar.parentesco ? "Elige un parentesco distinto al actual." : null)}
            onSubmit={async () => {
                const data = await cambiarParentescoFamiliar(estudianteId, familiar.relacion_id, { parentesco, desde });
                onDone(`Parentesco actualizado a ${parentescoLabel(data.familiar.parentesco).toLowerCase()}. La relación anterior quedó en el historial.`);
            }}
        >
            <p className="est-form-note">
                La relación actual ({parentescoLabel(familiar.parentesco)}) se cierra y se crea otra con la misma persona. El historial no se pierde.
            </p>
            <div className="est-form-row">
                <div className="dp-form-group">
                    <label htmlFor="par-nuevo">Nuevo parentesco *</label>
                    <select id="par-nuevo" value={parentesco} onChange={(e) => setParentesco(e.target.value)}>
                        <option value="">Selecciona...</option>
                        {PARENTESCOS.filter((p) => p.value !== familiar.parentesco).map((p) => (
                            <option key={p.value} value={p.value}>
                                {p.label}
                            </option>
                        ))}
                    </select>
                </div>
                <div className="dp-form-group">
                    <label htmlFor="par-desde">Desde</label>
                    <input id="par-desde" type="date" max={hoy()} value={desde} onChange={(e) => setDesde(e.target.value)} />
                </div>
            </div>
        </AccionModal>
    );
}

function EditarPersonaModal({ familiar, onClose, onDone, onReload }) {
    const original = {
        tipo_documento: familiar.persona.tipo_documento || "SIN_DOCUMENTO",
        numero_documento: familiar.persona.numero_documento || "",
        nombres: familiar.persona.nombres || "",
        apellido_paterno: familiar.persona.apellido_paterno || "",
        apellido_materno: familiar.persona.apellido_materno || "",
    };
    const [persona, setPersona] = useState(original);
    const cambios = () => {
        const final = personaPayload(persona);
        return Object.fromEntries(Object.entries(final).filter(([k, v]) => v !== original[k]));
    };
    return (
        <AccionModal
            title="Corregir datos de la persona"
            submitLabel="Guardar cambios"
            size="lg"
            onClose={onClose}
            onReload={onReload}
            validate={() => validarPersona(persona) || (Object.keys(cambios()).length === 0 ? "No hay cambios para guardar." : null)}
            onSubmit={async () => {
                await updatePersona(familiar.persona.id, { ...cambios(), expected_version: familiar.persona.version });
                onDone("Datos de la persona actualizados.");
            }}
        >
            <p className="est-form-note">El cambio se aplica a la persona en todos los estudiantes con los que está vinculada.</p>
            <CamposPersona persona={persona} onChange={setPersona} />
        </AccionModal>
    );
}

function DesvincularModal({ estudianteId, familiar, onClose, onDone }) {
    const [hasta, setHasta] = useState("");
    return (
        <AccionModal
            title="Desvincular responsable"
            submitLabel="Desvincular"
            danger
            onClose={onClose}
            onSubmit={async () => {
                await desvincularFamiliar(estudianteId, familiar.relacion_id, { hasta });
                onDone(`${nombrePersona(familiar.persona)} fue desvinculado(a). La relación queda en el historial.`);
            }}
        >
            <p>
                Se cerrará la relación de <strong>{nombrePersona(familiar.persona)}</strong> ({parentescoLabel(familiar.parentesco)}). No se borra:
                queda en el historial y la persona sigue registrada.
            </p>
            <div className="dp-form-group">
                <label htmlFor="des-hasta">Fecha de cierre</label>
                <input id="des-hasta" type="date" max={hoy()} value={hasta} onChange={(e) => setHasta(e.target.value)} />
                <span className="dp-form-hint">Vacío = hoy.</span>
            </div>
        </AccionModal>
    );
}

export default FamiliaTab;
