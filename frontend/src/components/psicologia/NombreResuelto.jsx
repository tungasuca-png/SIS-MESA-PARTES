import { useAuth } from "../../hooks/useAuth";

// Nombre de un usuario o estudiante ya resuelto (useUsuariosBasic /
// useEstudiantesBasic). A diferencia de SolicitanteNombre/EstudianteNombre,
// nunca muestra el UUID (ni acortado ni como tooltip): las pantallas de
// atenciones no exponen identificadores internos.
function NombreResuelto({ id, items, unavailable, esUsuario = true }) {
    const { user } = useAuth();

    if (!id) return <span className="dp-usuario-fallback">—</span>;
    if (esUsuario && id === user?.id) return <span>Tú</span>;

    const resuelto = items?.get(id);
    if (resuelto) return <span>{resuelto.nombre_completo}</span>;
    if (items?.has(id)) {
        return <span className="dp-usuario-fallback">{esUsuario ? "Sin perfil registrado" : "Estudiante no encontrado"}</span>;
    }
    if (unavailable) return <span className="dp-usuario-fallback">No disponible</span>;
    return <span className="dp-usuario-fallback">Cargando…</span>;
}

export default NombreResuelto;
