import { Routes, Route, Navigate } from "react-router-dom";

import Login from "./pages/Login/login";
import Dashboard from "./pages/Dashboard/dashboard";
import ExpedientesList from "./pages/Expedientes/ExpedientesList";
import ExpedientesDerivados from "./pages/Expedientes/ExpedientesDerivados";
import ExpedienteDetail from "./pages/Expedientes/ExpedienteDetail";
import FutDigital from "./pages/FUT/FutDigital";
import MesaPartesHome from "./pages/MesaDePartes/MesaPartesHome";
import Seguimiento from "./pages/MesaDePartes/Seguimiento";
import SeguimientoDetalle from "./pages/MesaDePartes/SeguimientoDetalle";
import MiPerfil from "./pages/MesaDePartes/MiPerfil";
import Notificaciones from "./pages/MesaDePartes/Notificaciones";
import RolesPermisos from "./pages/Configuracion/RolesPermisos/RolesPermisos";
import Usuarios from "./pages/Configuracion/Usuarios/Usuarios";
import PsicologiaExpedientesList from "./pages/Psicologia/PsicologiaExpedientesList";
import PsicologiaExpedienteDetail from "./pages/Psicologia/PsicologiaExpedienteDetail";
import PsicologiaAtencion from "./pages/Psicologia/PsicologiaAtencion";
import EstudiantesList from "./pages/Estudiantes/EstudiantesList";
import EstudianteDetail from "./pages/Estudiantes/EstudianteDetail";
import ProtectedRoute from "./components/ProtectedRoute";
import PermissionRoute from "./components/PermissionRoute";

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
                        <PermissionRoute permission="dashboard.view">
                            <Dashboard />
                        </PermissionRoute>
                    </ProtectedRoute>
                }
            />

            {/* Expedientes (Expedientes Service real). "view_own" cubre al
                SOLICITANTE — el guard solo exige UNO de los dos, el backend
                sigue acotando qué expediente puntual puede ver cada cual. */}
            <Route
                path="/expedientes"
                element={
                    <ProtectedRoute>
                        <PermissionRoute permissions={["expedientes.view", "expedientes.view_own"]}>
                            <ExpedientesList />
                        </PermissionRoute>
                    </ProtectedRoute>
                }
            />
            {/* "Derivados" (bandeja, ver constants/bandejas.js): ruta propia
                porque va antes que ":id" — no es un expediente puntual. Es
                una bandeja administrativa (no aplica a SOLICITANTE), por eso
                exige expedientes.view sin la variante "_own". */}
            <Route
                path="/expedientes/derivados"
                element={
                    <ProtectedRoute>
                        <PermissionRoute permission="expedientes.view">
                            <ExpedientesDerivados />
                        </PermissionRoute>
                    </ProtectedRoute>
                }
            />
            <Route
                path="/expedientes/:id"
                element={
                    <ProtectedRoute>
                        <PermissionRoute permissions={["expedientes.view", "expedientes.view_own"]}>
                            <ExpedienteDetail />
                        </PermissionRoute>
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
                        <PermissionRoute permission="solicitudes.create">
                            <FutDigital />
                        </PermissionRoute>
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
            {/* Seguimiento (Paso 8): usa exactamente las mismas capacidades de
                backend que /expedientes (GetExpediente/ListExpedientes), así
                que se migra al mismo permiso — ya no depende de user.role. */}
            <Route
                path="/mesa-de-partes/seguimiento"
                element={
                    <ProtectedRoute>
                        <PermissionRoute permissions={["expedientes.view", "expedientes.view_own"]}>
                            <Seguimiento />
                        </PermissionRoute>
                    </ProtectedRoute>
                }
            />
            <Route
                path="/mesa-de-partes/seguimiento/:id"
                element={
                    <ProtectedRoute>
                        <PermissionRoute permissions={["expedientes.view", "expedientes.view_own"]}>
                            <SeguimientoDetalle />
                        </PermissionRoute>
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
            <Route
                path="/mesa-de-partes/notificaciones"
                element={
                    <ProtectedRoute>
                        <Notificaciones />
                    </ProtectedRoute>
                }
            />

            {/* Psicología → Expedientes (módulo real, backend en
                backend/services/psicologia). El scope real (view = alcance
                general, view_own = solo expedientes propios) lo decide el
                backend; el guard de ruta solo exige UNO de los dos, igual
                que /expedientes. */}
            <Route
                path="/psicologia/expedientes"
                element={
                    <ProtectedRoute>
                        <PermissionRoute permissions={["psicologia.expedientes.view", "psicologia.expedientes.view_own"]}>
                            <PsicologiaExpedientesList />
                        </PermissionRoute>
                    </ProtectedRoute>
                }
            />
            <Route
                path="/psicologia/expedientes/:id"
                element={
                    <ProtectedRoute>
                        <PermissionRoute permissions={["psicologia.expedientes.view", "psicologia.expedientes.view_own"]}>
                            <PsicologiaExpedienteDetail />
                        </PermissionRoute>
                    </ProtectedRoute>
                }
            />
            {/* Psicología → Expediente → Atención (Hoja de Atención). El
                backend exige además el alcance del expediente; el guard solo
                evita montar la pantalla sin el permiso. */}
            <Route
                path="/psicologia/expedientes/:expedienteId/atenciones/nueva"
                element={
                    <ProtectedRoute>
                        <PermissionRoute permission="psicologia.atenciones.create">
                            <PsicologiaAtencion nueva />
                        </PermissionRoute>
                    </ProtectedRoute>
                }
            />
            <Route
                path="/psicologia/atenciones/:id"
                element={
                    <ProtectedRoute>
                        <PermissionRoute permission="psicologia.atenciones.view">
                            <PsicologiaAtencion />
                        </PermissionRoute>
                    </ProtectedRoute>
                }
            />

            {/* Estudiantes (FASE E-06): directorio del maestro institucional.
                "estudiantes.view" para entrar; crear/editar/cambiar estado se
                muestran solo con estudiantes.create/.update/.change_estado
                (usePermissions dentro de las páginas). El backend sigue siendo
                la autoridad en cada operación. */}
            <Route
                path="/estudiantes"
                element={
                    <ProtectedRoute>
                        <PermissionRoute permission="estudiantes.view">
                            <EstudiantesList />
                        </PermissionRoute>
                    </ProtectedRoute>
                }
            />
            <Route
                path="/estudiantes/:id"
                element={
                    <ProtectedRoute>
                        <PermissionRoute permission="estudiantes.view">
                            <EstudianteDetail />
                        </PermissionRoute>
                    </ProtectedRoute>
                }
            />

            {/* Configuración → Roles y permisos (Paso 21E). El backend
                (Auth Service, vía Gateway) sigue siendo la única autoridad
                real sobre "roles.view" — PermissionRoute (Paso 6) es solo
                UX de navegación: evita montar la página cuando el permiso
                ya se sabe ausente, pero RolesPermisos.jsx conserva su
                propio manejo de 403 (accessDenied) para las operaciones
                que ejecuta después de montarse. */}
            <Route
                path="/configuracion/roles-permisos"
                element={
                    <ProtectedRoute>
                        <PermissionRoute permission="roles.view">
                            <RolesPermisos />
                        </PermissionRoute>
                    </ProtectedRoute>
                }
            />

            {/* Configuración → Usuarios (Usuario -> Rol). "usuarios.view"
                para entrar a la pantalla; "usuarios.change_role" (chequeado
                dentro de Usuarios.jsx via usePermissions) para poder editar
                el rol — quien solo tiene "usuarios.view" ve la lista pero sin
                selector ni botón de guardar. */}
            <Route
                path="/configuracion/usuarios"
                element={
                    <ProtectedRoute>
                        <PermissionRoute permission="usuarios.view">
                            <Usuarios />
                        </PermissionRoute>
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
