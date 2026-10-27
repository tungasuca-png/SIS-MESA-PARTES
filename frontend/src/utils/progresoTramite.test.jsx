import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { calcularSeguimiento } from "./progresoTramite";
import { LineaTiempoTramite, ProgresoTramite } from "../components/portal/SeguimientoTramite";

// Datos ficticios: solo campos que el backend realmente devuelve.
const base = {
    id: "exp-1",
    codigo: "EXP-2026-000001",
    fecha_registro: "2026-10-20T10:00:00Z",
    fecha_actualizacion: "2026-10-22T15:00:00Z",
};

const estados = (s) => s.etapas.map((e) => e.estado);

describe("calcularSeguimiento", () => {
    it("recién registrado: 1 de 4 etapas (25 %), revisión en curso", () => {
        const s = calcularSeguimiento({ ...base, estado: "PENDIENTE" });
        expect(s.porcentaje).toBe(25);
        expect(estados(s)).toEqual(["completada", "actual", "pendiente", "pendiente"]);
        expect(s.etapaActual).toBe("Revisión en Mesa de Partes");
    });

    it("en proceso: 50 %, la etapa actual no cuenta como completada", () => {
        const s = calcularSeguimiento({ ...base, estado: "EN_PROCESO" });
        expect(s.porcentaje).toBe(50);
        expect(estados(s)).toEqual(["completada", "completada", "actual", "pendiente"]);
    });

    it("atendido: 100 % y fecha de cierre real", () => {
        const s = calcularSeguimiento({ ...base, estado: "ATENDIDO" });
        expect(s.porcentaje).toBe(100);
        expect(s.finalizado).toBe(true);
        expect(s.etapas[3].fecha).toBe(base.fecha_actualizacion);
    });

    it("no inventa fechas: solo registro y cierre las tienen", () => {
        const s = calcularSeguimiento({ ...base, estado: "EN_PROCESO" });
        expect(s.etapas.map((e) => e.fecha)).toEqual([base.fecha_registro, "", "", ""]);
    });

    it("observado: revisión bloqueada, no avanza hacia el final", () => {
        const s = calcularSeguimiento({ ...base, estado: "OBSERVADO" });
        expect(s.porcentaje).toBe(25);
        expect(estados(s)).toEqual(["completada", "bloqueada", "pendiente", "pendiente"]);
        expect(s.motivoObservacion).toBe("");
    });

    it("observado por rechazo: toma el motivo de la derivación simultánea a Secretaría", () => {
        const derivaciones = [
            { id: "d2", destino: "SECRETARIA", motivo: "Falta constancia médica", fecha_registro: "2026-10-22T15:00:01Z" },
            { id: "d1", destino: "DIRECTOR", motivo: "Para su firma", fecha_registro: "2026-10-21T09:00:00Z" },
        ];
        const s = calcularSeguimiento({ ...base, estado: "OBSERVADO" }, derivaciones);
        expect(s.motivoObservacion).toBe("Falta constancia médica");
    });

    it("no usa como motivo una derivación de otro momento", () => {
        const derivaciones = [
            { id: "d1", destino: "SECRETARIA", motivo: "Antiguo", fecha_registro: "2026-10-01T09:00:00Z" },
        ];
        const s = calcularSeguimiento({ ...base, estado: "OBSERVADO" }, derivaciones);
        expect(s.motivoObservacion).toBe("");
    });

    it("estado desconocido o sin expediente: sin porcentaje", () => {
        expect(calcularSeguimiento({ ...base, estado: "OTRO" })).toBeNull();
        expect(calcularSeguimiento(null)).toBeNull();
    });
});

describe("componentes de seguimiento", () => {
    afterEach(cleanup);

    it("muestra porcentaje accesible y etapa actual", () => {
        render(<ProgresoTramite seguimiento={calcularSeguimiento({ ...base, estado: "EN_PROCESO" })} />);
        expect(screen.getByRole("progressbar").getAttribute("aria-valuenow")).toBe("50");
        expect(screen.getByText("Etapa actual: En atención")).toBeTruthy();
    });

    it("observado: indica el motivo faltante y la acción de corrección", () => {
        render(
            <LineaTiempoTramite
                seguimiento={calcularSeguimiento({ ...base, estado: "OBSERVADO" })}
                accionObservacion={<a href="/expedientes/exp-1">Corregir y volver a presentar</a>}
            />,
        );
        expect(screen.getByText("Trámite observado")).toBeTruthy();
        expect(screen.getByText(/No se registró un motivo/)).toBeTruthy();
        expect(screen.getByText("Corregir y volver a presentar")).toBeTruthy();
    });

    it("etapas completadas sin fecha lo dicen en vez de inventarla", () => {
        render(<LineaTiempoTramite seguimiento={calcularSeguimiento({ ...base, estado: "EN_PROCESO" })} areaActual="Dirección" />);
        expect(screen.getByText("Fecha no registrada en el sistema")).toBeTruthy();
        expect(screen.getByText("Área a cargo: Dirección")).toBeTruthy();
    });
});
