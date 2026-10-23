import { afterEach, describe, expect, it } from "vitest";
import api from "./api";
import { confirmarImportacionEstudiantes, previsualizarImportacionEstudiantes } from "./estudiantesService";

// La instancia de axios tiene "Content-Type: application/json" por defecto;
// con ese tipo, axios 1.x serializa un FormData como JSON. Esta prueba usa la
// instancia REAL y verifica que el archivo viaje como multipart/form-data.
const adaptadorOriginal = api.defaults.adapter;
afterEach(() => {
    api.defaults.adapter = adaptadorOriginal;
});

function capturar() {
    const enviados = [];
    api.defaults.adapter = async (config) => {
        enviados.push(config);
        return { data: { huella: "h", resumen: {}, filas: [] }, status: 200, statusText: "OK", headers: {}, config };
    };
    return enviados;
}

describe("envío del Excel a importar", () => {
    it("vista previa: multipart con el campo «archivo», sin convertirlo a JSON", async () => {
        const enviados = capturar();
        const archivo = new File(["PK"], "estudiantes.xlsx");
        await previsualizarImportacionEstudiantes(archivo);
        const config = enviados[0];
        expect(config.url).toBe("/api/estudiantes/importacion/preview");
        expect(config.data).toBeInstanceOf(FormData);
        expect(config.data.get("archivo").name).toBe("estudiantes.xlsx");
        expect(config.data.get("huella")).toBeNull();
        expect(String(config.headers.getContentType())).toContain("multipart/form-data");
    });

    it("confirmación: envía además la huella de la vista previa", async () => {
        const enviados = capturar();
        await confirmarImportacionEstudiantes(new File(["PK"], "e.xlsx"), "abc123");
        expect(enviados[0].url).toBe("/api/estudiantes/importacion/confirmar");
        expect(enviados[0].data.get("huella")).toBe("abc123");
    });
});
