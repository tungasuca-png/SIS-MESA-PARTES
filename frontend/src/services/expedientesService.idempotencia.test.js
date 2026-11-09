import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("./api", () => ({ default: { patch: vi.fn() } }));

import api from "./api";
import {
    changeEstado,
    corregirExpediente,
    derivarExpediente,
    nuevaClaveIdempotencia,
    rechazarExpediente,
    resolverExpediente,
} from "./expedientesService";

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
const errorHttp = (status) => Object.assign(new Error(`HTTP ${status}`), { response: { status, data: "" } });

describe("operaciones idempotentes (A + B)", () => {
    beforeEach(() => api.patch.mockReset());

    it("derivar envía Idempotency-Key, origen esperado y los datos", async () => {
        api.patch.mockResolvedValueOnce({ data: { repetido: false } });
        await derivarExpediente("e1", { areaDestino: "DIRECTOR", areaOrigenEsperada: "SECRETARIA", motivo: "m", condicion: "" }, "clave-1");
        expect(api.patch).toHaveBeenCalledWith(
            "/api/expedientes/e1/derivar",
            { area_destino: "DIRECTOR", area_origen_esperada: "SECRETARIA", motivo: "m", condicion: "" },
            { headers: { "Idempotency-Key": "clave-1" } }
        );
    });

    it("ante 504 reintenta UNA vez con la MISMA clave", async () => {
        api.patch.mockRejectedValueOnce(errorHttp(504)).mockResolvedValueOnce({ data: { repetido: true } });
        const data = await rechazarExpediente("e2", "motivo", "DIRECTOR", "clave-2");
        expect(data.repetido).toBe(true);
        expect(api.patch).toHaveBeenCalledTimes(2);
        expect(api.patch.mock.calls[0][2]).toEqual({ headers: { "Idempotency-Key": "clave-2" } });
        expect(api.patch.mock.calls[1][2]).toEqual({ headers: { "Idempotency-Key": "clave-2" } });
    });

    it("ante un error de red también reintenta una sola vez", async () => {
        api.patch.mockRejectedValueOnce(new Error("Network Error")).mockRejectedValueOnce(new Error("Network Error"));
        await expect(resolverExpediente("e3", "clave-3")).rejects.toThrow("Network Error");
        expect(api.patch).toHaveBeenCalledTimes(2);
    });

    it("ante 409 (cambió entretanto) NO reintenta", async () => {
        api.patch.mockRejectedValueOnce(errorHttp(409));
        await expect(changeEstado("e4", "EN_PROCESO", "PENDIENTE", "clave-4")).rejects.toMatchObject({ response: { status: 409 } });
        expect(api.patch).toHaveBeenCalledTimes(1);
        expect(api.patch.mock.calls[0][1]).toEqual({ nuevo_estado: "EN_PROCESO", estado_esperado: "PENDIENTE" });
    });

    it("sin clave explícita genera un UUID v4 distinto por operación", async () => {
        api.patch.mockResolvedValue({ data: {} });
        await corregirExpediente("e5", "nueva");
        await corregirExpediente("e5", "nueva");
        const k1 = api.patch.mock.calls[0][2].headers["Idempotency-Key"];
        const k2 = api.patch.mock.calls[1][2].headers["Idempotency-Key"];
        expect(k1).toMatch(UUID);
        expect(k2).toMatch(UUID);
        expect(k1).not.toBe(k2);
        expect(nuevaClaveIdempotencia()).toMatch(UUID);
    });
});
