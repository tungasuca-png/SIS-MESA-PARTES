import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { AuthContext } from "../../context/auth-context";
import EstudianteNombre from "../../components/dashboard/EstudianteNombre";
import { estudianteBasico, httpError, renderEstudiantes } from "../../test/estudiantesTestUtils";
import * as service from "../../services/estudiantesService";

// Integraciones con endpoints reales que la interfaz no usaba: posibles
// duplicados, otros estudiantes de un responsable y el enlace desde
// Psicología a la ficha. Datos ficticios; HTTP simulado.
vi.mock("../../services/estudiantesService", () => ({
    searchEstudiantes: vi.fn(),
    listPosiblesDuplicados: vi.fn(),
    getEstudiante: vi.fn(),
    getEstudiantesBasic: vi.fn(),
    getMatriculasEstudiante: vi.fn(),
    getFamiliaresEstudiante: vi.fn(),
    getPersona: vi.fn(),
    listSecciones: vi.fn(),
}));
vi.mock("../../services/usuariosService", () => ({ getUsuariosBasic: vi.fn().mockResolvedValue({ usuarios: [] }) }));
vi.mock("../../components/portal/RoleLayout", () => ({ default: ({ children }) => <div>{children}</div> }));

const ana = estudianteBasico();
const hermano = estudianteBasico({ id: "22222222-2222-4222-8222-222222222222", codigo: "EST-002", nombre_completo: "Luis Pérez Soto" });

beforeEach(() => {
    vi.clearAllMocks();
    service.searchEstudiantes.mockResolvedValue({ estudiantes: [ana], total: 1 });
    service.getEstudiante.mockResolvedValue({ estudiante: ana });
    service.getEstudiantesBasic.mockResolvedValue({ estudiantes: [ana] });
    service.getMatriculasEstudiante.mockResolvedValue({ matriculas: [] });
    service.listSecciones.mockResolvedValue({ secciones: [] });
});
afterEach(cleanup);

describe("Directorio — posibles duplicados (estudiantes.update)", () => {
    it("sin estudiantes.update no aparece la vista ni se consulta", async () => {
        renderEstudiantes("/estudiantes");
        await screen.findByText("EST-001");
        expect(screen.queryByRole("tab", { name: "Posibles duplicados" })).toBeNull();
        expect(service.listPosiblesDuplicados).not.toHaveBeenCalled();
    });

    it("muestra los grupos para revisión humana, con documento enmascarado y enlace a cada ficha", async () => {
        service.listPosiblesDuplicados.mockResolvedValue({
            total: 1,
            grupos: [{ criterio: "MISMO_NOMBRE_SIN_DOCUMENTO", estudiantes: [ana, { ...hermano, nombre_completo: ana.nombre_completo, numero_documento: "" }] }],
        });
        renderEstudiantes("/estudiantes", { permissions: ["estudiantes.view", "estudiantes.update"] });
        await screen.findByText("EST-001");
        fireEvent.click(screen.getByRole("tab", { name: "Posibles duplicados" }));
        expect(await screen.findByText("Mismo nombre completo y al menos uno sin documento")).toBeTruthy();
        expect(service.listPosiblesDuplicados).toHaveBeenCalledWith({ page: 1, pageSize: 10 });
        expect(screen.getByText(/no fusiona ni borra nada/)).toBeTruthy();
        expect(screen.getByText(/••••5678/)).toBeTruthy();
        expect(screen.queryByText(/12345678/)).toBeNull();
        expect(screen.getAllByRole("link", { name: "Ver ficha" }).map((a) => a.getAttribute("href"))).toEqual([
            `/estudiantes/${ana.id}`,
            `/estudiantes/${hermano.id}`,
        ]);
    });

    it("sin grupos lo indica y un 403 se traduce", async () => {
        service.listPosiblesDuplicados.mockResolvedValue({ total: 0, grupos: [] });
        renderEstudiantes("/estudiantes", { permissions: ["estudiantes.view", "estudiantes.update"] });
        await screen.findByText("EST-001");
        fireEvent.click(screen.getByRole("tab", { name: "Posibles duplicados" }));
        expect(await screen.findByText("No se encontraron posibles duplicados.")).toBeTruthy();
        cleanup();

        service.listPosiblesDuplicados.mockRejectedValue(httpError(403, "permission_denied: x"));
        renderEstudiantes("/estudiantes", { permissions: ["estudiantes.view", "estudiantes.update"] });
        await screen.findByText("EST-001");
        fireEvent.click(screen.getByRole("tab", { name: "Posibles duplicados" }));
        expect(await screen.findByText("No tienes permiso para revisar duplicados.")).toBeTruthy();
    });
});

