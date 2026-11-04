import { useEffect, useState } from "react";
import DashboardLayout from "../../components/dashboard/DashboardLayout";
import { listIdentidades, reconciliarIdentidades, rechazarIdentidad, verificarIdentidad } from "../../services/identidadService";
import { extractBackendMessage } from "../../utils/apiErrors";
import "../../components/dashboard/forms.css";
import "../../components/dashboard/RecentExpedientes.css";
import "./IdentidadSolicitantes.css";

// Bandeja de verificación de identidad (Secretaría, permiso
// solicitantes.identidad.verify). El registro público solo DECLARA el DNI:
// aquí el personal lo coteja (documento a la vista o copia adjunta a un
// trámite) antes de que quede como DNI definitivo de la cuenta. Un DNI que ya
// pertenece a otra cuenta se marca como conflicto y nunca se sobrescribe.
const ESTADOS = [
    { id: "PENDIENTE", label: "Pendientes" },
    { id: "VERIFICADA", label: "Verificadas" },
    { id: "RECHAZADA", label: "Rechazadas" },
];
const POR_PAGINA = 50;
const METODOS = { PRESENCIAL: "Presencial (documento a la vista)", DOCUMENTO: "Documento adjunto a un trámite" };

function mensajeError(err) {
    if (!err?.response) return "No se pudo conectar con el servidor. Verifica tu conexión e intenta nuevamente.";
    const { status, data } = err.response;
    const desc = extractBackendMessage(data) ?? "";
    const m = desc.match(/^identidad_[a-z_]+:\s*(.+)$/s);
    if (m) return m[1];
    if (status === 403) return "No tienes permiso para verificar identidades.";
    if (status === 404) return "El solicitante ya no tiene una identidad pendiente.";
    if (status >= 500) return "El servicio no está disponible en este momento. Inténtalo nuevamente.";
    return "No se pudo completar la operación.";
}

function fecha(valor) {
    if (!valor) return "—";
    const d = new Date(valor);
    return Number.isNaN(d.getTime()) ? "—" : d.toLocaleDateString("es-PE", { day: "2-digit", month: "2-digit", year: "numeric" });
}

function Fila({ item, ocupado, onVerificar, onRechazar }) {
    const [accion, setAccion] = useState(null);
    const [metodo, setMetodo] = useState("PRESENCIAL");
    const [motivo, setMotivo] = useState("");
    const resoluble = item.estado !== "VERIFICADA";

    return (
        <tr>
            <td>{item.nombre_completo || <span className="dp-usuario-fallback">Sin perfil registrado</span>}</td>
            <td className="id-dni">{item.dni_declarado}</td>
            <td>{item.origen === "google" ? "Google" : "Correo"}</td>
            <td>{fecha(item.created_at)}</td>
            <td>
                {item.conflicto && item.estado !== "VERIFICADA" ? (
                    <span className="id-conflicto" title="El DNI declarado ya está asignado a otra cuenta">DNI asignado a otra cuenta</span>
                ) : item.estado === "VERIFICADA" && item.sincronizacion === "PERFIL_PENDIENTE" ? (
                    <span title={item.sincronizacion_error || undefined}>
                        Verificada; perfil pendiente de actualizar
                        {item.sincronizacion_error ? ` (${item.sincronizacion_error})` : ""}
                    </span>
                ) : item.estado === "VERIFICADA" ? (
                    `Verificada el ${fecha(item.verificado_at)}`
                ) : item.estado === "RECHAZADA" ? (
                    `Rechazada: ${item.motivo_rechazo}`
                ) : (
                    "Pendiente"
                )}
            </td>
            <td>
                {resoluble && (
                    <div className="id-acciones">
                        <button type="button" className="id-btn" onClick={() => setAccion(accion === "verificar" ? null : "verificar")}
                            disabled={ocupado || item.conflicto}>
                            Verificar
                        </button>
                        {item.estado === "PENDIENTE" && (
                            <button type="button" className="id-btn id-btn--peligro" onClick={() => setAccion(accion === "rechazar" ? null : "rechazar")}
                                disabled={ocupado}>
                                Rechazar
                            </button>
                        )}
                    </div>
                )}
                {accion === "verificar" && (
                    <form className="id-formulario" onSubmit={(e) => { e.preventDefault(); onVerificar(item, metodo); }}
                        aria-label={`Verificar la identidad de ${item.nombre_completo || "el solicitante"}`}>
                        <label>
                            Cómo se cotejó el DNI {item.dni_declarado}
                            <select value={metodo} onChange={(e) => setMetodo(e.target.value)} disabled={ocupado}>
                                {Object.entries(METODOS).map(([valor, texto]) => (
                                    <option key={valor} value={valor}>{texto}</option>
                                ))}
                            </select>
                        </label>
                        <button type="submit" className="dp-btn-primary" disabled={ocupado}>Confirmar verificación</button>
                    </form>
                )}
                {accion === "rechazar" && (
                    <form className="id-formulario" onSubmit={(e) => { e.preventDefault(); onRechazar(item, motivo.trim()); }}
                        aria-label={`Rechazar la identidad de ${item.nombre_completo || "el solicitante"}`}>
                        <label>
                            Motivo del rechazo
                            <textarea value={motivo} onChange={(e) => setMotivo(e.target.value)} maxLength={200} rows={2} disabled={ocupado} />
                        </label>
                        <button type="submit" className="id-btn id-btn--peligro" disabled={ocupado || motivo.trim().length < 3}>
                            Confirmar rechazo
                        </button>
                    </form>
                )}
            </td>
        </tr>
    );
}

