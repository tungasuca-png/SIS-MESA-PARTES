import { describe, expect, it } from "vitest";
import { estudianteError, parseCodigoError } from "./estudiantesErrors";
import { httpError } from "../test/estudiantesTestUtils";

describe("estudianteError", () => {
    it("extrae el código estable del backend", () => {
        expect(parseCodigoError("rpc error: code = Aborted desc = version_conflict: el estudiante fue modificado\n")).toEqual({
            code: "version_conflict",
            detail: "el estudiante fue modificado",
        });
    });

    it.each([
        [409, "duplicate_codigo: x", "conflict", /código/],
        [409, "duplicate_document: x", "conflict", /documento/],
        [409, "duplicate_codigo_siagie: x", "conflict", /SIAGIE/],
        [409, "version_conflict: x", "conflict", /recarga/i],
        [404, "student_not_found: x", "not_found", /no existe/],
        [403, "permission_denied: x", "forbidden", /permiso/],
        [401, null, "auth", /sesión/],
    ])("%i %s -> %s", (status, desc, kind, pattern) => {
        const result = estudianteError(httpError(status, desc));
        expect(result.kind).toBe(kind);
        expect(result.message).toMatch(pattern);
    });

    it("validación: muestra el texto del backend tras un prefijo fijo", () => {
        const result = estudianteError(httpError(400, "invalid_document: el DNI debe tener 8 dígitos"));
        expect(result).toMatchObject({ kind: "validation", code: "invalid_document" });
        expect(result.message).toBe("Documento inválido: el DNI debe tener 8 dígitos.");
        expect(estudianteError(httpError(400, "invalid_state: estado inválido")).message).toMatch(/^Estado inválido/);
        expect(estudianteError(httpError(400, "validation_error: nombres es obligatorio")).message).toMatch(/^Datos inválidos/);
    });

    it("nunca muestra el texto crudo del Gateway ni errores internos", () => {
        const result = estudianteError(httpError(500, "ERROR: duplicate key value violates unique constraint (SQLSTATE 23505)"));
        expect(result.kind).toBe("server");
        expect(result.message).not.toMatch(/SQLSTATE|duplicate key|rpc error/);
        expect(estudianteError(httpError(400, "algo sin codigo")).message).toBe("Los datos ingresados no son válidos.");
        expect(estudianteError(new Error("network")).kind).toBe("network");
    });
});
