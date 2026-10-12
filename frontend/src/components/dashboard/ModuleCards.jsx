import { Link } from "react-router-dom";
import { usePermissions } from "../../hooks/usePermissions";
import { NAV_GROUPS } from "./navItems";
import Icon from "./Icon";
import "./ModuleCards.css";

// Vista general de acceso a los módulos del Dashboard ADMINISTRATIVO. Toma
// los módulos de NAV_GROUPS (la misma fuente que el Sidebar): enlaza solo a
// rutas que ya existen y respeta los permisos reales (usePermissions). Un
// módulo sin ruta todavía (path: null) se muestra inerte, igual que en el
// Sidebar. Una sección sin ningún módulo visible no se dibuja.
const SECTIONS = ["mesa-de-partes", "psicologia", "administracion", "reportes"];

function ModuleCard({ item }) {
    const content = (
        <>
            <span className="dp-module-icon">
                <Icon name={item.icon} size={20} />
            </span>
            <span className="dp-module-body">
                <span className="dp-module-name">{item.label}</span>
                {item.description && <span className="dp-module-desc">{item.description}</span>}
            </span>
            {item.path ? (
                <span className="dp-module-action">Acceder →</span>
            ) : (
                <span className="dp-module-soon">Próximamente</span>
            )}
        </>
    );

    return item.path ? (
        <Link to={item.path} className="dp-module-card">
            {content}
        </Link>
    ) : (
        <div className="dp-module-card dp-module-card--soon" aria-disabled="true">
            {content}
        </div>
    );
}

function ModuleCards() {
    const { can } = usePermissions();

    const sections = SECTIONS.map((id) => NAV_GROUPS.find((group) => group.id === id))
        .filter(Boolean)
        .map((group) => ({ ...group, items: group.items.filter((item) => can(item.permission)) }))
        .filter((group) => group.items.length > 0);

    if (sections.length === 0) return null;

    return (
        <div className="dp-modules">
            {sections.map((section) => (
                <section key={section.id} className="dp-module-section" aria-labelledby={`dp-mod-${section.id}`}>
                    <h2 id={`dp-mod-${section.id}`} className="dp-module-section-title">
                        {section.label}
                    </h2>
                    <div className="dp-module-grid">
                        {section.items.map((item) => (
                            <ModuleCard key={item.key} item={item} />
                        ))}
                    </div>
                </section>
            ))}
        </div>
    );
}

export default ModuleCards;
