import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { AuthContext } from "../../context/auth-context";
import { httpError } from "../../test/estudiantesTestUtils";
import Login from "./login";
import GoogleCallback from "./GoogleCallback";
import RegistroGoogle from "./RegistroGoogle";
import VinculoGoogle from "../MesaDePartes/VinculoGoogle";
import * as authService from "../../services/authService";

// Acceso con Google en el frontend. HTTP simulado: sin Google ni cuentas reales.
vi.mock("../../services/authService", async (original) => ({
    ...(await original()),
    googleCanjear: vi.fn(),
    googleRegistro: vi.fn(),
    googleVincular: vi.fn(),
    googleEstado: vi.fn(),
}));

const SESION = { access_token: "at", refresh_token: "rt", expires_in: 900, user_id: "u-1", username: "sol_1", email: "ana@gmail.example", role: "SOLICITANTE" };

let iniciarSesion;
beforeEach(() => {
    vi.clearAllMocks();
    sessionStorage.clear();
    window.history.replaceState(null, "", "/");
    iniciarSesion = vi.fn(async (s) => ({ id: s.user_id, role: s.role }));
});
afterEach(cleanup);

function montar(ruta, elemento, path) {
    const auth = { user: null, permissions: [], loading: false, isAuthenticated: false, login: vi.fn(), iniciarSesion };
    return render(
        <AuthContext.Provider value={auth}>
            <MemoryRouter initialEntries={[ruta]}>
                <Routes>
                    <Route path={path} element={elemento} />
                    <Route path="/mesa-de-partes" element={<p>Portal del solicitante</p>} />
                    <Route path="/registro/google" element={<RegistroGoogle />} />
                    <Route path="/login" element={<p>Pantalla de login</p>} />
                </Routes>
            </MemoryRouter>
        </AuthContext.Provider>
    );
}

describe("Login", () => {
    it("conserva DNI y clave y agrega «Continuar con Google» hacia el backend", () => {
        montar("/login-real", <Login />, "/login-real");
        expect(screen.getByLabelText("DNI")).toBeTruthy();
        expect(screen.getByLabelText("Clave")).toBeTruthy();
        const google = screen.getByRole("link", { name: /Continuar con Google/ });
        expect(google.getAttribute("href")).toBe("/api/auth/google/iniciar");
        expect(screen.getByText(/El personal de la institución ingresa con su DNI/)).toBeTruthy();
    });
});

describe("Regreso desde Google", () => {
    it("identidad vinculada: canjea el código, inicia sesión y borra el código de la URL", async () => {
        window.history.replaceState(null, "", "/login/google#codigo=abc");
        authService.googleCanjear.mockResolvedValue({ resultado: "SESION", sesion: SESION });
        montar("/login/google", <GoogleCallback />, "/login/google");
        expect(await screen.findByText("Portal del solicitante")).toBeTruthy();
        expect(authService.googleCanjear).toHaveBeenCalledTimes(1);
        expect(authService.googleCanjear).toHaveBeenCalledWith("abc");
        expect(iniciarSesion).toHaveBeenCalledWith(SESION);
        expect(window.location.hash).toBe("");
    });

    it("primera vez: guarda el registro pendiente y abre el formulario obligatorio", async () => {
        window.history.replaceState(null, "", "/login/google#codigo=reg1");
        authService.googleCanjear.mockResolvedValue({ resultado: "REGISTRO_PENDIENTE", correo: "ana@gmail.example", nombres: "Ana", apellidos: "Quispe Ñahui" });
        montar("/login/google", <GoogleCallback />, "/login/google");
        expect(await screen.findByRole("heading", { name: "Completa tu registro" })).toBeTruthy();
        expect(iniciarSesion).not.toHaveBeenCalled();
        expect(screen.getByText("ana@gmail.example")).toBeTruthy();
        expect(screen.getByLabelText(/Nombres/).value).toBe("Ana");
        expect(screen.getByLabelText(/Apellido paterno/).value).toBe("Quispe");
        expect(screen.getByLabelText(/Apellido materno/).value).toBe("Ñahui");
    });

    it("muestra el error que indica el backend (sin canjear nada)", async () => {
        window.history.replaceState(null, "", "/login/google#error=correo_existente");
        montar("/login/google", <GoogleCallback />, "/login/google");
        expect((await screen.findByRole("alert")).textContent).toMatch(/Ya existe una cuenta con este correo/);
        expect(authService.googleCanjear).not.toHaveBeenCalled();
    });

    it("código vencido o reutilizado", async () => {
        window.history.replaceState(null, "", "/login/google#codigo=viejo");
        authService.googleCanjear.mockRejectedValue(httpError(401, "google_codigo_invalido: El enlace de acceso venció o ya fue utilizado. Vuelve a intentarlo."));
        montar("/login/google", <GoogleCallback />, "/login/google");
        expect((await screen.findByRole("alert")).textContent).toMatch(/venció o ya fue utilizado/);
        expect(iniciarSesion).not.toHaveBeenCalled();
    });
});

