// Estado del REGISTRO del estudiante (ACTIVO/INACTIVO). No es el estado de
// matrícula.
function EstadoEstudianteBadge({ estado }) {
    const activo = estado === "ACTIVO";
    return (
        <span className={activo ? "est-badge est-badge--activo" : "est-badge est-badge--inactivo"}>
            {activo ? "Activo" : estado === "INACTIVO" ? "Inactivo" : estado || "—"}
        </span>
    );
}

export default EstadoEstudianteBadge;
