import { useEffect, useState } from "react";
import { getExpedientes } from "../services/expedientesService";
import { useAuth } from "./useAuth";
import { usePermissions } from "./usePermissions";
import { INTERVALO_VIVO_MS, notificarCambioExpedientes } from "./useActualizacionEnVivo";

// Notificaciones de la campana: "Trámite recibido (F2, F4) -> Secretaría"
// (docs/etapa-12-especificacion-funcional.md §17). No existe un servicio de
// notificaciones: se usan datos REALES de Expedientes Service — los trámites
// en estado PENDIENTE (FUT y solicitudes por revisar), con el alcance que el
// backend aplica a "expedientes.view". Sin ese permiso no se consulta nada.
// Se consulta cada INTERVALO_VIVO_MS (10 s) y, si llega un trámite nuevo,
// avisa a las vistas abiertas para que se actualicen sin recargar.
//
// "Nuevo" = registrado después de la última vez que el usuario marcó las
// notificaciones como leídas. Esa marca es la fecha de registro (del
// SERVIDOR) del trámite más reciente visto, no la hora de este equipo, y se
// guarda por usuario en este navegador.
export const INTERVALO_MS = INTERVALO_VIVO_MS;
const LIMITE = 20;

const claveVisto = (userId) => `dp_notif_visto_${userId}`;

function leerVisto(userId) {
    try {
        return localStorage.getItem(claveVisto(userId)) || "";
    } catch {
        return "";
    }
}

function guardarVisto(userId, valor) {
    try {
        localStorage.setItem(claveVisto(userId), valor);
    } catch {
        // almacenamiento no disponible: el contador se reinicia al recargar
    }
}

const tiempo = (iso) => {
    const t = new Date(iso).getTime();
    return Number.isNaN(t) ? 0 : t;
};

export function useNotificacionesTramites() {
    const { user } = useAuth();
    const { can } = usePermissions();
    const userId = user?.id;
    const habilitado = Boolean(userId) && can("expedientes.view");

    const [estado, setEstado] = useState({ cargado: false, tramites: [], total: 0, error: false });
    const [visto, setVisto] = useState(() => (userId ? leerVisto(userId) : ""));

    useEffect(() => {
        if (!habilitado) return undefined;
        let active = true;
        let masRecienteVisto = null;
        const consultar = () =>
            getExpedientes({ estado: "PENDIENTE", page: 1, pageSize: LIMITE })
                .then((data) => {
                    if (!active) return;
                    const tramites = data.expedientes || [];
                    setEstado({ cargado: true, tramites, total: data.total || 0, error: false });
                    // Llegó un trámite nuevo desde la consulta anterior: las
                    // vistas abiertas (bandejas, dashboard) se actualizan ya.
                    const reciente = tramites.reduce((max, t) => Math.max(max, tiempo(t.fecha_registro)), 0);
                    if (masRecienteVisto !== null && reciente > masRecienteVisto) notificarCambioExpedientes();
                    masRecienteVisto = Math.max(masRecienteVisto ?? 0, reciente);
                })
                .catch(() => {
                    // Sin conexión o sin sesión: se conserva lo último conocido.
                    if (active) setEstado((actual) => ({ ...actual, cargado: true, error: true }));
                });
        const alVolver = () => {
            if (document.visibilityState === "visible") consultar();
        };
        consultar();
        const timer = setInterval(alVolver, INTERVALO_MS);
        document.addEventListener("visibilitychange", alVolver);
        return () => {
            active = false;
            clearInterval(timer);
            document.removeEventListener("visibilitychange", alVolver);
        };
    }, [habilitado]);

    const limite = tiempo(visto);
    const nuevos = estado.tramites.filter((t) => tiempo(t.fecha_registro) > limite);

    const marcarLeidas = () => {
        const masReciente = estado.tramites.reduce(
            (max, t) => (tiempo(t.fecha_registro) > tiempo(max) ? t.fecha_registro : max),
            visto
        );
        if (masReciente && userId) {
            guardarVisto(userId, masReciente);
            setVisto(masReciente);
        }
    };

    return {
        habilitado,
        cargado: estado.cargado,
        error: estado.error,
        tramites: estado.tramites,
        totalPendientes: estado.total,
        nuevos,
        marcarLeidas,
    };
}
