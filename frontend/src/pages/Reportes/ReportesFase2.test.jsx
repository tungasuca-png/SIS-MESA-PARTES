import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { AuthContext } from "../../context/auth-context";
import Reportes from "./Reportes";
import { cargaPendientePorArea, etiquetaSerie, tendencia, textoDiferencia, textoVariacion, validarComparacion } from "./reportesData";
import * as reportes from "../../services/reportesService";
import * as descargas from "../../utils/descargas";

// Reportes, Fase 2. HTTP simulado; datos agregados ficticios (sin personas).
vi.mock("../../services/reportesService", () => ({
    getReporteExpedientes: vi.fn(),
    exportarReporteExpedientes: vi.fn(),
    getReporteDerivaciones: vi.fn(),
    exportarReporteDerivaciones: vi.fn(),
    getReporteDocumentos: vi.fn(),
    exportarReporteDocumentos: vi.fn(),
    getReporteActividad: vi.fn(),
    exportarReporteActividad: vi.fn(),
    getReporteComparacion: vi.fn(),
    exportarReporteComparacion: vi.fn(),
}));
vi.mock("../../utils/descargas", () => ({ descargarBase64: vi.fn() }));
vi.mock("../../components/dashboard/DashboardLayout", () => ({ default: ({ children }) => <div>{children}</div> }));

const c = (clave, cantidad) => ({ clave, cantidad });
const EXPEDIENTES = {
    total: 6, pendientes: 3, por_estado: [c("PENDIENTE", 2), c("EN_PROCESO", 1), c("OBSERVADO", 0), c("ATENDIDO", 3)],
    por_tipo: [c("CONSTANCIA", 4), c("PERMISO", 2)], por_prioridad: [c("NORMAL", 6), c("URGENTE", 0)], por_area: [c("SECRETARIA", 2), c("DIRECTOR", 4)],
    por_mes: [c("2026-10", 6)], pendientes_por_antiguedad: [c("0-3", 3)], alcance: "INSTITUCION", area_alcance: "", generado: "2026-10-30T09:15:00-05:00",
    granularidad: "SEMANA", serie: [c("2026-10-19", 2), c("2026-10-26", 4)],
    area_estado: [{ fila: "SECRETARIA", columna: "PENDIENTE", cantidad: 2 }, { fila: "DIRECTOR", columna: "EN_PROCESO", cantidad: 1 }, { fila: "DIRECTOR", columna: "ATENDIDO", cantidad: 3 }],
    tipo_estado: [{ fila: "CONSTANCIA", columna: "ATENDIDO", cantidad: 3 }, { fila: "CONSTANCIA", columna: "PENDIENTE", cantidad: 1 }, { fila: "PERMISO", columna: "PENDIENTE", cantidad: 1 }, { fila: "PERMISO", columna: "EN_PROCESO", cantidad: 1 }],
};
const DERIVACIONES = {
    total: 5, sin_clasificar: 1, granularidad: "MES", alcance: "AREA", area_alcance: "DIRECTOR", generado: "2026-10-30T09:15:00-05:00",
    por_destino: [c("SECRETARIA", 1), c("DIRECTOR", 3), c("SUBDIRECTOR", 0), c("SIN_CLASIFICAR", 1)],
    por_origen: [c("SECRETARIA", 4), c("SIN_CLASIFICAR", 1)],
    flujo: [{ origen: "SECRETARIA", destino: "DIRECTOR", cantidad: 3 }, { origen: "SIN_CLASIFICAR", destino: "SIN_CLASIFICAR", cantidad: 1 }, { origen: "DIRECTOR", destino: "SECRETARIA", cantidad: 1 }],
    por_tipo_tramite: [c("CONSTANCIA", 5)], serie: [c("2026-10", 5)], otros_movimientos: [c("NOTIFICACION", 2), c("APROBACION", 1)],
};
const DOCUMENTOS = { total: 3, granularidad: "DIA", alcance: "INSTITUCION", generado: "2026-10-30T09:15:00-05:00",
    por_tipo_documento: [c("ADJUNTO", 2), c("PROVEIDO", 1), c("ACTA", 0)], por_tipo_tramite: [c("PERMISO", 3)], serie: [c("2026-10-30", 3)] };
