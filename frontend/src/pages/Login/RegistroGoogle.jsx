import { useRef, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "../../hooks/useAuth";
import { googleRegistro } from "../../services/authService";
import { borrarRegistroPendiente, errorGoogle, leerRegistroPendiente } from "./googleErrores";
import TarjetaAcceso from "./TarjetaAcceso";

// Formulario OBLIGATORIO tras el primer acceso con Google. Sin completarlo no
// hay sesión ni acceso al portal. El correo es el que Google verificó (no se
// edita) y el rol no se elige: el backend registra siempre SOLICITANTE. La
// disponibilidad del DNI la decide el backend, nunca esta pantalla.
const DNI = /^\d{8}$/;
const TELEFONO = /^[0-9+\s-]{0,20}$/;

function separarApellidos(apellidos = "") {
    const partes = apellidos.trim().split(/\s+/).filter(Boolean);
    return { paterno: partes[0] ?? "", materno: partes.slice(1).join(" ") };
}

function validar(f) {
    const e = {};
    if (!f.nombres.trim()) e.nombres = "Ingresa tus nombres.";
    else if (f.nombres.trim().length > 100) e.nombres = "Máximo 100 caracteres.";
    if (!f.apellidoPaterno.trim()) e.apellidoPaterno = "Ingresa tu apellido paterno.";
    if ((f.apellidoPaterno.trim() + " " + f.apellidoMaterno.trim()).trim().length > 100) e.apellidoMaterno = "Los apellidos no pueden superar 100 caracteres.";
    if (!DNI.test(f.dni)) e.dni = "El DNI debe tener 8 dígitos numéricos.";
    if (!TELEFONO.test(f.telefono.trim())) e.telefono = "Usa solo números (máximo 20 caracteres).";
    if (f.direccion.trim().length > 255) e.direccion = "Máximo 255 caracteres.";
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

function RegistroGoogle() {
    const navigate = useNavigate();
    const { iniciarSesion } = useAuth();
    const [pendiente] = useState(leerRegistroPendiente);
    const [form, setForm] = useState(() => {
        const ap = separarApellidos(pendiente?.apellidos);
        return { nombres: pendiente?.nombres ?? "", apellidoPaterno: ap.paterno, apellidoMaterno: ap.materno, dni: "", telefono: "", direccion: "" };
    });
    const [errores, setErrores] = useState({});
    const [error, setError] = useState(null);
    const [enviando, setEnviando] = useState(false);
    const enCurso = useRef(false);

    if (!pendiente?.codigo) {
        return (
            <TarjetaAcceso titulo="Registro con Google">
                <div className="login-error" role="alert">
                    <p>No hay un registro en curso. Inicia el acceso con Google desde la pantalla de inicio de sesión.</p>
                </div>
                <Link to="/login" className="login-button login-button--enlace">
                    Ir al inicio de sesión
                </Link>
            </TarjetaAcceso>
        );
    }

    const cambiar = (campo, filtro) => (ev) => {
        const valor = filtro ? filtro(ev.target.value) : ev.target.value;
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
        try {
            const sesion = await googleRegistro({
                codigo: pendiente.codigo,
                dni: form.dni,
                nombres: form.nombres.trim(),
                apellidoPaterno: form.apellidoPaterno.trim(),
                apellidoMaterno: form.apellidoMaterno.trim(),
                telefono: form.telefono.trim(),
                direccion: form.direccion.trim(),
            });
            borrarRegistroPendiente();
            await iniciarSesion(sesion);
            navigate("/mesa-de-partes", { replace: true });
        } catch (err) {
            const g = errorGoogle(err);
            if (g.clave === "codigo_invalido" || g.clave === "ya_vinculado" || g.clave === "correo_existente") borrarRegistroPendiente();
            setError(g);
            enCurso.current = false;
            setEnviando(false);
        }
    };

    const cuentaExistente = error && ["dni_registrado", "correo_existente", "ya_vinculado"].includes(error.clave);
    const reiniciar = error?.clave === "codigo_invalido";
    // Con un correo o una identidad ya registrados este registro no puede seguir.
    const sinContinuar = reiniciar || ["correo_existente", "ya_vinculado"].includes(error?.clave);

    return (
        <TarjetaAcceso
            ancha
            titulo="Completa tu registro"
            descripcion="Antes de ingresar al portal necesitamos tus datos. Los campos con * son obligatorios."
        >
            <form onSubmit={enviar} noValidate aria-label="Registro de solicitante">
                <div className="registro-correo">
                    <span>Correo verificado por Google</span>
                    <strong>{pendiente.correo}</strong>
                </div>
                <p className="registro-nota">
                    La institución verificará tu DNI. Hasta entonces podrás ingresar con Google y presentar tus trámites con normalidad.
                </p>
                <div className="registro-grid">
                    <Campo id="reg-nombres" label="Nombres" obligatorio error={errores.nombres}>
                        <input id="reg-nombres" value={form.nombres} onChange={cambiar("nombres")} autoComplete="given-name" maxLength={100}
                            disabled={enviando} aria-invalid={Boolean(errores.nombres)} aria-describedby={errores.nombres ? "reg-nombres-error" : undefined} />
                    </Campo>
                    <Campo id="reg-dni" label="DNI" obligatorio error={errores.dni}>
                        <input id="reg-dni" value={form.dni} onChange={cambiar("dni", (v) => v.replace(/\D/g, "").slice(0, 8))} inputMode="numeric"
                            maxLength={8} placeholder="8 dígitos" disabled={enviando} aria-invalid={Boolean(errores.dni)}
                            aria-describedby={errores.dni ? "reg-dni-error" : undefined} />
                    </Campo>
                    <Campo id="reg-paterno" label="Apellido paterno" obligatorio error={errores.apellidoPaterno}>
                        <input id="reg-paterno" value={form.apellidoPaterno} onChange={cambiar("apellidoPaterno")} autoComplete="family-name" maxLength={100}
                            disabled={enviando} aria-invalid={Boolean(errores.apellidoPaterno)} />
                    </Campo>
                    <Campo id="reg-materno" label="Apellido materno" error={errores.apellidoMaterno}>
                        <input id="reg-materno" value={form.apellidoMaterno} onChange={cambiar("apellidoMaterno")} maxLength={100} disabled={enviando}
                            aria-invalid={Boolean(errores.apellidoMaterno)} />
                    </Campo>
                    <Campo id="reg-telefono" label="Teléfono" error={errores.telefono}>
                        <input id="reg-telefono" value={form.telefono} onChange={cambiar("telefono")} inputMode="tel" autoComplete="tel" maxLength={20}
                            disabled={enviando} aria-invalid={Boolean(errores.telefono)} />
                    </Campo>
                    <Campo id="reg-direccion" label="Dirección" error={errores.direccion}>
                        <input id="reg-direccion" value={form.direccion} onChange={cambiar("direccion")} autoComplete="street-address" maxLength={255}
                            disabled={enviando} aria-invalid={Boolean(errores.direccion)} />
                    </Campo>
                </div>

                {error && (
                    <div className="login-error" role="alert">
                        <p>{error.mensaje}</p>
                    </div>
                )}
                {cuentaExistente || reiniciar ? (
                    <Link to="/login" className="login-button login-button--enlace">
                        {reiniciar ? "Volver a empezar" : "Ir a iniciar sesión"}
                    </Link>
                ) : null}
                {!sinContinuar && (
                    <button type="submit" className="login-button" disabled={enviando} aria-busy={enviando}>
                        {enviando && <span className="login-spinner" aria-hidden="true" />}
                        {enviando ? "Registrando..." : "Completar registro"}
                    </button>
                )}
            </form>
        </TarjetaAcceso>
    );
}

export default RegistroGoogle;
