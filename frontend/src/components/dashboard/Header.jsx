import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../../hooks/useAuth";
import { usePermissions } from "../../hooks/usePermissions";
import Icon from "./Icon";
import NotificationButton from "./NotificationButton";
import UserMenu from "./UserMenu";
import "./Header.css";

// Buscador global de estudiantes: solo con estudiantes.view. Abre el
// directorio con la búsqueda (?q=), que resuelve el backend por código,
// SIAGIE, documento, nombres o apellidos.
function BuscadorEstudiantes() {
    const navigate = useNavigate();
    const [q, setQ] = useState("");
    return (
        <form
            className="dp-header-search"
            role="search"
            onSubmit={(event) => {
                event.preventDefault();
                const texto = q.trim();
                if (texto.length < 2) return;
                navigate(`/estudiantes?q=${encodeURIComponent(texto)}`);
            }}
        >
            <Icon name="search" size={16} />
            <input
                type="search"
                aria-label="Buscar estudiante"
                placeholder="Buscar estudiante por código, DNI o nombre"
                maxLength={100}
                value={q}
                onChange={(event) => setQ(event.target.value)}
            />
        </form>
    );
}

function Header({ title, onToggleSidebar }) {
    const { user } = useAuth();
    const { can } = usePermissions();

    return (
        <header className="dp-header">
            <button
                type="button"
                className="dp-header-menu-btn"
                onClick={onToggleSidebar}
                aria-label="Abrir menú"
            >
                <Icon name="menu" size={20} />
            </button>

            <div className="dp-header-titles">
                <h1>{title}</h1>
                <p>
                    Bienvenido, <strong>{user?.username}</strong>
                </p>
            </div>

            <div className="dp-header-actions">
                {can("estudiantes.view") && <BuscadorEstudiantes />}
                <NotificationButton />
                <UserMenu />
            </div>
        </header>
    );
}

export default Header;
