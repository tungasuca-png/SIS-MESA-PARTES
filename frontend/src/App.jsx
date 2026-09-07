import { Routes, Route, Navigate } from "react-router-dom";

import Login from "./pages/Login/login";
import Dashboard from "./pages/Dashboard/dashboard";
import ExpedientesList from "./pages/Expedientes/ExpedientesList";
import ExpedienteDetail from "./pages/Expedientes/ExpedienteDetail";
import DocumentosList from "./pages/Documentos/DocumentosList";
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

            {/* Documentos (Documentos Service real) */}
            <Route
                path="/documentos"
                element={
                    <ProtectedRoute>
                        <DocumentosList />
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
