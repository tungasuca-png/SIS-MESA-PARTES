import { base64ToBlob } from "../services/documentosService";

// Descarga un archivo recibido en base64 (plantilla/exportación Excel, PDF)
// sin guardarlo en el almacenamiento del navegador: un Blob temporal que se
// libera enseguida.
export function descargarBase64(contenidoBase64, nombre, mimeType) {
    const url = URL.createObjectURL(base64ToBlob(contenidoBase64, mimeType));
    const enlace = document.createElement("a");
    enlace.href = url;
    enlace.download = nombre;
    document.body.appendChild(enlace);
    enlace.click();
    enlace.remove();
    setTimeout(() => URL.revokeObjectURL(url), 0);
}

export function formatearTamano(bytes) {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