const ACTIVIDAD = { alcance: "INSTITUCION", generado: "2026-10-30T09:15:00-05:00", roles: [
    { rol: "DIRECTOR", expedientes_registrados: 0, movimientos_registrados: 4, derivaciones_registradas: 3, documentos_subidos: 1, total: 5 },
    { rol: "SECRETARIA", expedientes_registrados: 5, movimientos_registrados: 2, derivaciones_registradas: 2, documentos_subidos: 0, total: 7 },
    { rol: "SOLICITANTE", expedientes_registrados: 3, movimientos_registrados: 0, derivaciones_registradas: 0, documentos_subidos: 0, total: 3 },
] };
const COMPARABLE = { desde_a: "2026-10-01", hasta_a: "2026-10-30", desde_b: "2026-09-01", hasta_b: "2026-09-30", total_a: 12, total_b: 10, comparable: true, minimo: 10,
    mensaje: "", alcance: "INSTITUCION", generado: "2026-10-30T09:15:00-05:00", filas: [
        { grupo: "RESUMEN", clave: "TOTAL", actual: 12, anterior: 10, tiene_variacion: true, variacion: 20 },
        { grupo: "RESUMEN", clave: "PENDIENTES", actual: 4, anterior: 0, tiene_variacion: false, variacion: 0 },
        { grupo: "ESTADO", clave: "ATENDIDO", actual: 8, anterior: 10, tiene_variacion: true, variacion: -20 },
    ] };
const INSUFICIENTE = { ...COMPARABLE, total_a: 5, total_b: 3, comparable: false, mensaje: "Datos insuficientes para comparar", filas: [
    { grupo: "RESUMEN", clave: "TOTAL", actual: 5, anterior: 3, tiene_variacion: false, variacion: 0 },
] };

function renderReportes(ruta = "/reportes") {
    return render(
        <AuthContext.Provider value={{ user: { id: "u-1", role: "SECRETARIA", username: "demo" }, permissions: ["reportes.view"], loading: false, isAuthenticated: true }}>
            <MemoryRouter initialEntries={[ruta]}>
                <Reportes />
            </MemoryRouter>
        </AuthContext.Provider>
    );
}

const formulario = () => screen.getByRole("form", { name: "Filtros del reporte" });
const aplicar = () => fireEvent.click(within(formulario()).getByRole("button", { name: "Aplicar filtros" }));
const pestana = (nombre) => fireEvent.click(screen.getByRole("tab", { name: nombre }));

beforeEach(() => {
    vi.clearAllMocks();
    reportes.getReporteExpedientes.mockResolvedValue(EXPEDIENTES);
    reportes.getReporteDerivaciones.mockResolvedValue(DERIVACIONES);
    reportes.getReporteDocumentos.mockResolvedValue(DOCUMENTOS);
    reportes.getReporteActividad.mockResolvedValue(ACTIVIDAD);
    reportes.getReporteComparacion.mockResolvedValue(COMPARABLE);
    const archivo = { nombre: "r.csv", mime: "text/csv", contenido_base64: "YQ==" };
    for (const f of ["exportarReporteExpedientes", "exportarReporteDerivaciones", "exportarReporteDocumentos", "exportarReporteActividad", "exportarReporteComparacion"]) {
        reportes[f].mockResolvedValue(archivo);
    }
});
afterEach(cleanup);

describe("Utilidades Fase 2", () => {
    it("etiquetas de serie, variación y validación de comparación", () => {
        expect(etiquetaSerie("2026-10-26", "SEMANA")).toEqual({ corta: "sem 26/10", larga: "Semana del 26/10/2026" });
        expect(etiquetaSerie("2026-10-30", "DIA").larga).toBe("30/10/2026");
        expect(etiquetaSerie("2026-10", "MES").larga).toBe("Octubre 2026");
        expect(textoVariacion(20)).toBe("+20,0 %");
        expect(textoVariacion(-5)).toBe("−5,0 %");
        expect(validarComparacion({ desde: "", hasta: "" })).toMatch(/indica la fecha desde/);
        expect(validarComparacion({ desde: "2026-10-01", hasta: "2026-10-30", modo: "PERSONALIZADO" })).toMatch(/periodo de referencia/);
        expect(validarComparacion({ desde: "2026-10-01", hasta: "2026-10-30", modo: "ANTERIOR" })).toBe("");
    });
});