describe("Formulario obligatorio", () => {
    const pendiente = () =>
        sessionStorage.setItem("mesa_partes_registro_google", JSON.stringify({ codigo: "reg1", correo: "ana@gmail.example", nombres: "Ana", apellidos: "Quispe" }));
    const completar = (dni = "12345678") => {
        fireEvent.change(screen.getByLabelText(/DNI/), { target: { value: dni } });
        fireEvent.change(screen.getByLabelText(/Apellido materno/), { target: { value: "Ñahui" } });
        fireEvent.change(screen.getByLabelText(/Teléfono/), { target: { value: "987654321" } });
    };

    it("sin registro en curso no muestra el formulario", () => {
        montar("/registro/google", <RegistroGoogle />, "/registro/google");
        expect(screen.getByRole("alert").textContent).toMatch(/No hay un registro en curso/);
        expect(screen.queryByRole("button", { name: "Completar registro" })).toBeNull();
    });

    it("valida en el navegador y no envía datos inválidos", () => {
        pendiente();
        montar("/registro/google", <RegistroGoogle />, "/registro/google");
        fireEvent.change(screen.getByLabelText(/DNI/), { target: { value: "1234567" } });
        fireEvent.change(screen.getByLabelText(/Nombres/), { target: { value: " " } });
        fireEvent.click(screen.getByRole("button", { name: "Completar registro" }));
        expect(screen.getByText("El DNI debe tener 8 dígitos numéricos.")).toBeTruthy();
        expect(screen.getByText("Ingresa tus nombres.")).toBeTruthy();
        expect(authService.googleRegistro).not.toHaveBeenCalled();
        // El DNI solo admite dígitos.
        fireEvent.change(screen.getByLabelText(/DNI/), { target: { value: "12a34" } });
        expect(screen.getByLabelText(/DNI/).value).toBe("1234");
    });

    it("registra (sin rol), inicia sesión y entra al portal; el doble envío no duplica", async () => {
        pendiente();
        let resolver;
        authService.googleRegistro.mockReturnValue(new Promise((r) => (resolver = r)));
        montar("/registro/google", <RegistroGoogle />, "/registro/google");
        completar();
        const boton = screen.getByRole("button", { name: "Completar registro" });
        fireEvent.click(boton);
        fireEvent.click(boton);
        fireEvent.submit(boton.closest("form"));
        expect(authService.googleRegistro).toHaveBeenCalledTimes(1);
        const enviado = authService.googleRegistro.mock.calls[0][0];
        expect(enviado).toEqual({ codigo: "reg1", dni: "12345678", nombres: "Ana", apellidoPaterno: "Quispe", apellidoMaterno: "Ñahui", telefono: "987654321", direccion: "" });
        expect(Object.keys(enviado)).not.toContain("role");
        resolver(SESION);
        expect(await screen.findByText("Portal del solicitante")).toBeTruthy();
        expect(iniciarSesion).toHaveBeenCalledWith(SESION);
        expect(sessionStorage.getItem("mesa_partes_registro_google")).toBeNull();
    });

    it("DNI ya registrado: no inicia sesión y orienta al login institucional", async () => {
        pendiente();
        authService.googleRegistro.mockRejectedValue(
            httpError(409, "google_dni_registrado: Este DNI ya tiene una cuenta registrada. Inicia sesión con tus credenciales institucionales. Si deseas utilizar Google, podrás vincularlo desde tu perfil después de iniciar sesión.")
        );
        montar("/registro/google", <RegistroGoogle />, "/registro/google");
        completar();
        fireEvent.click(screen.getByRole("button", { name: "Completar registro" }));
        expect((await screen.findByRole("alert")).textContent).toMatch(/Este DNI ya tiene una cuenta registrada/);
        expect(screen.getByRole("link", { name: "Ir a iniciar sesión" })).toBeTruthy();
        expect(iniciarSesion).not.toHaveBeenCalled();
        // Puede corregir un DNI mal escrito y reintentar.
        expect(screen.getByRole("button", { name: "Completar registro" }).disabled).toBe(false);
    });

    it("servicio no preparado (503): mensaje claro, conserva el registro y permite reintentar", async () => {
        pendiente();
        authService.googleRegistro.mockRejectedValue(httpError(503, "servicio no disponible temporalmente"));
        montar("/registro/google", <RegistroGoogle />, "/registro/google");
        completar();
        fireEvent.click(screen.getByRole("button", { name: "Completar registro" }));
        expect((await screen.findByRole("alert")).textContent).toMatch(/El registro no está disponible en este momento/);
        expect(iniciarSesion).not.toHaveBeenCalled();
        expect(sessionStorage.getItem("mesa_partes_registro_google")).not.toBeNull();
        expect(screen.getByRole("button", { name: "Completar registro" }).disabled).toBe(false);
    });
});

