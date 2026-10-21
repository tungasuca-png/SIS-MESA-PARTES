
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../../hooks/useAuth";
import { extractBackendMessage } from "../../utils/apiErrors";
import "./login.css";

function Login() {
    const navigate = useNavigate();
    const { login: loginUser } = useAuth();

    const [dni, setDni] = useState("");
    const [password, setPassword] = useState("");
    const [mostrarClave, setMostrarClave] = useState(false);
    const [error, setError] = useState("");
    const [loading, setLoading] = useState(false);

    const handleSubmit = async (e) => {
        e.preventDefault();
        setError("");

        const dniLimpio = dni.trim();
        if (!dniLimpio || !password) {
            setError("Ingresa tu DNI y tu clave");
            return;
        }
        if (!/^[0-9]{8}$/.test(dniLimpio)) {
            setError("El DNI debe tener 8 dígitos numéricos");
            return;
        }

        setLoading(true);

        try {
            const loggedInUser = await loginUser(dniLimpio, password);
            // El SOLICITANTE entra a su propio portal (Mesa de Partes
            // Virtual, sin sidebar administrativo); el resto de roles sigue
            // entrando al Dashboard de siempre.
            navigate(loggedInUser.role === "SOLICITANTE" ? "/mesa-de-partes" : "/dashboard");
        } catch (err) {
            // Mensaje genérico: no revela si el DNI existe.
            if (!err?.response) {
                setError("No se pudo conectar con el servidor. Inténtalo nuevamente.");
            } else if (err.response.status === 400) {
                setError("El DNI debe tener 8 dígitos numéricos");
            } else if (err.response.status === 429) {
                // Auth bloquea la cuenta 15 minutos tras 5 intentos fallidos
                // ("account_locked: ... en N minutos").
                const minutos = extractBackendMessage(err.response.data)?.match(/en (\d+) minuto/)?.[1];
                setError(
                    `Cuenta bloqueada temporalmente por demasiados intentos fallidos. ${
                        minutos ? `Intenta de nuevo en ${minutos} ${minutos === "1" ? "minuto" : "minutos"}.` : "Intenta de nuevo más tarde."
                    }`
                );
            } else if (err.response.status >= 500) {
                setError("El servicio no está disponible en este momento. Inténtalo nuevamente.");
            } else {
                setError("DNI o clave incorrectos");
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

                    {/* DNI */}
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

                <p className="login-help">
                    ¿No tienes una cuenta? Solicita tus credenciales a la institución educativa.
                </p>

            </section>

            <footer className="login-footer">
                © 2026 I.E. Tungasuca
            </footer>

        </main>
    );
}

export default Login;