describe("Pestañas", () => {
    it("el Resumen es la vista por defecto y las pestañas se recuerdan en la URL", async () => {
        renderReportes();
        expect(screen.getByRole("tab", { name: "Resumen" }).getAttribute("aria-selected")).toBe("true");
        await screen.findByText("Total de expedientes");
        expect(reportes.getReporteExpedientes).toHaveBeenCalledWith({ desde: "", hasta: "", estado: "", tipo: "", prioridad: "", area: "" });
        cleanup();
        renderReportes("/reportes?vista=derivaciones");
        expect(screen.getByRole("tab", { name: "Derivaciones" }).getAttribute("aria-selected")).toBe("true");
        expect(await screen.findByText("Derivaciones entre áreas")).toBeTruthy();
    });

    it("cada pestaña desactiva los filtros que no le aplican", async () => {
        renderReportes();
        await screen.findByText("Total de expedientes");
        pestana("Derivaciones");
        const f = within(formulario());
        expect(f.getByLabelText("Estado").disabled).toBe(true);
        expect(f.getByLabelText("Prioridad").disabled).toBe(true);
        expect(f.getByLabelText("Área actual").disabled).toBe(true);
        expect(f.getByLabelText("Tipo de trámite").disabled).toBe(false);
        expect(f.getByLabelText("Agrupar por")).toBeTruthy();
        expect(screen.getByText(/solo aplican el periodo, la agrupación y el tipo de trámite/)).toBeTruthy();
        pestana("Actividad por rol");
        expect(within(formulario()).queryByLabelText("Agrupar por")).toBeNull();
    });
});

describe("Expedientes y áreas", () => {
    it("pide la granularidad elegida y muestra las tablas cruzadas con totales", async () => {
        renderReportes("/reportes?vista=expedientes");
        await screen.findByLabelText("Expedientes por área actual y estado");
        fireEvent.change(within(formulario()).getByLabelText("Agrupar por"), { target: { value: "SEMANA" } });
        aplicar();
        await waitFor(() => expect(reportes.getReporteExpedientes).toHaveBeenLastCalledWith(expect.objectContaining({ granularidad: "SEMANA" })));
        expect(await screen.findByRole("heading", { name: "Expedientes registrados por semana" })).toBeTruthy();

        const area = screen.getByLabelText("Expedientes por área actual y estado");
        const direccion = within(area).getByText("Dirección").closest("tr");
        // Columna final: participación en el total (4 de 6).
        expect([...direccion.querySelectorAll("td")].map((td) => td.textContent)).toEqual(["Dirección", "0", "1", "0", "3", "4", "66,7 %"]);
        const total = within(area).getByText("Total", { selector: "th[scope='row']" }).closest("tr");
        expect([...total.querySelectorAll("td")].map((td) => td.textContent)).toEqual(["2", "1", "0", "3", "6", "100,0 %"]);
        const tipos = screen.getByLabelText("Expedientes por tipo de trámite y estado");
        expect(within(tipos).getByText("Permiso")).toBeTruthy();
        expect(within(tipos).queryByText("Oficio")).toBeNull(); // solo filas con datos
    });
});

describe("Derivaciones", () => {
    it("muestra áreas normalizadas, «Sin clasificar», otros movimientos y el aviso de rechazos", async () => {
        renderReportes("/reportes?vista=derivaciones");
        const flujo = await screen.findByLabelText("Flujo de derivaciones entre áreas");
        expect(within(flujo).getByText("Secretaría → Dirección")).toBeTruthy();
        expect(within(flujo).getByText("Sin clasificar → Sin clasificar")).toBeTruthy();
        expect(within(screen.getByLabelText("Derivaciones por área de destino")).queryByText("Subdirección")).toBeNull();
        expect(within(screen.getByLabelText("Otros movimientos por tipo")).getByText("Notificación")).toBeTruthy();
        expect(screen.getByText(/no se registran rechazos diferenciados/)).toBeTruthy();
        expect(screen.getAllByText(/actualmente en el área Dirección/).length).toBeGreaterThan(0);
        expect(screen.queryByText(/rechazad/i)).toBeNull(); // no hay métrica de rechazos
    });

    it("envía solo periodo, tipo y agrupación (no estado, prioridad ni área)", async () => {
        renderReportes();
        await screen.findByText("Total de expedientes");
        fireEvent.change(within(formulario()).getByLabelText("Estado"), { target: { value: "ATENDIDO" } });
        fireEvent.change(within(formulario()).getByLabelText("Tipo de trámite"), { target: { value: "PERMISO" } });
        aplicar();
        pestana("Derivaciones");
        await waitFor(() => expect(reportes.getReporteDerivaciones).toHaveBeenLastCalledWith({ desde: "", hasta: "", tipo: "PERMISO", granularidad: "MES" }));
    });
});

