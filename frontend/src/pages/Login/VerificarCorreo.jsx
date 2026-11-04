import { useEffect, useRef, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { reenviarCodigo, verificarCorreo } from "../../services/authService";
import { CLAVES_FINALES, errorRegistro } from "./registroErrores";
import TarjetaAcceso from "./TarjetaAcceso";

// Verificación del correo del registro manual. El id del registro llega en el
// estado de la navegación (no en la URL); el código no se guarda en ningún
// lado. Al verificar, el backend crea la cuenta y la activa.
const CODIGO = /^\d{6}$/;

function mmss(segundos) {
    const s = Math.max(0, Math.ceil(segundos));
    return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;
}

function VerificarCorreo() {
    const navigate = useNavigate();
    const { state } = useLocation();
    const registroId = state?.registroId;
    const correo = state?.correo;

    const [codigo, setCodigo] = useState("");
    const [error, setError] = useState(null);
    const [aviso, setAviso] = useState("");
    const [ocupado, setOcupado] = useState(false);
    const [activada, setActivada] = useState(null);
    const [plazos, setPlazos] = useState(() => {
        const ahora = Date.now();
        return { expira: ahora + (state?.expiraEn ?? 600) * 1000, reenviar: ahora + (state?.reenviarEn ?? 60) * 1000 };
    });
    const [ahora, setAhora] = useState(() => Date.now());
    const enCurso = useRef(false);

    useEffect(() => {
        const t = setInterval(() => setAhora(Date.now()), 1000);
        return () => clearInterval(t);
    }, []);

    if (!registroId) {
        return (
            <TarjetaAcceso titulo="Verificar correo">
                <div className="login-error" role="alert">
                    <p>No hay un registro en curso. Completa el formulario de registro para recibir un código.</p>
                </div>
                <Link to="/registro" className="login-button login-button--enlace">
                    Ir al registro
                </Link>
            </TarjetaAcceso>
        );
    }

    if (activada) {
        return (
            <TarjetaAcceso titulo="Cuenta activada">
                <div className="registro-aviso" role="status">
                    <p>{activada.mensaje}</p>
                </div>
                <button
                    type="button"
                    className="login-button"
                    onClick={() => navigate("/login", { replace: true, state: { correo: activada.correo } })}
                >
                    Iniciar sesión
                </button>
            </TarjetaAcceso>
        );
    }

    const restanteExpira = (plazos.expira - ahora) / 1000;
    const restanteReenvio = (plazos.reenviar - ahora) / 1000;
    const final = error && CLAVES_FINALES.includes(error.clave);

    const verificar = async (ev) => {
        ev.preventDefault();
        if (enCurso.current) return;
        setAviso("");
        if (!CODIGO.test(codigo)) {
            setError({ clave: "codigo_formato", mensaje: "Ingresa el código de 6 dígitos que recibiste." });
            return;
        }
        enCurso.current = true;
        setOcupado(true);
        setError(null);
        try {
            const r = await verificarCorreo(registroId, codigo);
            setActivada({ mensaje: r.mensaje, correo: r.correo || correo });
        } catch (err) {
            setError(errorRegistro(err, "verificar"));
            setCodigo("");
        } finally {
            enCurso.current = false;
            setOcupado(false);
        }
    };

    const reenviar = async () => {
        if (enCurso.current) return;
        enCurso.current = true;
        setOcupado(true);
        setError(null);
        setAviso("");
        try {
            const r = await reenviarCodigo(registroId);
            const t = Date.now();
            setPlazos({ expira: t + r.expira_en * 1000, reenviar: t + r.reenviar_en * 1000 });
            setAhora(t);
            setCodigo("");
            setAviso("Te enviamos un código nuevo. El anterior ya no sirve.");
        } catch (err) {
            setError(errorRegistro(err, "reenviar"));
        } finally {
            enCurso.current = false;
            setOcupado(false);
        }
    };

    return (
        <TarjetaAcceso titulo="Verifica tu correo" descripcion="Ingresa el código de 6 dígitos que enviamos para activar tu cuenta.">
            <div className="registro-correo">
                <span>Código enviado a</span>
                <strong>{correo}</strong>
            </div>
            <form onSubmit={verificar} noValidate aria-label="Verificación del correo">
                <div className="form-group">
                    <label htmlFor="vc-codigo">Código de verificación</label>
                    <div className="input-box">
                        <input
                            id="vc-codigo"
                            className="codigo-verificacion"
                            value={codigo}
                            onChange={(e) => setCodigo(e.target.value.replace(/\D/g, "").slice(0, 6))}
                            inputMode="numeric"
                            autoComplete="one-time-code"
                            maxLength={6}
                            placeholder="000000"
                            disabled={ocupado || final}
                            aria-describedby="vc-plazo"
                        />
                    </div>
                    <p className="registro-temporizador" id="vc-plazo" aria-live="polite">
                        {restanteExpira > 0 ? `El código vence en ${mmss(restanteExpira)}.` : "El código venció. Solicita uno nuevo."}
                    </p>
                </div>

                {aviso && (
                    <div className="registro-aviso" role="status">
                        <p>{aviso}</p>
                    </div>
                )}
                {error && (
                    <div className="login-error" role="alert">
                        <p>{error.mensaje}</p>
                    </div>
                )}

                {!final && (
                    <button type="submit" className="login-button" disabled={ocupado} aria-busy={ocupado}>
                        {ocupado && <span className="login-spinner" aria-hidden="true" />}
                        {ocupado ? "Verificando..." : "Verificar y activar"}
                    </button>
                )}
            </form>

            {!final && (
                <button type="button" className="login-enlace-boton" onClick={reenviar} disabled={ocupado || restanteReenvio > 0}>
                    {restanteReenvio > 0 ? `Reenviar código en ${mmss(restanteReenvio)}` : "Reenviar código"}
                </button>
            )}
            <p className="login-help">
                ¿Te equivocaste de correo? <Link to="/registro">Volver al registro</Link>
                {final && (
                    <>
                        {" · "}
                        <Link to="/login">Iniciar sesión</Link>
                    </>
                )}
            </p>
        </TarjetaAcceso>
    );
}

export default VerificarCorreo;
