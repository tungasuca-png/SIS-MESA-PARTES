import { createExpediente } from "./expedientesService";
import { fileToBase64, uploadDocumento } from "./documentosService";
import { formatDate } from "../utils/format";

// El FUT no tiene un número de trámite propio en el backend real (no existe
// un "FUT Service" ni un campo para eso): el ÚNICO identificador oficial es
// el código que ya genera Expedientes Service (EXP-YYYY-NNNNNN). Se usa ese
// mismo código como "N° de FUT / expediente" para no inventar un segundo
// numerador en el frontend.

// Expedientes Service no tiene columnas para nombres/DNI/teléfono/domicilio/
// distrito/correo del solicitante (eso es responsabilidad de Usuarios
// Service, pero hoy no existe una operación de autoservicio para que un
// SOLICITANTE actualice su propio perfil — ver README de Usuarios Service).
// Para no perder esos datos, se guardan estructurados dentro de la
// "descripcion" del propio expediente, junto con la fundamentación.
function buildDescripcion({ nombres, dni, telefono, domicilio, distrito, correo, fundamentacion, folios }) {
    const lineas = [
        "DATOS DEL SOLICITANTE",
        `Nombres y apellidos: ${nombres}`,
        `DNI: ${dni}`,
        telefono ? `Teléfono: ${telefono}` : null,
        domicilio || distrito ? `Domicilio: ${domicilio || "-"} — Distrito: ${distrito || "-"}` : null,
        correo ? `Correo: ${correo}` : null,
        "",
        "FUNDAMENTACIÓN DE LO QUE SOLICITA",
        fundamentacion,
        "",
        `N° de folios adjuntos: ${folios}`,
    ].filter((line) => line !== null);

    return lineas.join("\n");
}

// Recupera del texto libre de "descripcion" los dos datos que hacen falta
// para reconstruir el cargo del FUT (nombres y folios) — el resto (asunto,
// código, fecha) ya vive en columnas propias del expediente. Si no calzan
// los patrones (expediente que no vino del FUT Digital), no hay cargo que
// generar.
function parseDatosSolicitante(descripcion) {
    if (!descripcion) return null;
    const nombresMatch = descripcion.match(/Nombres y apellidos:\s*(.+)/);
    const foliosMatch = descripcion.match(/N° de folios adjuntos:\s*(\d+)/);
    if (!nombresMatch || !foliosMatch) return null;
    return { nombres: nombresMatch[1].trim(), folios: foliosMatch[1] };
}

// Convierte un array de bytes a un PDF de una sola página que solo contiene
// esa imagen a tamaño completo, escrito a mano (sin librerías: nada de
// jsPDF/pdf-lib). La imagen se embebe tal cual como JPEG (filtro
// /DCTDecode), que es el único formato para el que un visor de PDF puede
// leer los bytes crudos del canvas sin necesidad de decodificarlos primero.
function bytesToBinaryString(bytes) {
    let binary = "";
    const chunkSize = 0x8000;
    for (let i = 0; i < bytes.length; i += chunkSize) {
        binary += String.fromCharCode.apply(null, bytes.subarray(i, i + chunkSize));
    }
    return binary;
}

function buildImagePdf({ jpegBase64, width, height }) {
    const jpegBinary = bytesToBinaryString(
        Uint8Array.from(atob(jpegBase64), (char) => char.charCodeAt(0))
    );

    const parts = [];
    const offsets = {};
    let pos = 0;
    const push = (text) => {
        parts.push(text);
        pos += text.length;
    };

    push("%PDF-1.4\n");

    offsets[1] = pos;
    push("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n");

    offsets[2] = pos;
    push("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n");

    offsets[3] = pos;
    push(
        `3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 ${width} ${height}] ` +
            `/Resources << /XObject << /Im0 4 0 R >> >> /Contents 5 0 R >>\nendobj\n`
    );

    offsets[4] = pos;
    push(
        `4 0 obj\n<< /Type /XObject /Subtype /Image /Width ${width} /Height ${height} ` +
            `/ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode /Length ${jpegBinary.length} >>\nstream\n`
    );
    push(jpegBinary);
    push("\nendstream\nendobj\n");

    const contenido = `q\n${width} 0 0 ${height} 0 0 cm\n/Im0 Do\nQ`;
    offsets[5] = pos;
    push(`5 0 obj\n<< /Length ${contenido.length} >>\nstream\n${contenido}\nendstream\nendobj\n`);

    const xrefOffset = pos;
    push("xref\n0 6\n0000000000 65535 f \n");
    for (let i = 1; i <= 5; i += 1) {
        push(`${String(offsets[i]).padStart(10, "0")} 00000 n \n`);
    }
    push(`trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n${xrefOffset}\n%%EOF`);

    return btoa(parts.join(""));
}

