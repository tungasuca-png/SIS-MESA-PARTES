import { useEffect, useState } from "react";
import { Navigate } from "react-router-dom";
import PortalLayout from "../../components/portal/PortalLayout";
import { useAuth } from "../../hooks/useAuth";
import { getUsuario } from "../../services/usuariosService";
import { friendlyErrorMessage } from "../../utils/apiErrors";
import "./mesaDePartes.css";

// Usuarios Service SÍ tiene un perfil completo (nombres, apellidos, DNI,
// teléfono, dirección) y autoriza a cualquier usuario a consultar el suyo
// (ver authorization.CanViewFullProfile: rol interno O el propio usuario
// sobre sí mismo) — pero esa fila no existe automáticamente para toda cuenta
// creada en Auth Service (son bases de datos separadas, sin sincronización
// entre servicios). Por eso esta pantalla intenta la consulta real y, si
// no hay perfil registrado (404), lo dice explícitamente en vez de inventar
// datos.
function MiPerfil() {
    const { user } = useAuth();
    const [perfil, setPerfil] = useState(null);
    const [sinPerfilExtendido, setSinPerfilExtendido] = useState(false);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");

    useEffect(() => {
        let active = true;

        async function load() {
            if (!user?.id) return;
            setLoading(true);
            setError("");
            try {
                const data = await getUsuario(user.id);
                if (!active) return;
                setPerfil(data.usuario);
            } catch (err) {
                if (!active) return;
                if (err?.response?.status === 404) {
                    setSinPerfilExtendido(true);
                } else {
                    setError(friendlyErrorMessage(err));
                }
            } finally {
                if (active) setLoading(false);
            }
        }

        load();
        return () => {
            active = false;
        };
    }, [user?.id]);

    if (user && user.role !== "SOLICITANTE") {
        return <Navigate to="/dashboard" replace />;
    }

    return (
        <PortalLayout title="Mi perfil">
            <section className="dp-panel">
                <h2 className="dp-panel-title">Datos de mi cuenta</h2>

                <dl className="mdp-perfil-list">
                    <div>
                        <dt>Usuario</dt>
                        <dd>{user?.username}</dd>
                    </div>
                    <div>
                        <dt>Correo</dt>
                        <dd>{user?.email}</dd>
                    </div>
                    <div>
                        <dt>Rol</dt>
                        <dd>{user?.role}</dd>
                    </div>

                    {perfil && (
                        <>
                            <div>
                                <dt>Nombres y apellidos</dt>
                                <dd>
                                    {perfil.nombres} {perfil.apellidos}
                                </dd>
                            </div>
                            {perfil.dni && (
                                <div>
                                    <dt>DNI</dt>
                                    <dd>{perfil.dni}</dd>
                                </div>
                            )}
                            {perfil.telefono && (
                                <div>
                                    <dt>Teléfono</dt>
                                    <dd>{perfil.telefono}</dd>
                                </div>
                            )}
                            {perfil.direccion && (
                                <div>
                                    <dt>Dirección</dt>
                                    <dd>{perfil.direccion}</dd>
                                </div>
                            )}
                        </>
                    )}
                </dl>

                {loading && <p className="dp-table-empty">Cargando datos adicionales del perfil...</p>}
                {!loading && error && <p className="dp-form-error">{error}</p>}
                {!loading && sinPerfilExtendido && (
                    <p className="mdp-perfil-nota">
                        Todavía no tienes un perfil extendido registrado (DNI, teléfono, dirección). Estos datos se
                        completan al presentar una solicitud.
                    </p>
                )}

                <p className="mdp-perfil-nota">
                    Por el momento no es posible editar esta información desde aquí.
                </p>
            </section>
        </PortalLayout>
    );
}

export default MiPerfil;
