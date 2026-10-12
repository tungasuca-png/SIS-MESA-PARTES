import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import RoleLayout from "../../components/portal/RoleLayout";
import HojaAtencion from "../../components/psicologia/HojaAtencion";
import AtencionEstadoBadge from "../../components/psicologia/AtencionEstadoBadge";
import AccionModal from "../../components/psicologia/AccionModal";
import NombreResuelto from "../../components/psicologia/NombreResuelto";
import { usePermissions } from "../../hooks/usePermissions";
import { useUsuariosBasic } from "../../hooks/useUsuariosBasic";
import { useEstudiantesBasic } from "../../hooks/useEstudiantesBasic";
import {
    annulAtencion,
    correctAtencion,
    createAtencionDraft,
    discardAtencionDraft,
    exportAtencionPdf,
    getAtencion,
    getAtencionVersion,
    getExpedientePsicologico,
    listAtencionVersions,
    registerAtencion,
    updateAtencionDraft,
} from "../../services/psicologiaService";
import { base64ToBlob } from "../../services/documentosService";
import { friendlyErrorMessage } from "../../utils/apiErrors";
import { formatDateTime } from "../../utils/format";
import { CONTENIDO_ATENCION_VACIO, contenidoAtencionDesdeApi, numeroAtencionLabel } from "../../constants/psicologia";
import "../../components/dashboard/forms.css";
import "../Expedientes/expedienteDetail.css";
import "../../components/psicologia/hojaAtencion.css";

const NOT_FOUND = "La atención no fue encontrada o no está disponible para tu usuario.";

