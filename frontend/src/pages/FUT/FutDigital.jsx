import { useState } from "react";
import { useNavigate } from "react-router-dom";
import RoleLayout from "../../components/portal/RoleLayout";
import FutHeader from "../../components/dashboard/FutHeader";
import SignaturePad from "../../components/dashboard/SignaturePad";
import Icon from "../../components/dashboard/Icon";
import { useAuth } from "../../hooks/useAuth";
import { submitFut } from "../../services/futService";
import { friendlyErrorMessage } from "../../utils/apiErrors";
import { formatBytes, formatDate } from "../../utils/format";
import { EXTENSIONES_PERMITIDAS, MAX_TAMANO_BYTES } from "../../constants/documentos";
import "../../components/dashboard/forms.css";
import "../Expedientes/expedienteDetail.css";
import "./fut.css";

const DNI_PATTERN = /^\d{8}$/;
const TELEFONO_PATTERN = /^\d{7,9}$/;
const CORREO_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

function DerivacionTable({ numeroExpediente }) {
    return (
        <div className="fut-table-wrapper">
            <table className="fut-derivacion-table">
                <thead>
                    <tr>
                        <th>N° Expediente</th>
                        <th>Dirección</th>
                        <th>Subdirección</th>
                        <th>Recursos financieros</th>
                        <th>Secretaría u otros</th>
                    </tr>
                </thead>
                <tbody>
                    <tr>
                        <td className="fut-derivacion-codigo">{numeroExpediente || "—"}</td>
                        <td className="fut-derivacion-pendiente">Pendiente</td>
                        <td className="fut-derivacion-pendiente">Pendiente</td>
                        <td className="fut-derivacion-pendiente">Pendiente</td>
                        <td className="fut-derivacion-pendiente">Pendiente</td>
                    </tr>
                </tbody>
            </table>
            <p className="fut-derivacion-nota">
                Esta sección es de control interno: la completa el personal responsable cuando derive el
                expediente entre áreas (aún no disponible — ver Derivaciones Service).
            </p>
        </div>
    );
}

