import { useEffect, useRef, useState } from "react";
import { googleEstado, googleVincular } from "../../services/authService";
import { errorGoogle, mensajeGoogle } from "../Login/googleErrores";
import "../../components/dashboard/forms.css";

// "Cuenta de Google" en Mi perfil (solo solicitantes). Vincular exige volver
// a escribir la clave actual (reautenticación); el backend solo vincula a la
// cuenta de la sesión, sin cambiar rol, correo, DNI ni clave.
function leerResultado() {
    const p = new URLSearchParams(window.location.search);
    if (p.get("google") === "vinculado") return { tipo: "ok", texto: "Tu cuenta de Google quedó vinculada. Ya puedes ingresar con Google." };
    if (p.get("google_error")) return { tipo: "error", texto: mensajeGoogle(p.get("google_error")) };
    return null;
}

function VinculoGoogle() {
    const [estado, setEstado] = useState(null);
    const [cargaFallida, setCargaFallida] = useState(false);
    const [resultado, setResultado] = useState(leerResultado);
    const [clave, setClave] = useState("");
    const [enviando, setEnviando] = useState(false);
    const enCurso = useRef(false);

    useEffect(() => {
        // El resultado del regreso desde Google no queda en la URL.
        if (window.location.search) window.history.replaceState(null, "", window.location.pathname);
        let activo = true;
        googleEstado()
            .then((e) => activo && setEstado(e))
            .catch(() => activo && setCargaFallida(true));
        return () => {
            activo = false;
        };
    }, []);

    const vincular = async (ev) => {
        ev.preventDefault();
        if (enCurso.current) return;
        if (!clave) {
            setResultado({ tipo: "error", texto: "Ingresa tu clave actual para vincular Google." });
            return;
        }
        enCurso.current = true;
        setEnviando(true);
        setResultado(null);
        try {
            const r = await googleVincular(clave);
            setClave("");
            window.location.assign(r.url); // navegación completa: el backend redirige a Google
        } catch (err) {
            setResultado({ tipo: "error", texto: errorGoogle(err).mensaje });
            enCurso.current = false;
            setEnviando(false);
        }
    };

    return (
        <section className="dp-panel mdp-google" aria-labelledby="mdp-google-titulo">
            <h2 id="mdp-google-titulo" className="dp-panel-title">
                Cuenta de Google
            </h2>
            {resultado && (
                <p className={resultado.tipo === "ok" ? "mdp-google-ok" : "dp-form-error"} role={resultado.tipo === "ok" ? "status" : "alert"}>
                    {resultado.texto}
                </p>
            )}
            {!estado && !cargaFallida && <p className="dp-table-empty">Consultando la vinculación...</p>}
            {cargaFallida && <p className="dp-form-error">No se pudo consultar la vinculación con Google.</p>}
            {estado?.vinculado && (
                <p className="mdp-google-estado">
                    <span className="mdp-google-insignia">Google vinculado</span>
                    {estado.correo && <span>{estado.correo}</span>}
                </p>
            )}
            {estado && !estado.vinculado && !estado.disponible && (
                <p className="mdp-perfil-nota">El acceso con Google no está habilitado en este momento.</p>
            )}
            {estado && !estado.vinculado && estado.disponible && (
                <form className="mdp-google-form" onSubmit={vincular} noValidate>
                    <p className="mdp-perfil-nota">
                        <span className="mdp-google-insignia mdp-google-insignia--no">Google no vinculado</span>
                        Vincula tu cuenta de Google para ingresar también con ella. Por seguridad, confirma tu clave actual.
                    </p>
                    <div className="mdp-google-fila">
                        <div className="dp-form-group">
                            <label htmlFor="mdp-google-clave">Clave actual</label>
                            <input
                                id="mdp-google-clave"
                                type="password"
                                value={clave}
                                onChange={(e) => setClave(e.target.value)}
                                autoComplete="current-password"
                                disabled={enviando}
                            />
                        </div>
                        <button type="submit" className="dp-btn-primary" disabled={enviando} aria-busy={enviando}>
                            {enviando ? "Redirigiendo..." : "Vincular cuenta de Google"}
                        </button>
                    </div>
                </form>
            )}
        </section>
    );
}

export default VinculoGoogle;
