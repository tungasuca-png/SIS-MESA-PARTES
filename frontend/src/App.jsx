import { Routes, Route, Navigate } from "react-router-dom";

import Login from "./pages/Login/login";
import Dashboard from "./pages/Dashboard/dashboard";
import ExpedientesList from "./pages/Expedientes/ExpedientesList";
import ExpedienteDetail from "./pages/Expedientes/ExpedienteDetail";
import MisDocumentos from "./pages/Documentos/MisDocumentos";
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

            {/* Portal del SOLICITANTE — Mesa de Partes Virtual. Sin sidebar
                azul administrativo (ver PortalLayout). "Registrar solicitud",
                "Mis expedientes" y "Documentos" reutilizan /fut, /expedientes
                y /documentos (arriba); acá solo van las páginas que no
                existían antes: el inicio del portal, seguimiento y perfil. */}
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
