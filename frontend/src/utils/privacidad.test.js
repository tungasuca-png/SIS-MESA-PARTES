import { describe, expect, it } from "vitest";
import { enmascararCorreo, enmascararDocumento, enmascararTelefono } from "./privacidad";

describe("privacidad", () => {
    it("enmascara el documento dejando solo el final", () => {
        expect(enmascararDocumento("12345678")).toBe("••••5678");
        expect(enmascararDocumento("AB12")).toBe("••12");
        expect(enmascararDocumento("")).toBe("");
        expect(enmascararDocumento(undefined)).toBe("");
        expect(enmascararDocumento("12345678")).not.toContain("1234");
    });

    it("enmascara correo y teléfono", () => {
        expect(enmascararCorreo("ana.perez@colegio.pe")).toBe("a•••@colegio.pe");
        expect(enmascararCorreo("sin-arroba")).toBe("•••");
        expect(enmascararTelefono("987 654 321")).toBe("••••••321");
        expect(enmascararTelefono("")).toBe("");
    });
});
