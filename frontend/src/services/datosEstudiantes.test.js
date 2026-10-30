import { afterEach, describe, expect, it } from "vitest";
import api from "./api";
import { analizarDatos, confirmarDatos, eliminarPlantillaDatos, exportarDatos } from "./datosEstudiantesService";

// Usa la instancia REAL de axios (con "Content-Type: application/json" por
// defecto) y verifica lo que realmente se enviaría.
const adaptador = api.defaults.adapter;
afterEach(() => {
    api.defaults.adapter = adaptador;
});

function capturar() {
    const enviados = [];
    api.defaults.adapter = async (config) => {
        enviados.push(config);
        return { data: {}, status: 200, statusText: "OK", headers: {}, config };
    };
    return enviados;
}

describe("servicio del motor de datos", () => {
    it("analizar/confirmar envían multipart con el archivo y la solicitud en JSON", async () => {
        const enviados = capturar();
        const archivo = new File(["dni;nombre"], "alumnos.csv");
        await analizarDatos(archivo, { plantilla_id: "p1" });
        await confirmarDatos(archivo, { huella: "h", mapeo_confirmado: true, aceptadas: [4] });
        expect(enviados[0].url).toBe("/api/estudiantes/datos/analizar");
        expect(enviados[0].data).toBeInstanceOf(FormData);
        expect(enviados[0].data.get("archivo").name).toBe("alumnos.csv");
        expect(JSON.parse(enviados[0].data.get("solicitud"))).toEqual({ plantilla_id: "p1" });
        expect(String(enviados[0].headers.getContentType())).toContain("multipart/form-data");
        expect(JSON.parse(enviados[1].data.get("solicitud"))).toMatchObject({ huella: "h", aceptadas: [4] });
    });

    it("exportar envía filtros y configuración como JSON; eliminar usa POST (sin DELETE)", async () => {
        const enviados = capturar();
        await exportarDatos({ q: "ana", formato: "csv", columnas: [{ clave: "codigo", encabezado: "Código" }] });
        await eliminarPlantillaDatos("id 1", "EXPORTACION");
        expect(enviados[0].method).toBe("post");
        expect(JSON.parse(enviados[0].data)).toMatchObject({ q: "ana", formato: "csv" });
        expect(enviados[1].method).toBe("post");
        expect(enviados[1].url).toBe("/api/estudiantes/datos/plantillas/id%201/eliminar");
    });
});
