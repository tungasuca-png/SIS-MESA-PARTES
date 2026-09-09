import { createExpediente } from "./expedientesService";
import { downloadDocumento, fileToBase64, getDocumentosByExpediente, uploadDocumento } from "./documentosService";
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

// Recupera del texto libre de "descripcion" los datos que buildDescripcion()
// empaquetó ahí (nombres/DNI/teléfono/domicilio/distrito/correo,
// fundamentación y folios) — el resto (asunto, código, fecha) ya vive en
// columnas propias del expediente. Si no calzan los patrones básicos
// (nombres y folios), el expediente no vino del FUT Digital y no hay nada
// que reconstruir/mostrar por separado.
//
// Se usa tanto para regenerar el cargo del FUT (renderCargoPdf, que solo
// necesita nombres/folios) como para mostrar la descripción ordenada por
// campos en el detalle del expediente (ver ExpedienteDetail.jsx), en vez de
// un bloque de texto plano.
export function parseDatosSolicitante(descripcion) {
    if (!descripcion) return null;
    const nombresMatch = descripcion.match(/Nombres y apellidos:\s*(.+)/);
    const foliosMatch = descripcion.match(/N° de folios adjuntos:\s*(\d+)/);
    if (!nombresMatch || !foliosMatch) return null;

    const dniMatch = descripcion.match(/DNI:\s*(.+)/);
    const telefonoMatch = descripcion.match(/Teléfono:\s*(.+)/);
    const domicilioMatch = descripcion.match(/Domicilio:\s*(.+?)\s*—\s*Distrito:\s*(.+)/);
    const correoMatch = descripcion.match(/Correo:\s*(.+)/);
    const fundamentacionMatch = descripcion.match(
        /FUNDAMENTACIÓN DE LO QUE SOLICITA\n([\s\S]*?)\n\nN° de folios adjuntos:/
    );

    const domicilio = domicilioMatch && domicilioMatch[1].trim() !== "-" ? domicilioMatch[1].trim() : "";
    const distrito = domicilioMatch && domicilioMatch[2].trim() !== "-" ? domicilioMatch[2].trim() : "";

    return {
        nombres: nombresMatch[1].trim(),
        folios: foliosMatch[1],
        dni: dniMatch ? dniMatch[1].trim() : "",
        telefono: telefonoMatch ? telefonoMatch[1].trim() : "",
        domicilio,
        distrito,
        correo: correoMatch ? correoMatch[1].trim() : "",
        fundamentacion: fundamentacionMatch ? fundamentacionMatch[1].trim() : "",
    };
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

// Layout tipo A4 (ancho fijo, alto variable según el contenido real) para
// que el PDF descargado se parezca al "Cargo digital" en pantalla
// (FutDigital.jsx), con TODOS los campos del solicitante y la
// fundamentación completa (con salto de línea real), no solo 5 campos en
// una sola línea como antes.
const PAGE_WIDTH = 850;
const MARGIN = 50;
const CONTENT_WIDTH = PAGE_WIDTH - MARGIN * 2;

// Envuelve texto por ancho disponible, respetando los saltos de línea que
// ya tenga (p. ej. párrafos de la fundamentación). ctx.font debe estar
// seteado antes de llamar a esto.
function wrapText(ctx, text, maxWidth) {
    const paragraphs = String(text ?? "").split("\n");
    const lines = [];
    paragraphs.forEach((paragraph) => {
        if (paragraph === "") {
            lines.push("");
            return;
        }
        const words = paragraph.split(" ");
        let current = "";
        words.forEach((word) => {
            const attempt = current ? `${current} ${word}` : word;
            if (current && ctx.measureText(attempt).width > maxWidth) {
                lines.push(current);
                current = word;
            } else {
                current = attempt;
            }
        });
        if (current) lines.push(current);
    });
    return lines;
}

function loadImage(dataUrl) {
    return new Promise((resolve, reject) => {
        const img = new Image();
        img.onload = () => resolve(img);
        img.onerror = () => reject(new Error("No se pudo cargar la imagen de la firma."));
        img.src = dataUrl;
    });
}

function drawSectionTitle(ctx, y, text) {
    ctx.fillStyle = "#68769F";
    ctx.font = "700 11px Segoe UI, Arial, sans-serif";
    ctx.fillText(text.toUpperCase(), MARGIN, y);
    return y + 20;
}

// Dos campos (label + valor) por fila, con una línea debajo de cada uno —
// igual que la grilla ".fut-cargo-grid" del cargo en pantalla.
function drawFieldGrid(ctx, y, fields) {
    const colGap = 24;
    const colWidth = (CONTENT_WIDTH - colGap) / 2;
    for (let i = 0; i < fields.length; i += 2) {
        fields.slice(i, i + 2).forEach(([label, value], col) => {
            const x = MARGIN + col * (colWidth + colGap);
            ctx.fillStyle = "#68769F";
            ctx.font = "700 9.5px Segoe UI, Arial, sans-serif";
            ctx.fillText(label.toUpperCase(), x, y);
            ctx.fillStyle = "#1B2559";
            ctx.font = "500 13.5px Segoe UI, Arial, sans-serif";
            ctx.fillText(String(value || "—"), x, y + 16);
            ctx.strokeStyle = "#E9EDF7";
            ctx.beginPath();
            ctx.moveTo(x, y + 34);
            ctx.lineTo(x + colWidth, y + 34);
            ctx.stroke();
        });
        y += 44;
    }
    return y;
}

// Párrafo con líneas de cuaderno de fondo (como ".fut-cargo-lined-text"),
// con un mínimo de líneas aunque el texto sea corto para que se note el
// espacio destinado a la fundamentación.
function drawLinedParagraph(ctx, y, text, minLines = 4) {
    const lineHeight = 22;
    ctx.font = "500 12.5px Segoe UI, Arial, sans-serif";
    const lines = text ? wrapText(ctx, text, CONTENT_WIDTH) : [];
    const totalLines = Math.max(lines.length, minLines);

    for (let i = 0; i < totalLines; i += 1) {
        const baseline = y + i * lineHeight + lineHeight - 6;
        ctx.strokeStyle = "#E9EDF7";
        ctx.beginPath();
        ctx.moveTo(MARGIN, baseline);
        ctx.lineTo(MARGIN + CONTENT_WIDTH, baseline);
        ctx.stroke();
        if (lines[i]) {
            ctx.fillStyle = "#1B2559";
            ctx.fillText(lines[i], MARGIN, baseline - 16);
        }
    }
    if (!text) {
        ctx.fillStyle = "#A0A8C0";
        ctx.font = "italic 500 12.5px Segoe UI, Arial, sans-serif";
        ctx.fillText("—", MARGIN, y + lineHeight - 22);
    }
    return y + totalLines * lineHeight + 14;
}

function drawResumenRow(ctx, y, label, value) {
    const labelWidth = CONTENT_WIDTH * 0.32;
    ctx.strokeStyle = "#E9EDF7";
    ctx.strokeRect(MARGIN, y, labelWidth, 30);
    ctx.strokeRect(MARGIN + labelWidth, y, CONTENT_WIDTH - labelWidth, 30);
    ctx.fillStyle = "#68769F";
    ctx.font = "700 11px Segoe UI, Arial, sans-serif";
    ctx.fillText(label.toUpperCase(), MARGIN + 10, y + 11);
    ctx.fillStyle = "#1B2559";
    ctx.font = "600 12.5px Segoe UI, Arial, sans-serif";
    ctx.fillText(String(value), MARGIN + labelWidth + 10, y + 11, CONTENT_WIDTH - labelWidth - 20);
    return y + 30;
}

// Dibuja el cargo completo sobre el contexto recibido y devuelve la
// posición Y final usada. Se llama dos veces: una sobre un canvas
// descartable solo para medir cuánto alto hace falta (el texto de la
// fundamentación y la lista de documentos son de largo variable), y otra
// sobre el canvas final ya con el alto correcto — así no queda ni espacio
// vacío de más ni contenido cortado.
function drawCargo(ctx, datos) {
    const { nombres, dni, telefono, domicilio, distrito, correo, sumilla, fundamentacion, folios, documentos, codigo, fecha, firmaImg } = datos;

    ctx.textBaseline = "alphabetic";
    let y = MARGIN;

    ctx.fillStyle = "#68769F";
    ctx.font = "700 12px Segoe UI, Arial, sans-serif";
    ctx.fillText("I.E. TUNGASUCA", MARGIN, y);
    y += 22;
    ctx.fillStyle = "#1B2559";
    ctx.font = "800 20px Segoe UI, Arial, sans-serif";
    ctx.fillText("Formulario Único de Trámite (FUT)", MARGIN, y);
    y += 26;
    ctx.fillStyle = "#1B2559";
    ctx.font = "700 13px Segoe UI, Arial, sans-serif";
    ctx.fillText('SEÑORA DIRECTORA DE LA I.E. "TUNGASUCA":', MARGIN, y);
    y += 26;

    y = drawSectionTitle(ctx, y, "Datos del solicitante");
    y = drawFieldGrid(ctx, y, [
        ["Nombres y apellidos", nombres],
        ["DNI", dni],
        ["Teléfono", telefono],
        ["Domicilio actual", domicilio],
        ["Distrito", distrito],
        ["Correo electrónico", correo],
    ]);
    y += 6;

    y = drawSectionTitle(ctx, y, "Asunto");
    ctx.fillStyle = "#1B2559";
    ctx.font = "500 13px Segoe UI, Arial, sans-serif";
    const asuntoLines = wrapText(ctx, sumilla, CONTENT_WIDTH);
    asuntoLines.forEach((line, index) => ctx.fillText(line, MARGIN, y + index * 18));
    y += asuntoLines.length * 18 + 14;

    y = drawSectionTitle(ctx, y, "Fundamentación de lo que solicita");
    y = drawLinedParagraph(ctx, y, fundamentacion, 5);

    y = drawSectionTitle(ctx, y, "Documento que se adjunta (sustentatorio de su solicitud)");
    ctx.font = "500 12.5px Segoe UI, Arial, sans-serif";
    if (!documentos || documentos.length === 0) {
        ctx.fillStyle = "#A0A8C0";
        ctx.font = "italic 500 12.5px Segoe UI, Arial, sans-serif";
        ctx.fillText("Ninguno", MARGIN, y + 10);
        y += 22;
    } else {
        documentos.forEach((nombreDoc) => {
            ctx.fillStyle = "#1B2559";
            ctx.font = "500 12.5px Segoe UI, Arial, sans-serif";
            const lines = wrapText(ctx, `•  ${nombreDoc}`, CONTENT_WIDTH);
            lines.forEach((line, index) => ctx.fillText(line, MARGIN, y + 10 + index * 17));
            y += lines.length * 17 + 2;
        });
    }
    ctx.fillStyle = "#68769F";
    ctx.font = "500 11.5px Segoe UI, Arial, sans-serif";
    ctx.fillText(`N° de folios: ${folios}`, MARGIN, y + 12);
    y += 32;

    const fechaFirmaY = y;
    ctx.fillStyle = "#1B2559";
    ctx.font = "600 13px Segoe UI, Arial, sans-serif";
    ctx.fillText(`Fecha: Carabayllo, ${fecha}`, MARGIN, fechaFirmaY + 40);

    const firmaBoxWidth = 180;
    const firmaX = MARGIN + CONTENT_WIDTH - firmaBoxWidth;
    if (firmaImg) {
        const maxH = 60;
        const scale = Math.min(firmaBoxWidth / firmaImg.width, maxH / firmaImg.height);
        const drawW = firmaImg.width * scale;
        const drawH = firmaImg.height * scale;
        ctx.drawImage(firmaImg, firmaX + (firmaBoxWidth - drawW) / 2, fechaFirmaY, drawW, drawH);
    } else {
        ctx.fillStyle = "#A0A8C0";
        ctx.font = "italic 500 12px Segoe UI, Arial, sans-serif";
        ctx.fillText("Firma no capturada", firmaX, fechaFirmaY + 34);
    }
    ctx.strokeStyle = "#E9EDF7";
    ctx.beginPath();
    ctx.moveTo(firmaX, fechaFirmaY + 44);
    ctx.lineTo(firmaX + firmaBoxWidth, fechaFirmaY + 44);
    ctx.stroke();
    ctx.fillStyle = "#68769F";
    ctx.font = "700 9.5px Segoe UI, Arial, sans-serif";
    ctx.fillText("FIRMA DEL SOLICITANTE", firmaX, fechaFirmaY + 58);
    y = fechaFirmaY + 74;

    ctx.strokeStyle = "#E9EDF7";
    ctx.beginPath();
    ctx.moveTo(MARGIN, y + 16);
    ctx.lineTo(MARGIN + CONTENT_WIDTH, y + 16);
    ctx.stroke();
    y += 40;

    y = drawSectionTitle(ctx, y, "Cargo de recepción");
    y = drawResumenRow(ctx, y, "Apellidos y nombres", nombres);
    y = drawResumenRow(ctx, y, "Asunto", sumilla);
    y = drawResumenRow(ctx, y, "Fecha", `Carabayllo, ${fecha}`);
    y = drawResumenRow(ctx, y, "N° expediente", codigo);
    y = drawResumenRow(ctx, y, "N° folios", folios);

    return y + MARGIN;
}

// Genera un PDF (canvas nativo + armado manual del PDF, sin librerías) con
// el mismo contenido que el "Cargo digital" en pantalla. Se reconstruye al
// vuelo a partir de los datos ya guardados en el propio expediente y en sus
// documentos reales (no se guarda como archivo aparte — ver comentario en
// submitFut).
async function renderCargoPdf(datos) {
    const firmaImg = datos.firmaDataUrl ? await loadImage(datos.firmaDataUrl).catch(() => null) : null;
    const full = { ...datos, firmaImg };

    // Primera pasada sobre un canvas descartable solo para medir el alto
    // real que va a ocupar el contenido (varía según la fundamentación y la
    // cantidad de documentos adjuntos).
    const measureCanvas = document.createElement("canvas");
    measureCanvas.width = PAGE_WIDTH;
    measureCanvas.height = 10;
    const contentHeight = drawCargo(measureCanvas.getContext("2d"), full);

    const canvas = document.createElement("canvas");
    canvas.width = PAGE_WIDTH;
    canvas.height = Math.ceil(contentHeight);
    const ctx = canvas.getContext("2d");
    ctx.fillStyle = "#ffffff";
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    drawCargo(ctx, full);

    const jpegBase64 = canvas.toDataURL("image/jpeg", 0.95).split(",")[1];
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
//
// Además intenta traer los documentos REALES ya subidos a este expediente
// (Documentos Service) para listarlos en el cargo con su nombre real, y si
// entre ellos está la firma capturada en el FUT ("firma.png", subida por
// submitFut), la descarga y la embebe como imagen — igual que se ve en
// pantalla. Si esa consulta falla (p. ej. el usuario ya no tiene acceso a
// ese detalle), el cargo se genera igual, solo que sin esos dos extras.
export async function exportFutDelExpediente(expediente) {
    const datos = parseDatosSolicitante(expediente.descripcion);
    if (!datos) return null;

    let documentos = [];
    let firmaDataUrl = null;
    try {
        const { documentos: lista } = await getDocumentosByExpediente(expediente.id);
        const firma = (lista || []).find((item) => item.nombre === "firma.png");
        documentos = (lista || []).filter((item) => item.nombre !== "firma.png").map((item) => item.nombre);
        if (firma) {
            const { contenido } = await downloadDocumento(firma.id);
            firmaDataUrl = `data:image/png;base64,${contenido}`;
        }
    } catch {
        // Sin documentos/firma extra — el cargo igual sale con los datos del
        // propio expediente.
    }

    const contenidoBase64 = await renderCargoPdf({
        nombres: datos.nombres,
        dni: datos.dni,
        telefono: datos.telefono,
        domicilio: datos.domicilio,
        distrito: datos.distrito,
        correo: datos.correo,
        sumilla: expediente.asunto,
        fundamentacion: datos.fundamentacion,
        folios: datos.folios,
        documentos,
        firmaDataUrl,
        codigo: expediente.codigo,
        fecha: formatDate(expediente.fecha_registro),
    });

    return { nombre: `FUT-${expediente.codigo}.pdf`, contenidoBase64 };
}
