import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import DashboardLayout from "../../components/dashboard/DashboardLayout";
import Icon from "../../components/dashboard/Icon";
import { usePermissions } from "../../hooks/usePermissions";
import {
    base64ToBlob,
    deleteDocumento,
    downloadDocumento,
    getDocumentos,
} from "../../services/documentosService";
import { friendlyErrorMessage } from "../../utils/apiErrors";
import { formatBytes, formatDateTime, shortId } from "../../utils/format";
import { DEFAULT_PAGE_SIZE } from "../../constants/expedientes";
import { MIME_POR_EXTENSION, TIPOS_DOCUMENTO } from "../../constants/documentos";
import "../../components/dashboard/RecentExpedientes.css";
import "../../components/dashboard/forms.css";
import "../../components/dashboard/documentosPanel.css";
import "../Expedientes/expedientesList.css";

function tipoLabel(value) {
    return TIPOS_DOCUMENTO.find((item) => item.value === value)?.label ?? value;
}

function DocumentosList() {
    const { can } = usePermissions();
    const canDelete = can("documentos.delete");
    const canViewAll = can("documentos.view");

    const [items, setItems] = useState([]);
    const [total, setTotal] = useState(0);
    const [page, setPage] = useState(1);
    const [tipoDocumento, setTipoDocumento] = useState("");
    const [expedienteId, setExpedienteId] = useState("");
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");
    const [refreshKey, setRefreshKey] = useState(0);

    const [downloadingId, setDownloadingId] = useState(null);
    const [deletingId, setDeletingId] = useState(null);

    useEffect(() => {
        let active = true;

        async function load() {
            setLoading(true);
            setError("");
            try {
                const data = await getDocumentos({
                    expedienteId,
                    tipoDocumento,
                    page,
                    pageSize: DEFAULT_PAGE_SIZE,
                });
                if (!active) return;
                setItems(data.documentos || []);
                setTotal(data.total || 0);
            } catch (err) {
                if (active) setError(friendlyErrorMessage(err));
            } finally {
                if (active) setLoading(false);
            }
        }

        load();
        return () => {
            active = false;
        };
    }, [tipoDocumento, expedienteId, page, refreshKey]);

    const totalPages = Math.max(1, Math.ceil(total / DEFAULT_PAGE_SIZE));

    const handleFilterChange = (setter) => (event) => {
        setPage(1);
        setter(event.target.value);
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

    const handleDelete = async (id) => {
        setDeletingId(id);
        try {
            await deleteDocumento(id);
            setRefreshKey((current) => current + 1);
        } catch (err) {
            setError(friendlyErrorMessage(err));
        } finally {
            setDeletingId(null);
        }
    };

    const emptyMessage = canViewAll
        ? "No hay documentos registrados."
        : "Aún no tienes documentos registrados.";

    return (
        <DashboardLayout title="Documentos">
            <section className="dp-panel">
                <div className="dp-list-header">
                    <h2 className="dp-panel-title">Documentos</h2>
                </div>

                <div className="dp-filters">
                    <select
                        value={tipoDocumento}
                        onChange={handleFilterChange(setTipoDocumento)}
                        aria-label="Filtrar por tipo de documento"
                    >
                        <option value="">Todos los tipos</option>
                        {TIPOS_DOCUMENTO.map((item) => (
                            <option key={item.value} value={item.value}>
                                {item.label}
                            </option>
                        ))}
                    </select>
                    <input
                        type="text"
                        value={expedienteId}
                        onChange={handleFilterChange(setExpedienteId)}
                        placeholder="Filtrar por ID de expediente"
                        aria-label="Filtrar por expediente"
                    />
                </div>

                {loading && <p className="dp-table-empty">Cargando documentos...</p>}

                {!loading && error && <p className="dp-form-error">{error}</p>}

                {!loading && !error && items.length === 0 && (
                    <p className="dp-table-empty">{emptyMessage}</p>
                )}

                {!loading && !error && items.length > 0 && (
                    <>
                        <div className="dp-table-wrapper">
                            <table className="dp-table">
                                <thead>
                                    <tr>
                                        <th>Documento</th>
                                        <th>Tipo</th>
                                        <th>Expediente</th>
                                        <th>Tamaño</th>
                                        <th>Fecha</th>
                                        <th>Acción</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {items.map((item) => (
                                        <tr key={item.id}>
                                            <td className="dp-table-code">{item.nombre}</td>
                                            <td>{tipoLabel(item.tipo_documento)}</td>
                                            <td>
                                                <Link to={`/expedientes/${item.expediente_id}`} className="dp-table-action">
                                                    {shortId(item.expediente_id)}
                                                </Link>
                                            </td>
                                            <td>{formatBytes(item.tamano_bytes)}</td>
                                            <td>{formatDateTime(item.fecha_registro)}</td>
                                            <td>
                                                <div className="dp-documentos-actions">
                                                    <button
                                                        type="button"
                                                        className="dp-table-action"
                                                        onClick={() => handleDownload(item)}
                                                        disabled={downloadingId === item.id}
                                                    >
                                                        <Icon name="eye" size={16} />
                                                        {downloadingId === item.id ? "Descargando..." : "Descargar"}
                                                    </button>
                                                    {canDelete && (
                                                        <button
                                                            type="button"
                                                            className="dp-table-action dp-documentos-delete"
                                                            onClick={() => handleDelete(item.id)}
                                                            disabled={deletingId === item.id}
                                                        >
                                                            {deletingId === item.id ? "Eliminando..." : "Eliminar"}
                                                        </button>
                                                    )}
                                                </div>
                                            </td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>
                        </div>

                        <div className="dp-pagination">
                            <span>
                                Página {page} de {totalPages} · {total} documento{total === 1 ? "" : "s"}
                            </span>
                            <div className="dp-pagination-buttons">
                                <button
                                    type="button"
                                    className="dp-btn-secondary"
                                    disabled={page <= 1}
                                    onClick={() => setPage((current) => current - 1)}
                                >
                                    Anterior
                                </button>
                                <button
                                    type="button"
                                    className="dp-btn-secondary"
                                    disabled={page >= totalPages}
                                    onClick={() => setPage((current) => current + 1)}
                                >
                                    Siguiente
                                </button>
                            </div>
                        </div>
                    </>
                )}
            </section>
        </DashboardLayout>
    );
}

export default DocumentosList;
