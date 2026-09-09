import { useNavigate } from "react-router-dom";
import { usePermissions } from "../../hooks/usePermissions";
import Icon from "./Icon";
import "./QuickActions.css";

// Acciones rápidas del Dashboard ADMINISTRATIVO. Un SOLICITANTE nunca
// renderiza este componente (Dashboard.jsx lo redirige antes a su propio
// portal, /mesa-de-partes, que tiene sus propias tarjetas de acción) — por
// eso esta lista ya no necesita distinguir su rol.
//
// "to: null" = módulo todavía no implementado (permanece visual/inerte).
const ACTIONS = [
    {
        key: "nuevo-expediente",
        label: "Nuevo expediente",
        icon: "plus",
        permission: "expedientes.create",
        to: "/expedientes",
        state: { openCreate: true },
    },
    // No hay una accion "Registrar documento" independiente: un documento
    // siempre pertenece a un expediente (no existen documentos huerfanos), asi
    // que subir uno se hace entrando al expediente concreto ("Ver
    // expedientes" -> detalle -> panel de Documentos), no desde aqui.
    {
        key: "ver-expedientes",
        label: "Ver expedientes",
        icon: "folder",
        permission: "expedientes.view",
        to: "/expedientes",
    },
    {
        key: "ver-seguimiento",
        label: "Ver seguimiento",
        icon: "trending",
        permission: "seguimiento.view",
        to: null,
    },
];

function QuickActions() {
    const { can } = usePermissions();
    const navigate = useNavigate();
    const actions = ACTIONS.filter((action) => can(action.permission));

    if (actions.length === 0) return null;

    return (
        <section className="dp-panel dp-quick-actions">
            <h2 className="dp-panel-title">Acciones rápidas</h2>

            <div className="dp-quick-actions-grid">
                {actions.map((action) => (
                    <button
                        key={action.key}
                        type="button"
                        className="dp-quick-action"
                        aria-disabled={!action.to}
                        title={action.to ? undefined : "Disponible cuando se conecte el módulo correspondiente"}
                        onClick={
                            action.to
                                ? () => navigate(action.to, action.state ? { state: action.state } : undefined)
                                : undefined
                        }
                    >
                        <span className="dp-quick-action-icon">
                            <Icon name={action.icon} size={19} />
                        </span>
                        {action.label}
                    </button>
                ))}
            </div>
        </section>
    );
}

export default QuickActions;
