import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { AuthContext } from "../../context/auth-context";
import ExpedientesList from "./ExpedientesList";
import { INTERVALO_VIVO_MS, notificarCambioExpedientes } from "../../hooks/useActualizacionEnVivo";
import * as expedientes from "../../services/expedientesService";

// Mesa de Partes en vivo: una solicitud nueva aparece en la bandeja sin
// recargar la página. Datos ficticios; HTTP simulado.
vi.mock("../../services/expedientesService", () => ({ getExpedientes: vi.fn(), deleteExpediente: vi.fn() }));
vi.mock("../../services/usuariosService", () => ({ getUsuariosBasic: vi.fn().mockResolvedValue({ usuarios: [] }) }));
vi.mock("../../components/portal/RoleLayout", () => ({ default: ({ children }) => <div>{children}</div> }));

const exp = (n) => ({
    id: `exp-${n}`,
    codigo: `EXP-2026-00000${n}`,
    tipo: "CONSTANCIA",
    asunto: `Solicitud ficticia ${n}`,
    estado: "PENDIENTE",
    prioridad: "MEDIA",
    solicitante_id: "00000000-0000-4000-8000-0000000000aa",
    fecha_registro: `2026-10-20T1${n}:00:00Z`,
});

function renderBandeja(ruta = "/expedientes?estado=PENDIENTE") {
    const auth = { user: { id: "u-1", role: "SECRETARIA" }, permissions: ["expedientes.view"], loading: false, isAuthenticated: true };
    return render(
        <AuthContext.Provider value={auth}>
            <MemoryRouter initialEntries={[ruta]}>
                <Routes>
                    <Route path="/expedientes" element={<ExpedientesList />} />
                </Routes>
            </MemoryRouter>
        </AuthContext.Provider>
    );
}

beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers({ toFake: ["setInterval", "clearInterval"] });
});
afterEach(() => {
    vi.useRealTimers();
    cleanup();
});

describe("Bandeja de Mesa de Partes en vivo", () => {
    it("una solicitud nueva aparece sola, resaltada, con los mismos filtros y sin «Cargando»", async () => {
        expedientes.getExpedientes.mockResolvedValue({ expedientes: [exp(1)], total: 1 });
        renderBandeja();
        expect(await screen.findByText("EXP-2026-000001")).toBeTruthy();

        // El solicitante envía su FUT: la siguiente consulta periódica lo trae.
        expedientes.getExpedientes.mockResolvedValue({ expedientes: [exp(2), exp(1)], total: 2 });
        await act(async () => {
            vi.advanceTimersByTime(INTERVALO_VIVO_MS);
        });
        const nueva = await screen.findByText("EXP-2026-000002");
        expect(nueva.closest("tr").className).toContain("dp-row-new");
        expect(screen.getByText("EXP-2026-000001").closest("tr").className).not.toContain("dp-row-new");
        expect(screen.queryByText(/Cargando/)).toBeNull();
        expect(expedientes.getExpedientes).toHaveBeenLastCalledWith(expect.objectContaining({ estado: "PENDIENTE", page: 1 }));
    });

    it("cuando la campana detecta un trámite nuevo, la bandeja se actualiza de inmediato", async () => {
        expedientes.getExpedientes.mockResolvedValue({ expedientes: [exp(1)], total: 1 });
        renderBandeja();
        await screen.findByText("EXP-2026-000001");
        expedientes.getExpedientes.mockResolvedValue({ expedientes: [exp(3), exp(1)], total: 2 });
        await act(async () => {
            notificarCambioExpedientes();
        });
        expect(await screen.findByText("EXP-2026-000003")).toBeTruthy();
    });

    it("un fallo momentáneo no borra lo que ya se ve", async () => {
        expedientes.getExpedientes.mockResolvedValue({ expedientes: [exp(1)], total: 1 });
        renderBandeja();
        await screen.findByText("EXP-2026-000001");
        expedientes.getExpedientes.mockRejectedValue(new Error("Network Error"));
        await act(async () => {
            vi.advanceTimersByTime(INTERVALO_VIVO_MS);
        });
        expect(screen.getByText("EXP-2026-000001")).toBeTruthy();
    });

    it("si el usuario cambió el filtro, una respuesta en vivo vieja se descarta", async () => {
        let resolverVieja;
        expedientes.getExpedientes.mockResolvedValue({ expedientes: [exp(1)], total: 1 });
        renderBandeja();
        await screen.findByText("EXP-2026-000001");

        expedientes.getExpedientes.mockReturnValueOnce(new Promise((r) => (resolverVieja = r)));
        await act(async () => {
            vi.advanceTimersByTime(INTERVALO_VIVO_MS);
        });
        // Cambia el filtro a Atendidos mientras la consulta en vivo sigue pendiente.
        expedientes.getExpedientes.mockResolvedValue({ expedientes: [], total: 0 });
        fireEvent.change(screen.getByDisplayValue(/Pendiente|Por revisar/), { target: { value: "ATENDIDO" } });
        await act(async () => {
            resolverVieja({ expedientes: [exp(9)], total: 1 });
        });
        expect(screen.queryByText("EXP-2026-000009")).toBeNull();
    });
});
