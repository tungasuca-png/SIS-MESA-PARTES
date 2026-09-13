import api from "./api";
import { createExpediente } from "./expedientesService";
import { fileToBase64, uploadDocumento } from "./documentosService";

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

export async function submitFut(fut) {
    const descripcion = buildDescripcion(fut);

    const { expediente } = await createExpediente({
        tipo: fut.tipo,
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

// Chequeo barato (sin llamar al backend) para decidir si mostrar el botón
// "Descargar FUT" — exportFutDelExpediente() sí hace la petición real y
// solo debe llamarse al momento de descargar, no en cada render.
export function tieneFutExportable(expediente) {
    return Boolean(parseDatosSolicitante(expediente?.descripcion));
}

// Reconstruye el PDF del cargo a partir de los datos ya guardados en el
// propio expediente (no de un documento subido — ver el comentario en
// submitFut). Devuelve null si el expediente no vino del FUT Digital (no
// tiene "Nombres y apellidos" / "N° de folios" en su descripción) — mismo
// criterio que tieneFutExportable(), verificado antes de llamar al backend
// para no generar una petición (y su error) cuando el botón ni debería
// mostrarse.
//
// El PDF en sí ya NO se genera en el navegador (antes era un canvas
// rasterizado a JPEG y embebido en un PDF armado a mano — por eso se veía
// borroso al hacer zoom o imprimir). Ahora lo genera el Gateway con texto
// vectorial real (Go + github.com/go-pdf/fpdf), reutilizando los mismos
// datos (descripción del expediente + documentos/firma reales de
// Documentos Service) — ver backend/gateway/internal/logic/exportfutpdflogic.go.
export async function exportFutDelExpediente(expediente) {
    if (!tieneFutExportable(expediente)) return null;

    const response = await api.get(`/api/expedientes/${encodeURIComponent(expediente.id)}/fut-pdf`);
    return { nombre: response.data.nombre, contenidoBase64: response.data.contenido_base64 };
}