describe("Ficha — otros estudiantes a cargo de un responsable", () => {
    const madre = {
        relacion_id: "r-1",
        parentesco: "MADRE",
        es_apoderado: true,
        vigente: true,
        persona: { id: "per-1", version: 1, tipo_documento: "DNI", numero_documento: "40000001", nombres: "Carmen", apellido_paterno: "Soto" },
    };

    it("consulta a pedido (auditado) y lista solo otros vínculos vigentes con enlace a su ficha", async () => {
        service.getFamiliaresEstudiante.mockResolvedValue({ familiares: [madre] });
        service.getPersona.mockResolvedValue({
            persona: madre.persona,
            vinculos: [
                { relacion_id: "r-1", estudiante: ana, parentesco: "MADRE", es_apoderado: true, vigente: true },
                { relacion_id: "r-2", estudiante: hermano, parentesco: "MADRE", es_apoderado: false, vigente: true },
            ],
        });
        renderEstudiantes(`/estudiantes/${ana.id}?tab=familia`, { permissions: ["estudiantes.view", "estudiantes.familia.view"] });
        const boton = await screen.findByRole("button", { name: "Ver otros estudiantes a su cargo" });
        // No se consulta hasta que se pide (cada lectura de persona se audita).
        expect(service.getPersona).not.toHaveBeenCalled();
        fireEvent.click(boton);
        const lista = await screen.findByRole("list", { name: "Otros estudiantes a su cargo" });
        expect(service.getPersona).toHaveBeenCalledWith("per-1");
        const enlaces = within(lista).getAllByRole("link");
        expect(enlaces).toHaveLength(1);
        expect(enlaces[0].textContent).toBe("Luis Pérez Soto");
        expect(enlaces[0].getAttribute("href")).toBe(`/estudiantes/${hermano.id}`);
    });

    it("sin otros vínculos lo indica", async () => {
        service.getFamiliaresEstudiante.mockResolvedValue({ familiares: [madre] });
        service.getPersona.mockResolvedValue({ persona: madre.persona, vinculos: [{ relacion_id: "r-1", estudiante: ana, parentesco: "MADRE", vigente: true }] });
        renderEstudiantes(`/estudiantes/${ana.id}?tab=familia`, { permissions: ["estudiantes.view", "estudiantes.familia.view"] });
        fireEvent.click(await screen.findByRole("button", { name: "Ver otros estudiantes a su cargo" }));
        await waitFor(() => expect(screen.getByText("No tiene otros estudiantes vinculados.")).toBeTruthy());
    });
});

describe("Psicología — enlace a la ficha del estudiante", () => {
    const renderNombre = (permissions) =>
        render(
            <AuthContext.Provider value={{ user: { id: "u", role: "ADMIN" }, permissions, loading: false, isAuthenticated: true }}>
                <MemoryRouter>
                    <EstudianteNombre id={ana.id} estudiantes={new Map([[ana.id, { nombre_completo: "Ana Lucía Pérez Soto" }]])} />
                </MemoryRouter>
            </AuthContext.Provider>
        );

    it("con estudiantes.view el nombre enlaza a la ficha por UUID", () => {
        renderNombre(["psicologia.expedientes.view", "estudiantes.view"]);
        expect(screen.getByRole("link", { name: "Ana Lucía Pérez Soto" }).getAttribute("href")).toBe(`/estudiantes/${ana.id}`);
    });

    it("sin estudiantes.view es solo texto", () => {
        renderNombre(["psicologia.expedientes.view"]);
        expect(screen.getByText("Ana Lucía Pérez Soto")).toBeTruthy();
        expect(screen.queryByRole("link")).toBeNull();
    });
});
