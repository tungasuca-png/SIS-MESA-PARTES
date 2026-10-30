import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { AuthContext } from "../../context/auth-context";
import NotificationButton from "./NotificationButton";
import { INTERVALO_MS } from "../../hooks/useNotificacionesTramites";
import * as expedientes from "../../services/expedientesService";

vi.mock("../../services/expedientesService", () => ({ getExpedientes: vi.fn() }));

// Datos ficticios de trámites (FUT / solicitudes) en estado PENDIENTE.
const tramite = (n, fecha) => ({
    id: `exp-${n}`,
    codigo: `EXP-2026-00000${n}`,
    tipo: "CONSTANCIA",
    asunto: `Solicitud de prueba ${n}`,
    estado: "PENDIENTE",
    fecha_registro: fecha,
});
const T1 = tramite(1, "2026-10-20T10:00:00Z");
const T2 = tramite(2, "2026-10-20T11:00:00Z");
const T3 = tramite(3, "2026-10-20T12:00:00Z");

function renderCampana(permissions = ["expedientes.view"]) {
    const auth = { user: { id: "u-1", username: "secretaria", role: "SECRETARIA" }, permissions, loading: false, isAuthenticated: true };
    return render(
        <AuthContext.Provider value={auth}>
            <MemoryRouter>
                <NotificationButton />
            </MemoryRouter>
        </AuthContext.Provider>
    );
}

beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    vi.useFakeTimers({ toFake: ["setInterval", "clearInterval"] });
});
afterEach(() => {
    vi.useRealTimers();
    cleanup();
});

describe("Campana de notificaciones", () => {
    it("sin expedientes.view no consulta ni muestra contador", () => {
        renderCampana(["estudiantes.view"]);
        expect(expedientes.getExpedientes).not.toHaveBeenCalled();
        fireEvent.click(screen.getByRole("button", { name: "Notificaciones" }));
        expect(screen.getByText("No tienes notificaciones.")).toBeTruthy();
    });

    it("muestra los trámites por revisar reales y cuenta los nuevos", async () => {
        expedientes.getExpedientes.mockResolvedValue({ expedientes: [T2, T1], total: 2 });
        renderCampana();
        const boton = await screen.findByRole("button", { name: "Notificaciones: 2 trámites nuevos" });
        expect(expedientes.getExpedientes).toHaveBeenCalledWith({ estado: "PENDIENTE", page: 1, pageSize: 20 });
        expect(within(boton).getByText("2")).toBeTruthy();

        fireEvent.click(boton);
        const panel = screen.getByRole("dialog", { name: "Notificaciones" });
        expect(within(panel).getByText("EXP-2026-000002", { exact: false })).toBeTruthy();
        expect(within(panel).getByText("Solicitud de prueba 1")).toBeTruthy();
        expect(within(panel).getAllByRole("link")[0].getAttribute("href")).toBe("/expedientes/exp-2");
        expect(within(panel).getByRole("link", { name: "Ver los 2 trámites por revisar" }).getAttribute("href")).toBe("/expedientes?estado=PENDIENTE");
    });

    it("marcar como leídas quita el contador y un trámite que llega después vuelve a sumarlo", async () => {
        expedientes.getExpedientes.mockResolvedValue({ expedientes: [T2, T1], total: 2 });
        renderCampana();
        fireEvent.click(await screen.findByRole("button", { name: "Notificaciones: 2 trámites nuevos" }));
        fireEvent.click(screen.getByRole("button", { name: "Marcar como leídas" }));
        expect(screen.getByRole("button", { name: "Notificaciones" })).toBeTruthy();
        expect(localStorage.getItem("dp_notif_visto_u-1")).toBe(T2.fecha_registro);

        // Llega un FUT nuevo: se detecta en la siguiente consulta periódica.
        expedientes.getExpedientes.mockResolvedValue({ expedientes: [T3, T2, T1], total: 3 });
        await act(async () => {
            vi.advanceTimersByTime(INTERVALO_MS);
        });
        expect(await screen.findByRole("button", { name: "Notificaciones: 1 trámites nuevos" })).toBeTruthy();
        expect(expedientes.getExpedientes).toHaveBeenCalledTimes(2);
    });

    it("lo leído se recuerda entre páginas (misma sesión del navegador)", async () => {
        localStorage.setItem("dp_notif_visto_u-1", T2.fecha_registro);
        expedientes.getExpedientes.mockResolvedValue({ expedientes: [T2, T1], total: 2 });
        renderCampana();
        await waitFor(() => expect(expedientes.getExpedientes).toHaveBeenCalled());
        expect(await screen.findByRole("button", { name: "Notificaciones" })).toBeTruthy();
    });

    it("sin conexión lo indica sin inventar trámites", async () => {
        expedientes.getExpedientes.mockRejectedValue(new Error("Network Error"));
        renderCampana();
        await waitFor(() => expect(expedientes.getExpedientes).toHaveBeenCalled());
        fireEvent.click(screen.getByRole("button", { name: "Notificaciones" }));
        expect(await screen.findByText(/No se pudieron consultar los trámites/)).toBeTruthy();
    });
});
