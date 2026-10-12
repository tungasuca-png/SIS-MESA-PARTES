import { useEffect, useState } from "react";
import { useAuth } from "../../hooks/useAuth";
import { listUsuarios } from "../../services/usuariosService";

// Selector del profesional responsable de un expediente psicológico. El
// backend (Psicología Service, validateResponsable) exige que responsable_id
// sea un usuario real con tipo_usuario = PSICOLOGO — este campo solo ofrece
// psicólogos reales (GET /api/usuarios?tipo_usuario=PSICOLOGO, el mismo
// endpoint que ya usa Configuración → Usuarios), nunca una lista inventada.
//
// Esa consulta exige "usuarios.view" (permiso de OTRO módulo, Administración).
// Un PSICOLOGO típico no lo tiene por qué tener: si la consulta falla (403 u
// otro error), el campo se degrada solo a "asignarme a mí mismo" — nunca
// bloquea crear/editar el expediente por esto.
function ResponsablePsicologoField({ id, value, onChange, disabled }) {
    const { user } = useAuth();
    const [psicologos, setPsicologos] = useState(null); // null = aún no se sabe
    const [unavailable, setUnavailable] = useState(false);

    useEffect(() => {
        let active = true;
        listUsuarios({ tipoUsuario: "PSICOLOGO" })
            .then((data) => {
                if (active) setPsicologos(data.usuarios || []);
            })
            .catch(() => {
                if (active) setUnavailable(true);
            });
        return () => {
            active = false;
        };
    }, []);

    const otros = (psicologos || []).filter((item) => item.id !== user?.id);

    return (
        <div className="dp-form-group">
            <label htmlFor={id}>Responsable</label>
            <select
                id={id}
                value={value}
                onChange={(event) => onChange(event.target.value)}
                disabled={disabled}
            >
                <option value="">Yo mismo ({user?.username ?? "tú"})</option>
                {otros.map((item) => (
                    <option key={item.id} value={item.id}>
                        {`${item.nombres} ${item.apellidos}`.trim() || item.id}
                    </option>
                ))}
            </select>
            {unavailable && (
                <p className="dp-form-hint">
                    No se pudo cargar la lista de psicólogos (requiere el permiso "usuarios.view").
                    Puedes seguir asignándote a ti mismo.
                </p>
            )}
            {!unavailable && psicologos !== null && otros.length === 0 && (
                <p className="dp-form-hint">No hay otros psicólogos registrados todavía.</p>
            )}
        </div>
    );
}

export default ResponsablePsicologoField;