function FutDigital() {
    const { user } = useAuth();
    const navigate = useNavigate();

    const [step, setStep] = useState("form"); // form | preview | confirmation

    const [sumilla, setSumilla] = useState("");
    const [nombres, setNombres] = useState(user?.username ?? "");
    const [dni, setDni] = useState("");
    const [telefono, setTelefono] = useState("");
    const [domicilio, setDomicilio] = useState("");
    const [distrito, setDistrito] = useState("");
    const [correo, setCorreo] = useState(user?.email ?? "");
    const [fundamentacion, setFundamentacion] = useState("");
    const [folios, setFolios] = useState("1");
    const [documentos, setDocumentos] = useState([]);
    const [documentoDescripcion, setDocumentoDescripcion] = useState("");
    const [firmaDataUrl, setFirmaDataUrl] = useState(null);

    const [errors, setErrors] = useState({});
    const [fileError, setFileError] = useState("");
    const [submitError, setSubmitError] = useState("");
    const [submitting, setSubmitting] = useState(false);
    const [result, setResult] = useState(null);
    const [showCargo, setShowCargo] = useState(false);

    const fechaHoy = formatDate(new Date().toISOString());

    const validate = () => {
        const next = {};
        if (!sumilla.trim()) next.sumilla = "La sumilla es obligatoria.";
        if (!nombres.trim()) next.nombres = "Los nombres y apellidos son obligatorios.";
        if (!DNI_PATTERN.test(dni.trim())) next.dni = "El DNI debe tener 8 dígitos.";
        if (telefono.trim() && !TELEFONO_PATTERN.test(telefono.trim())) {
            next.telefono = "El teléfono no tiene un formato válido.";
        }
        if (correo.trim() && !CORREO_PATTERN.test(correo.trim())) {
            next.correo = "El correo no tiene un formato válido.";
        }
        if (!fundamentacion.trim()) next.fundamentacion = "La fundamentación es obligatoria.";
        const foliosNum = Number(folios);
        if (!Number.isInteger(foliosNum) || foliosNum <= 0) {
            next.folios = "El número de folios debe ser un entero positivo.";
        }
        return next;
    };

    const handleAddFiles = (fileList) => {
        setFileError("");
        const files = Array.from(fileList ?? []);
        if (files.length === 0) return;

        const accepted = [];
        for (const file of files) {
            const extension = file.name.split(".").pop()?.toLowerCase() ?? "";
            if (!EXTENSIONES_PERMITIDAS.includes(extension)) {
                setFileError(`"${file.name}": extensión no permitida (use ${EXTENSIONES_PERMITIDAS.join(", ")}).`);
                continue;
            }
            if (file.size > MAX_TAMANO_BYTES) {
                setFileError(`"${file.name}": supera el tamaño máximo (${formatBytes(MAX_TAMANO_BYTES)}).`);
                continue;
            }
            accepted.push({ file, descripcion: documentoDescripcion.trim() });
        }

        if (accepted.length > 0) {
            setDocumentos((current) => [...current, ...accepted]);
            setDocumentoDescripcion("");
        }
    };

    const handleRemoveFile = (index) => {
        setDocumentos((current) => current.filter((_, i) => i !== index));
    };

    const handleReview = (event) => {
        event.preventDefault();
        const validation = validate();
        setErrors(validation);
        if (Object.keys(validation).length > 0) return;
        setStep("preview");
    };

    const handleConfirmSend = async () => {
        setSubmitting(true);
        setSubmitError("");
        try {
            const res = await submitFut({
                sumilla: sumilla.trim(),
                nombres: nombres.trim(),
                dni: dni.trim(),
                telefono: telefono.trim(),
                domicilio: domicilio.trim(),
                distrito: distrito.trim(),
                correo: correo.trim(),
                fundamentacion: fundamentacion.trim(),
                folios,
                documentos,
                firmaDataUrl,
            });
            setResult(res);
            setStep("confirmation");
        } catch (err) {
            setSubmitError(friendlyErrorMessage(err));
        } finally {
            setSubmitting(false);
        }
    };

    return (
        <RoleLayout title="FUT Digital">
            <div className="fut-page">
                <FutHeader numero={result?.expediente?.codigo} />

                {step !== "confirmation" && (
                    <>
                        <h1 className="fut-titulo">Formulario Único de Trámite</h1>
                        <p className="fut-destinatario">SEÑORA DIRECTORA DE LA I.E &quot;TUNGASUCA&quot;:</p>
                    </>
                )}

                {step === "form" && (
                    <form className="dp-form fut-form" onSubmit={handleReview}>
                        <section className="fut-section">
                            <label htmlFor="fut-sumilla" className="fut-section-title">
                                Sumilla
                            </label>
                            <input
                                id="fut-sumilla"
                                value={sumilla}
                                onChange={(event) => setSumilla(event.target.value)}
                                placeholder="Ej. Solicito certificado de estudios"
                                maxLength={255}
                            />
                            {errors.sumilla && <p className="dp-form-error">{errors.sumilla}</p>}
                        </section>

                        <section className="fut-section">
                            <h2 className="fut-section-title">Datos del solicitante</h2>

                            <div className="fut-grid-3">
                                <div className="dp-form-group">
                                    <label htmlFor="fut-nombres">Nombres y apellidos *</label>
                                    <input
                                        id="fut-nombres"
                                        value={nombres}
                                        onChange={(event) => setNombres(event.target.value)}
                                    />
                                    {errors.nombres && <p className="dp-form-error">{errors.nombres}</p>}
                                </div>
                                <div className="dp-form-group">
                                    <label htmlFor="fut-dni">DNI *</label>
                                    <input
                                        id="fut-dni"
                                        value={dni}
                                        onChange={(event) => setDni(event.target.value)}
                                        maxLength={8}
                                        inputMode="numeric"
                                    />
                                    {errors.dni && <p className="dp-form-error">{errors.dni}</p>}
                                </div>
                                <div className="dp-form-group">
                                    <label htmlFor="fut-telefono">Teléfono</label>
                                    <input
                                        id="fut-telefono"
                                        value={telefono}
                                        onChange={(event) => setTelefono(event.target.value)}
                                        maxLength={9}
                                        inputMode="numeric"
                                    />
                                    {errors.telefono && <p className="dp-form-error">{errors.telefono}</p>}
                                </div>
                            </div>

                            <div className="fut-grid-3">
                                <div className="dp-form-group">
                                    <label htmlFor="fut-domicilio">Domicilio actual</label>
                                    <input
                                        id="fut-domicilio"
                                        value={domicilio}
                                        onChange={(event) => setDomicilio(event.target.value)}
                                    />
                                </div>
                                <div className="dp-form-group">
                                    <label htmlFor="fut-distrito">Distrito</label>
                                    <input
                                        id="fut-distrito"
                                        value={distrito}
                                        onChange={(event) => setDistrito(event.target.value)}
                                    />
                                </div>
                                <div className="dp-form-group">
                                    <label htmlFor="fut-correo">Correo electrónico</label>
                                    <input
                                        id="fut-correo"
                                        type="email"
                                        value={correo}
                                        onChange={(event) => setCorreo(event.target.value)}
                                    />
                                    {errors.correo && <p className="dp-form-error">{errors.correo}</p>}
                                </div>
                            </div>
                        </section>

                        <section className="fut-section">
                            <label htmlFor="fut-fundamentacion" className="fut-section-title">
                                Fundamentación de lo que solicita
                            </label>
                            <textarea
                                id="fut-fundamentacion"
                                value={fundamentacion}
                                onChange={(event) => setFundamentacion(event.target.value)}
                                rows={6}
                                placeholder="Explique con detalle el motivo de su solicitud..."
                            />
                            {errors.fundamentacion && <p className="dp-form-error">{errors.fundamentacion}</p>}
                        </section>

                        <section className="fut-section">
                            <h2 className="fut-section-title">
                                Documento que se adjunta
                                <span className="fut-section-subtitle"> (sustentatorio de su solicitud)</span>
                            </h2>

                            <div className="fut-grid-3">
                                <div className="dp-form-group" style={{ gridColumn: "span 2" }}>
                                    <label htmlFor="fut-doc-descripcion">Descripción del documento</label>
                                    <input
                                        id="fut-doc-descripcion"
                                        value={documentoDescripcion}
                                        onChange={(event) => setDocumentoDescripcion(event.target.value)}
                                        placeholder="Ej. Copia de DNI"
                                    />
                                </div>
                                <div className="dp-form-group">
                                    <label htmlFor="fut-doc-file">Seleccionar archivo</label>
                                    <input
                                        id="fut-doc-file"
                                        type="file"
                                        multiple
                                        accept={EXTENSIONES_PERMITIDAS.map((ext) => `.${ext}`).join(",")}
                                        onChange={(event) => {
                                            handleAddFiles(event.target.files);
                                            event.target.value = "";
                                        }}
                                    />
                                </div>
                            </div>
                            {fileError && <p className="dp-form-error">{fileError}</p>}

                            {documentos.length > 0 && (
                                <ul className="fut-file-list">
                                    {documentos.map((item, index) => (
                                        <li key={`${item.file.name}-${index}`} className="fut-file-item">
                                            <Icon name="file" size={16} />
                                            <span className="fut-file-name">
                                                {item.descripcion ? `${item.descripcion} — ` : ""}
                                                {item.file.name}
                                            </span>
                                            <span className="fut-file-size">{formatBytes(item.file.size)}</span>
                                            <button
                                                type="button"
                                                className="fut-file-remove"
                                                onClick={() => handleRemoveFile(index)}
                                                aria-label={`Eliminar ${item.file.name}`}
                                            >
                                                <Icon name="close" size={14} />
                                            </button>
                                        </li>
                                    ))}
                                </ul>
                            )}

                            <div className="dp-form-group fut-folios">
                                <label htmlFor="fut-folios">N° de folios</label>
                                <input
                                    id="fut-folios"
                                    type="number"
                                    min="1"
                                    value={folios}
                                    onChange={(event) => setFolios(event.target.value)}
                                />
                                {errors.folios && <p className="dp-form-error">{errors.folios}</p>}
                            </div>
                        </section>

                        <section className="fut-section">
                            <div className="fut-fecha-firma">
                                <div className="dp-form-group">
                                    <label>Fecha</label>
                                    <p className="fut-fecha-valor">Carabayllo, {fechaHoy}</p>
                                </div>
                            </div>

                            <label className="fut-section-title">Firma del solicitante</label>
                            <SignaturePad onChange={setFirmaDataUrl} />
                        </section>

                        <DerivacionTable numeroExpediente={null} />

                        <div className="dp-form-actions">
                            <button type="submit" className="dp-btn-primary">
                                Revisar FUT →
                            </button>
                        </div>
                    </form>
                )}

                {step === "preview" && (
                    <div className="fut-preview">
                        <h2 className="fut-section-title">Revisar FUT</h2>

                        <dl className="dp-detail-list">
                            <div>
                                <dt>Sumilla</dt>
                                <dd>{sumilla}</dd>
                            </div>
                            <div>
                                <dt>Solicitante</dt>
                                <dd>
                                    {nombres} · DNI {dni}
                                    {telefono ? ` · Tel. ${telefono}` : ""}
                                </dd>
                            </div>
                            <div>
                                <dt>Domicilio</dt>
                                <dd>
                                    {domicilio || "—"} {distrito ? `— ${distrito}` : ""}
                                </dd>
                            </div>
                            <div>
                                <dt>Correo</dt>
                                <dd>{correo || "—"}</dd>
                            </div>
                            <div>
                                <dt>Fundamentación</dt>
                                <dd className="fut-preview-fundamentacion">{fundamentacion}</dd>
                            </div>
                            <div>
                                <dt>Documentos adjuntos</dt>
                                <dd>
                                    {documentos.length === 0
                                        ? "Ninguno"
                                        : documentos.map((item) => item.file.name).join(", ")}
                                    {" · "}
                                    {folios} folio{folios === "1" ? "" : "s"}
                                </dd>
                            </div>
                            <div>
                                <dt>Firma</dt>
                                <dd>{firmaDataUrl ? "Capturada" : "No capturada"}</dd>
                            </div>
                        </dl>

                        {submitError && <p className="dp-form-error">{submitError}</p>}

                        <div className="dp-form-actions">
                            <button
                                type="button"
                                className="dp-btn-secondary"
                                onClick={() => setStep("form")}
                                disabled={submitting}
                            >
                                ← Volver a editar
                            </button>
                            <button
                                type="button"
                                className="dp-btn-primary"
                                onClick={handleConfirmSend}
                                disabled={submitting}
                            >
                                {submitting ? "Enviando..." : "Presentar solicitud"}
                            </button>
                        </div>
                    </div>
                )}

                {step === "confirmation" && result && !showCargo && (
                    <div className="fut-confirmacion">
                        <p className="fut-confirmacion-check">✓ Solicitud registrada</p>
                        <p>Su solicitud ha sido registrada correctamente.</p>
                        <p>Guarde su número de expediente para realizar el seguimiento.</p>
                        <p>
                            Puede descargar el FUT cuando quiera desde el detalle del expediente (personal de
                            Secretaría también podrá hacerlo); no se guarda como documento adjunto, ahí solo quedan
                            los archivos que usted adjuntó.
                        </p>

                        <dl className="dp-detail-list">
                            <div>
                                <dt>N° de expediente</dt>
                                <dd className="fut-confirmacion-codigo">{result.expediente.codigo}</dd>
                            </div>
                            <div>
                                <dt>Estado</dt>
                                <dd>{result.expediente.estado}</dd>
                            </div>
                        </dl>

                        {result.failedUploads > 0 && (
                            <p className="dp-form-error">
                                Se registró el expediente, pero {result.failedUploads} de {result.totalUploads}{" "}
                                archivo(s) no se pudieron subir. Puede volver a intentarlo desde el detalle del
                                expediente.
                            </p>
                        )}

                        <div className="dp-form-actions">
                            <button type="button" className="dp-btn-secondary" onClick={() => setShowCargo(true)}>
                                Ver cargo digital
                            </button>
                            <button
                                type="button"
                                className="dp-btn-secondary"
                                onClick={() => navigate(`/expedientes/${result.expediente.codigo}`)}
                            >
                                Ver expediente
                            </button>
                            <button type="button" className="dp-btn-primary" onClick={() => navigate("/mesa-de-partes")}>
                                Volver a Mesa de Partes
                            </button>
                        </div>
                    </div>
                )}

                {step === "confirmation" && result && showCargo && (
                    <div className="fut-cargo">
                        <p className="fut-destinatario">SEÑORA DIRECTORA DE LA I.E &quot;TUNGASUCA&quot;:</p>

                        <section className="fut-cargo-section">
                            <h3 className="fut-section-title">Datos del solicitante</h3>
                            <div className="fut-cargo-grid">
                                <div className="fut-cargo-field">
                                    <span>{nombres}</span>
                                    <label>Nombres y apellidos</label>
                                </div>
                                <div className="fut-cargo-field">
                                    <span>{dni}</span>
                                    <label>DNI</label>
                                </div>
                                <div className="fut-cargo-field">
                                    <span>{telefono || "—"}</span>
                                    <label>Teléfono</label>
                                </div>
                                <div className="fut-cargo-field">
                                    <span>{domicilio || "—"}</span>
                                    <label>Domicilio actual</label>
                                </div>
                                <div className="fut-cargo-field">
                                    <span>{distrito || "—"}</span>
                                    <label>Distrito</label>
                                </div>
                                <div className="fut-cargo-field">
                                    <span>{correo || "—"}</span>
                                    <label>Correo electrónico</label>
                                </div>
                            </div>
                        </section>

                        <section className="fut-cargo-section">
                            <h3 className="fut-section-title">Asunto</h3>
                            <p className="fut-cargo-linea">{sumilla}</p>
                        </section>

                        <section className="fut-cargo-section">
                            <h3 className="fut-section-title">Fundamentación de lo que solicita</h3>
                            <p className="fut-cargo-lined-text">{fundamentacion}</p>
                        </section>

                        <section className="fut-cargo-section">
                            <h3 className="fut-section-title">
                                Documento que se adjunta
                                <span className="fut-section-subtitle"> (sustentatorio de su solicitud)</span>
                            </h3>
                            {documentos.length === 0 ? (
                                <p className="fut-cargo-lined-text fut-cargo-vacio">Ninguno</p>
                            ) : (
                                <ul className="fut-cargo-doc-list">
                                    {documentos.map((item, index) => (
                                        <li key={`${item.file.name}-${index}`}>
                                            {item.descripcion ? `${item.descripcion} — ` : ""}
                                            {item.file.name}
                                        </li>
                                    ))}
                                </ul>
                            )}
                            <p className="fut-cargo-folios">N° de folios: {folios}</p>
                        </section>

                        <section className="fut-cargo-section fut-cargo-fecha-firma">
                            <p>
                                <strong>Fecha:</strong> Carabayllo, {fechaHoy}
                            </p>
                            <div className="fut-cargo-firma">
                                {firmaDataUrl ? (
                                    <img src={firmaDataUrl} alt="Firma del solicitante" />
                                ) : (
                                    <p className="fut-cargo-vacio">Firma no capturada</p>
                                )}
                                <span>Firma del solicitante</span>
                            </div>
                        </section>

                        <DerivacionTable numeroExpediente={result.expediente.codigo} />

                        <div className="fut-cargo-separator">
                            - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
                        </div>

                        <section className="fut-cargo-section">
                            <h3 className="fut-section-title">Cargo de recepción</h3>
                            <table className="fut-cargo-resumen-table">
                                <tbody>
                                    <tr>
                                        <th>Apellidos y nombres</th>
                                        <td>{nombres}</td>
                                    </tr>
                                    <tr>
                                        <th>Asunto</th>
                                        <td>{sumilla}</td>
                                    </tr>
                                    <tr>
                                        <th>Fecha</th>
                                        <td>Carabayllo, {fechaHoy}</td>
                                    </tr>
                                    <tr>
                                        <th>N° expediente</th>
                                        <td className="fut-confirmacion-codigo">{result.expediente.codigo}</td>
                                    </tr>
                                    <tr>
                                        <th>N° folios</th>
                                        <td>{folios}</td>
                                    </tr>
                                </tbody>
                            </table>
                        </section>

                        <div className="dp-form-actions">
                            <button type="button" className="dp-btn-secondary" onClick={() => setShowCargo(false)}>
                                ← Volver
                            </button>
                            <button type="button" className="dp-btn-primary" onClick={() => window.print()}>
                                Imprimir
                            </button>
                        </div>
                    </div>
                )}
            </div>
        </RoleLayout>
    );
}

export default FutDigital;
