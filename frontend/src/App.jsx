import { Routes, Route, Navigate } from "react-router-dom";

import Login from "./pages/Login/login";
import Dashboard from "./pages/Dashboard/dashboard";
import ExpedientesList from "./pages/Expedientes/ExpedientesList";
import ExpedienteDetail from "./pages/Expedientes/ExpedienteDetail";
import FutDigital from "./pages/FUT/FutDigital";
import MesaPartesHome from "./pages/MesaDePartes/MesaPartesHome";
import Seguimiento from "./pages/MesaDePartes/Seguimiento";
import SeguimientoDetalle from "./pages/MesaDePartes/SeguimientoDetalle";
import MiPerfil from "./pages/MesaDePartes/MiPerfil";
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

            {/* No hay ruta "/documentos" aparte: sus documentos se consultan
                desde el detalle de cada expediente (panel de Documentos en
                ExpedienteDetail), tanto para personal interno como para el
                SOLICITANTE. */}

            {/* FUT Digital (crea un Expediente real vía Expedientes Service) */}
            <Route
                path="/fut"
                element={
                    <ProtectedRoute>
                        <FutDigital />
                    </ProtectedRoute>
                }
            />

            {/* Portal del SOLICITANTE — Mesa de Partes Virtual. Sin sidebar
                azul administrativo (ver PortalLayout). "Registrar solicitud"
                y "Mis expedientes" reutilizan /fut y /expedientes (arriba);
                acá solo van las páginas que no existían antes: el inicio del
                portal, seguimiento y perfil. */}
            <Route
                path="/mesa-de-partes"
                element={
                    <ProtectedRoute>
                        <MesaPartesHome />
                    </ProtectedRoute>
                }
            />
            <Route
                path="/mesa-de-partes/seguimiento"
                element={
                    <ProtectedRoute>
                        <Seguimiento />
                    </ProtectedRoute>
                }
            />
            <Route
                path="/mesa-de-partes/seguimiento/:id"
                element={
                    <ProtectedRoute>
                        <SeguimientoDetalle />
                    </ProtectedRoute>
                }
            />
            <Route
                path="/mesa-de-partes/perfil"
                element={
                    <ProtectedRoute>
                        <MiPerfil />
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