describe("Documentos y actividad", () => {
    it("documentos por tipo documental (sin ceros) y serie", async () => {
        renderReportes("/reportes?vista=documentos");
        const tipos = await screen.findByLabelText("Documentos por tipo documental");
        expect(within(tipos).getByText("Proveído")).toBeTruthy();
        expect(within(tipos).queryByText("Acta")).toBeNull();
        expect(screen.getByRole("heading", { name: "Documentos registrados por día" })).toBeTruthy();
    });

    it("actividad solo por rol, con totales y sin personas", async () => {
        const { container } = renderReportes("/reportes?vista=actividad");
        const tabla = await screen.findByLabelText("Actividad por rol");
        expect(within(tabla).getByText("Solicitante (externo)")).toBeTruthy();
        const total = within(tabla).getByText("Total", { selector: "th[scope='row']" }).closest("tr");
        // Expedientes, movimientos, de ellos derivaciones, documentos, total de acciones y %.
        expect([...total.querySelectorAll("td")].map((td) => td.textContent)).toEqual(["8", "6", "5", "1", "15", "100,0 %"]);
        expect(within(tabla).getByText("Movimientos registrados", { selector: "th" })).toBeTruthy();
        expect(within(tabla).queryByText("Derivaciones registradas")).toBeNull();
        const direccion = within(tabla).getByText("Dirección").closest("tr");
        expect([...direccion.querySelectorAll("td")].map((td) => td.textContent)).toEqual(["Dirección", "0", "4", "3", "1", "5", "33,3 %"]);
        expect(screen.getByText(/No se muestran personas/)).toBeTruthy();
        expect(container.textContent).not.toMatch(/u-1|demo\b.*registr/);
    });
});

describe("Comparación", () => {
    it("sin periodo completo no consulta y lo explica; la exportación queda desactivada", async () => {
        renderReportes("/reportes?vista=comparacion");
        expect(await screen.findByText(/Para comparar, indica la fecha desde y la fecha hasta/)).toBeTruthy();
        expect(reportes.getReporteComparacion).not.toHaveBeenCalled();
        expect(screen.getByRole("button", { name: "Exportar Excel" }).disabled).toBe(true);
    });

    it("muestra variaciones solo si es comparable", async () => {
        renderReportes("/reportes?vista=comparacion");
        fireEvent.change(within(formulario()).getByLabelText("Desde"), { target: { value: "2026-10-01" } });
        fireEvent.change(within(formulario()).getByLabelText("Hasta"), { target: { value: "2026-10-30" } });
        aplicar();
        const resumen = await screen.findByLabelText("Comparación Resumen");
        expect(within(resumen).getByText("+20,0 %")).toBeTruthy();
        expect(within(resumen).getByText("Sin base")).toBeTruthy();
        expect(within(screen.getByLabelText("Comparación Por estado")).getByText("−20,0 %")).toBeTruthy();
        expect(screen.queryByText(/Datos insuficientes/)).toBeNull();
        expect(reportes.getReporteComparacion).toHaveBeenLastCalledWith(expect.objectContaining({ desde: "2026-10-01", hasta: "2026-10-30", modo: "ANTERIOR" }));
    });

    it("con menos de 10 registros: «Datos insuficientes para comparar» y ninguna variación", async () => {
        reportes.getReporteComparacion.mockResolvedValue(INSUFICIENTE);
        renderReportes("/reportes?vista=comparacion");
        fireEvent.change(within(formulario()).getByLabelText("Desde"), { target: { value: "2026-10-01" } });
        fireEvent.change(within(formulario()).getByLabelText("Hasta"), { target: { value: "2026-10-30" } });
        aplicar();
        expect(await screen.findByText("Datos insuficientes para comparar.")).toBeTruthy();
        expect(screen.getByText(/actual: 5; referencia: 3/)).toBeTruthy();
        expect(screen.queryByText(/%/, { selector: "td" })).toBeNull();
    });
});

