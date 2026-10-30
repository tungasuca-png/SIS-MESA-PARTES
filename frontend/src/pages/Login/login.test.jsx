import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { httpError } from "../../test/estudiantesTestUtils";
import { MemoryRouter } from "react-router-dom";
import { AuthContext } from "../../context/auth-context";
import Login from "./login";

afterEach(cleanup);

function renderLogin(login = vi.fn()) {
    const auth = { user: null, permissions: [], loading: false, isAuthenticated: false, login };
    return render(
        <AuthContext.Provider value={auth}>
            <MemoryRouter>
                <Login />
            </MemoryRouter>
        </AuthContext.Provider>
    );
}

describe("Login — mostrar contraseña", () => {
    it("la clave empieza oculta y el botón la muestra y la vuelve a ocultar", () => {
        renderLogin();
        const clave = screen.getByLabelText("Clave");
        fireEvent.change(clave, { target: { value: "secreta" } });
        expect(clave.getAttribute("type")).toBe("password");

        const boton = screen.getByRole("button", { name: "Mostrar contraseña" });
        expect(boton.getAttribute("type")).toBe("button"); // no envía el formulario
        fireEvent.click(boton);
        expect(clave.getAttribute("type")).toBe("text");
        expect(clave.value).toBe("secreta");

        const ocultar = screen.getByRole("button", { name: "Ocultar contraseña" });
        expect(ocultar.getAttribute("aria-pressed")).toBe("true");
        fireEvent.click(ocultar);
        expect(clave.getAttribute("type")).toBe("password");
    });
});

describe("Login — mensajes de error", () => {
    const intentar = async (error) => {
        renderLogin(vi.fn().mockRejectedValue(error));
        fireEvent.change(screen.getByLabelText("DNI"), { target: { value: "12345678" } });
        fireEvent.change(screen.getByLabelText("Clave"), { target: { value: "x" } });
        fireEvent.click(screen.getByRole("button", { name: /ingresar/i }));
    };

    it("cuenta bloqueada (429): lo dice con los minutos que faltan", async () => {
        await intentar(httpError(429, "account_locked: cuenta bloqueada temporalmente por demasiados intentos fallidos; intenta de nuevo en 12 minutos"));
        expect(await screen.findByText("Cuenta bloqueada temporalmente por demasiados intentos fallidos. Intenta de nuevo en 12 minutos.")).toBeTruthy();
    });

    it("credenciales incorrectas (401): mensaje genérico", async () => {
        await intentar(httpError(401, "credenciales inválidas"));
        expect(await screen.findByText("DNI o clave incorrectos")).toBeTruthy();
    });

    it("servicio caído (503): no se presenta como clave incorrecta", async () => {
        await intentar(httpError(503, "servicio no disponible temporalmente"));
        expect(await screen.findByText("El servicio no está disponible en este momento. Inténtalo nuevamente.")).toBeTruthy();
    });
});
