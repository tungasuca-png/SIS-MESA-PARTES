import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { AuthContext } from "../../context/auth-context";
import EstudianteSelector from "../../components/dashboard/EstudianteSelector";
import EstudianteNombre from "../../components/dashboard/EstudianteNombre";
import PsicologiaExpedientesList from "./PsicologiaExpedientesList";
import * as psicologia from "../../services/psicologiaService";
import * as estudiantesService from "../../services/estudiantesService";

// Psicología obtiene los datos de estudiantes SOLO a través de su propio
// backend (credencial de servicio): nunca /api/estudiantes. Datos ficticios.
vi.mock("../../services/psicologiaService", () => ({
    buscarEstudiantesPsicologia: vi.fn(),
    listExpedientesPsicologicos: vi.fn(),
    getExpedientePsicologicoByEstudiante: vi.fn(),
}));
vi.mock("../../services/usuariosService", () => ({ getUsuariosBasic: vi.fn().mockResolvedValue({ usuarios: [] }) }));
vi.mock("../../components/portal/RoleLayout", () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock("../../services/estudiantesService", () => ({ searchEstudiantes: vi.fn(), getEstudiantesBasic: vi.fn() }));

const ID = "11111111-1111-4111-8111-111111111111";
const DNI = "45127823";

afterEach(() => {
    cleanup();
    vi.clearAllMocks();
});

describe("EstudianteSelector (Psicología)", () => {
    it("busca por DNI con la ruta de Psicología y muestra solo nombre, grado y sección", async () => {
        // Aunque la respuesta trajera datos de más, la vista no los muestra.
        psicologia.buscarEstudiantesPsicologia.mockResolvedValue({
            total: 1,
            estudiantes: [{ id: ID, nombre_completo: "Ana María Quispe", grado: "3", seccion: "B", numero_documento: DNI, codigo: "EST-0001" }],
        });
        const onChange = vi.fn();
        const { container } = render(<EstudianteSelector id="sel" value={null} onChange={onChange} />);
        fireEvent.change(screen.getByLabelText("Estudiante *"), { target: { value: DNI } });

        await waitFor(() => expect(psicologia.buscarEstudiantesPsicologia).toHaveBeenCalledTimes(1));
        expect(psicologia.buscarEstudiantesPsicologia).toHaveBeenCalledWith({ q: DNI, page: 1, pageSize: 10 });
        expect(estudiantesService.searchEstudiantes).not.toHaveBeenCalled();
        expect(estudiantesService.getEstudiantesBasic).not.toHaveBeenCalled();

        const resultado = await screen.findByText("Ana María Quispe");
        const lista = container.querySelector(".dp-estudiante-results");
        expect(lista.textContent).toContain("3.º B");
        expect(lista.textContent).not.toContain(DNI);
        expect(lista.textContent).not.toContain("EST-0001");
        expect(lista.textContent).not.toContain("Código");

        fireEvent.click(resultado);
        expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ id: ID }));
    });

    it("estudiante seleccionado: nombre y grado/sección, sin DNI", () => {
        const { container } = render(
            <EstudianteSelector id="sel" value={{ id: ID, nombre_completo: "Ana María Quispe", grado: "3", seccion: "B", numero_documento: DNI }} onChange={vi.fn()} />
        );
        expect(container.textContent).toContain("Ana María Quispe");
        expect(container.textContent).toContain("3.º B");
        expect(container.textContent).not.toContain(DNI);
    });

    it("menos de 2 caracteres no consulta; sin resultados y error se muestran", async () => {
        render(<EstudianteSelector id="sel" value={null} onChange={vi.fn()} />);
        fireEvent.change(screen.getByLabelText("Estudiante *"), { target: { value: "a" } });
        await new Promise((r) => setTimeout(r, 400));
        expect(psicologia.buscarEstudiantesPsicologia).not.toHaveBeenCalled();

        psicologia.buscarEstudiantesPsicologia.mockResolvedValueOnce({ total: 0, estudiantes: [] });
        fireEvent.change(screen.getByLabelText("Estudiante *"), { target: { value: "zz" } });
        expect(await screen.findByText("No se encontraron estudiantes.")).toBeTruthy();

        psicologia.buscarEstudiantesPsicologia.mockRejectedValueOnce(new Error("red"));
        fireEvent.change(screen.getByLabelText("Estudiante *"), { target: { value: "zzz" } });
        // Sin respuesta HTTP (fallo de red), friendlyErrorMessage informa la conexión.
        expect(await screen.findByText(/No se pudo conectar con el servidor/)).toBeTruthy();
    });

    it("escritura rápida: una sola petición (debounce)", async () => {
        psicologia.buscarEstudiantesPsicologia.mockResolvedValue({ total: 0, estudiantes: [] });
        render(<EstudianteSelector id="sel" value={null} onChange={vi.fn()} />);
        const input = screen.getByLabelText("Estudiante *");
        for (const v of ["qu", "qui", "quis", "quisp", "quispe"]) fireEvent.change(input, { target: { value: v } });
        await waitFor(() => expect(psicologia.buscarEstudiantesPsicologia).toHaveBeenCalledTimes(1));
        expect(psicologia.buscarEstudiantesPsicologia).toHaveBeenCalledWith({ q: "quispe", page: 1, pageSize: 10 });
    });
});

