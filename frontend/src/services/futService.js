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

export async function submitFut(fut) {
    const descripcion = buildDescripcion(fut);

    const { expediente } = await createExpediente({
        tipo: "SOLICITUD",
        asunto: fut.sumilla,
        descripcion,
        prioridad: "NORMAL",
    });

    const uploads = [];

    if (fut.firmaDataUrl) {
        const base64 = fut.firmaDataUrl.split(",")[1];
        uploads.push(
            uploadDocumento({
                expedienteId: expediente.id,
                nombre: "firma.png",
                tipoDocumento: "OTRO",
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
