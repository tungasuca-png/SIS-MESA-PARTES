import { useEffect, useRef } from "react";

// Actualización "en vivo" de las vistas de Mesa de Partes sin recargar la
// página. No hay un canal push en el backend (ni WebSocket ni eventos): se
// vuelve a consultar la API real cada pocos segundos mientras la pestaña está
// visible, al volver a ella, y de inmediato cuando otra parte de la interfaz
// (la campana) detecta un trámite nuevo. La consulta la hace quien usa el
// hook, en silencio: sin "Cargando..." ni perder filtros o página.
export const INTERVALO_VIVO_MS = 10000;
export const EVENTO_CAMBIO_EXPEDIENTES = "expedientes:cambio";

export function notificarCambioExpedientes() {
    window.dispatchEvent(new Event(EVENTO_CAMBIO_EXPEDIENTES));
}

export function useActualizacionEnVivo(refrescar, habilitado = true) {
    const ultimo = useRef(refrescar);
    useEffect(() => {
        ultimo.current = refrescar;
    });

    useEffect(() => {
        if (!habilitado) return undefined;
        const tick = () => {
            if (document.visibilityState === "visible") ultimo.current();
        };
        const timer = setInterval(tick, INTERVALO_VIVO_MS);
        document.addEventListener("visibilitychange", tick);
        window.addEventListener(EVENTO_CAMBIO_EXPEDIENTES, tick);
        return () => {
            clearInterval(timer);
            document.removeEventListener("visibilitychange", tick);
            window.removeEventListener(EVENTO_CAMBIO_EXPEDIENTES, tick);
        };
    }, [habilitado]);
}
