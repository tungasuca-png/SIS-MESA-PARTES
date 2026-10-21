import { Link } from "react-router-dom";
import { usePermissions } from "../../hooks/usePermissions";
import { NAV_GROUPS } from "./navItems";
import Icon from "./Icon";
import "./ModuleCards.css";

// Accesos a los módulos del Dashboard ADMINISTRATIVO, en un panel compacto.
// Toma los módulos de NAV_GROUPS (la misma fuente que el Sidebar), respeta
// los permisos reales (usePermissions) y enlaza solo a rutas que existen:
// los módulos sin ruta todavía (path: null) no se listan aquí, ya figuran
// como "Próximamente" en el Sidebar. Las bandejas por estado tampoco: el
// Dashboard ya las enlaza desde sus indicadores.
const SECTIONS = ["mesa-de-partes", "psicologia", "estudiantes", "reportes", "administracion"];

function useModulosDisponibles() {
    const { can } = usePermissions();
    return SECTIONS.map((id) => NAV_GROUPS.find((group) => group.id === id))
        .filter(Boolean)
        .flatMap((group) =>
            group.items
                .filter((item) => item.path && can(item.permission))
                .map((item) => ({ ...item, grupo: group.label }))
        );
}

function ModuleCards({ title = "Accesos" }) {
    const modulos = useModulosDisponibles();
    if (modulos.length === 0) return null;

    return (
        <section className="dp-panel dp-modules" aria-labelledby="dp-modules-title">
            <h2 id="dp-modules-title" className="dp-panel-title">
                {title}
            </h2>
            <ul className="dp-module-list">
                {modulos.map((item) => (
                    <li key={item.key}>
                        <Link to={item.path} className="dp-module-link">
                            <span className="dp-module-icon">
                                <Icon name={item.icon} size={16} />
                            </span>
                            <span className="dp-module-body">
                                <span className="dp-module-name">{item.label}</span>
                                <span className="dp-module-group">{item.grupo}</span>
                            </span>
                            <Icon name="chevronDown" size={14} className="dp-module-arrow" />
                        </Link>
                    </li>
                ))}
            </ul>
        </section>
    );
}

export default ModuleCards;
