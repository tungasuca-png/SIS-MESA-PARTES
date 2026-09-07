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

// Genera una imagen (PNG, canvas nativo, sin librerías) con el mismo
// contenido que el "Cargo digital" en pantalla. Se sube como documento del
// propio expediente para que quede descargable después de enviar la
// solicitud — no solo para el solicitante en el momento, también para
// Secretaría/personal interno cuando revisen el expediente más adelante.
function renderCargoPng({ nombres, sumilla, codigo, folios }) {
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

    return canvas.toDataURL("image/png").split(",")[1];
}

export async function submitFut(fut) {
    const descripcion = buildDescripcion(fut);

    const { expediente } = await createExpediente({
        tipo: "SOLICITUD",
        asunto: fut.sumilla,
        descripcion,
        prioridad: "NORMAL",
    });

    const uploads = [];

    // Cargo/FUT descargable: queda como documento del expediente para que el
    // solicitante y el personal interno (Secretaría, etc.) puedan bajarlo
    // más adelante, no solo verlo en pantalla al momento de enviar.
    // tipoDocumento="ADJUNTO" porque un SOLICITANTE solo puede subir ese tipo
    // (ver authorization.CanUpload de Documentos Service).
    uploads.push(
        uploadDocumento({
            expedienteId: expediente.id,
            nombre: `FUT-${expediente.codigo}.png`,
            tipoDocumento: "ADJUNTO",
            extension: "png",
            contenidoBase64: renderCargoPng({
                nombres: fut.nombres,
                sumilla: fut.sumilla,
                codigo: expediente.codigo,
                folios: fut.folios,
            }),
        })
    );

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
