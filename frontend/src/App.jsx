import { Routes, Route, Navigate } from "react-router-dom";

import Login from "./pages/Login/login";
import Dashboard from "./pages/Dashboard/dashboard";
import ExpedientesList from "./pages/Expedientes/ExpedientesList";
import ExpedienteDetail from "./pages/Expedientes/ExpedienteDetail";
import MisDocumentos from "./pages/Documentos/MisDocumentos";
import FutDigital from "./pages/FUT/FutDigital";
import ProtectedRoute from "./components/ProtectedRoute";

function App() {
    return (
        <Routes>

            {/* Login */}
            <Route
                path="/login"
                element={<Login />}
            />

            {/* Dashboard protegido */}
            <Route
                path="/dashboard"
                element={
                    <ProtectedRoute>
                        <Dashboard />
                    </ProtectedRoute>
                }
            />

            {/* Expedientes (Expedientes Service real) */}
            <Route
                path="/expedientes"
                element={
                    <ProtectedRoute>
                        <ExpedientesList />
                    </ProtectedRoute>
                }
            />
            <Route
                path="/expedientes/:id"
                element={
                    <ProtectedRoute>
                        <ExpedienteDetail />
                    </ProtectedRoute>
                }
            />

            {/* "Mis documentos": listado propio del SOLICITANTE (el backend ya
                acota a lo que el usuario subió). El personal interno sigue
                viendo/subiendo documentos desde el detalle del expediente
                (panel de Documentos), no desde acá. */}
            <Route
                path="/documentos"
                element={
                    <ProtectedRoute>
                        <MisDocumentos />
                    </ProtectedRoute>
                }
            />

            {/* FUT Digital (crea un Expediente real vía Expedientes Service) */}
            <Route
                path="/fut"
                element={
                    <ProtectedRoute>
                        <FutDigital />
                    </ProtectedRoute>
                }
            />

            {/* Cualquier ruta desconocida */}
            <Route
                path="*"
                element={<Navigate to="/login" replace />}
            />

        </Routes>
    );
}

export default App;