function IdentidadSolicitantes() {
    const [estado, setEstado] = useState("PENDIENTE");
    const [pagina, setPagina] = useState(0);
    const [version, setVersion] = useState(0);
    // Resultado de la última consulta, con la clave que la originó: mientras
    // la clave no coincide con la actual, la bandeja se muestra "cargando".
    const [resultado, setResultado] = useState({ clave: "", items: [], total: 0, error: "" });
    const [error, setError] = useState("");
    const [aviso, setAviso] = useState("");
    const [ocupado, setOcupado] = useState(false);
    const clave = `${estado}|${pagina}|${version}`;

    useEffect(() => {
        let activo = true;
        listIdentidades({ estado, limit: POR_PAGINA, offset: pagina * POR_PAGINA })
            .then((r) => activo && setResultado({ clave, items: r.items ?? [], total: r.total ?? 0, error: "" }))
            .catch((err) => activo && setResultado({ clave, items: [], total: 0, error: mensajeError(err) }));
        return () => {
            activo = false;
        };
    }, [clave, estado, pagina]);

    const cargando = resultado.clave !== clave;
    const datos = cargando ? { items: [], total: 0 } : resultado;
    const errorVisible = error || (cargando ? "" : resultado.error);

    const ejecutar = async (operacion, exito) => {
        if (ocupado) return;
        setOcupado(true);
        setError("");
        setAviso("");
        try {
            const r = await operacion();
            setAviso(typeof exito === "function" ? exito(r) : exito);
            setVersion((v) => v + 1);
        } catch (err) {
            setError(mensajeError(err));
        } finally {
            setOcupado(false);
        }
    };

    const paginas = Math.max(1, Math.ceil(datos.total / POR_PAGINA));

    return (
        <DashboardLayout title="Identidad de solicitantes">
            <section className="dp-panel">
                <div className="id-header">
                    <div>
                        <h2 className="dp-panel-title">Identidad de solicitantes</h2>
                        <p className="id-subtitle">
                            El registro público solo declara el DNI. Verifícalo con el documento a la vista o con la copia adjunta a un
                            trámite. Hasta entonces el solicitante ingresa con su correo o Google y sus trámites se marcan como
                            «Identidad no verificada».
                        </p>
                    </div>
                </div>

                <div className="id-tabs" role="tablist" aria-label="Estado de la identidad">
                    {ESTADOS.map((e) => (
                        <button key={e.id} type="button" role="tab" className="id-tab" aria-selected={estado === e.id}
                            onClick={() => { setEstado(e.id); setPagina(0); setAviso(""); }}>
                            {e.label}
                        </button>
                    ))}
                </div>

                {estado === "VERIFICADA" && (
                    <div className="id-acciones" style={{ marginBottom: 12 }}>
                        <button type="button" className="id-btn" disabled={ocupado}
                            onClick={() => ejecutar(reconciliarIdentidades, (r) =>
                                `Sincronización: ${r?.completadas ?? 0} completadas, ${r?.revertidas ?? 0} revertidas por conflicto, ${r?.pendientes ?? 0} pendientes.`)}>
                            Reintentar sincronización de perfiles
                        </button>
                    </div>
                )}

                {aviso && <p className="id-banner id-banner--ok" role="status">{aviso}</p>}
                {errorVisible && <p className="dp-form-error" role="alert">{errorVisible}</p>}

                {cargando ? (
                    <p className="dp-table-empty">Cargando solicitudes...</p>
                ) : datos.items.length === 0 ? (
                    !errorVisible && <p className="dp-table-empty">No hay solicitudes en este estado.</p>
                ) : (
                    <div className="dp-table-wrapper">
                        <table className="dp-table">
                            <thead>
                                <tr>
                                    <th>Solicitante</th>
                                    <th>DNI declarado</th>
                                    <th>Registro</th>
                                    <th>Fecha</th>
                                    <th>Estado</th>
                                    <th>Acciones</th>
                                </tr>
                            </thead>
                            <tbody>
                                {datos.items.map((item) => (
                                    <Fila
                                        key={item.usuario_id}
                                        item={item}
                                        ocupado={ocupado}
                                        onVerificar={(i, metodo) => ejecutar(() => verificarIdentidad(i.usuario_id, metodo), (r) =>
                                            r?.sincronizacion === "PERFIL_PENDIENTE"
                                                ? "Identidad verificada. El perfil se actualizará cuando el servicio de usuarios responda (usa «Reintentar sincronización»)."
                                                : "Identidad verificada. El solicitante ya puede ingresar también con su DNI.")}
                                        onRechazar={(i, motivo) => ejecutar(() => rechazarIdentidad(i.usuario_id, motivo), "Identidad rechazada.")}
                                    />
                                ))}
                            </tbody>
                        </table>
                    </div>
                )}

                {datos.total > POR_PAGINA && (
                    <div className="id-paginacion">
                        <button type="button" className="id-btn" disabled={pagina === 0 || cargando} onClick={() => setPagina((p) => p - 1)}>
                            Anterior
                        </button>
                        <span>Página {pagina + 1} de {paginas}</span>
                        <button type="button" className="id-btn" disabled={pagina + 1 >= paginas || cargando} onClick={() => setPagina((p) => p + 1)}>
                            Siguiente
                        </button>
                    </div>
                )}
            </section>
        </DashboardLayout>
    );
}

export default IdentidadSolicitantes;
