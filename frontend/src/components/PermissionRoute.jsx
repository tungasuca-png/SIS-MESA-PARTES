import { useAuth } from "../hooks/useAuth";
import { usePermissions } from "../hooks/usePermissions";
import RoleLayout from "./portal/RoleLayout";
import "./dashboard/RecentExpedientes.css";

// Guard de RUTA por permiso real (Auth Service, vía usePermissions/
// AuthContext) — separado de ProtectedRoute a propósito: ProtectedRoute
// solo responde "¿hay sesión?", este componente responde "¿tiene el
// permiso?". Bloquea el render de la página protegida ANTES de que monte
// (y antes de que sus useEffect disparen peticiones innecesarias) cuando
// el permiso falta.
//
// `permission` (string) o `permissions` (array, semántica OR — basta con
// tener uno) delegan en can(), que ya soporta ambas formas. Nunca decide
// por user.role ni por una matriz local: el backend sigue siendo la única autoridad
// real, esto es solo UX de navegación.
function PermissionRoute({ permission, permissions, children }) {
    const { loading } = useAuth();
    const { can } = usePermissions();

    if (loading) {
        return <div>Cargando...</div>;
    }

    if (!can(permissions ?? permission)) {
        return (
            <RoleLayout title="Acceso denegado">
                <section className="dp-panel" style={{ textAlign: "center", padding: "40px 20px" }}>
                    <h2 className="dp-panel-title">Acceso denegado</h2>
                    <p>El usuario no tiene el permiso requerido.</p>
                </section>
            </RoleLayout>
        );
    }

    return children;
}

export default PermissionRoute;