describe("EstudianteNombre (resumen entregado por Psicología)", () => {
    const renderNombre = (resumen) =>
        render(
            <AuthContext.Provider value={{ user: { id: "u", role: "PSICOLOGO" }, permissions: ["psicologia.expedientes.view"], loading: false, isAuthenticated: true }}>
                <MemoryRouter>
                    <EstudianteNombre id={ID} resumen={resumen} />
                </MemoryRouter>
            </AuthContext.Provider>
        );

    it.each([
        [{ nombre_completo: "Ana María Quispe", resolucion: "RESUELTO" }, "Ana María Quispe"],
        [{ nombre_completo: "", resolucion: "NO_ENCONTRADO" }, "Estudiante no encontrado"],
        [{ nombre_completo: "", resolucion: "NO_DISPONIBLE" }, "Nombre no disponible"],
        [undefined, "Nombre no disponible"],
    ])("%o", (resumen, texto) => {
        const { container } = renderNombre(resumen);
        expect(screen.getByText(texto)).toBeTruthy();
        expect(container.innerHTML).not.toContain(ID); // ni texto ni tooltip con el UUID
        expect(estudiantesService.getEstudiantesBasic).not.toHaveBeenCalled();
    });
});

describe("Psicología no consulta Estudiantes directamente", () => {
    const fuentes = {
        ...import.meta.glob(["./*.jsx", "!./*.test.jsx"], { query: "?raw", import: "default", eager: true }),
        ...import.meta.glob(["../../components/psicologia/*.jsx", "!../../components/psicologia/*.test.jsx"], { query: "?raw", import: "default", eager: true }),
        ...import.meta.glob(
            ["../../components/dashboard/EstudianteSelector.jsx", "../../components/dashboard/EstudianteNombre.jsx", "../../components/dashboard/PsicologiaExpedienteFormModal.jsx"],
            { query: "?raw", import: "default", eager: true }
        ),
    };

    it("ningún archivo de Psicología usa estudiantesService, /api/estudiantes ni basic-batch", () => {
        expect(Object.keys(fuentes).length).toBeGreaterThan(5);
        for (const [archivo, codigo] of Object.entries(fuentes)) {
            expect(codigo, archivo).not.toMatch(/estudiantesService|\/api\/estudiantes|basic-batch|useEstudiantesBasic/);
        }
    });
});

describe("Lista de expedientes: panel «Buscar alumno»", () => {
    const renderLista = (permissions) => {
        psicologia.listExpedientesPsicologicos.mockResolvedValue({ total: 1, expedientes: [
            { id: "e1", codigo: "PSI-2026-000001", estudiante_id: ID, estado: "ABIERTO", responsable_id: "u", fecha_apertura: "2026-10-20T10:00:00Z",
              estudiante: { nombre_completo: "Ana María Quispe", resolucion: "RESUELTO" } },
        ] });
        return render(
            <AuthContext.Provider value={{ user: { id: "u", role: "PSICOLOGO" }, permissions, loading: false, isAuthenticated: true }}>
                <MemoryRouter>
                    <PsicologiaExpedientesList />
                </MemoryRouter>
            </AuthContext.Provider>
        );
    };

    it("un psicólogo con permisos de Psicología lo ve, aunque no tenga estudiantes.view", async () => {
        const { container } = renderLista(["psicologia.expedientes.view_own"]);
        expect(await screen.findByText("Ana María Quispe")).toBeTruthy();
        expect(container.querySelector("#psi-buscar-alumno")).not.toBeNull();
        expect(estudiantesService.getEstudiantesBasic).not.toHaveBeenCalled();
    });

    it("estudiantes.view sin permisos de Psicología no lo habilita", async () => {
        const { container } = renderLista(["estudiantes.view"]);
        await screen.findByText("Ana María Quispe");
        expect(container.querySelector("#psi-buscar-alumno")).toBeNull();
    });
});
