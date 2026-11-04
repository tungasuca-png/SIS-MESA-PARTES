import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { AuthContext } from "../../context/auth-context";
import Dashboard from "./dashboard";
import { agruparPorDia, cargarSerieDiaria } from "./dashboardData";
import * as expedientes from "../../services/expedientesService";
import * as reportes from "../../services/reportesService";

// Dashboard: cifras calculadas solo con el listado real de expedientes,
// visibles solo con expedientes.view. Datos ficticios; HTTP simulado.
vi.mock("../../services/expedientesService", () => ({ getExpedientes: vi.fn() }));
vi.mock("../../services/reportesService", () => ({ getReporteExpedientes: vi.fn() }));
vi.mock("../../services/usuariosService", () => ({ getUsuariosBasic: vi.fn().mockResolvedValue({ usuarios: [] }) }));
vi.mock("../../components/dashboard/DashboardLayout", () => ({ default: ({ children }) => <div>{children}</div> }));

const haceDias = (n) => {
    const d = new Date();
    d.setHours(10, 0, 0, 0);
    d.setDate(d.getDate() - n);
    return d.toISOString();
};

const DATOS = [
    { codigo: "EXP-2026-000005", tipo: "CONSTANCIA", estado: "PENDIENTE", prioridad: "URGENTE", fecha_registro: haceDias(0) },
    { codigo: "EXP-2026-000004", tipo: "CONSTANCIA", estado: "EN_PROCESO", prioridad: "NORMAL", fecha_registro: haceDias(1) },
    { codigo: "EXP-2026-000003", tipo: "CERTIFICADO", estado: "ATENDIDO", prioridad: "URGENTE", fecha_registro: haceDias(3) },
    { codigo: "EXP-2026-000002", tipo: "PERMISO", estado: "OBSERVADO", prioridad: "NORMAL", fecha_registro: haceDias(20) },
].map((e, i) => ({ ...e, id: `id-${i}`, asunto: `Asunto ficticio ${i}`, solicitante_id: "s-1" }));

// Simula el backend: filtra por estado/prioridad/tipo, total = COUNT.
function backendCon(datos) {
    return async ({ estado, prioridad, tipo, page = 1, pageSize = 20 } = {}) => {
        const f = datos.filter(
            (e) => (!estado || e.estado === estado) && (!prioridad || e.prioridad === prioridad) && (!tipo || e.tipo === tipo)
        );
        return { total: f.length, expedientes: f.slice((page - 1) * pageSize, page * pageSize) };
    };
}

function renderDashboard(permissions, role = "SECRETARIA") {
    return render(
        <AuthContext.Provider value={{ user: { id: "u-1", role, username: "demo" }, permissions, loading: false, isAuthenticated: true }}>
            <MemoryRouter>
                <Dashboard />
            </MemoryRouter>
        </AuthContext.Provider>
    );
}

beforeEach(() => vi.clearAllMocks());
afterEach(cleanup);

describe("agruparPorDia", () => {
    it("cuenta por día local, rellena con ceros e ignora fechas fuera del periodo o inválidas", () => {
        const hoy = new Date(2026, 9, 21, 15, 0);
        const puntos = agruparPorDia(
            [new Date(2026, 9, 21, 8).toISOString(), new Date(2026, 9, 21, 9).toISOString(), new Date(2026, 9, 19, 23).toISOString(), new Date(2026, 9, 10).toISOString(), "no-es-fecha"],
            7,
            hoy
        );
        expect(puntos).toHaveLength(7);
        expect(puntos[0].fecha).toBe("2026-10-15");
        expect(puntos[6]).toMatchObject({ fecha: "2026-10-21", total: 2 });
        expect(puntos[4]).toMatchObject({ fecha: "2026-10-19", total: 1 });
        expect(puntos.reduce((s, p) => s + p.total, 0)).toBe(3);
    });
});

describe("cargarSerieDiaria", () => {
    it("pagina hasta salir del periodo y no pide más páginas", async () => {
        expedientes.getExpedientes.mockImplementation(backendCon(DATOS));
        const { puntos, completo } = await cargarSerieDiaria(7);
        expect(completo).toBe(true);
        expect(puntos.reduce((s, p) => s + p.total, 0)).toBe(3);
        expect(expedientes.getExpedientes).toHaveBeenCalledTimes(1);
    });

    it("marca la serie como parcial si alcanza el tope de páginas dentro del periodo", async () => {
        const muchos = Array.from({ length: 1100 }, (_, i) => ({ ...DATOS[0], codigo: `X-${i}`, fecha_registro: haceDias(0) }));
        expedientes.getExpedientes.mockImplementation(backendCon(muchos));
        const { completo } = await cargarSerieDiaria(7);
        expect(completo).toBe(false);
        expect(expedientes.getExpedientes).toHaveBeenCalledTimes(10);
    });
});

