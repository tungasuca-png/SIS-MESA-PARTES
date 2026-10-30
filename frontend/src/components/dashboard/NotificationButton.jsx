import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { useNotificacionesTramites } from "../../hooks/useNotificacionesTramites";
import { tipoLabel } from "../../constants/expedientes";
import { formatDateTime } from "../../utils/format";
import Icon from "./Icon";
import "./NotificationButton.css";

// Campana del encabezado: trámites nuevos por revisar (FUT y solicitudes),
// con datos reales de Expedientes Service (ver useNotificacionesTramites).
function NotificationButton() {
    const [open, setOpen] = useState(false);
    const containerRef = useRef(null);
    const { habilitado, cargado, error, tramites, totalPendientes, nuevos, marcarLeidas } = useNotificacionesTramites();

    useEffect(() => {
        function handleClickOutside(event) {
            if (containerRef.current && !containerRef.current.contains(event.target)) {
                setOpen(false);
            }
        }

        document.addEventListener("mousedown", handleClickOutside);
        return () => document.removeEventListener("mousedown", handleClickOutside);
    }, []);

    const idsNuevos = new Set(nuevos.map((t) => t.id));
    const cantidad = nuevos.length;

    return (
        <div className="dp-notif" ref={containerRef}>
            <button
                type="button"
                className="dp-notif-trigger"
                onClick={() => setOpen((value) => !value)}
                aria-label={cantidad > 0 ? `Notificaciones: ${cantidad} trámites nuevos` : "Notificaciones"}
                aria-expanded={open}
            >
                <Icon name="bell" size={19} />
                {cantidad > 0 && <span className="dp-notif-badge">{cantidad > 9 ? "9+" : cantidad}</span>}
            </button>

            {open && (
                <div className="dp-notif-dropdown" role="dialog" aria-label="Notificaciones">
                    <div className="dp-notif-head">
                        <p className="dp-notif-title">Notificaciones</p>
                        {cantidad > 0 && (
                            <button type="button" className="dp-notif-mark" onClick={marcarLeidas}>
                                Marcar como leídas
                            </button>
                        )}
                    </div>

                    {!habilitado && <p className="dp-notif-empty">No tienes notificaciones.</p>}
                    {habilitado && !cargado && <p className="dp-notif-empty">Cargando...</p>}
                    {habilitado && cargado && error && tramites.length === 0 && (
                        <p className="dp-notif-empty">No se pudieron consultar los trámites. Se reintentará automáticamente.</p>
                    )}
                    {habilitado && cargado && !error && tramites.length === 0 && (
                        <p className="dp-notif-empty">No hay trámites por revisar.</p>
                    )}

                    {tramites.length > 0 && (
                        <ul className="dp-notif-list">
                            {tramites.map((t) => (
                                <li key={t.id} className={idsNuevos.has(t.id) ? "dp-notif-item is-new" : "dp-notif-item"}>
                                    <Link to={`/expedientes/${t.id}`} onClick={() => setOpen(false)}>
                                        <p>
                                            {idsNuevos.has(t.id) && <span className="dp-notif-dot" aria-label="Nuevo" />}
                                            <strong>{tipoLabel(t.tipo)}</strong> · {t.codigo}
                                        </p>
                                        <p className="dp-notif-asunto">{t.asunto}</p>
                                        <span>{formatDateTime(t.fecha_registro)}</span>
                                    </Link>
                                </li>
                            ))}
                        </ul>
                    )}

                    {habilitado && totalPendientes > 0 && (
                        <Link className="dp-notif-all" to="/expedientes?estado=PENDIENTE" onClick={() => setOpen(false)}>
                            Ver los {totalPendientes} trámites por revisar
                        </Link>
                    )}
                </div>
            )}
        </div>
    );
}

export default NotificationButton;
