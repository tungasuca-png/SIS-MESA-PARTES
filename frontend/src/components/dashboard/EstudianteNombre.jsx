import { Link } from "react-router-dom";
import { usePermissions } from "../../hooks/usePermissions";
import { shortId } from "../../utils/format";

/**
 * Muestra el nombre del estudiante resuelto por Estudiantes Service. Mismo
 * criterio que SolicitanteNombre.jsx: distingue "no existe" de "servicio no
 * disponible" de "todavía resolviendo", sin mostrar el UUID salvo como
 * respaldo cuando no hay nada mejor que mostrar.
 *
 * Con "estudiantes.view" el nombre enlaza a la ficha del estudiante (por su
 * UUID estable); sin ese permiso es solo texto. La ficha aplica sus propios
 * permisos: el enlace no da acceso a nada adicional.
 */
function EstudianteNombre({ id, estudiantes, unavailable }) {
    const { can } = usePermissions();
    if (!id) return <span className="dp-usuario-fallback">—</span>;

    const resuelto = estudiantes?.get(id);
    if (resuelto) {
        return can("estudiantes.view") ? (
            <Link to={`/estudiantes/${id}`} title="Ver ficha del estudiante">
                {resuelto.nombre_completo}
            </Link>
        ) : (
            <span title={id}>{resuelto.nombre_completo}</span>
        );
    }

    if (estudiantes?.has(id)) {
        return (
            <span className="dp-usuario-fallback" title={id}>
                Estudiante no encontrado
            </span>
        );
    }

    if (unavailable) {
        return (
            <span className="dp-usuario-fallback" title={id}>
                Estudiante no disponible
            </span>
        );
    }

    return (
        <span className="dp-usuario-fallback" title={id}>
            {shortId(id)}
        </span>
    );
}

export default EstudianteNombre;