describe("Dashboard", () => {
    it("muestra indicadores reales por estado con porcentaje y enlace a su bandeja", async () => {
        expedientes.getExpedientes.mockImplementation(backendCon(DATOS));
        renderDashboard(["dashboard.view", "expedientes.view"]);

        const total = (await screen.findByText("Total de expedientes")).closest("a");
        expect(within(total).getByText("4")).toBeTruthy();
        // Urgentes sin atender: solo el PENDIENTE urgente (el ATENDIDO urgente no cuenta).
        expect(within(total).getByText("1 urgente sin atender")).toBeTruthy();

        const porRevisar = screen.getByText("Por revisar").closest("a");
        expect(porRevisar.getAttribute("href")).toBe("/expedientes?estado=PENDIENTE");
        expect(within(porRevisar).getByText("1")).toBeTruthy();
        expect(within(porRevisar).getByText("25% del total")).toBeTruthy();

        // Por tipo: ordenado de mayor a menor y sin tipos en cero.
        const tipos = await screen.findByLabelText("Expedientes por tipo de trámite");
        const etiquetas = within(tipos).getAllByText(/Estudios|Permiso/).map((n) => n.textContent);
        expect(etiquetas).toEqual(["Constancia de Estudios", "Certificado de Estudios", "Permiso"]);

        expect(await screen.findByText("4 en los últimos 30 días")).toBeTruthy();
        expect(screen.getByText("EXP-2026-000005")).toBeTruthy();
        // Sin expedientes.create no se ofrece registrar.
        expect(screen.queryByRole("button", { name: /Nuevo expediente/ })).toBeNull();
    });

    it("sin expedientes.view no consulta expedientes y solo ofrece los módulos permitidos", () => {
        renderDashboard(["dashboard.view", "estudiantes.view"], "DOCENTE");
        expect(expedientes.getExpedientes).not.toHaveBeenCalled();
        expect(screen.getByText("Tus módulos")).toBeTruthy();
        expect(screen.getByRole("link", { name: /Directorio/ })).toBeTruthy();
        expect(screen.queryByText("Total de expedientes")).toBeNull();
        expect(screen.queryByRole("link", { name: /Usuarios/ })).toBeNull();
        expect(screen.queryByRole("link", { name: /Expedientes/ })).toBeNull();
    });

    it("informa el error sin inventar cifras", async () => {
        expedientes.getExpedientes.mockRejectedValue(new Error("network"));
        renderDashboard(["dashboard.view", "expedientes.view", "expedientes.create"]);
        expect(await screen.findByText(/No se pudieron cargar los indicadores/)).toBeTruthy();
        expect(screen.queryByText("Total de expedientes")).toBeNull();
        expect(screen.getByRole("button", { name: /Nuevo expediente/ })).toBeTruthy();
    });

    it("sin expedientes muestra ceros y el estado vacío de los gráficos", async () => {
        expedientes.getExpedientes.mockImplementation(backendCon([]));
        renderDashboard(["dashboard.view", "expedientes.view"]);
        expect(await screen.findByText("Todavía no hay expedientes registrados.")).toBeTruthy();
        expect(screen.getAllByText("Sin registros").length).toBe(4);
        expect(screen.getByText("No hay expedientes registrados.")).toBeTruthy();
    });

    it("el SOLICITANTE no ve el dashboard administrativo", () => {
        renderDashboard(["dashboard.view", "expedientes.view_own"], "SOLICITANTE");
        expect(expedientes.getExpedientes).not.toHaveBeenCalled();
        expect(screen.queryByText("Resumen de Mesa de Partes")).toBeNull();
    });
});

// Con reportes.view, los indicadores y "por tipo" salen del endpoint agregado
// (menos consultas); misma presentación. Si falla, se usa el listado.
function reporteDe(datos) {
    return async ({ prioridad } = {}) => {
        const f = datos.filter((e) => !prioridad || e.prioridad === prioridad);
        const contar = (campo, valor) => f.filter((e) => e[campo] === valor).length;
        return {
            total: f.length,
            por_estado: ["PENDIENTE", "EN_PROCESO", "OBSERVADO", "ATENDIDO"].map((v) => ({ clave: v, cantidad: contar("estado", v) })),
            por_tipo: ["CERTIFICADO", "CONSTANCIA", "PERMISO"].map((v) => ({ clave: v, cantidad: contar("tipo", v) })),
        };
    };
}

describe("Dashboard con reportes.view", () => {
    it("usa el endpoint agregado: mismas cifras con menos consultas al listado", async () => {
        expedientes.getExpedientes.mockImplementation(backendCon(DATOS));
        reportes.getReporteExpedientes.mockImplementation(reporteDe(DATOS));
        renderDashboard(["dashboard.view", "expedientes.view", "reportes.view"]);

        const total = (await screen.findByText("Total de expedientes")).closest("a");
        expect(within(total).getByText("4")).toBeTruthy();
        expect(within(total).getByText("1 urgente sin atender")).toBeTruthy();
        expect(within(screen.getByText("Por revisar").closest("a")).getByText("25% del total")).toBeTruthy();
        const tipos = await screen.findByLabelText("Expedientes por tipo de trámite");
        expect(within(tipos).getAllByText(/Estudios|Permiso/).map((n) => n.textContent)).toEqual(["Constancia de Estudios", "Certificado de Estudios", "Permiso"]);
        await screen.findByText("4 en los últimos 30 días");

        expect(reportes.getReporteExpedientes).toHaveBeenCalledWith({ prioridad: "URGENTE" });
        // Ningún conteo con page_size=1 (por estado / tipo): solo recientes y la serie diaria.
        const conteos = expedientes.getExpedientes.mock.calls.filter(([f]) => f?.pageSize === 1);
        expect(conteos).toHaveLength(0);
    });

    it("si el endpoint agregado falla, calcula con el listado sin perder cifras", async () => {
        expedientes.getExpedientes.mockImplementation(backendCon(DATOS));
        reportes.getReporteExpedientes.mockRejectedValue(new Error("no desplegado"));
        renderDashboard(["dashboard.view", "expedientes.view", "reportes.view"]);
        const total = (await screen.findByText("Total de expedientes")).closest("a");
        expect(within(total).getByText("4")).toBeTruthy();
        expect(screen.queryByText(/No se pudieron cargar los indicadores/)).toBeNull();
    });
});
