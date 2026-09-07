import { useAuth } from "../../hooks/useAuth";
import Icon from "./Icon";
import NotificationButton from "./NotificationButton";
import UserMenu from "./UserMenu";
import "./Header.css";

function Header({ title, onToggleSidebar }) {
    const { user } = useAuth();

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
                <NotificationButton />
                <UserMenu />
            </div>
        </header>
    );
}

export default Header;