describe("Exportación independiente por pestaña", () => {
    it("cada pestaña exporta con su endpoint y sus filtros aplicados", async () => {
        renderReportes("/reportes?vista=expedientes");
        await screen.findByLabelText("Expedientes por área actual y estado");
        fireEvent.change(within(formulario()).getByLabelText("Tipo de trámite"), { target: { value: "CONSTANCIA" } });
        fireEvent.change(within(formulario()).getByLabelText("Agrupar por"), { target: { value: "DIA" } });
        aplicar();
        await waitFor(() => expect(reportes.getReporteExpedientes).toHaveBeenLastCalledWith(expect.objectContaining({ tipo: "CONSTANCIA", granularidad: "DIA" })));

        fireEvent.click(screen.getByRole("button", { name: "Exportar CSV" }));
        await waitFor(() => expect(reportes.exportarReporteExpedientes).toHaveBeenLastCalledWith(expect.objectContaining({ vista: "EXPEDIENTES", tipo: "CONSTANCIA", granularidad: "DIA" }), "csv"));

        pestana("Derivaciones");
        fireEvent.click(screen.getByRole("button", { name: "Exportar Excel" }));
        await waitFor(() => expect(reportes.exportarReporteDerivaciones).toHaveBeenLastCalledWith({ desde: "", hasta: "", tipo: "CONSTANCIA", granularidad: "DIA" }, "xlsx"));

        pestana("Documentos");
        fireEvent.click(screen.getByRole("button", { name: "Exportar CSV" }));
        await waitFor(() => expect(reportes.exportarReporteDocumentos).toHaveBeenCalledWith({ desde: "", hasta: "", tipo: "CONSTANCIA", granularidad: "DIA" }, "csv"));

        pestana("Actividad por rol");
        fireEvent.click(screen.getByRole("button", { name: "Exportar CSV" }));
        await waitFor(() => expect(reportes.exportarReporteActividad).toHaveBeenCalledWith({ desde: "", hasta: "", tipo: "CONSTANCIA" }, "csv"));

        pestana("Resumen");
        fireEvent.click(screen.getByRole("button", { name: "Exportar CSV" }));
        await waitFor(() => expect(reportes.exportarReporteExpedientes).toHaveBeenLastCalledWith({ desde: "", hasta: "", estado: "", tipo: "CONSTANCIA", prioridad: "", area: "" }, "csv"));
        expect(descargas.descargarBase64).toHaveBeenCalledTimes(5);
    });
});