// Genera un PDF (canvas nativo + armado manual del PDF, sin librerías) con el
// mismo contenido que el "Cargo digital" en pantalla. Se sube como documento
// del propio expediente para que quede descargable después de enviar la
// solicitud — no solo para el solicitante en el momento, también para
// Secretaría/personal interno cuando revisen el expediente más adelante.
function renderCargoPdf({ nombres, sumilla, codigo, folios }) {
    const canvas = document.createElement("canvas");
    canvas.width = 800;
    canvas.height = 420;
    const ctx = canvas.getContext("2d");

    ctx.fillStyle = "#ffffff";
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    ctx.textBaseline = "top";

    let y = 36;
    ctx.fillStyle = "#68769F";
    ctx.font = "700 12px Segoe UI, Arial, sans-serif";
    ctx.fillText("I.E. TUNGASUCA", 40, y);
    y += 22;
    ctx.fillStyle = "#1B2559";
    ctx.font = "800 21px Segoe UI, Arial, sans-serif";
    ctx.fillText("Formulario Único de Trámite (FUT)", 40, y);
    y += 40;

    ctx.strokeStyle = "#E9EDF7";
    ctx.beginPath();
    ctx.moveTo(40, y);
    ctx.lineTo(760, y);
    ctx.stroke();
    y += 22;

    const field = (label, value) => {
        ctx.fillStyle = "#68769F";
        ctx.font = "700 10.5px Segoe UI, Arial, sans-serif";
        ctx.fillText(label.toUpperCase(), 40, y);
        y += 17;
        ctx.fillStyle = "#1B2559";
        ctx.font = "500 14.5px Segoe UI, Arial, sans-serif";
        ctx.fillText(String(value), 40, y);
        y += 28;
    };

    field("Apellidos y nombres", nombres);
    field("Asunto", sumilla);
    field("Fecha", `Carabayllo, ${formatDate(new Date().toISOString())}`);
    field("N° expediente", codigo);
    field("N° folios", folios);

    y += 8;
    const columnas = ["N° Expediente", "Dirección", "Subdirección", "Recursos financieros", "Secretaría u otros"];
    const valores = [codigo, "Pendiente", "Pendiente", "Pendiente", "Pendiente"];
    const anchoColumna = 144;
    const alturaFila = 34;

    columnas.forEach((columna, index) => {
        const x = 40 + index * anchoColumna;
        ctx.strokeRect(x, y, anchoColumna, alturaFila);
        ctx.fillStyle = "#68769F";
        ctx.font = "700 9px Segoe UI, Arial, sans-serif";
        ctx.fillText(columna.toUpperCase(), x + 6, y + 8, anchoColumna - 12);
    });
    y += alturaFila;

    valores.forEach((valor, index) => {
        const x = 40 + index * anchoColumna;
        ctx.strokeRect(x, y, anchoColumna, alturaFila);
        ctx.fillStyle = "#1B2559";
        ctx.font = "500 11px Segoe UI, Arial, sans-serif";
        ctx.fillText(valor, x + 6, y + 11, anchoColumna - 12);
    });

    const jpegBase64 = canvas.toDataURL("image/jpeg", 0.92).split(",")[1];
    return buildImagePdf({ jpegBase64, width: canvas.width, height: canvas.height });
}

export async function submitFut(fut) {
    const descripcion = buildDescripcion(fut);

    const { expediente } = await createExpediente({
        tipo: "SOLICITUD",
        asunto: fut.sumilla,
        descripcion,
        prioridad: "NORMAL",
    });

    // "Documentos adjuntos" del expediente es solo para lo que el solicitante
    // adjunta de verdad (sus sustentos) — el cargo del FUT NO se sube ahí. Se
    // recupera después bajo demanda con exportFutDelExpediente(), a partir de
    // los datos que ya quedaron en el expediente (ver parseDatosSolicitante),
    // así que no hace falta guardarlo como archivo aparte.
    const uploads = [];

    if (fut.firmaDataUrl) {
        const base64 = fut.firmaDataUrl.split(",")[1];
        uploads.push(
            uploadDocumento({
                expedienteId: expediente.id,
                nombre: "firma.png",
                tipoDocumento: "ADJUNTO",
                extension: "png",
                contenidoBase64: base64,
            })
        );
    }

    for (const item of fut.documentos) {
        const extension = item.file.name.split(".").pop().toLowerCase();
        const nombre = item.descripcion ? `${item.descripcion} (${item.file.name})` : item.file.name;
        uploads.push(
            fileToBase64(item.file).then((contenidoBase64) =>
                uploadDocumento({
                    expedienteId: expediente.id,
                    nombre,
                    tipoDocumento: "ADJUNTO",
                    extension,
                    contenidoBase64,
                })
            )
        );
    }

    const results = await Promise.allSettled(uploads);
    const failedUploads = results.filter((item) => item.status === "rejected").length;

    return { expediente, failedUploads, totalUploads: uploads.length };
}

// Chequeo barato (sin tocar el canvas) para decidir si mostrar el botón
// "Descargar FUT" — exportFutDelExpediente() sí hace el trabajo pesado y
// solo debe llamarse al momento de descargar, no en cada render.
export function tieneFutExportable(expediente) {
    return Boolean(parseDatosSolicitante(expediente?.descripcion));
}

// Reconstruye el PDF del cargo a partir de los datos ya guardados en el
// propio expediente (no de un documento subido — ver el comentario en
// submitFut). Devuelve null si el expediente no vino del FUT Digital (no
// tiene "Nombres y apellidos" / "N° de folios" en su descripción).
export function exportFutDelExpediente(expediente) {
    const datos = parseDatosSolicitante(expediente.descripcion);
    if (!datos) return null;

    const contenidoBase64 = renderCargoPdf({
        nombres: datos.nombres,
        sumilla: expediente.asunto,
        codigo: expediente.codigo,
        folios: datos.folios,
    });

    return { nombre: `FUT-${expediente.codigo}.pdf`, contenidoBase64 };
}
