import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { AuthContext } from "../../context/auth-context";
import PsicologiaExpedienteDetail from "./PsicologiaExpedienteDetail";
import PsicologiaExpedienteFormModal from "../../components/dashboard/PsicologiaExpedienteFormModal";
import * as psicologia from "../../services/psicologiaService";

// El resumen del caso es contenido clínico: el backend solo lo entrega y solo
// acepta modificarlo con "psicologia.atenciones.view". La interfaz lo refleja
// (no lo muestra ni lo envía sin ese permiso). Datos ficticios.
vi.mock("../../services/psicologiaService", () => ({
    getExpedientePsicologico: vi.fn(),
    updateExpedientePsicologico: vi.fn(),
    closeExpedientePsicologico: vi.fn(),
    reopenExpedientePsicologico: vi.fn(),
    openExpedientePsicologico: vi.fn(),
    listAtenciones: vi.fn().mockResolvedValue({ atenciones: [], total: 0 }),
}));
vi.mock("../../services/usuariosService", () => ({ getUsuariosBasic: vi.fn().mockResolvedValue({ usuarios: [] }), searchUsuarios: vi.fn().mockResolvedValue({ usuarios: [] }) }));
vi.mock("../../services/estudiantesService", () => ({ getEstudiantesBasic: vi.fn().mockResolvedValue({ estudiantes: [] }), searchEstudiantes: vi.fn() }));
vi.mock("../../components/portal/RoleLayout", () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock("../../components/dashboard/ResponsablePsicologoField", () => ({ default: () => null }));

const EXP = {
    id: "x-1",
    codigo: "PSI-2026-000001",
    estudiante_id: "11111111-1111-4111-8111-111111111111",
    estado: "ABIERTO",
    responsable_id: "u-1",
    abierto_por: "u-1",
    fecha_apertura: "2026-04-01T10:00:00Z",
};

function renderDetalle(permissions, expediente) {
    psicologia.getExpedientePsicologico.mockResolvedValue({ expediente });
    return render(
        <AuthContext.Provider value={{ user: { id: "u-1", role: "PSICOLOGO" }, permissions, loading: false, isAuthenticated: true }}>
            <MemoryRouter initialEntries={["/psicologia/expedientes/x-1"]}>
                <Routes>
                    <Route path="/psicologia/expedientes/:id" element={<PsicologiaExpedienteDetail />} />
                </Routes>
            </MemoryRouter>
        </AuthContext.Provider>
    );
}

beforeEach(() => vi.clearAllMocks());
afterEach(cleanup);

describe("Resumen del caso en el detalle del expediente", () => {
    it("con psicologia.atenciones.view se muestra el resumen que entrega el backend", async () => {
        renderDetalle(["psicologia.expedientes.view", "psicologia.atenciones.view"], { ...EXP, resumen_caso: "Resumen ficticio" });
        expect(await screen.findByText("Resumen ficticio")).toBeTruthy();
    });

    it("sin psicologia.atenciones.view indica que requiere autorización (no «Sin registrar»)", async () => {
        renderDetalle(["psicologia.expedientes.view"], EXP);
        expect(await screen.findByText("Requiere la autorización para consultar contenido clínico.")).toBeTruthy();
        expect(screen.queryByText("Sin registrar.")).toBeNull();
    });

    it("al editar sin acceso clínico no ofrece ni envía el resumen", async () => {
        psicologia.updateExpedientePsicologico.mockResolvedValue({ expediente: { ...EXP, estado: "EN_SEGUIMIENTO" } });
        renderDetalle(["psicologia.expedientes.view", "psicologia.expedientes.update"], EXP);
        fireEvent.click(await screen.findByRole("button", { name: /Editar/ }));
        expect(screen.queryByLabelText("Resumen del caso")).toBeNull();
        fireEvent.change(screen.getByLabelText("Estado"), { target: { value: "EN_SEGUIMIENTO" } });
        fireEvent.click(screen.getByRole("button", { name: "Guardar cambios" }));
        await waitFor(() => expect(psicologia.updateExpedientePsicologico).toHaveBeenCalled());
        expect(psicologia.updateExpedientePsicologico.mock.calls[0][1].resumenCaso).toBeUndefined();
    });

    it("con acceso clínico el resumen se edita y se envía", async () => {
        psicologia.updateExpedientePsicologico.mockResolvedValue({ expediente: { ...EXP, resumen_caso: "Nuevo" } });
        renderDetalle(["psicologia.expedientes.view", "psicologia.expedientes.update", "psicologia.atenciones.view"], { ...EXP, resumen_caso: "Viejo" });
        fireEvent.click(await screen.findByRole("button", { name: /Editar/ }));
        fireEvent.change(screen.getByLabelText("Resumen del caso"), { target: { value: "Nuevo" } });
        fireEvent.click(screen.getByRole("button", { name: "Guardar cambios" }));
        await waitFor(() => expect(psicologia.updateExpedientePsicologico.mock.calls[0][1].resumenCaso).toBe("Nuevo"));
    });
});

describe("Resumen inicial al abrir un expediente", () => {
    const renderModal = (permissions) =>
        render(
            <AuthContext.Provider value={{ user: { id: "u-1", role: "PSICOLOGO" }, permissions, loading: false, isAuthenticated: true }}>
                <PsicologiaExpedienteFormModal onClose={vi.fn()} onCreated={vi.fn()} initialEstudiante={{ id: EXP.estudiante_id, nombre_completo: "Alumno Ficticio" }} />
            </AuthContext.Provider>
        );

    it("sin psicologia.atenciones.view no se ofrece el campo ni se envía", async () => {
        psicologia.openExpedientePsicologico.mockResolvedValue({ expediente: EXP });
        renderModal(["psicologia.expedientes.create", "estudiantes.view"]);
        expect(screen.queryByLabelText("Resumen del caso")).toBeNull();
        fireEvent.submit(document.querySelector("form"));
        await waitFor(() => expect(psicologia.openExpedientePsicologico).toHaveBeenCalled());
        expect(psicologia.openExpedientePsicologico.mock.calls[0][1].resumenCaso).toBeUndefined();
    });

    it("con psicologia.atenciones.view se ofrece el campo", () => {
        renderModal(["psicologia.expedientes.create", "psicologia.atenciones.view", "estudiantes.view"]);
        expect(screen.getByLabelText("Resumen del caso")).toBeTruthy();
    });
});
