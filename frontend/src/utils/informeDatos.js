import { descargarBase64 } from "./descargas";

// Informe de errores/avisos de una importación, como CSV (UTF-8 con BOM para
// que Excel lea tildes). Igual que en el backend, un valor que empiece con
// = + @ (o "-" sin ser número) se antepone con "'" para que Excel no lo
// ejecute como fórmula.
function proteger(valor) {
    const v = valor == null ? "" : String(valor);
    if (/^[=+@\t\r]/.test(v) || (/^-/.test(v) && !/^-\d+([.,]\d+)?$/.test(v))) return `'${v}`;
    return v;
}

function celda(valor) {
    const v = proteger(valor);
    return /[";\n\r]/.test(v) ? `"${v.replace(/"/g, '""')}"` : v;
}

const ETIQUETAS = {
    NUEVO: "Nuevo",
    EXISTENTE: "Ya registrado",
    DUPLICADO: "Duplicado en el archivo",
    POR_CONFIRMAR: "Por confirmar",
    CONFLICTO: "Conflicto",
    INVALIDO: "Inválido",
};

export function etiquetaCategoria(categoria) {
    return ETIQUETAS[categoria] ?? categoria;
}

export function informeCSV(registros, { soloProblemas = true } = {}) {
    const filas = soloProblemas
        ? registros.filter((r) => r.errores?.length || r.avisos?.length || r.correcciones?.length || r.categoria !== "NUEVO")
        : registros;
    const encabezados = ["Fila", "Categoría", "Resultado", "Documento", "Apellidos", "Nombres", "Errores", "Avisos", "Correcciones", "Coincidencia"];
    const lineas = [encabezados.join(";")];
    for (const r of filas) {
        lineas.push(
            [
                r.fila,
                etiquetaCategoria(r.categoria),
                r.resultado || "",
                r.documento,
                r.apellidos,
                r.nombres,
                (r.errores || []).join(" | "),
                (r.avisos || []).join(" | "),
                (r.correcciones || []).join(" | "),
                r.coincidencia || "",
            ]
                .map(celda)
                .join(";")
        );
    }
    return "\uFEFF" + lineas.join("\r\n") + "\r\n";
}

export function descargarInforme(registros, nombre) {
    const texto = informeCSV(registros);
    const bytes = new TextEncoder().encode(texto);
    let binario = "";
    for (const b of bytes) binario += String.fromCharCode(b);
    const base64 = btoa(binario);
    descargarBase64(base64, nombre, "text/csv");
}
