import { useAuth } from "../../hooks/useAuth";
import DashboardLayout from "../dashboard/DashboardLayout";
import PortalLayout from "./PortalLayout";

// Elige el layout según el rol, para páginas que comparten contenido entre
// personal interno y SOLICITANTE (Mis expedientes, detalle, FUT, Documentos):
// un SOLICITANTE nunca debe ver el sidebar azul administrativo, así que en su
// caso se usa PortalLayout en vez de DashboardLayout. Para cualquier otro rol
// el comportamiento es IDÉNTICO al de antes (DashboardLayout sin cambios).
function RoleLayout({ title, children }) {
    const { user } = useAuth();

    if (user?.role === "SOLICITANTE") {
        return <PortalLayout title={title}>{children}</PortalLayout>;
    }

    return <DashboardLayout title={title}>{children}</DashboardLayout>;
}

export default RoleLayout;
