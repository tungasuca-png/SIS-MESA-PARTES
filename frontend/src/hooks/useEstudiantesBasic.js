import { useEffect, useState } from "react";
import { getEstudiantesBasic } from "../services/estudiantesService";

// Mismo patrón que useUsuariosBasic.js: resuelve por lote el nombre real de
// varios estudiantes (Estudiantes Service, vía Gateway) para mostrarlo en
// vez del UUID crudo en el listado/detalle de expedientes psicológicos.
// Cache en memoria (vive lo que dure la pestaña).
const cache = new Map();

/**
 * Resuelve por lote los datos básicos de varios estudiantes.
 * Devuelve el mapa id → estudiante (null si no existe) y una bandera cuando
 * Estudiantes Service no está disponible, para distinguir "no existe" de
 * "servicio caído".
 */
export function useEstudiantesBasic(ids) {
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

        getEstudiantesBasic(missing)
            .then((data) => {
                (data.estudiantes || []).forEach((estudiante) => cache.set(estudiante.id, estudiante));
                missing.forEach((id) => {
                    if (!cache.has(id)) cache.set(id, null);
                });
                if (active) {
                    setVersion((current) => current + 1);
                    setUnavailable(false);
                }
            })
            .catch(() => {
                if (active) {
                    setUnavailable(true);
                }
            });

        return () => {
            active = false;
        };
    }, [key]);

    return { estudiantes: cache, unavailable };
}
