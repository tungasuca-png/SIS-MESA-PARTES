import { render } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { AuthContext } from "../context/auth-context";
import PermissionRoute from "../components/PermissionRoute";
import EstudiantesList from "../pages/Estudiantes/EstudiantesList";
import EstudianteDetail from "../pages/Estudiantes/EstudianteDetail";

// Pruebas del módulo Estudiantes (FASE E-06). Monta las rutas reales
// (/estudiantes y /estudiantes/:id con el mismo PermissionRoute que App.jsx)
// con un AuthContext de prueba: los permisos se indican por prueba, igual
// que los devolvería Auth Service. Los servicios HTTP se simulan con vi.mock
// en cada archivo de prueba.
export function renderEstudiantes(path, { permissions = ["estudiantes.view"] } = {}) {
    const auth = {
        user: { id: "u-1", username: "tester", role: "ADMIN" },
        permissions,
        loading: false,
        isAuthenticated: true,
    };
    return render(
        <AuthContext.Provider value={auth}>
            <MemoryRouter initialEntries={[path]}>
                <Routes>
                    <Route
                        path="/estudiantes"
                        element={
                            <PermissionRoute permission="estudiantes.view">
                                <EstudiantesList />
                            </PermissionRoute>
                        }
                    />
                    <Route
                        path="/estudiantes/:id"
                        element={
                            <PermissionRoute permission="estudiantes.view">
                                <EstudianteDetail />
                            </PermissionRoute>
                        }
                    />
                </Routes>
            </MemoryRouter>
        </AuthContext.Provider>
    );
}

// Error como lo entrega axios cuando el Gateway responde con el texto plano
// de go-zero ("rpc error: code = ... desc = <codigo>: <texto>").
export function httpError(status, desc) {
    const error = new Error(`HTTP ${status}`);
    error.response = { status, data: desc ? `rpc error: code = X desc = ${desc}\n` : "" };
    return error;
}

export const TODOS_LOS_PERMISOS = [
    "estudiantes.view",
    "estudiantes.create",
    "estudiantes.update",
    "estudiantes.change_estado",
];

export function estudianteBasico(overrides = {}) {
    return {
        id: "11111111-1111-4111-8111-111111111111",
        codigo: "EST-001",
        nombre_completo: "Ana Lucía Pérez Soto",
        nombres: "Ana Lucía",
        apellidos: "Pérez Soto",
        apellido_paterno: "Pérez",
        apellido_materno: "Soto",
        codigo_siagie: "",
        tipo_documento: "DNI",
        numero_documento: "12345678",
        estado: "ACTIVO",
        nivel: "PRIMARIA",
        grado: "3",
        seccion: "A",
        turno: "MAÑANA",
        estado_matricula: "ACTIVO",
        version: 4,
        ...overrides,
    };
}
