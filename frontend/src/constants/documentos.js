// Valores REALES del contrato de Documentos Service (no son mock: deben
// coincidir exactamente con internal/validation/validation.go del
// microservicio).

export const TIPOS_DOCUMENTO = [
    { value: "ADJUNTO", label: "Adjunto (sustento)" },
    { value: "PROVEIDO", label: "Proveído" },
    { value: "ACTA", label: "Acta" },
    { value: "INFORME", label: "Informe" },
    { value: "FORMATO", label: "Formato" },
    { value: "OTRO", label: "Otro" },
];

export const EXTENSIONES_PERMITIDAS = ["pdf", "jpg", "jpeg", "png", "doc", "docx"];

export const MAX_TAMANO_BYTES = 5 * 1024 * 1024; // 5 MiB, ver README de Documentos Service

export const MIME_POR_EXTENSION = {
    pdf: "application/pdf",
    jpg: "image/jpeg",
    jpeg: "image/jpeg",
    png: "image/png",
    doc: "application/msword",
    docx: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
};
