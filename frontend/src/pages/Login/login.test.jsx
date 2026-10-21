import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { AuthContext } from "../../context/auth-context";
import Login from "./login";

afterEach(cleanup);

function renderLogin() {
    const auth = { user: null, permissions: [], loading: false, isAuthenticated: false, login: vi.fn() };
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
