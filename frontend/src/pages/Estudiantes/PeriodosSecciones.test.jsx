import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { AuthContext } from "../../context/auth-context";
import PeriodosSecciones from "./PeriodosSecciones";
import { httpError } from "../../test/estudiantesTestUtils";
import * as service from "../../services/estudiantesService";

vi.mock("../../services/estudiantesService", () => ({
    listPeriodos: vi.fn(),
    createPeriodo: vi.fn(),
    changeEstadoPeriodo: vi.fn(),
    listSecciones: vi.fn(),
    createSeccion: vi.fn(),
}));
vi.mock("../../components/portal/RoleLayout", () => ({ default: ({ children }) => <div>{children}</div> }));

const P2026 = { id: "p-2026", anio: 2026, nombre: "Año escolar 2026", estado: "PLANIFICADO" };

function renderPagina(permissions) {
    return render(
        <AuthContext.Provider value={{ user: { id: "u", role: "ADMIN" }, permissions, loading: false, isAuthenticated: true }}>
            <MemoryRouter>
                <PeriodosSecciones />
            </MemoryRouter>
        </AuthContext.Provider>
    );
}

beforeEach(() => {
    vi.clearAllMocks();
    service.listPeriodos.mockResolvedValue({ periodos: [P2026] });
    service.listSecciones.mockResolvedValue({ secciones: [{ id: "s-1", nivel: "PRIMARIA", grado: "1", seccion: "A", matriculados: 3, vacantes: null }] });
});
afterEach(cleanup);

describe("Periodos y secciones", () => {
    it("solo lectura sin estudiantes.matricula.update", async () => {
        renderPagina(["estudiantes.view"]);
        expect(await screen.findByText("Año escolar 2026")).toBeTruthy();
        expect(screen.queryByRole("button", { name: "Nuevo periodo" })).toBeNull();
        expect(screen.queryByRole("button", { name: "Iniciar periodo" })).toBeNull();
        fireEvent.click(screen.getByRole("button", { name: "Ver secciones" }));
        expect(await screen.findByText("Sin límite")).toBeTruthy();
        expect(screen.queryByRole("button", { name: "Nueva sección" })).toBeNull();
    });

    it("registra un periodo y valida el año", async () => {
        service.createPeriodo.mockResolvedValue({ periodo: { ...P2026, id: "p-2027", anio: 2027, nombre: "Año escolar 2027" } });
        renderPagina(["estudiantes.view", "estudiantes.matricula.update"]);
        fireEvent.click(await screen.findByRole("button", { name: "Nuevo periodo" }));
        const dialog = screen.getByRole("dialog", { name: "Nuevo periodo académico" });
        fireEvent.change(within(dialog).getByLabelText("Año *"), { target: { value: "1999" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Registrar periodo" }));
        expect(within(dialog).getByText(/entre 2000 y 2100/)).toBeTruthy();
        fireEvent.change(within(dialog).getByLabelText("Año *"), { target: { value: "2027" } });
        fireEvent.change(within(dialog).getByLabelText("Nombre *"), { target: { value: "Año escolar 2027" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Registrar periodo" }));
        await waitFor(() =>
            expect(service.createPeriodo).toHaveBeenCalledWith({ anio: "2027", nombre: "Año escolar 2027", fechaInicio: "", fechaFin: "" })
        );
        expect(await screen.findByText("Periodo Año escolar 2027 registrado.")).toBeTruthy();
    });

    it("avanza el estado del periodo y muestra el error de duplicado de sección", async () => {
        service.changeEstadoPeriodo.mockResolvedValue({ periodo: { ...P2026, estado: "EN_CURSO" } });
        service.createSeccion.mockRejectedValue(httpError(409, "duplicate_seccion: ya existe"));
        renderPagina(["estudiantes.view", "estudiantes.matricula.update"]);
        fireEvent.click(await screen.findByRole("button", { name: "Iniciar periodo" }));
        fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Confirmar" }));
        await waitFor(() => expect(service.changeEstadoPeriodo).toHaveBeenCalledWith("p-2026", "EN_CURSO"));

        fireEvent.click(await screen.findByRole("button", { name: "Ver secciones" }));
        fireEvent.click(await screen.findByRole("button", { name: "Nueva sección" }));
        const dialog = screen.getByRole("dialog", { name: /Nueva sección/ });
        fireEvent.change(within(dialog).getByLabelText("Nivel *"), { target: { value: "PRIMARIA" } });
        fireEvent.change(within(dialog).getByLabelText("Grado *"), { target: { value: "1" } });
        fireEvent.change(within(dialog).getByLabelText("Sección *"), { target: { value: "A" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Registrar sección" }));
        expect(await within(dialog).findByText(/Ya existe esa sección/)).toBeTruthy();
        expect(service.createSeccion).toHaveBeenCalledWith("p-2026", { nivel: "PRIMARIA", grado: "1", seccion: "A", turno: "", aula: "", vacantes: "" });
    });

    it("403 muestra acceso denegado", async () => {
        service.listPeriodos.mockRejectedValue(httpError(403, "permission_denied: x"));
        renderPagina(["estudiantes.view"]);
        expect(await screen.findByText("Acceso denegado")).toBeTruthy();
    });
});
