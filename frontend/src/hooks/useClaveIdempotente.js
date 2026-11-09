import { useRef } from "react";
import { nuevaClaveIdempotencia } from "../services/expedientesService";

// Claves de idempotencia por operación (A + B): mientras el usuario reintenta
// la MISMA intención (mismos datos = misma "firma"), se reutiliza la misma
// clave, así un reintento tras un error incierto nunca registra dos veces.
// Se descarta tras el éxito; si cambian los datos, se genera una nueva.
export function useClaveIdempotente() {
    const claves = useRef(new Map());

    const obtener = (operacion, firma) => {
        const actual = claves.current.get(operacion);
        if (actual && actual.firma === firma) {
            return actual.clave;
        }
        const clave = nuevaClaveIdempotencia();
        claves.current.set(operacion, { firma, clave });
        return clave;
    };

    const descartar = (operacion) => {
        claves.current.delete(operacion);
    };

    return { obtener, descartar };
}
