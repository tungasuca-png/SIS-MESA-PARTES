import { useEffect, useState } from "react";
import { usePermissions } from "../../hooks/usePermissions";
import {
    base64ToBlob,
    deleteDocumento,
    downloadDocumento,
    fileToBase64,
    getDocumentosByExpediente,
    uploadDocumento,
} from "../../services/documentosService";
import { friendlyErrorMessage } from "../../utils/apiErrors";
import { formatBytes, formatDateTime } from "../../utils/format";
import { EXTENSIONES_PERMITIDAS, MAX_TAMANO_BYTES, MIME_POR_EXTENSION, TIPOS_DOCUMENTO } from "../../constants/documentos";
import { exportFutDelExpediente, tieneFutExportable } from "../../services/futService";
import Icon from "./Icon";
import "./forms.css";
import "./documentosPanel.css";

function extensionOf(filename) {
    const parts = filename.split(".");
    return parts.length > 1 ? parts.pop().toLowerCase() : "";
}

function tipoLabel(value) {
    return TIPOS_DOCUMENTO.find((item) => item.value === value)?.label ?? value;
}

function DocumentosPanel({ expedienteId, expediente }) {
    const { can } = usePermissions();
    const canUpload = can("documentos.create");
    const canDelete = can("documentos.delete");

    const [documentos, setDocumentos] = useState([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");

    const [tipoDocumento, setTipoDocumento] = useState("ADJUNTO");
    const [uploading, setUploading] = useState(false);
    const [uploadError, setUploadError] = useState("");

    const [downloadingId, setDownloadingId] = useState(null);
    const [deletingId, setDeletingId] = useState(null);

    const load = async () => {
        setLoading(true);
        setError("");
        try {
            const data = await getDocumentosByExpediente(expedienteId);
            setDocumentos(data.documentos || []);
        } catch (err) {
            setError(friendlyErrorMessage(err));
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        // `load` tambien se reutiliza desde handleUpload/handleDelete para
        // refrescar tras una mutacion, por eso vive fuera del efecto.
        // eslint-disable-next-line react-hooks/set-state-in-effect
        load();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [expedienteId]);

    const handleUpload = async (event) => {
        event.preventDefault();
        setUploadError("");

        const file = event.target.elements.docFile.files?.[0];
        if (!file) {
            setUploadError("Selecciona un archivo.");
            return;
        }
        const extension = extensionOf(file.name);
        if (!EXTENSIONES_PERMITIDAS.includes(extension)) {
            setUploadError(`Extensión no permitida. Usa: ${EXTENSIONES_PERMITIDAS.join(", ")}.`);
            return;
        }
        if (file.size > MAX_TAMANO_BYTES) {
            setUploadError(`El archivo supera el tamaño máximo (${formatBytes(MAX_TAMANO_BYTES)}).`);
            return;
        }

        setUploading(true);
        try {
            const contenidoBase64 = await fileToBase64(file);
            await uploadDocumento({ expedienteId, nombre: file.name, tipoDocumento, extension, contenidoBase64 });
            event.target.reset();
            await load();
        } catch (err) {
            setUploadError(friendlyErrorMessage(err));
        } finally {
            setUploading(false);
        }
    };

    const handleDownload = async (item) => {
        setDownloadingId(item.id);
        try {
            const data = await downloadDocumento(item.id);
            const mime = MIME_POR_EXTENSION[item.extension] || "application/octet-stream";
            const blob = base64ToBlob(data.contenido, mime);
            const url = URL.createObjectURL(blob);
            const link = document.createElement("a");
            link.href = url;
            link.download = item.nombre;
            document.body.appendChild(link);
            link.click();
            link.remove();
            URL.revokeObjectURL(url);
        } catch (err) {
            setError(friendlyErrorMessage(err));
        } finally {
            setDownloadingId(null);
        }
    };

    // El cargo del FUT ya NO se guarda como documento del expediente (solo
    // van ahí los adjuntos que realmente subió el solicitante) — se
    // reconstruye al vuelo con los datos que ya tiene el expediente. Ver
    // futService.js.
    const [downloadingFut, setDownloadingFut] = useState(false);

    const handleDownloadFut = async () => {
        setDownloadingFut(true);
        try {
            const fut = await exportFutDelExpediente(expediente);
            if (!fut) return;
            const blob = base64ToBlob(fut.contenidoBase64, "application/pdf");
            const url = URL.createObjectURL(blob);
            const link = document.createElement("a");
            link.href = url;
            link.download = fut.nombre;
            document.body.appendChild(link);
            link.click();
            link.remove();
            URL.revokeObjectURL(url);
        } catch (err) {
            setError(friendlyErrorMessage(err));
        } finally {
            setDownloadingFut(false);
        }
    };

    const handleDelete = async (id) => {
        setDeletingId(id);
        try {
            await deleteDocumento(id);
            await load();
        } catch (err) {
            setError(friendlyErrorMessage(err));
        } finally {
            setDeletingId(null);
        }
    };

    return (
        <section className="dp-panel">
            <h2 className="dp-panel-title">Documentos</h2>

            {tieneFutExportable(expediente) && (
                <div className="dp-fut-callout">
                    <div className="dp-fut-callout-info">
                        <Icon name="file" size={20} />
                        <div>
                            <p className="dp-fut-callout-title">FUT de este expediente</p>
                            <p className="dp-fut-callout-subtitle">
                                Formulario con el registro completo de la solicitud.
                            </p>
                        </div>
                    </div>
                    <button
                        type="button"
                        className="dp-btn-primary"
                        onClick={handleDownloadFut}
                        disabled={downloadingFut}
                    >
                        {downloadingFut ? "Descargando..." : "Descargar FUT"}
                    </button>
                </div>
            )}

            {canUpload && (
                <form className="dp-form dp-documentos-upload" onSubmit={handleUpload}>
                    <div className="dp-form-group">
                        <label htmlFor="doc-tipo">Tipo de documento</label>
                        <select
                            id="doc-tipo"
                            value={tipoDocumento}
                            onChange={(event) => setTipoDocumento(event.target.value)}
                            disabled={uploading}
                        >
                            {TIPOS_DOCUMENTO.map((item) => (
                                <option key={item.value} value={item.value}>
                                    {item.label}
                                </option>
                            ))}
                        </select>
                    </div>
                    <div className="dp-form-group">
                        <label htmlFor="doc-file">Archivo (máx. {formatBytes(MAX_TAMANO_BYTES)})</label>
                        <input
                            id="doc-file"
                            name="docFile"
                            type="file"
                            accept={EXTENSIONES_PERMITIDAS.map((ext) => `.${ext}`).join(",")}
                            disabled={uploading}
                        />
                    </div>
                    {uploadError && <p className="dp-form-error">{uploadError}</p>}
                    <div className="dp-form-actions">
                        <button type="submit" className="dp-btn-primary" disabled={uploading}>
                            {uploading ? "Subiendo..." : "Subir documento"}
                        </button>
                    </div>
                </form>
            )}

            {loading && <p className="dp-table-empty">Cargando documentos...</p>}
            {!loading && error && <p className="dp-form-error">{error}</p>}
            {!loading && !error && documentos.length === 0 && (
                <p className="dp-table-empty">No hay documentos registrados.</p>
            )}

            {!loading && !error && documentos.length > 0 && (
                <ul className="dp-documentos-list">
                    {documentos.map((item) => (
                        <li key={item.id} className="dp-documentos-item">
                            <div className="dp-documentos-info">
                                <Icon name="file" size={18} />
                                <div>
                                    <p className="dp-documentos-nombre">{item.nombre}</p>
                                    <p className="dp-documentos-meta">
                                        {tipoLabel(item.tipo_documento)} · {formatBytes(item.tamano_bytes)} ·{" "}
                                        {formatDateTime(item.fecha_registro)}
                                    </p>
                                </div>
                            </div>
                            <div className="dp-documentos-actions">
                                <button
                                    type="button"
                                    className="dp-btn-secondary"
                                    onClick={() => handleDownload(item)}
                                    disabled={downloadingId === item.id}
                                >
                                    {downloadingId === item.id ? "Descargando..." : "Descargar"}
                                </button>
                                {canDelete && (
                                    <button
                                        type="button"
                                        className="dp-btn-secondary dp-documentos-delete"
                                        onClick={() => handleDelete(item.id)}
                                        disabled={deletingId === item.id}
                                    >
                                        {deletingId === item.id ? "Eliminando..." : "Eliminar"}
                                    </button>
                                )}
                            </div>
                        </li>
                    ))}
                </ul>
            )}
        </section>
    );
}

export default DocumentosPanel;