describe("Mi perfil: vincular Google", () => {
    const montarPerfil = (ruta = "/mesa-de-partes/perfil") => {
        window.history.replaceState(null, "", ruta);
        return render(
            <MemoryRouter>
                <VinculoGoogle />
            </MemoryRouter>
        );
    };

    it("no vinculado: exige la clave actual y luego navega al inicio del flujo", async () => {
        authService.googleEstado.mockResolvedValue({ vinculado: false, correo: "", disponible: true });
        authService.googleVincular.mockResolvedValue({ url: "/api/auth/google/iniciar?modo=vincular", expira_en: 600 });
        const asignar = vi.fn();
        const original = window.location;
        Object.defineProperty(window, "location", { configurable: true, value: { ...original, assign: asignar, search: "", pathname: "/mesa-de-partes/perfil" } });
        try {
            montarPerfil();
            expect(await screen.findByText("Google no vinculado")).toBeTruthy();
            fireEvent.click(screen.getByRole("button", { name: "Vincular cuenta de Google" }));
            expect(screen.getByRole("alert").textContent).toMatch(/Ingresa tu clave actual/);
            expect(authService.googleVincular).not.toHaveBeenCalled();
            fireEvent.change(screen.getByLabelText("Clave actual"), { target: { value: "ClaveActual1" } });
            fireEvent.click(screen.getByRole("button", { name: "Vincular cuenta de Google" }));
            await waitFor(() => expect(asignar).toHaveBeenCalledWith("/api/auth/google/iniciar?modo=vincular"));
            expect(authService.googleVincular).toHaveBeenCalledWith("ClaveActual1");
        } finally {
            Object.defineProperty(window, "location", { configurable: true, value: original });
        }
    });

    it("clave incorrecta: muestra el error sin salir del perfil", async () => {
        authService.googleEstado.mockResolvedValue({ vinculado: false, disponible: true });
        authService.googleVincular.mockRejectedValue(httpError(400, "google_clave_incorrecta: La clave no es correcta."));
        montarPerfil();
        await screen.findByText("Google no vinculado");
        fireEvent.change(screen.getByLabelText("Clave actual"), { target: { value: "mala" } });
        fireEvent.click(screen.getByRole("button", { name: "Vincular cuenta de Google" }));
        expect((await screen.findByRole("alert")).textContent).toBe("La clave no es correcta.");
    });

    it("vinculado y resultado del regreso desde Google", async () => {
        authService.googleEstado.mockResolvedValue({ vinculado: true, correo: "ana@gmail.example", disponible: true });
        montarPerfil("/mesa-de-partes/perfil?google=vinculado");
        expect(await screen.findByText("Google vinculado")).toBeTruthy();
        expect(screen.getByText("ana@gmail.example")).toBeTruthy();
        expect(screen.getByRole("status").textContent).toMatch(/quedó vinculada/);
        expect(screen.queryByRole("button", { name: "Vincular cuenta de Google" })).toBeNull();
        expect(window.location.search).toBe("");
    });
});
