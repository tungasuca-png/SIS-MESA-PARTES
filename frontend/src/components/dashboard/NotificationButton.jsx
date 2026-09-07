import { useEffect, useRef, useState } from "react";
import { mockNotifications } from "../../data/mockDashboardData";
import Icon from "./Icon";
import "./NotificationButton.css";

function NotificationButton() {
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
        <div className="dp-notif" ref={containerRef}>
            <button
                type="button"
                className="dp-notif-trigger"
                onClick={() => setOpen((value) => !value)}
                aria-label="Notificaciones"
            >
                <Icon name="bell" size={19} />
                {mockNotifications.length > 0 && (
                    <span className="dp-notif-badge">{mockNotifications.length}</span>
                )}
            </button>

            {open && (
                <div className="dp-notif-dropdown">
                    <p className="dp-notif-title">Notificaciones</p>
                    {mockNotifications.map((notification) => (
                        <div key={notification.id} className="dp-notif-item">
                            <p>{notification.text}</p>
                            <span>{notification.time}</span>
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
}

export default NotificationButton;