describe("Mejora incremental", () => {
    it("indicadores derivados: carga pendiente, diferencia y tendencia", () => {
        const carga = cargaPendientePorArea([
            { fila: "SECRETARIA", columna: "PENDIENTE", cantidad: 1 },
            { fila: "DIRECTOR", columna: "PENDIENTE", cantidad: 2 },
            { fila: "DIRECTOR", columna: "EN_PROCESO", cantidad: 3 },
            { fila: "DOCENTE", columna: "ATENDIDO", cantidad: 4 },
        ]);
        expect(carga).toEqual([
            { area: "DIRECTOR", pendiente: 2, enProceso: 3, total: 5 },
            { area: "SECRETARIA", pendiente: 1, enProceso: 0, total: 1 },
        ]);
        expect(textoDiferencia(2)).toBe("+2");
        expect(textoDiferencia(-3)).toBe("−3");
        expect(textoDiferencia(0)).toBe("0");
        expect(tendencia(5, 3).texto).toBe("Aumento");
        expect(tendencia(3, 5).texto).toBe("Disminución");
        expect(tendencia(4, 4).texto).toBe("Sin cambio");
    });

    it("evolución por estado (barras apiladas con leyenda y tabla) y áreas con mayor carga pendiente", async () => {
        reportes.getReporteExpedientes.mockResolvedValue({
            ...EXPEDIENTES,
            serie_estado: [
                { fila: "2026-10-19", columna: "PENDIENTE", cantidad: 1 },
                { fila: "2026-10-19", columna: "ATENDIDO", cantidad: 1 },
                { fila: "2026-10-26", columna: "PENDIENTE", cantidad: 1 },
                { fila: "2026-10-26", columna: "EN_PROCESO", cantidad: 1 },
                { fila: "2026-10-26", columna: "ATENDIDO", cantidad: 2 },
            ],
        });
        renderReportes("/reportes?vista=expedientes");
        const grafico = await screen.findByRole("img", { name: "Expedientes registrados por semana, según su estado actual" });
        expect(grafico).toBeTruthy();
        const leyenda = screen.getByRole("list", { name: "Estados" });
        expect(within(leyenda).getAllByRole("listitem").map((li) => li.textContent)).toEqual(["Pendiente", "En proceso", "Observado", "Atendido"]);
        // Tabla accesible del gráfico: el total por periodo coincide con la serie.
        const tablaGrafico = screen.getByText("Expedientes registrados por semana, según su estado actual", { selector: "caption" }).closest("table");
        const fila = within(tablaGrafico).getByText("Semana del 26/10/2026").closest("tr");
        expect([...fila.querySelectorAll("td")].map((td) => td.textContent)).toEqual(["1", "1", "0", "2", "4"]);
        expect(screen.getByText(/no reflejan su situación en fechas pasadas/)).toBeTruthy();

        const carga = screen.getByLabelText("Carga pendiente por área actual");
        const filas = [...carga.querySelectorAll("tbody tr")].map((tr) => [...tr.querySelectorAll("td")].map((td) => td.textContent));
        expect(filas).toEqual([
            ["Secretaría", "2", "0", "2", "66,7 %"],
            ["Dirección", "0", "1", "1", "33,3 %"],
        ]);
        // Tipo × estado de más a menos frecuente.
        const tipos = screen.getByLabelText("Expedientes por tipo de trámite y estado");
        expect([...tipos.querySelectorAll("tbody tr")].map((tr) => tr.querySelector("td").textContent)).toEqual(["Constancia de Estudios", "Permiso"]);
    });

    it("sin evolución por estado (servicio anterior) usa la serie simple", async () => {
        renderReportes("/reportes?vista=expedientes");
        expect(await screen.findByRole("img", { name: "Expedientes registrados por semana" })).toBeTruthy();
        expect(screen.queryByRole("list", { name: "Estados" })).toBeNull();
    });

    it("derivaciones: movimientos, derivaciones y aprobaciones diferenciados", async () => {
        renderReportes("/reportes?vista=derivaciones");
        await screen.findByText("Derivaciones entre áreas");
        const valor = (etiqueta) => screen.getByText(etiqueta, { selector: ".dp-stat-label" }).closest(".dp-stat-card").querySelector(".dp-stat-value").textContent;
        expect(valor("Movimientos registrados")).toBe("8"); // 5 derivaciones + 3 otros
        expect(valor("Derivaciones entre áreas")).toBe("5");
        expect(valor("Aprobaciones")).toBe("1");
        expect(valor("Sin clasificar")).toBe("1");
    });

    it("comparación: diferencia y tendencia solo si es comparable", async () => {
        renderReportes("/reportes?vista=comparacion");
        fireEvent.change(within(formulario()).getByLabelText("Desde"), { target: { value: "2026-10-01" } });
        fireEvent.change(within(formulario()).getByLabelText("Hasta"), { target: { value: "2026-10-30" } });
        aplicar();
        const resumen = await screen.findByLabelText("Comparación Resumen");
        const total = within(resumen).getByText("Total de expedientes").closest("tr");
        expect([...total.querySelectorAll("td")].map((td) => td.textContent)).toEqual(["Total de expedientes", "12", "10", "▲+2 (Aumento)", "+20,0 %"]);
        const atendido = within(screen.getByLabelText("Comparación Por estado")).getByText("Atendido").closest("tr");
        expect(atendido.textContent).toContain("Disminución");
        expect(screen.getByText(/\+20,0 % respecto de la referencia/)).toBeTruthy();
    });

    it("comparación insuficiente: sin diferencias ni tendencias", async () => {
        reportes.getReporteComparacion.mockResolvedValue(INSUFICIENTE);
        renderReportes("/reportes?vista=comparacion");
        fireEvent.change(within(formulario()).getByLabelText("Desde"), { target: { value: "2026-10-01" } });
        fireEvent.change(within(formulario()).getByLabelText("Hasta"), { target: { value: "2026-10-30" } });
        aplicar();
        const resumen = await screen.findByLabelText("Comparación Resumen");
        const total = within(resumen).getByText("Total de expedientes").closest("tr");
        expect([...total.querySelectorAll("td")].map((td) => td.textContent)).toEqual(["Total de expedientes", "5", "3", "—", "—"]);
        expect(screen.queryByText(/Aumento|Disminución/)).toBeNull();
        expect(screen.getByText("No comparable: datos insuficientes")).toBeTruthy();
    });

    it("sin resultados muestra un estado vacío distinto del error", async () => {
        reportes.getReporteDocumentos.mockResolvedValue({ ...DOCUMENTOS, total: 0, por_tipo_documento: [], por_tipo_tramite: [], serie: [] });
        renderReportes("/reportes?vista=documentos");
        const vacio = await screen.findByRole("status");
        expect(vacio.textContent).toMatch(/No hay documentos que coincidan/);
        expect(screen.queryByRole("alert")).toBeNull();
    });
});

