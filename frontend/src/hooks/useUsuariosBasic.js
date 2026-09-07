import { useEffect, useState } from "react";
import { getUsuariosBasic } from "../services/usuariosService";

// Cache en memoria (vive lo que dure la pestaña): evita volver a resolver el
// mismo usuario en cada render o al cambiar de página. No es Redis ni
// infraestructura nueva; solo evita peticiones repetidas.
const cache = new Map();

/**
 * Resuelve por lote los datos básicos de varios usuarios.
 * Devuelve el mapa id → usuario (null si el servicio respondió que no existe)
 * y una bandera cuando Usuarios Service no está disponible, para poder
 * distinguir "usuario inexistente" de "servicio caído".
 */
export function useUsuariosBasic(ids) {
    const [, setVersion] = useState(0);
    const [unavailable, setUnavailable] = useState(false);

    const key = [...new Set((ids || []).filter(Boolean))].sort().join(",");

    useEffect(() => {
        const unique = key ? key.split(",") : [];
        const missing = unique.filter((id) => !cache.has(id));
        if (missing.length === 0) {
            return undefined;
        }

        let active = true;

        getUsuariosBasic(missing)
            .then((data) => {
                (data.usuarios || []).forEach((usuario) => cache.set(usuario.id, usuario));
                // Los ids que el servicio no devolvió no existen: se marcan
                // para no reintentarlos en bucle.
                missing.forEach((id) => {
                    if (!cache.has(id)) cache.set(id, null);
                });
                if (active) {
                    setVersion((current) => current + 1);
                    setUnavailable(false);
                }
            })
            .catch(() => {
                // Servicio caído: no se cachea el fallo, para poder reintentar
                // en la siguiente navegación.
                if (active) {
                    setUnavailable(true);
                }
            });

        return () => {
            active = false;
        };
    }, [key]);

    return { usuarios: cache, unavailable };
}
