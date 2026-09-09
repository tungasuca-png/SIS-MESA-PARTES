import { useEffect, useState } from "react";
import { Navigate } from "react-router-dom";
import DashboardLayout from "../../components/dashboard/DashboardLayout";
import StatCard from "../../components/dashboard/StatCard";
import RecentExpedientes from "../../components/dashboard/RecentExpedientes";
import QuickActions from "../../components/dashboard/QuickActions";
import "../../components/dashboard/forms.css";
import { useAuth } from "../../hooks/useAuth";
import { usePermissions } from "../../hooks/usePermissions";
import { getExpedientes } from "../../services/expedientesService";
import { friendlyErrorMessage } from "../../utils/apiErrors";
import "./dashboard.css";

// Este es el Dashboard ADMINISTRATIVO: personal interno (ADMIN, DIRECTOR,
// SUBDIRECTOR, SECRETARIA, DOCENTE, AUXILIAR). Un SOLICITANTE nunca debe
// entrar aquí — tiene su propio portal sin sidebar administrativo (ver
// PortalLayout / /mesa-de-partes) — así que se lo redirige antes de cargar
// nada de este componente.
//
// Cada tarjeta pide el total real (COUNT del backend) filtrado por estado,
// con page_size=1 porque solo se necesita el campo "total" de la respuesta,
// no la lista. No se calcula ninguna variación (%) porque el backend no
// entrega datos históricos: mostrarla sería inventar un número.
const STAT_DEFS = [
    { key: "recibidos", label: "Expedientes recibidos", icon: "inbox", estado: undefined },
    { key: "pendientes", label: "Pendientes", icon: "clock", estado: "PENDIENTE" },
    { key: "proceso", label: "En proceso", icon: "loader", estado: "EN_PROCESO" },
    { key: "atendidos", label: "Atendidos", icon: "check", estado: "ATENDIDO" },
];

function Dashboard() {
    const { user } = useAuth();
    const { can } = usePermissions();
    const [stats, setStats] = useState([]);
    const [recientes, setRecientes] = useState([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");

    useEffect(() => {
        let active = true;

        async function load() {
            setLoading(true);
            setError("");
            try {
                const [statResults, recentesResp] = await Promise.all([
                    Promise.all(
                        STAT_DEFS.map((def) => getExpedientes({ estado: def.estado, page: 1, pageSize: 1 }))
                    ),
                    getExpedientes({ page: 1, pageSize: 5 }),
                ]);

                if (!active) return;

                setStats(
                    STAT_DEFS.map((def, index) => ({
                        key: def.key,
                        label: def.label,
                        icon: def.icon,
                        value: statResults[index].total ?? 0,
                    }))
                );
                setRecientes(recentesResp.expedientes || []);
            } catch (err) {
                if (active) setError(friendlyErrorMessage(err));
            } finally {
                if (active) setLoading(false);
            }
        }

        load();
        return () => {
            active = false;
        };
    }, []);

    if (user?.role === "SOLICITANTE") {
        return <Navigate to="/mesa-de-partes" replace />;
    }

    const emptyMessage = can("expedientes.view")
        ? "No hay expedientes registrados."
        : "Aún no tienes expedientes registrados.";

    return (
        <DashboardLayout title="Dashboard">
            {loading && <p className="dp-table-empty">Cargando información...</p>}

            {!loading && error && <p className="dp-form-error">{error}</p>}

            {!loading && !error && (
                <>
                    <div className="dp-stats-grid">
                        {stats.map((stat) => (
                            <StatCard key={stat.key} {...stat} />
                        ))}
                    </div>

                    <div className="dp-dashboard-sections">
                        <RecentExpedientes items={recientes} emptyMessage={emptyMessage} />
                        <QuickActions />
                    </div>
                </>
            )}
        </DashboardLayout>
    );
}

export default Dashboard;
