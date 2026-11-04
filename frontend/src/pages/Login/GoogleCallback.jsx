import { useEffect, useRef, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "../../hooks/useAuth";
import { googleCanjear } from "../../services/authService";
import { errorGoogle, guardarRegistroPendiente, mensajeGoogle } from "./googleErrores";
import TarjetaAcceso from "./TarjetaAcceso";

// Regreso desde Google. El backend redirige aquí con un código de un solo
// uso (o la clave de un error) en el fragmento (#) de la URL; el fragmento se
// borra de inmediato y el código se canjea:
//   - SESION: identidad ya vinculada -> sesión normal y portal.
//   - REGISTRO_PENDIENTE: primera vez -> formulario obligatorio.
function leerFragmento() {
    const p = new URLSearchParams(window.location.hash.slice(1));
    return { codigo: p.get("codigo") ?? "", error: p.get("error") ?? "" };
}

function GoogleCallback() {
    const navigate = useNavigate();
    const { iniciarSesion } = useAuth();
    const [inicial] = useState(leerFragmento);
    const [error, setError] = useState(() =>
        inicial.error || !inicial.codigo ? mensajeGoogle(inicial.error || "codigo_invalido") : ""
    );
    const procesado = useRef(false);

    useEffect(() => {
        // El código no debe quedar en la barra de direcciones ni en el historial.
        window.history.replaceState(null, "", window.location.pathname);
        if (procesado.current || inicial.error || !inicial.codigo) return;
        procesado.current = true; // el código es de un solo uso
        googleCanjear(inicial.codigo)
            .then(async (r) => {
                if (r.resultado === "SESION" && r.sesion) {
                    const usuario = await iniciarSesion(r.sesion);
                    navigate(usuario.role === "SOLICITANTE" ? "/mesa-de-partes" : "/dashboard", { replace: true });
                    return;
                }
                guardarRegistroPendiente({ codigo: inicial.codigo, correo: r.correo, nombres: r.nombres, apellidos: r.apellidos });
                navigate("/registro/google", { replace: true });
            })
            .catch((err) => setError(errorGoogle(err).mensaje));
    }, [inicial, iniciarSesion, navigate]);

    return (
        <TarjetaAcceso titulo="Acceso con Google" descripcion={error ? undefined : "Estamos verificando tu cuenta de Google."}>
            {error ? (
                <>
                    <div className="login-error" role="alert">
                        <p>{error}</p>
                    </div>
                    <Link to="/login" className="login-button login-button--enlace">
                        Volver al inicio de sesión
                    </Link>
                </>
            ) : (
                <p className="login-estado" role="status" aria-live="polite">
                    <span className="login-spinner login-spinner--oscuro" aria-hidden="true" />
                    Verificando...
                </p>
            )}
        </TarjetaAcceso>
    );
}

export default GoogleCallback;
