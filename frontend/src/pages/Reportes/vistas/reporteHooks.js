import { useEffect, useRef, useState } from "react";
import { mensajeError } from "../reportesData";

// Utilidades de las pestañas de Reportes (Fase 2): hook de carga, fecha de
// generación y texto de alcance.

const formatoGenerado = new Intl.DateTimeFormat("es-PE", { dateStyle: "long", timeStyle: "short" });

export const fechaGenerado = (iso) => (iso ? formatoGenerado.format(new Date(iso)) : "");

export function textoAlcanceDe(datos, etiquetaArea) {
    if (datos?.alcance === "AREA") return `Expedientes que están actualmente en el área ${etiquetaArea(datos.area_alcance)}.`;
    return "Expedientes de toda la institución.";
}

// Carga los datos de una pestaña cada vez que cambia `clave` (filtros
// aplicados) o se pide reintentar. Mientras recarga conserva los datos
// anteriores (se muestran atenuados). El estado solo cambia en los
// callbacks de la promesa.
export function useReporte(cargar, clave) {
    const cargarRef = useRef(cargar);
    useEffect(() => {
        cargarRef.current = cargar;
    });
    const [recarga, setRecarga] = useState(0);
    const claveCompleta = `${clave}#${recarga}`;
    const [estado, setEstado] = useState({ clave: "", datos: null, error: "" });
    useEffect(() => {
        let activo = true;
        cargarRef.current()
            .then((datos) => activo && setEstado({ clave: claveCompleta, datos, error: "" }))
            .catch((err) => activo && setEstado((prev) => ({ clave: claveCompleta, datos: prev.datos, error: mensajeError(err) })));
        return () => {
            activo = false;
        };
    }, [claveCompleta]);
    return {
        datos: estado.error ? null : estado.datos,
        error: estado.error,
        cargando: estado.clave !== claveCompleta,
        reintentar: () => setRecarga((n) => n + 1),
    };
}