// Psicología → Expediente → Atención (Hoja de Atención).
//  - nueva: /psicologia/expedientes/:expedienteId/atenciones/nueva
//  - existente: /psicologia/atenciones/:id
// Borrador (solo su autor): editar, guardar, registrar, descartar.
// Registrada: lectura por versión, corregir (nueva versión), anular, PDF.
// Anulada: lectura y PDF (el PDF indica el estado).
// Todo dato clínico vive solo en el estado de React: nada en localStorage.
function PsicologiaAtencion({ nueva = false }) {
    const { id, expedienteId } = useParams();
    const navigate = useNavigate();
    const { can } = usePermissions();

    const [expediente, setExpediente] = useState(null);
    const [atencion, setAtencion] = useState(null); // respuesta vigente/borrador
    const [versiones, setVersiones] = useState([]);
    const [versionVista, setVersionVista] = useState(null); // atención con otra versión seleccionada
    const [contenido, setContenido] = useState(CONTENIDO_ATENCION_VACIO);
    const [dirty, setDirty] = useState(false);
    const [corrigiendo, setCorrigiendo] = useState(false);

    const [loading, setLoading] = useState(true);
    const [loadError, setLoadError] = useState("");
    const [busy, setBusy] = useState(false);
    const [actionError, setActionError] = useState("");
    const [notice, setNotice] = useState("");
    const [modal, setModal] = useState(null); // registrar | descartar | anular | corregir

    const canCreate = can("psicologia.atenciones.create");
    const canCorrect = can("psicologia.atenciones.correct");
    const canAnnul = can("psicologia.atenciones.annul");

    const cargarVersiones = useCallback(async (atencionId) => {
        try {
            const data = await listAtencionVersions(atencionId);
            setVersiones(data.versiones || []);
        } catch {
            setVersiones([]);
        }
    }, []);

    useEffect(() => {
        let active = true;
        async function load() {
            setLoading(true);
            setLoadError("");
            try {
                if (nueva) {
                    const data = await getExpedientePsicologico(expedienteId);
                    if (!active) return;
                    setExpediente(data.expediente);
                    setContenido(CONTENIDO_ATENCION_VACIO);
                    return;
                }
                const data = await getAtencion(id);
                if (!active) return;
                setAtencion(data.atencion);
                setContenido(contenidoAtencionDesdeApi(data.atencion.version?.contenido));
                setVersionVista(null);
                setCorrigiendo(false);
                setDirty(false);
                const exp = await getExpedientePsicologico(data.atencion.expediente_id);
                if (!active) return;
                setExpediente(exp.expediente);
                if (data.atencion.estado !== "BORRADOR") cargarVersiones(data.atencion.id);
            } catch (err) {
                if (active) setLoadError(friendlyErrorMessage(err, NOT_FOUND));
            } finally {
                if (active) setLoading(false);
            }
        }
        load();
        return () => {
            active = false;
        };
    }, [nueva, id, expedienteId, cargarVersiones]);

    // Aviso del navegador si se intenta salir con cambios sin guardar.
    useEffect(() => {
        if (!dirty) return undefined;
        const handler = (event) => {
            event.preventDefault();
            event.returnValue = "";
        };
        window.addEventListener("beforeunload", handler);
        return () => window.removeEventListener("beforeunload", handler);
    }, [dirty]);

    const version = versionVista?.version ?? atencion?.version;
    const { estudiantes, unavailable: estudianteUnavailable } = useEstudiantesBasic(
        expediente ? [expediente.estudiante_id] : []
    );
    const { usuarios, unavailable } = useUsuariosBasic(
        [atencion?.autor_id, atencion?.registrada_por, atencion?.anulada_por, version?.creada_por].filter(Boolean)
    );

    const handleChange = (next) => {
        setContenido(next);
        setDirty(true);
        setNotice("");
    };

    const aplicarRespuesta = (data) => {
        setAtencion(data.atencion);
        setContenido(contenidoAtencionDesdeApi(data.atencion.version?.contenido));
        setVersionVista(null);
        setDirty(false);
    };

    // Guarda el borrador (crea si todavía no existe). Devuelve la atención
    // guardada, o lanza el error original.
    const guardar = async () => {
        if (!atencion) {
            const data = await createAtencionDraft(expediente.id, contenido);
            aplicarRespuesta(data);
            return data.atencion;
        }
        if (!dirty) return atencion;
        const data = await updateAtencionDraft(atencion.id, atencion.revision, contenido);
        aplicarRespuesta(data);
        return data.atencion;
    };

    const handleGuardar = async () => {
        if (busy) return;
        setBusy(true);
        setActionError("");
        setNotice("");
        const eraNueva = !atencion;
        try {
            const guardada = await guardar();
            if (eraNueva) {
                navigate(`/psicologia/atenciones/${guardada.id}`, { replace: true });
                return;
            }
            setNotice("Borrador guardado.");
        } catch (err) {
            setActionError(friendlyErrorMessage(err, NOT_FOUND));
        } finally {
            setBusy(false);
        }
    };

    const confirmarRegistro = async () => {
        try {
            const guardada = await guardar();
            const data = await registerAtencion(guardada.id, guardada.revision);
            setModal(null);
            if (nueva) {
                navigate(`/psicologia/atenciones/${data.atencion.id}`, { replace: true });
                return undefined;
            }
            aplicarRespuesta(data);
            cargarVersiones(data.atencion.id);
            setNotice(`Atención ${numeroAtencionLabel(data.atencion.numero)} registrada (versión 1).`);
            return undefined;
        } catch (err) {
            return friendlyErrorMessage(err, NOT_FOUND);
        }
    };

    const confirmarDescarte = async () => {
        try {
            await discardAtencionDraft(atencion.id, atencion.revision);
            setDirty(false);
            navigate(`/psicologia/expedientes/${atencion.expediente_id}`, { replace: true });
            return undefined;
        } catch (err) {
            return friendlyErrorMessage(err, NOT_FOUND);
        }
    };

    const confirmarCorreccion = async (motivo) => {
        try {
            const data = await correctAtencion(atencion.id, {
                versionBase: atencion.version_vigente,
                motivoCorreccion: motivo,
                contenido,
            });
            setModal(null);
            aplicarRespuesta(data);
            setCorrigiendo(false);
            cargarVersiones(data.atencion.id);
            setNotice(`Corrección registrada como versión ${data.atencion.version_vigente}. Las versiones anteriores se conservan sin cambios.`);
            return undefined;
        } catch (err) {
            return friendlyErrorMessage(err, NOT_FOUND);
        }
    };

    const confirmarAnulacion = async (motivo) => {
        try {
            const data = await annulAtencion(atencion.id, motivo);
            setModal(null);
            aplicarRespuesta(data);
            setNotice("La atención fue anulada. Se conserva con su historial de versiones.");
            return undefined;
        } catch (err) {
            return friendlyErrorMessage(err, NOT_FOUND);
        }
    };

    const handleVersion = async (event) => {
        const numero = Number(event.target.value);
        setActionError("");
        if (numero === atencion.version_vigente) {
            setVersionVista(null);
            return;
        }
        setBusy(true);
        try {
            setVersionVista(await getAtencionVersion(atencion.id, numero).then((data) => data.atencion));
        } catch (err) {
            setActionError(friendlyErrorMessage(err, "La versión solicitada no existe."));
        } finally {
            setBusy(false);
        }
    };

    const handlePdf = async () => {
        if (busy) return;
        setBusy(true);
        setActionError("");
        try {
            const data = await exportAtencionPdf(atencion.id, version?.numero_version);
            const url = URL.createObjectURL(base64ToBlob(data.contenido_base64, "application/pdf"));
            const link = document.createElement("a");
            link.href = url;
            link.download = data.nombre;
            document.body.appendChild(link);
            link.click();
            link.remove();
            setTimeout(() => URL.revokeObjectURL(url), 1000);
        } catch (err) {
            setActionError(friendlyErrorMessage(err, NOT_FOUND));
        } finally {
            setBusy(false);
        }
    };

    const iniciarCorreccion = () => {
        setActionError("");
        setNotice("");
        setVersionVista(null);
        setContenido(contenidoAtencionDesdeApi(atencion.version?.contenido));
        setDirty(false);
        setCorrigiendo(true);
    };

    const cancelarCorreccion = () => {
        setContenido(contenidoAtencionDesdeApi(atencion.version?.contenido));
        setDirty(false);
        setCorrigiendo(false);
    };

    if (loading) {
        return (
            <RoleLayout title="Atención psicológica">
                <p className="dp-table-empty">Cargando atención...</p>
            </RoleLayout>
        );
    }

    if (loadError) {
        return (
            <RoleLayout title="Atención psicológica">
                <section className="dp-panel">
                    <p className="dp-form-error">{loadError}</p>
                    <Link to="/psicologia/expedientes" className="dp-btn-secondary" style={{ marginTop: 12, display: "inline-block" }}>
                        Volver a expedientes
                    </Link>
                </section>
            </RoleLayout>
        );
    }

    const estado = atencion?.estado ?? "BORRADOR";
    const esBorrador = estado === "BORRADOR";
    const esRegistrada = estado === "REGISTRADA";
    const expedienteCerrado = expediente?.estado === "CERRADO";
    const editable = (esBorrador && canCreate) || corrigiendo;
    const titulo = atencion?.numero ? `Atención ${numeroAtencionLabel(atencion.numero)}` : "Nueva atención";

    return (
        <RoleLayout title={titulo}>
            <section className="dp-panel">
                <div className="psi-toolbar">
                    <h2 className="dp-panel-title" style={{ margin: 0 }}>
                        Hoja de Atención {corrigiendo && "· Corrección"}
                    </h2>
                    {!esBorrador && !corrigiendo && versiones.length > 0 && (
                        <label className="psi-version-select">
                            Versión
                            <select value={version?.numero_version ?? atencion.version_vigente} onChange={handleVersion} disabled={busy}>
                                {versiones.map((v) => (
                                    <option key={v.numero_version} value={v.numero_version}>
                                        v{v.numero_version}
                                        {v.numero_version === atencion.version_vigente ? " (vigente)" : ""}
                                    </option>
                                ))}
                            </select>
                        </label>
                    )}
                </div>

                <dl className="psi-ident">
                    <div>
                        <dt>Estudiante</dt>
                        <dd>
                            <NombreResuelto
                                id={expediente?.estudiante_id}
                                items={estudiantes}
                                unavailable={estudianteUnavailable}
                                esUsuario={false}
                            />
                        </dd>
                    </div>
                    <div>
                        <dt>Expediente</dt>
                        <dd>{expediente?.codigo}</dd>
                    </div>
                    <div>
                        <dt>Atención</dt>
                        <dd>{numeroAtencionLabel(atencion?.numero)}</dd>
                    </div>
                    <div>
                        <dt>Versión</dt>
                        <dd>{esBorrador ? "Borrador" : corrigiendo ? `Nueva (v${atencion.version_vigente + 1})` : `v${version?.numero_version}`}</dd>
                    </div>
                    <div>
                        <dt>Estado</dt>
                        <dd>
                            <AtencionEstadoBadge estado={estado} />
                        </dd>
                    </div>
                </dl>

                {estado === "ANULADA" && (
                    <div className="psi-banner psi-banner--anulada">
                        <strong>Atención anulada</strong> · {formatDateTime(atencion.anulada_en)} · Anulada por:{" "}
                        <NombreResuelto id={atencion.anulada_por} items={usuarios} unavailable={unavailable} />
                        <br />
                        Motivo: {atencion.motivo_anulacion}
                    </div>
                )}
                {versionVista && (
                    <div className="psi-banner psi-banner--warn">
                        Estás viendo la versión {version.numero_version}, que no es la vigente (v{atencion.version_vigente}).
                    </div>
                )}
                {esBorrador && (
                    <div className="psi-banner psi-banner--info">
                        Borrador: solo tú puedes verlo. No es un documento oficial hasta que lo registres.
                    </div>
                )}
                {corrigiendo && (
                    <div className="psi-banner psi-banner--info">
                        Estás corrigiendo la versión {atencion.version_vigente}. Al guardar se creará una nueva versión; la
                        anterior se conserva sin cambios.
                    </div>
                )}
                {expedienteCerrado && (esBorrador || esRegistrada) && (
                    <p className="dp-form-hint">
                        El expediente está cerrado: no se pueden registrar, corregir ni anular atenciones hasta reabrirlo.
                    </p>
                )}

                {!esBorrador && !corrigiendo && version && (
                    <dl className="dp-detail-list" style={{ marginBottom: 16 }}>
                        <div>
                            <dt>Registrada por</dt>
                            <dd>
                                <NombreResuelto id={atencion.registrada_por} items={usuarios} unavailable={unavailable} /> ·{" "}
                                {formatDateTime(atencion.registrada_en)}
                            </dd>
                        </div>
                        {version.numero_version > 1 && (
                            <div>
                                <dt>Versión {version.numero_version}</dt>
                                <dd>
                                    <NombreResuelto id={version.creada_por} items={usuarios} unavailable={unavailable} /> ·{" "}
                                    {formatDateTime(version.creada_en)} · Motivo de la corrección: {version.motivo_correccion}
                                </dd>
                            </div>
                        )}
                    </dl>
                )}

                <HojaAtencion
                    value={editable ? contenido : contenidoAtencionDesdeApi(version?.contenido)}
                    onChange={handleChange}
                    readOnly={!editable}
                    disabled={busy || expedienteCerrado}
                    etiquetas={editable ? undefined : { motivos: version?.motivos, derivaciones: version?.derivaciones }}
                />

                {notice && (
                    <p className="dp-form-hint" role="status" style={{ marginTop: 12 }}>
                        {notice}
                    </p>
                )}
                {actionError && (
                    <p className="dp-form-error" role="alert" style={{ marginTop: 12 }}>
                        {actionError}
                    </p>
                )}

                <div className="psi-hoja-actions">
                    {esBorrador && canCreate && (
                        <>
                            {atencion && (
                                <button type="button" className="psi-btn-danger" onClick={() => setModal("descartar")} disabled={busy}>
                                    Descartar borrador
                                </button>
                            )}
                            <button
                                type="button"
                                className="dp-btn-secondary"
                                onClick={handleGuardar}
                                disabled={busy || expedienteCerrado || (atencion && !dirty)}
                            >
                                {busy ? "Guardando..." : "Guardar borrador"}
                            </button>
                            <button
                                type="button"
                                className="dp-btn-primary"
                                onClick={() => {
                                    setActionError("");
                                    setModal("registrar");
                                }}
                                disabled={busy || expedienteCerrado}
                            >
                                Registrar atención
                            </button>
                        </>
                    )}

                    {corrigiendo && (
                        <>
                            <button type="button" className="dp-btn-secondary" onClick={cancelarCorreccion} disabled={busy}>
                                Cancelar corrección
                            </button>
                            <button type="button" className="dp-btn-primary" onClick={() => setModal("corregir")} disabled={busy || !dirty}>
                                Guardar como nueva versión
                            </button>
                        </>
                    )}

                    {!esBorrador && !corrigiendo && (
                        <>
                            {esRegistrada && canAnnul && !expedienteCerrado && (
                                <button type="button" className="psi-btn-danger" onClick={() => setModal("anular")} disabled={busy}>
                                    Anular
                                </button>
                            )}
                            {esRegistrada && canCorrect && !expedienteCerrado && (
                                <button type="button" className="dp-btn-secondary" onClick={iniciarCorreccion} disabled={busy}>
                                    Corregir
                                </button>
                            )}
                            <button type="button" className="dp-btn-primary" onClick={handlePdf} disabled={busy}>
                                {busy ? "Procesando..." : `Descargar PDF (v${version?.numero_version})`}
                            </button>
                        </>
                    )}
                </div>
            </section>

            {expediente && (
                <Link
                    to={`/psicologia/expedientes/${expediente.id}`}
                    className="dp-btn-secondary"
                    style={{ marginTop: 16, display: "inline-block" }}
                >
                    ← Volver al expediente {expediente.codigo}
                </Link>
            )}

            {modal === "registrar" && (
                <AccionModal
                    title="Registrar atención"
                    message="Al registrarla, la atención recibe su número correlativo y se convierte en la versión 1 oficial. Después ya no podrá editarse: solo corregirse mediante una nueva versión o anularse. ¿Deseas continuar?"
                    confirmLabel="Registrar"
                    submittingLabel="Registrando..."
                    onConfirm={confirmarRegistro}
                    onClose={() => setModal(null)}
                />
            )}
            {modal === "descartar" && (
                <AccionModal
                    title="Descartar borrador"
                    message="El borrador dejará de estar disponible. ¿Deseas descartarlo?"
                    confirmLabel="Descartar"
                    submittingLabel="Descartando..."
                    danger
                    onConfirm={confirmarDescarte}
                    onClose={() => setModal(null)}
                />
            )}
            {modal === "corregir" && (
                <AccionModal
                    title="Guardar corrección"
                    message={`Se creará la versión ${atencion.version_vigente + 1}. La versión ${atencion.version_vigente} se conserva sin cambios en el historial.`}
                    motivoLabel="Motivo de la corrección *"
                    confirmLabel="Guardar nueva versión"
                    submittingLabel="Guardando..."
                    onConfirm={confirmarCorreccion}
                    onClose={() => setModal(null)}
                />
            )}
            {modal === "anular" && (
                <AccionModal
                    title="Anular atención"
                    message="La atención quedará anulada: no se elimina, se conserva con todas sus versiones y seguirá siendo consultable. Esta acción no se puede deshacer."
                    motivoLabel="Motivo de la anulación *"
                    confirmLabel="Anular atención"
                    submittingLabel="Anulando..."
                    danger
                    onConfirm={confirmarAnulacion}
                    onClose={() => setModal(null)}
                />
            )}
        </RoleLayout>
    );
}

export default PsicologiaAtencion;
