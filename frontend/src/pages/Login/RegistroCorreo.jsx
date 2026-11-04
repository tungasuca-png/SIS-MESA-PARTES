import { useRef, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { registroCorreo } from "../../services/authService";
import { PRIVACIDAD_URL, TERMINOS_URL, TERMINOS_VERSION } from "../../constants/legales";
import { errorRegistro } from "./registroErrores";
import TarjetaAcceso from "./TarjetaAcceso";

// Registro manual de solicitantes. No crea la cuenta: el backend envía un
// código al correo y la cuenta se activa al verificarlo. El rol no se elige
// (siempre SOLICITANTE). La clave nunca se guarda en el navegador.
const DNI = /^\d{8}$/;
const CORREO = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const bytes = (v) => new TextEncoder().encode(v).length;

function validar(f) {
    const e = {};
    if (!f.nombres.trim()) e.nombres = "Ingresa tus nombres.";
    else if (f.nombres.trim().length > 100) e.nombres = "Máximo 100 caracteres.";
    if (!f.apellidoPaterno.trim()) e.apellidoPaterno = "Ingresa tu apellido paterno.";
    if ((f.apellidoPaterno.trim() + " " + f.apellidoMaterno.trim()).trim().length > 100) e.apellidoMaterno = "Los apellidos no pueden superar 100 caracteres.";
    if (!DNI.test(f.dni)) e.dni = "El DNI debe tener 8 dígitos numéricos.";
    if (!CORREO.test(f.correo.trim()) || f.correo.trim().length > 254) e.correo = "Ingresa un correo electrónico válido.";
    if (f.clave.length < 8) e.clave = "La contraseña debe tener al menos 8 caracteres.";
    else if (bytes(f.clave) > 72) e.clave = "La contraseña es demasiado larga.";
    if (!e.clave && f.confirmacion !== f.clave) e.confirmacion = "Las contraseñas no coinciden.";
    if (!f.terminos) e.terminos = "Debes aceptar los términos y la política de privacidad.";
    return e;
}

function Campo({ id, label, obligatorio, error, children }) {
    return (
        <div className="form-group">
            <label htmlFor={id}>
                {label}
                {obligatorio && <span className="registro-obligatorio" aria-hidden="true"> *</span>}
            </label>
            <div className="input-box">{children}</div>
            {error && (
                <p className="registro-error-campo" id={`${id}-error`}>
                    {error}
                </p>
            )}
        </div>
    );
}

function Documento({ url, children }) {
    if (!url) return children;
    return (
        <a href={url} target="_blank" rel="noopener noreferrer">
            {children}
        </a>
    );
}

const VACIO = { nombres: "", apellidoPaterno: "", apellidoMaterno: "", dni: "", correo: "", clave: "", confirmacion: "", terminos: false };

function RegistroCorreo() {
    const navigate = useNavigate();
    const [form, setForm] = useState(VACIO);
    const [errores, setErrores] = useState({});
    const [error, setError] = useState(null);
    const [enviando, setEnviando] = useState(false);
    const [mostrarClave, setMostrarClave] = useState(false);
    const enCurso = useRef(false);

    const cambiar = (campo, filtro) => (ev) => {
        const valor = ev.target.type === "checkbox" ? ev.target.checked : filtro ? filtro(ev.target.value) : ev.target.value;
        setForm((f) => ({ ...f, [campo]: valor }));
        setErrores((e) => ({ ...e, [campo]: undefined }));
    };

    const enviar = async (ev) => {
        ev.preventDefault();
        if (enCurso.current) return; // doble envío
        const e = validar(form);
        setErrores(e);
        setError(null);
        if (Object.keys(e).length) return;
        enCurso.current = true;
        setEnviando(true);
        const correo = form.correo.trim().toLowerCase();
        try {
            const r = await registroCorreo({
                nombres: form.nombres.trim(),
                apellidoPaterno: form.apellidoPaterno.trim(),
                apellidoMaterno: form.apellidoMaterno.trim(),
                dni: form.dni,
                correo,
                clave: form.clave,
                aceptaTerminos: true,
                versionTerminos: TERMINOS_VERSION,
            });
            navigate("/registro/verificar", {
                replace: true,
                state: { registroId: r.registro_id, correo, expiraEn: r.expira_en, reenviarEn: r.reenviar_en },
            });
        } catch (err) {
            setError(errorRegistro(err, "registro"));
            enCurso.current = false;
            setEnviando(false);
        }
    };

    const prop = (id, campo) => ({
        id,
        value: form[campo],
        disabled: enviando,
        "aria-invalid": Boolean(errores[campo]),
        "aria-describedby": errores[campo] ? `${id}-error` : undefined,
    });

    return (
        <TarjetaAcceso
            ancha
            titulo="Crear cuenta con correo"
            descripcion="Para solicitantes. Te enviaremos un código para verificar tu correo. Los campos con * son obligatorios."
        >
            <form onSubmit={enviar} noValidate aria-label="Registro con correo electrónico">
                <div className="registro-grid">
                    <Campo id="rc-nombres" label="Nombres" obligatorio error={errores.nombres}>
                        <input {...prop("rc-nombres", "nombres")} onChange={cambiar("nombres")} autoComplete="given-name" maxLength={100} />
                    </Campo>
                    <Campo id="rc-dni" label="DNI" obligatorio error={errores.dni}>
                        <input {...prop("rc-dni", "dni")} onChange={cambiar("dni", (v) => v.replace(/\D/g, "").slice(0, 8))}
                            inputMode="numeric" maxLength={8} placeholder="8 dígitos" />
                    </Campo>
                    <Campo id="rc-paterno" label="Apellido paterno" obligatorio error={errores.apellidoPaterno}>
                        <input {...prop("rc-paterno", "apellidoPaterno")} onChange={cambiar("apellidoPaterno")} autoComplete="family-name" maxLength={100} />
                    </Campo>
                    <Campo id="rc-materno" label="Apellido materno" error={errores.apellidoMaterno}>
                        <input {...prop("rc-materno", "apellidoMaterno")} onChange={cambiar("apellidoMaterno")} maxLength={100} />
                    </Campo>
                </div>
                <p className="registro-nota">
                    La institución verificará tu DNI. Hasta entonces ingresarás con tu correo y podrás presentar tus trámites
                    con normalidad.
                </p>
                <Campo id="rc-correo" label="Correo electrónico" obligatorio error={errores.correo}>
                    <input {...prop("rc-correo", "correo")} onChange={cambiar("correo")} type="email" autoComplete="email" maxLength={254} />
                </Campo>
                <div className="registro-grid">
                    <Campo id="rc-clave" label="Contraseña" obligatorio error={errores.clave}>
                        <input {...prop("rc-clave", "clave")} onChange={cambiar("clave")} type={mostrarClave ? "text" : "password"}
                            autoComplete="new-password" placeholder="Mínimo 8 caracteres" />
                    </Campo>
                    <Campo id="rc-confirmacion" label="Confirmar contraseña" obligatorio error={errores.confirmacion}>
                        <input {...prop("rc-confirmacion", "confirmacion")} onChange={cambiar("confirmacion")}
                            type={mostrarClave ? "text" : "password"} autoComplete="new-password" />
                    </Campo>
                </div>
                <label className="registro-check registro-check--suave">
                    <input type="checkbox" checked={mostrarClave} onChange={(e) => setMostrarClave(e.target.checked)} disabled={enviando} />
                    Mostrar contraseña
                </label>

                <label className="registro-check" htmlFor="rc-terminos">
                    <input id="rc-terminos" type="checkbox" checked={form.terminos} onChange={cambiar("terminos")} disabled={enviando}
                        aria-invalid={Boolean(errores.terminos)} aria-describedby={errores.terminos ? "rc-terminos-error" : undefined} />
                    <span>
                        Acepto los <Documento url={TERMINOS_URL}>Términos y condiciones</Documento> y la{" "}
                        <Documento url={PRIVACIDAD_URL}>Política de privacidad</Documento> de la I.E. Tungasuca.
                    </span>
                </label>
                {errores.terminos && (
                    <p className="registro-error-campo" id="rc-terminos-error">
                        {errores.terminos}
                    </p>
                )}

                {error && (
                    <div className="login-error" role="alert">
                        <p>{error.mensaje}</p>
                    </div>
                )}
                <button type="submit" className="login-button" disabled={enviando} aria-busy={enviando}>
                    {enviando && <span className="login-spinner" aria-hidden="true" />}
                    {enviando ? "Enviando código..." : "Crear cuenta"}
                </button>
            </form>
            <p className="login-help">
                ¿Ya tienes una cuenta? <Link to="/login">Inicia sesión</Link>
            </p>
        </TarjetaAcceso>
    );
}

export default RegistroCorreo;
