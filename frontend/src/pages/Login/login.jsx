
import { useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../../hooks/useAuth";
import { extractBackendMessage } from "../../utils/apiErrors";
import { GOOGLE_INICIAR_URL } from "../../services/authService";
import "./login.css";

function Login() {
    const navigate = useNavigate();
    const { state } = useLocation();
    const { login: loginUser } = useAuth();

    // Tras activar una cuenta con correo, el correo llega precargado: hasta
    // que la institución verifique su identidad, el solicitante ingresa con él.
    const [modo, setModo] = useState(() => (state?.correo ? "correo" : "dni"));
    const [correo, setCorreo] = useState(() => state?.correo ?? "");
    const [dni, setDni] = useState(() => (/^[0-9]{8}$/.test(state?.dni ?? "") ? state.dni : ""));
    const [password, setPassword] = useState("");
    const [mostrarClave, setMostrarClave] = useState(false);
    const [error, setError] = useState("");
    const [loading, setLoading] = useState(false);

    const handleSubmit = async (e) => {
        e.preventDefault();
        setError("");

        const porCorreo = modo === "correo";
        const dniLimpio = dni.trim();
        const correoLimpio = correo.trim().toLowerCase();
        if (porCorreo) {
            if (!correoLimpio || !password) {
                setError("Ingresa tu correo y tu clave");
                return;
            }
            if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(correoLimpio)) {
                setError("Ingresa un correo electrónico válido");
                return;
            }
        } else {
            if (!dniLimpio || !password) {
                setError("Ingresa tu DNI y tu clave");
                return;
            }
            if (!/^[0-9]{8}$/.test(dniLimpio)) {
                setError("El DNI debe tener 8 dígitos numéricos");
                return;
            }
        }

        setLoading(true);

        try {
            const loggedInUser = await loginUser(porCorreo ? correoLimpio : dniLimpio, password);
            // El SOLICITANTE entra a su propio portal (Mesa de Partes
            // Virtual, sin sidebar administrativo); el resto de roles sigue
            // entrando al Dashboard de siempre.
            navigate(loggedInUser.role === "SOLICITANTE" ? "/mesa-de-partes" : "/dashboard");
        } catch (err) {
            // Mensaje genérico: no revela si el DNI existe.
            if (!err?.response) {
                setError("No se pudo conectar con el servidor. Inténtalo nuevamente.");
            } else if (err.response.status === 400) {
                setError(porCorreo ? "Ingresa un correo electrónico válido" : "El DNI debe tener 8 dígitos numéricos");
            } else if (err.response.status === 429) {
                // Auth bloquea la cuenta 15 minutos tras 5 intentos fallidos
                // ("account_locked: ... en N minutos").
                const minutos = extractBackendMessage(err.response.data)?.match(/en (\d+) minuto/)?.[1];
                setError(
                    `Cuenta bloqueada temporalmente por demasiados intentos fallidos. ${
                        minutos ? `Intenta de nuevo en ${minutos} ${minutos === "1" ? "minuto" : "minutos"}.` : "Intenta de nuevo más tarde."
                    }`
                );
            } else if (err.response.status === 403) {
                // Registro con correo sin terminar (solo se informa si la clave
                // es correcta): "correo_no_verificado: ..." o "registro_incompleto: ...".
                const desc = extractBackendMessage(err.response.data) ?? "";
                const m = desc.match(/^(?:correo_no_verificado|registro_incompleto):\s*(.+)$/s);
                setError(m ? m[1] : "DNI o clave incorrectos");
            } else if (err.response.status >= 500) {
                setError("El servicio no está disponible en este momento. Inténtalo nuevamente.");
            } else {
                setError(porCorreo ? "Correo o clave incorrectos" : "DNI o clave incorrectos");
            }
        } finally {
            setLoading(false);
        }
    };

    return (
        <main className="login-page">

            <section className="login-card" aria-labelledby="login-titulo">

                {/* IDENTIDAD */}
                <header className="login-header">
                    <img
                        className="login-logo"
                        src="/logo-tungasuca.png"
                        alt="Logo de la I.E. Tungasuca"
                    />
                    <div className="login-brand-text">
                        <span className="login-brand-name">I.E. Tungasuca</span>
                        <span className="login-brand-system">SIS-MESA-PARTES</span>
                    </div>
                </header>

                <div className="login-title">
                    <h1 id="login-titulo">Iniciar sesión</h1>
                    <p>Ingresa tus credenciales para acceder al sistema.</p>
                </div>

                <form onSubmit={handleSubmit} noValidate>

                    {/* CORREO: solicitantes con identidad pendiente de verificación */}
                    {modo === "correo" && (
                        <div className="form-group">
                            <label htmlFor="correo">Correo electrónico</label>
                            <div className="input-box">
                                <svg className="input-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                                    <rect x="3" y="5" width="18" height="14" rx="2" />
                                    <path d="M3 7l9 6 9-6" />
                                </svg>
                                <input
                                    id="correo"
                                    type="email"
                                    value={correo}
                                    onChange={(e) => setCorreo(e.target.value)}
                                    placeholder="tu@correo.com"
                                    autoComplete="username"
                                    disabled={loading}
                                />
                            </div>
                        </div>
                    )}

                    {/* DNI */}
                    {modo === "dni" && (
                    <div className="form-group">
                        <label htmlFor="dni">DNI</label>
                        <div className="input-box">
                            <svg className="input-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                                <circle cx="12" cy="8" r="4" />
                                <path d="M4 21c0-4 3.6-7 8-7s8 3 8 7" />
                            </svg>
                            <input
                                id="dni"
                                type="text"
                                inputMode="numeric"
                                maxLength={8}
                                value={dni}
                                onChange={(e) =>
                                    setDni(e.target.value.replace(/[^0-9]/g, ""))
                                }
                                placeholder="8 dígitos"
                                autoComplete="username"
                                disabled={loading}
                            />
                        </div>
                    </div>
                    )}

                    {/* CONTRASEÑA */}
                    <div className="form-group">
                        <label htmlFor="password">Clave</label>
                        <div className="input-box">
                            <svg className="input-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                                <rect x="5" y="11" width="14" height="10" rx="2" />
                                <path d="M8 11V7a4 4 0 0 1 8 0v4" />
                            </svg>
                            <input
                                id="password"
                                className="input-con-accion"
                                type={mostrarClave ? "text" : "password"}
                                value={password}
                                onChange={(e) =>
                                    setPassword(e.target.value)
                                }
                                placeholder="Ingresa tu clave"
                                autoComplete="current-password"
                                disabled={loading}
                            />
                            <button
                                type="button"
                                className="toggle-clave"
                                onClick={() => setMostrarClave((v) => !v)}
                                aria-label={mostrarClave ? "Ocultar contraseña" : "Mostrar contraseña"}
                                aria-pressed={mostrarClave}
                                aria-controls="password"
                                title={mostrarClave ? "Ocultar contraseña" : "Mostrar contraseña"}
                                disabled={loading}
                            >
                                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                                    <path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12z" />
                                    <circle cx="12" cy="12" r="3" />
                                    {mostrarClave && <path d="M3 3l18 18" />}
                                </svg>
                            </button>
                        </div>
                    </div>

                    {/* ERROR */}
                    {error && (
                        <div className="login-error" role="alert">
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                                <circle cx="12" cy="12" r="9" />
                                <path d="M12 8v5M12 16h.01" />
                            </svg>
                            <p>{error}</p>
                        </div>
                    )}

                    {/* BOTÓN */}
                    <button
                        type="submit"
                        className="login-button"
                        disabled={loading}
                        aria-busy={loading}
                    >
                        {loading && <span className="login-spinner" aria-hidden="true" />}
                        {loading ? "Ingresando..." : "Ingresar"}
                    </button>

                </form>

                <button
                    type="button"
                    className="login-enlace-boton"
                    onClick={() => {
                        setModo((m) => (m === "dni" ? "correo" : "dni"));
                        setError("");
                    }}
                    disabled={loading}
                >
                    {modo === "dni" ? "¿Te registraste con correo? Usa tu correo" : "Usar mi DNI"}
                </button>

                {/* GOOGLE (solo solicitantes): navegación completa al backend,
                    que redirige a Google; el personal sigue con DNI y clave. */}
                <div className="login-separador" role="separator" aria-label="o">
                    <span>o</span>
                </div>
                <a
                    className={`login-google${loading ? " is-disabled" : ""}`}
                    href={GOOGLE_INICIAR_URL}
                    aria-disabled={loading}
                    onClick={(e) => loading && e.preventDefault()}
                >
                    <svg width="18" height="18" viewBox="0 0 48 48" aria-hidden="true">
                        <path fill="#EA4335" d="M24 9.5c3.54 0 6.71 1.22 9.21 3.6l6.85-6.85C35.9 2.38 30.47 0 24 0 14.62 0 6.51 5.38 2.56 13.22l7.98 6.19C12.43 13.72 17.74 9.5 24 9.5z" />
                        <path fill="#4285F4" d="M46.98 24.55c0-1.57-.15-3.09-.38-4.55H24v9.02h12.94c-.58 2.96-2.26 5.48-4.78 7.18l7.73 6c4.51-4.18 7.09-10.36 7.09-17.65z" />
                        <path fill="#FBBC05" d="M10.53 28.59c-.48-1.45-.76-2.99-.76-4.59s.27-3.14.76-4.59l-7.98-6.19C.92 16.46 0 20.12 0 24c0 3.88.92 7.54 2.56 10.78l7.97-6.19z" />
                        <path fill="#34A853" d="M24 48c6.48 0 11.93-2.13 15.89-5.81l-7.73-6c-2.15 1.45-4.92 2.3-8.16 2.3-6.26 0-11.57-4.22-13.47-9.91l-7.98 6.19C6.51 42.62 14.62 48 24 48z" />
                    </svg>
                    Continuar con Google
                </a>
                <p className="login-google-nota">Para solicitantes. El personal de la institución ingresa con su DNI y su clave.</p>

                <p className="login-help">
                    ¿Eres solicitante y no tienes cuenta? <Link to="/registro">Crear cuenta con correo</Link>
                </p>

            </section>

            <footer className="login-footer">
                © 2026 I.E. Tungasuca
            </footer>

        </main>
    );
}

export default Login;
