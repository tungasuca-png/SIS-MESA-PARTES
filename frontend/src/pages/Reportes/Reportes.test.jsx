import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { AuthContext } from "../../context/auth-context";
import PermissionRoute from "../../components/PermissionRoute";
import Reportes from "./Reportes";
import { NAV_GROUPS } from "../../components/dashboard/navItems";
import { atajosPeriodo, etiquetaMes, mensajeError, porcentaje, textoFiltros, textoPeriodo, validarFiltros } from "./reportesData";
import * as reportes from "../../services/reportesService";
import * as descargas from "../../utils/descargas";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import process from "node:process";

// Vitest no entrega el CSS como texto (?raw): se lee del disco (cwd = frontend/).
const estilos = readFileSync(resolve(process.cwd(), "src/pages/Reportes/reportes.css"), "utf8");

// Reportes (Fase 1). HTTP simulado; datos agregados ficticios (sin personas).
vi.mock("../../services/reportesService", () => ({ getReporteExpedientes: vi.fn(), exportarReporteExpedientes: vi.fn() }));
vi.mock("../../utils/descargas", () => ({ descargarBase64: vi.fn() }));
vi.mock("../../components/dashboard/DashboardLayout", () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock("../../components/portal/RoleLayout", () => ({ default: ({ children }) => <div>{children}</div> }));

const c = (clave, cantidad) => ({ clave, cantidad });
const REPORTE = {
    total: 10,
    por_estado: [c("PENDIENTE", 3), c("EN_PROCESO", 1), c("OBSERVADO", 2), c("ATENDIDO", 4)],
    por_tipo: [c("CERTIFICADO", 0), c("CONSTANCIA", 6), c("PERMISO", 4)],
    por_prioridad: [c("NORMAL", 8), c("URGENTE", 2)],
    por_area: [c("SECRETARIA", 7), c("DIRECTOR", 3), c("DOCENTE", 0)],
    por_mes: [c("2026-09", 4), c("2026-10", 6)],
    pendientes: 4,
    pendientes_por_antiguedad: [c("0-3", 1), c("4-7", 0), c("8-15", 2), c("16-30", 0), c("31+", 1)],
    alcance: "INSTITUCION",
    area_alcance: "",
    generado: "2026-10-30T09:15:00-05:00",
};
const VACIO = {
    ...REPORTE, total: 0, pendientes: 0, por_mes: [],
    por_estado: REPORTE.por_estado.map((x) => ({ ...x, cantidad: 0 })),
    pendientes_por_antiguedad: REPORTE.pendientes_por_antiguedad.map((x) => ({ ...x, cantidad: 0 })),
};

function renderReportes(permissions = ["reportes.view"]) {
    return render(
        <AuthContext.Provider value={{ user: { id: "u-1", role: "SECRETARIA", username: "demo" }, permissions, loading: false, isAuthenticated: true }}>
            <MemoryRouter>
                <PermissionRoute permission="reportes.view">
                    <Reportes />
                </PermissionRoute>
            </MemoryRouter>
        </AuthContext.Provider>
    );
}

const VACIOS = { desde: "", hasta: "", estado: "", tipo: "", prioridad: "", area: "" };

beforeEach(() => {
    vi.clearAllMocks();
    reportes.getReporteExpedientes.mockResolvedValue(REPORTE);
});
afterEach(cleanup);

describe("reportesData", () => {
    it("valida el periodo y arma textos de periodo y filtros", () => {
        const hoy = new Date(2026, 9, 30);
        expect(validarFiltros({ desde: "2026-10-10", hasta: "2026-10-01" }, hoy)).toMatch(/posterior a la fecha hasta/);
        expect(validarFiltros({ desde: "2026-10-31" }, hoy)).toMatch(/posterior a hoy/);
        expect(validarFiltros({ desde: "2026-10-01", hasta: "2026-10-30" }, hoy)).toBe("");
        expect(textoPeriodo({ desde: "2026-01-01", hasta: "2026-01-31" })).toBe("Del 01/01/2026 al 31/01/2026");
        expect(textoPeriodo({})).toBe("Todo el historial");
        expect(textoFiltros({ estado: "EN_PROCESO", area: "DIRECTOR" })).toBe("Estado: En proceso · Área: Dirección");
        expect(textoFiltros(VACIOS)).toBe("Sin filtros adicionales");
    });

    it("atajos de periodo, meses y porcentajes", () => {
        const [mes, anterior, , anio] = atajosPeriodo(new Date(2026, 2, 15));
        expect(mes).toMatchObject({ desde: "2026-03-01", hasta: "2026-03-15" });
        expect(anterior).toMatchObject({ desde: "2026-02-01", hasta: "2026-02-28" });
        expect(anio).toMatchObject({ desde: "2026-01-01" });
        expect(etiquetaMes("2026-03")).toEqual({ corta: "mar 26", larga: "Marzo 2026" });
        expect(porcentaje(1, 4)).toBe("25,0 %");
        expect(porcentaje(1, 0)).toBe("—");
    });

    it("mensajes de error: 429 propio y validaciones sin prefijo técnico", () => {
        expect(mensajeError({ response: { status: 429 } })).toMatch(/Demasiadas solicitudes/);
        expect(mensajeError({ response: { status: 400, data: "rpc error: code = InvalidArgument desc = validation_error: el periodo no puede superar 10 años" } }))
            .toBe("El periodo no puede superar 10 años.");
    });
});

describe("Pantalla Reportes", () => {
    it("muestra indicadores, gráficos, tendencia y antigüedad con porcentajes", async () => {
        const { container } = renderReportes();
        const total = (await screen.findByText("Total de expedientes")).closest(".dp-stat-card");
        expect(within(total).getByText("10")).toBeTruthy();
        const pendientes = screen.getByText("Pendientes", { selector: ".dp-stat-label" }).closest(".dp-stat-card");
        expect(within(pendientes).getByText("4")).toBeTruthy();
        expect(within(pendientes).getByText(/40,0 %/)).toBeTruthy();

        expect(within(screen.getByLabelText("Expedientes por estado")).getByText("Atendido")).toBeTruthy();
        // Tipos y áreas sin ceros.
        const tipos = within(screen.getByLabelText("Expedientes por tipo de trámite"));
        expect(tipos.getByText("Constancia de Estudios")).toBeTruthy();
        expect(tipos.queryByText("Certificado de Estudios")).toBeNull();
        expect(within(screen.getByLabelText("Expedientes por área actual")).queryByText("Docente")).toBeNull();
        expect(screen.getByLabelText("Expedientes por prioridad")).toBeTruthy();
        expect(screen.getByLabelText("Expedientes registrados por mes")).toBeTruthy();

        const tabla = container.querySelector(".rp-tabla");
        const filaMas30 = within(tabla).getByText("Más de 30 días").closest("tr");
        expect(within(filaMas30).getByText("1")).toBeTruthy();
        expect(within(filaMas30).getByText("25,0 %")).toBeTruthy(); // % sobre pendientes
        expect(screen.getByText(/no representan plazos institucionales/)).toBeTruthy();
        expect(screen.getByText(/Datos al/)).toBeTruthy();
        expect(screen.getAllByText(/Expedientes de toda la institución/).length).toBeGreaterThan(0);
        expect(reportes.getReporteExpedientes).toHaveBeenCalledWith(VACIOS);
    });

    it("aplica filtros combinados, valida fechas y limpia", async () => {
        renderReportes();
        await screen.findByText("Total de expedientes");
        const form = screen.getByRole("form", { name: "Filtros del reporte" });
        const campo = (nombre) => within(form).getByLabelText(nombre);

        fireEvent.change(campo("Desde"), { target: { value: "2026-10-10" } });
        fireEvent.change(campo("Hasta"), { target: { value: "2026-10-01" } });
        fireEvent.click(within(form).getByRole("button", { name: "Aplicar filtros" }));
        expect(await screen.findByText(/posterior a la fecha hasta/)).toBeTruthy();
        expect(reportes.getReporteExpedientes).toHaveBeenCalledTimes(1);

        fireEvent.change(campo("Desde"), { target: { value: "2026-01-01" } });
        fireEvent.change(campo("Hasta"), { target: { value: "2026-06-30" } });
        fireEvent.change(campo("Estado"), { target: { value: "EN_PROCESO" } });
        fireEvent.change(campo("Tipo de trámite"), { target: { value: "PERMISO" } });
        fireEvent.change(campo("Prioridad"), { target: { value: "URGENTE" } });
        fireEvent.change(campo("Área actual"), { target: { value: "DIRECTOR" } });
        fireEvent.click(within(form).getByRole("button", { name: "Aplicar filtros" }));
        const filtros = { desde: "2026-01-01", hasta: "2026-06-30", estado: "EN_PROCESO", tipo: "PERMISO", prioridad: "URGENTE", area: "DIRECTOR" };
        await waitFor(() => expect(reportes.getReporteExpedientes).toHaveBeenLastCalledWith(filtros));
        expect(screen.queryByText(/posterior a la fecha hasta/)).toBeNull();
        expect(screen.getByText("Del 01/01/2026 al 30/06/2026", { selector: "strong" })).toBeTruthy();

        fireEvent.click(within(form).getByRole("button", { name: "Limpiar filtros" }));
        await waitFor(() => expect(reportes.getReporteExpedientes).toHaveBeenLastCalledWith(VACIOS));
        expect(campo("Estado").value).toBe("");
    });

    it("un atajo de periodo completa las fechas (se aplica con el botón)", async () => {
        renderReportes();
        await screen.findByText("Total de expedientes");
        fireEvent.click(screen.getByRole("button", { name: "Este año" }));
        const form = screen.getByRole("form", { name: "Filtros del reporte" });
        expect(within(form).getByLabelText("Desde").value).toMatch(/^\d{4}-01-01$/);
        expect(reportes.getReporteExpedientes).toHaveBeenCalledTimes(1);
    });

    it("estado de carga y error con reintento", async () => {
        let rechazar;
        reportes.getReporteExpedientes.mockReturnValueOnce(new Promise((_, r) => (rechazar = r)));
        renderReportes();
        expect(screen.getByLabelText("Cargando reporte")).toBeTruthy();
        rechazar({ response: { status: 429 } });
        expect(await screen.findByText(/Demasiadas solicitudes/)).toBeTruthy();
        expect(screen.queryByText("Total de expedientes")).toBeNull();

        fireEvent.click(screen.getByRole("button", { name: "Reintentar" }));
        expect(await screen.findByText("Total de expedientes")).toBeTruthy();
    });

    it("sin resultados: mensaje claro, sin gráficos", async () => {
        reportes.getReporteExpedientes.mockResolvedValue(VACIO);
        renderReportes();
        expect(await screen.findByText("No hay expedientes que coincidan con los filtros aplicados.")).toBeTruthy();
        expect(screen.queryByLabelText("Expedientes por estado")).toBeNull();
        expect(screen.getAllByText("—").length).toBeGreaterThan(0);
    });

    it("exporta Excel y CSV con los filtros APLICADOS (no el borrador)", async () => {
        reportes.exportarReporteExpedientes.mockResolvedValue({ nombre: "reporte-expedientes-20261030-0915.xlsx", mime: "application/x", contenido_base64: "UEs=" });
        renderReportes();
        await screen.findByText("Total de expedientes");
        const form = screen.getByRole("form", { name: "Filtros del reporte" });
        fireEvent.change(within(form).getByLabelText("Tipo de trámite"), { target: { value: "PERMISO" } });
        fireEvent.click(within(form).getByRole("button", { name: "Aplicar filtros" }));
        await waitFor(() => expect(reportes.getReporteExpedientes).toHaveBeenCalledTimes(2));
        // Cambio sin aplicar: no debe afectar la exportación.
        fireEvent.change(within(form).getByLabelText("Estado"), { target: { value: "ATENDIDO" } });

        fireEvent.click(await screen.findByRole("button", { name: "Exportar Excel" }));
        await waitFor(() => expect(descargas.descargarBase64).toHaveBeenCalledWith("UEs=", "reporte-expedientes-20261030-0915.xlsx", "application/x"));
        expect(reportes.exportarReporteExpedientes).toHaveBeenLastCalledWith({ ...VACIOS, tipo: "PERMISO" }, "xlsx");

        reportes.exportarReporteExpedientes.mockResolvedValue({ nombre: "r.csv", mime: "text/csv", contenido_base64: "YQ==" });
        fireEvent.click(screen.getByRole("button", { name: "Exportar CSV" }));
        await waitFor(() => expect(descargas.descargarBase64).toHaveBeenCalledWith("YQ==", "r.csv", "text/csv"));
        expect(reportes.exportarReporteExpedientes).toHaveBeenLastCalledWith({ ...VACIOS, tipo: "PERMISO" }, "csv");
    });

    it("informa un error de exportación", async () => {
        reportes.exportarReporteExpedientes.mockRejectedValue({ response: { status: 403, data: "rpc error: code = PermissionDenied desc = sin permiso" } });
        renderReportes();
        await screen.findByText("Total de expedientes");
        fireEvent.click(screen.getByRole("button", { name: "Exportar CSV" }));
        expect(await screen.findByText(/No se pudo exportar el reporte/)).toBeTruthy();
        expect(descargas.descargarBase64).not.toHaveBeenCalled();
    });

    it("imprime con encabezado propio (periodo, filtros, alcance)", async () => {
        const imprimir = vi.spyOn(window, "print").mockImplementation(() => {});
        const { container } = renderReportes();
        await screen.findByText("Total de expedientes");
        fireEvent.click(screen.getByRole("button", { name: "Imprimir" }));
        expect(imprimir).toHaveBeenCalledTimes(1);
        const cabecera = container.querySelector(".rp-print-only");
        expect(cabecera.textContent).toContain("Periodo (fecha de registro): Todo el historial");
        expect(cabecera.textContent).toContain("Filtros: Sin filtros adicionales");
        expect(cabecera.textContent).toContain("Alcance:");
        imprimir.mockRestore();
    });

    it("hoja de impresión y diseño responsive definidos", () => {
        expect(estilos).toMatch(/@media print[\s\S]*\.dp-sidebar[\s\S]*display: none !important/);
        expect(estilos).toMatch(/@media print[\s\S]*\.rp-filtros/);
        expect(estilos).toMatch(/@media \(max-width: 640px\)/);
        expect(estilos).toMatch(/@media \(max-width: 900px\)/);
    });
});

describe("Control de acceso", () => {
    it("sin reportes.view no monta la pantalla ni consulta", () => {
        renderReportes(["dashboard.view", "expedientes.view"]);
        expect(screen.getByText("Acceso denegado")).toBeTruthy();
        expect(reportes.getReporteExpedientes).not.toHaveBeenCalled();
    });

    it("el menú habilita /reportes con reportes.view", () => {
        const item = NAV_GROUPS.find((g) => g.id === "reportes").items[0];
        expect(item).toMatchObject({ path: "/reportes", permission: "reportes.view" });
    });
});
