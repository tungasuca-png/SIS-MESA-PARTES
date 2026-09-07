import { useEffect, useRef, useState } from "react";
import { useAuth } from "../../hooks/useAuth";
import Icon from "./Icon";
import "./UserMenu.css";

function initials(name) {
    if (!name) return "?";
    return name.slice(0, 2).toUpperCase();
}

function UserMenu() {
    const { user, logout } = useAuth();
    const [open, setOpen] = useState(false);
    const containerRef = useRef(null);

    useEffect(() => {
        function handleClickOutside(event) {
            if (containerRef.current && !containerRef.current.contains(event.target)) {
                setOpen(false);
            }
        }

        document.addEventListener("mousedown", handleClickOutside);
        return () => document.removeEventListener("mousedown", handleClickOutside);
    }, []);

    return (
        <div className="dp-user-menu" ref={containerRef}>
            <button type="button" className="dp-user-trigger" onClick={() => setOpen((value) => !value)}>
                <span className="dp-avatar">{initials(user?.username)}</span>
                <span className="dp-user-info">
                    <span className="dp-user-name">{user?.username}</span>
                    <span className="dp-user-role">{user?.role}</span>
                </span>
                <Icon name="chevronDown" size={16} className="dp-user-chevron" />
            </button>

            {open && (
                <div className="dp-user-dropdown">
                    <p className="dp-user-dropdown-email">{user?.email}</p>
                    <button type="button" className="dp-user-dropdown-logout" onClick={logout}>
                        <Icon name="logout" size={16} />
                        Cerrar sesión
                    </button>
                </div>
            )}
        </div>
    );
}

export default UserMenu;
