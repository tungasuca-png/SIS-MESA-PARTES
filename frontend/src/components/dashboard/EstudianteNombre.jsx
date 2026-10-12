import { shortId } from "../../utils/format";

/**
 * Muestra el nombre del estudiante resuelto por Estudiantes Service. Mismo
 * criterio que SolicitanteNombre.jsx: distingue "no existe" de "servicio no
 * disponible" de "todavía resolviendo", sin mostrar el UUID salvo como
 * respaldo cuando no hay nada mejor que mostrar.
 */
function EstudianteNombre({ id, estudiantes, unavailable }) {
    if (!id) return <span className="dp-usuario-fallback">—</span>;

    const resuelto = estudiantes?.get(id);
    if (resuelto) {
        return <span title={id}>{resuelto.nombre_completo}</span>;
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
