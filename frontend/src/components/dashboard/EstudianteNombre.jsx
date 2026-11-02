import { Link } from "react-router-dom";
import { usePermissions } from "../../hooks/usePermissions";

/**
 * Nombre del estudiante de un expediente psicológico, tal como lo resolvió
 * Psicología con su credencial de servicio (expediente.estudiante:
 * { nombre_completo, resolucion }). El frontend no consulta Estudiantes.
 *
 * resolucion: RESUELTO (nombre) | NO_ENCONTRADO | NO_DISPONIBLE. Nunca muestra
 * el UUID, ni como texto ni como tooltip.
 *
 * Con "estudiantes.view" el nombre enlaza a la ficha del estudiante (por su
 * UUID estable); sin ese permiso es solo texto. La ficha aplica sus propios
 * permisos: el enlace no da acceso a nada adicional.
 */
function EstudianteNombre({ id, resumen }) {
    const { can } = usePermissions();
    if (!id) return <span className="dp-usuario-fallback">—</span>;
    if (resumen?.resolucion === "RESUELTO" && resumen.nombre_completo) {
        return can("estudiantes.view") ? (
            <Link to={`/estudiantes/${id}`}>{resumen.nombre_completo}</Link>
        ) : (
            <span>{resumen.nombre_completo}</span>
        );
    }
    if (resumen?.resolucion === "NO_ENCONTRADO") {
        return <span className="dp-usuario-fallback">Estudiante no encontrado</span>;
    }
    // NO_DISPONIBLE (o sin resumen): Estudiantes/Auth no respondieron.
    return <span className="dp-usuario-fallback">Nombre no disponible</span>;
}

export default EstudianteNombre;
