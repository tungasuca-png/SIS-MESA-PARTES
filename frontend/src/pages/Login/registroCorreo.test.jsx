import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { AuthContext } from "../../context/auth-context";
import { httpError } from "../../test/estudiantesTestUtils";
import Login from "./login";
import RegistroCorreo from "./RegistroCorreo";
import VerificarCorreo from "./VerificarCorreo";
import * as authService from "../../services/authService";

// Registro manual con verificación del correo. HTTP simulado: sin correo real.
vi.mock("../../services/authService", async (original) => ({
    ...(await original()),
    registroCorreo: vi.fn(),
    verificarCorreo: vi.fn(),
    reenviarCodigo: vi.fn(),
}));

beforeEach(() => vi.clearAllMocks());
afterEach(() => {
    cleanup();
    vi.useRealTimers();
});

function EstadoLogin() {
    const { state } = useLocation();
    return <p>Pantalla de login {state?.correo ?? state?.dni ?? ""}</p>;
}

function montar(entrada, elemento, path, login = vi.fn()) {
    const auth = { user: null, permissions: [], loading: false, isAuthenticated: false, login };
    return render(
        <AuthContext.Provider value={auth}>
            <MemoryRouter initialEntries={[entrada]}>
                <Routes>
                    <Route path={path} element={elemento} />
                    <Route path="/registro/verificar" element={<VerificarCorreo />} />
                    <Route path="/login" element={<EstadoLogin />} />
                    <Route path="/registro" element={<p>Formulario de registro</p>} />
                </Routes>
            </MemoryRouter>
        </AuthContext.Provider>
    );
}

// Etiqueta anclada al inicio: los campos obligatorios terminan en " *" y el
// formulario tiene su propio aria-label.
const etiqueta = (texto) => new RegExp("^" + texto);
const escribir = (texto, valor) => fireEvent.change(screen.getByLabelText(etiqueta(texto)), { target: { value: valor } });

function llenarFormulario({ confirmacion = "ClaveSegura1", terminos = true } = {}) {
    escribir("Nombres", "Ana María");
    escribir("DNI", "12345678");
    escribir("Apellido paterno", "Quispe");
    escribir("Apellido materno", "Huamán");
    escribir("Correo electrónico", "Ana@Correo.Example");
    fireEvent.change(screen.getByLabelText(/^Contraseña/), { target: { value: "ClaveSegura1" } });
    fireEvent.change(screen.getByLabelText(/^Confirmar contraseña/), { target: { value: confirmacion } });
    if (terminos) fireEvent.click(screen.getByLabelText(/Acepto los/));
}

describe("Login", () => {
    it("ofrece crear una cuenta con correo además de Google", () => {
        montar("/x", <Login />, "/x");
        expect(screen.getByRole("link", { name: "Crear cuenta con correo" }).getAttribute("href")).toBe("/registro");
        expect(screen.getByRole("link", { name: /Continuar con Google/ })).toBeTruthy();
    });

    it("informa que el correo no está verificado (solo con la clave correcta)", async () => {
        const login = vi.fn().mockRejectedValue(httpError(403, "correo_no_verificado: Tu correo todavía no está verificado. Ingresa el código."));
        montar("/x", <Login />, "/x", login);
        escribir("DNI", "12345678");
        escribir("Clave", "ClaveSegura1");
        fireEvent.click(screen.getByRole("button", { name: /ingresar/i }));
        expect(await screen.findByText("Tu correo todavía no está verificado. Ingresa el código.")).toBeTruthy();
    });

    it("precarga el DNI al volver de la activación", () => {
        render(
            <AuthContext.Provider value={{ user: null, permissions: [], loading: false, isAuthenticated: false, login: vi.fn() }}>
                <MemoryRouter initialEntries={[{ pathname: "/x", state: { dni: "12345678" } }]}>
                    <Routes>
                        <Route path="/x" element={<Login />} />
                    </Routes>
                </MemoryRouter>
            </AuthContext.Provider>
        );
        expect(screen.getByLabelText("DNI").value).toBe("12345678");
    });
});

