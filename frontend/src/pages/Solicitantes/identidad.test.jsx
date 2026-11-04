import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { AuthContext } from "../../context/auth-context";
import { httpError } from "../../test/estudiantesTestUtils";
import IdentidadSolicitantes from "./IdentidadSolicitantes";
import SolicitanteNombre from "../../components/dashboard/SolicitanteNombre";
import Login from "../Login/login";
import * as identidadService from "../../services/identidadService";
import api from "../../services/api";
import { login as loginRequest } from "../../services/authService";

// Verificación de identidad de solicitantes y login por correo. HTTP simulado.
vi.mock("../../components/dashboard/DashboardLayout", () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock("../../services/identidadService", () => ({
    listIdentidades: vi.fn(),
    verificarIdentidad: vi.fn(),
    rechazarIdentidad: vi.fn(),
    reconciliarIdentidades: vi.fn(),
}));

const AUTH = { user: { id: "staff-1", role: "SECRETARIA" }, permissions: [], loading: false, isAuthenticated: true, login: vi.fn() };

beforeEach(() => vi.clearAllMocks());
afterEach(cleanup);

const PENDIENTE = { usuario_id: "u-1", nombre_completo: "Ana Quispe", dni_declarado: "12345678", origen: "correo", estado: "PENDIENTE", created_at: "2026-11-02T10:00:00Z", conflicto: false };
const CONFLICTO = { ...PENDIENTE, usuario_id: "u-2", nombre_completo: "Luis Ccori", dni_declarado: "87654321", origen: "google", conflicto: true };

function montarBandeja() {
    return render(
        <AuthContext.Provider value={AUTH}>
            <MemoryRouter>
                <IdentidadSolicitantes />
            </MemoryRouter>
        </AuthContext.Provider>
    );
}

describe("Bandeja de verificación de identidad", () => {
    it("lista los pendientes y bloquea la verificación de un DNI en conflicto", async () => {
        identidadService.listIdentidades.mockResolvedValue({ items: [PENDIENTE, CONFLICTO], total: 2 });
        montarBandeja();
        expect(await screen.findByText("Ana Quispe")).toBeTruthy();
        expect(identidadService.listIdentidades).toHaveBeenCalledWith({ estado: "PENDIENTE", limit: 50, offset: 0 });
        const filaConflicto = screen.getByText("Luis Ccori").closest("tr");
        expect(within(filaConflicto).getByText("DNI asignado a otra cuenta")).toBeTruthy();
        expect(within(filaConflicto).getByRole("button", { name: "Verificar" }).disabled).toBe(true);
    });

    it("verifica indicando el método y recarga la bandeja", async () => {
        identidadService.listIdentidades.mockResolvedValue({ items: [PENDIENTE], total: 1 });
        identidadService.verificarIdentidad.mockResolvedValue({ estado: "VERIFICADA" });
        montarBandeja();
        fireEvent.click(await screen.findByRole("button", { name: "Verificar" }));
        fireEvent.change(screen.getByLabelText(/Cómo se cotejó el DNI 12345678/), { target: { value: "DOCUMENTO" } });
        fireEvent.click(screen.getByRole("button", { name: "Confirmar verificación" }));
        expect(await screen.findByText(/Identidad verificada/)).toBeTruthy();
        expect(identidadService.verificarIdentidad).toHaveBeenCalledWith("u-1", "DOCUMENTO");
        await waitFor(() => expect(identidadService.listIdentidades).toHaveBeenCalledTimes(2));
    });

    it("el rechazo exige un motivo", async () => {
        identidadService.listIdentidades.mockResolvedValue({ items: [PENDIENTE], total: 1 });
        identidadService.rechazarIdentidad.mockResolvedValue({ estado: "RECHAZADA" });
        montarBandeja();
        fireEvent.click(await screen.findByRole("button", { name: "Rechazar" }));
        const confirmar = screen.getByRole("button", { name: "Confirmar rechazo" });
        expect(confirmar.disabled).toBe(true);
        fireEvent.change(screen.getByLabelText("Motivo del rechazo"), { target: { value: "Los datos no coinciden" } });
        fireEvent.click(confirmar);
        await waitFor(() => expect(identidadService.rechazarIdentidad).toHaveBeenCalledWith("u-1", "Los datos no coinciden"));
    });

    it("perfil pendiente: lo informa y permite reintentar la sincronización", async () => {
        identidadService.listIdentidades.mockResolvedValue({ items: [PENDIENTE], total: 1 });
        identidadService.verificarIdentidad.mockResolvedValue({ estado: "VERIFICADA", sincronizacion: "PERFIL_PENDIENTE" });
        montarBandeja();
        fireEvent.click(await screen.findByRole("button", { name: "Verificar" }));
        fireEvent.click(screen.getByRole("button", { name: "Confirmar verificación" }));
        expect(await screen.findByText(/El perfil se actualizará cuando el servicio de usuarios responda/)).toBeTruthy();

        cleanup();
        const verificada = { ...PENDIENTE, estado: "VERIFICADA", sincronizacion: "PERFIL_PENDIENTE", sincronizacion_error: "usuarios_no_disponible" };
        identidadService.listIdentidades.mockResolvedValue({ items: [verificada], total: 1 });
        identidadService.reconciliarIdentidades.mockResolvedValue({ completadas: 1, revertidas: 0, pendientes: 0 });
        montarBandeja();
        fireEvent.click(await screen.findByRole("tab", { name: "Verificadas" }));
        expect(await screen.findByText(/perfil pendiente de actualizar \(usuarios_no_disponible\)/)).toBeTruthy();
        fireEvent.click(screen.getByRole("button", { name: "Reintentar sincronización de perfiles" }));
        expect(await screen.findByText("Sincronización: 1 completadas, 0 revertidas por conflicto, 0 pendientes.")).toBeTruthy();
        expect(identidadService.reconciliarIdentidades).toHaveBeenCalledTimes(1);
    });

    it("muestra el conflicto o la falta de permiso que responde el backend", async () => {
        identidadService.listIdentidades.mockResolvedValue({ items: [PENDIENTE], total: 1 });
        identidadService.verificarIdentidad.mockRejectedValue(httpError(409, "identidad_conflicto: El DNI declarado ya está asignado a otra cuenta. No se modificó ninguna cuenta."));
        montarBandeja();
        fireEvent.click(await screen.findByRole("button", { name: "Verificar" }));
        fireEvent.click(screen.getByRole("button", { name: "Confirmar verificación" }));
        expect(await screen.findByText(/No se modificó ninguna cuenta/)).toBeTruthy();

        cleanup();
        identidadService.listIdentidades.mockRejectedValue(httpError(403, "el usuario no tiene el permiso requerido"));
        montarBandeja();
        expect(await screen.findByText("No tienes permiso para verificar identidades.")).toBeTruthy();
    });
});

describe("Marca «Identidad no verificada»", () => {
    it("se muestra al personal junto al nombre del solicitante", () => {
        const usuarios = new Map([
            ["u-1", { id: "u-1", nombre_completo: "Ana Quispe", identidad_pendiente: true }],
            ["u-2", { id: "u-2", nombre_completo: "Rosa Huamán", identidad_pendiente: false }],
        ]);
        render(
            <AuthContext.Provider value={AUTH}>
                <SolicitanteNombre id="u-1" usuarios={usuarios} />
                <SolicitanteNombre id="u-2" usuarios={usuarios} />
            </AuthContext.Provider>
        );
        expect(screen.getAllByText("Identidad no verificada")).toHaveLength(1);
        expect(screen.getByText("Rosa Huamán")).toBeTruthy();
    });
});

describe("Login por correo (solicitantes con identidad pendiente)", () => {
    it("cambia al correo, lo precarga tras la activación y envía el correo", async () => {
        const login = vi.fn().mockResolvedValue({ role: "SOLICITANTE" });
        render(
            <AuthContext.Provider value={{ ...AUTH, user: null, isAuthenticated: false, login }}>
                <MemoryRouter initialEntries={[{ pathname: "/login", state: { correo: "ana@correo.example" } }]}>
                    <Routes>
                        <Route path="/login" element={<Login />} />
                        <Route path="/mesa-de-partes" element={<p>Portal</p>} />
                    </Routes>
                </MemoryRouter>
            </AuthContext.Provider>
        );
        expect(screen.getByLabelText("Correo electrónico").value).toBe("ana@correo.example");
        fireEvent.change(screen.getByLabelText("Clave"), { target: { value: "ClaveSegura1" } });
        fireEvent.click(screen.getByRole("button", { name: /^ingresar$/i }));
        expect(await screen.findByText("Portal")).toBeTruthy();
        expect(login).toHaveBeenCalledWith("ana@correo.example", "ClaveSegura1");
        // El enlace vuelve al DNI (el personal sigue ingresando solo con DNI).
        cleanup();
        render(
            <AuthContext.Provider value={{ ...AUTH, user: null, isAuthenticated: false, login }}>
                <MemoryRouter>
                    <Login />
                </MemoryRouter>
            </AuthContext.Provider>
        );
        expect(screen.getByLabelText("DNI")).toBeTruthy();
        fireEvent.click(screen.getByRole("button", { name: /Usa tu correo/ }));
        expect(screen.getByLabelText("Correo electrónico")).toBeTruthy();
    });

    it("authService envía correo o DNI según el identificador, nunca un rol", async () => {
        const post = vi.spyOn(api, "post").mockResolvedValue({ data: {} });
        await loginRequest("ana@correo.example", "x");
        await loginRequest("12345678", "y");
        expect(post).toHaveBeenNthCalledWith(1, "/api/auth/login", { correo: "ana@correo.example", password: "x" });
        expect(post).toHaveBeenNthCalledWith(2, "/api/auth/login", { dni: "12345678", password: "y" });
        post.mockRestore();
    });
});