describe("Exportación PDF", () => {
    it("las seis pestañas exportan PDF con su endpoint y sus filtros", async () => {
        const pdf = { nombre: "reporte-x.pdf", mime: "application/pdf", contenido_base64: "JVBERi0=" };
        for (const f of ["exportarReporteExpedientes", "exportarReporteDerivaciones", "exportarReporteDocumentos", "exportarReporteActividad", "exportarReporteComparacion"]) {
            reportes[f].mockResolvedValue(pdf);
        }
        renderReportes("/reportes?vista=resumen");
        await screen.findByText("Total de expedientes");
        fireEvent.change(within(formulario()).getByLabelText("Desde"), { target: { value: "2026-10-01" } });
        fireEvent.change(within(formulario()).getByLabelText("Hasta"), { target: { value: "2026-10-30" } });
        fireEvent.change(within(formulario()).getByLabelText("Tipo de trámite"), { target: { value: "PERMISO" } });
        aplicar();
        const periodo = { desde: "2026-10-01", hasta: "2026-10-30" };

        const exportarPDF = async () => {
            const boton = await screen.findByRole("button", { name: "Exportar PDF" });
            await waitFor(() => expect(boton.disabled).toBe(false));
            fireEvent.click(boton);
        };
        await exportarPDF();
        await waitFor(() => expect(reportes.exportarReporteExpedientes).toHaveBeenLastCalledWith({ ...periodo, estado: "", tipo: "PERMISO", prioridad: "", area: "" }, "pdf"));
        pestana("Expedientes y áreas");
        await exportarPDF();
        await waitFor(() => expect(reportes.exportarReporteExpedientes).toHaveBeenLastCalledWith(expect.objectContaining({ ...periodo, tipo: "PERMISO", vista: "EXPEDIENTES" }), "pdf"));
        pestana("Derivaciones");
        await exportarPDF();
        await waitFor(() => expect(reportes.exportarReporteDerivaciones).toHaveBeenLastCalledWith(expect.objectContaining({ ...periodo, tipo: "PERMISO" }), "pdf"));
        pestana("Documentos");
        await exportarPDF();
        await waitFor(() => expect(reportes.exportarReporteDocumentos).toHaveBeenLastCalledWith(expect.objectContaining({ ...periodo, tipo: "PERMISO" }), "pdf"));
        pestana("Actividad por rol");
        await exportarPDF();
        await waitFor(() => expect(reportes.exportarReporteActividad).toHaveBeenLastCalledWith({ ...periodo, tipo: "PERMISO" }, "pdf"));
        pestana("Comparación");
        await exportarPDF();
        await waitFor(() => expect(reportes.exportarReporteComparacion).toHaveBeenLastCalledWith(expect.objectContaining({ ...periodo, tipo: "PERMISO", modo: "ANTERIOR" }), "pdf"));

        await waitFor(() => expect(descargas.descargarBase64).toHaveBeenCalledTimes(6));
        expect(descargas.descargarBase64).toHaveBeenLastCalledWith("JVBERi0=", "reporte-x.pdf", "application/pdf");
        // Excel y CSV siguen disponibles junto al PDF.
        expect(screen.getByRole("button", { name: "Exportar Excel" })).toBeTruthy();
        expect(screen.getByRole("button", { name: "Exportar CSV" })).toBeTruthy();
    });

    it("muestra el error si el PDF no se puede generar", async () => {
        reportes.exportarReporteDocumentos.mockRejectedValue({ response: { status: 500, data: { message: "no se pudo generar el archivo del reporte" } } });
        renderReportes("/reportes?vista=documentos");
        await screen.findByText("Tipos documentales más frecuentes");
        fireEvent.click(screen.getByRole("button", { name: "Exportar PDF" }));
        expect(await screen.findByText(/No se pudo exportar el reporte/)).toBeTruthy();
        expect(descargas.descargarBase64).not.toHaveBeenCalled();
    });
});