describe("Formulario de registro", () => {
    it("valida en el navegador sin llamar al backend", () => {
        montar("/r", <RegistroCorreo />, "/r");
        llenarFormulario({ confirmacion: "OtraClave99", terminos: false });
        fireEvent.click(screen.getByRole("button", { name: "Crear cuenta" }));
        expect(screen.getByText("Las contraseñas no coinciden.")).toBeTruthy();
        expect(screen.getByText("Debes aceptar los términos y la política de privacidad.")).toBeTruthy();
        expect(authService.registroCorreo).not.toHaveBeenCalled();
    });

    it("envía los datos sin rol y pasa a la verificación", async () => {
        authService.registroCorreo.mockResolvedValue({ registro_id: "r-1", expira_en: 600, reenviar_en: 60, mensaje: "x" });
        montar("/r", <RegistroCorreo />, "/r");
        llenarFormulario();
        fireEvent.click(screen.getByRole("button", { name: "Crear cuenta" }));
        expect(await screen.findByText("ana@correo.example")).toBeTruthy();
        const enviado = authService.registroCorreo.mock.calls[0][0];
        expect(enviado).toMatchObject({ nombres: "Ana María", apellidoPaterno: "Quispe", dni: "12345678", correo: "ana@correo.example", aceptaTerminos: true });
        expect(enviado.versionTerminos).toBeTruthy();
        expect(enviado).not.toHaveProperty("role");
        expect(screen.getByLabelText("Código de verificación")).toBeTruthy();
    });

    it("muestra un mensaje comprensible si el proveedor de correo falla", async () => {
        authService.registroCorreo.mockRejectedValue(httpError(503, "servicio no disponible temporalmente"));
        montar("/r", <RegistroCorreo />, "/r");
        llenarFormulario();
        fireEvent.click(screen.getByRole("button", { name: "Crear cuenta" }));
        expect(await screen.findByText(/No pudimos enviar el código/)).toBeTruthy();
    });
});

const verificacion = (extra = {}) => ({ pathname: "/registro/verificar", state: { registroId: "r-1", correo: "ana@correo.example", expiraEn: 600, reenviarEn: 60, ...extra } });

describe("Verificación del correo", () => {
    it("sin registro en curso remite al formulario", () => {
        montar("/registro/verificar", <VerificarCorreo />, "/registro/verificar");
        expect(screen.getByText(/No hay un registro en curso/)).toBeTruthy();
    });

    it("código incorrecto: muestra los intentos restantes", async () => {
        authService.verificarCorreo.mockRejectedValue(httpError(400, "registro_codigo_incorrecto: El código no es correcto. Te quedan 4 intentos."));
        montar(verificacion(), <VerificarCorreo />, "/registro/verificar");
        escribir("Código de verificación", "123456");
        fireEvent.click(screen.getByRole("button", { name: "Verificar y activar" }));
        expect(await screen.findByText("El código no es correcto. Te quedan 4 intentos.")).toBeTruthy();
        expect(authService.verificarCorreo).toHaveBeenCalledWith("r-1", "123456");
    });

    it("código correcto: confirma la activación y lleva al login con el correo (sin devolver el DNI)", async () => {
        authService.verificarCorreo.mockResolvedValue({ mensaje: "Tu cuenta fue activada.", correo: "ana@correo.example" });
        montar(verificacion(), <VerificarCorreo />, "/registro/verificar");
        escribir("Código de verificación", "654321");
        fireEvent.click(screen.getByRole("button", { name: "Verificar y activar" }));
        expect(await screen.findByText("Tu cuenta fue activada.")).toBeTruthy();
        fireEvent.click(screen.getByRole("button", { name: "Iniciar sesión" }));
        expect(await screen.findByText("Pantalla de login ana@correo.example")).toBeTruthy();
    });

    it("el reenvío espera el temporizador y avisa del código nuevo", async () => {
        vi.useFakeTimers({ shouldAdvanceTime: true });
        authService.reenviarCodigo.mockResolvedValue({ expira_en: 600, reenviar_en: 60, mensaje: "x" });
        montar(verificacion({ reenviarEn: 2 }), <VerificarCorreo />, "/registro/verificar");
        const boton = screen.getByRole("button", { name: /Reenviar código en/ });
        expect(boton.disabled).toBe(true);
        await act(async () => {
            vi.advanceTimersByTime(3000);
        });
        fireEvent.click(screen.getByRole("button", { name: "Reenviar código" }));
        expect(await screen.findByText(/Te enviamos un código nuevo/)).toBeTruthy();
        expect(authService.reenviarCodigo).toHaveBeenCalledWith("r-1");
    });

    it("un registro que ya no puede continuar oculta el formulario", async () => {
        authService.verificarCorreo.mockRejectedValue(httpError(409, "registro_dni_registrado: Este DNI ya tiene una cuenta registrada."));
        montar(verificacion(), <VerificarCorreo />, "/registro/verificar");
        escribir("Código de verificación", "123456");
        fireEvent.click(screen.getByRole("button", { name: "Verificar y activar" }));
        expect(await screen.findByText("Este DNI ya tiene una cuenta registrada.")).toBeTruthy();
        await waitFor(() => expect(screen.queryByRole("button", { name: "Verificar y activar" })).toBeNull());
        expect(screen.getByRole("link", { name: "Iniciar sesión" })).toBeTruthy();
    });

    it("no muestra el código ni la clave en la consola", async () => {
        const espia = vi.spyOn(console, "log");
        authService.verificarCorreo.mockResolvedValue({ mensaje: "ok", correo: "ana@correo.example" });
        montar(verificacion(), <VerificarCorreo />, "/registro/verificar");
        escribir("Código de verificación", "112233");
        fireEvent.click(screen.getByRole("button", { name: "Verificar y activar" }));
        await screen.findByText("ok");
        expect(espia.mock.calls.flat().join(" ")).not.toContain("112233");
        espia.mockRestore();
    });
});
