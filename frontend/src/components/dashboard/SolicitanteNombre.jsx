import { useAuth } from "../../hooks/useAuth";
import { shortId } from "../../utils/format";

/**
 * Muestra el nombre del solicitante resuelto por Usuarios Service.
 * Distingue tres situaciones distintas para no confundirlas:
 *  - el usuario no tiene perfil registrado todavía
 *  - Usuarios Service no está disponible
 *  - todavía se está resolviendo
 */
function SolicitanteNombre({ id, usuarios, unavailable }) {
    const { user } = useAuth();

    if (!id) return <span className="dp-usuario-fallback">—</span>;
    if (id === user?.id) return <span>Tú</span>;

    const resuelto = usuarios?.get(id);
    if (resuelto) {
        return <span title={id}>{resuelto.nombre_completo}</span>;
    }

    if (usuarios?.has(id)) {
        return (
            <span className="dp-usuario-fallback" title={id}>
                Sin perfil registrado
            </span>
        );
    }

    if (unavailable) {
        return (
            <span className="dp-usuario-fallback" title={id}>
                Usuario no disponible
            </span>
        );
    }

    return (
        <span className="dp-usuario-fallback" title={id}>
            {shortId(id)}
        </span>
    );
}

export default SolicitanteNombre;
