
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

            <section className="login-container">

                {/* PANEL IZQUIERDO */}
                <div className="login-brand">

                    <div className="brand-content">

                        <div className="brand-logo">
                            <img
                                src="/logo-tungasuca.png"
                                alt="Logo de la I.E. Tungasuca"
                            />
                        </div>

                        <h1>
                            I.E. TUNGASUCA
                        </h1>

                        <div className="brand-line">
                            <span></span>
                            <span></span>
                            <span></span>
                        </div>

                        <h2>
                            Plataforma Educativa
                        </h2>

                        <p>
                            Sistema de gestión institucional
                        </p>

                        <div className="brand-description">
                            <p>
                                Accede al sistema para gestionar
                                información, documentos y procesos
                                de la institución educativa.
                            </p>
                        </div>

                    </div>

                    <div className="brand-footer">
                        © 2026 I.E. Tungasuca
                    </div>

                </div>


                {/* PANEL DERECHO */}
                <div className="login-form-panel">

                    <div className="form-content">

                        <div className="form-title">

                            <span className="welcome">
                                BIENVENIDO
                            </span>

                            <h2>
                                Iniciar sesión
                            </h2>

                            <p>
                                Ingresa tus credenciales para acceder
                                al sistema.
                            </p>

                        </div>


                        <form onSubmit={handleSubmit}>

                            {/* DNI */}
                            <div className="form-group">

                                <label htmlFor="dni">
                                    DNI
                                </label>

                                <div className="input-box">

                                    <span className="input-icon">
                                        👤
                                    </span>

                                    <input
                                        id="dni"
                                        type="text"
                                        inputMode="numeric"
                                        maxLength={8}
                                        value={dni}
                                        onChange={(e) =>
                                            setDni(e.target.value.replace(/[^0-9]/g, ""))
                                        }
                                        placeholder="Ingrese su DNI"
                                        autoComplete="username"
                                        disabled={loading}
                                    />

                                </div>

                            </div>


                            {/* CONTRASEÑA */}
                            <div className="form-group">

                                <label htmlFor="password">
                                    Clave
                                </label>

                                <div className="input-box">

                                    <span className="input-icon">
                                        🔒
                                    </span>

                                    <input
                                        id="password"
                                        className="input-con-accion"
                                        type={mostrarClave ? "text" : "password"}
                                        value={password}
                                        onChange={(e) =>
                                            setPassword(e.target.value)
                                        }
                                        placeholder="Ingrese su clave"
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
                                        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                                            <path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12z" />
                                            <circle cx="12" cy="12" r="3" />
                                            {mostrarClave && <path d="M3 3l18 18" />}
                                        </svg>
                                    </button>

                                </div>

                            </div>


                            {/* ERROR */}
                            {error && (
                                <div className="login-error">
                                    <span>!</span>
                                    <p>{error}</p>
                                </div>
                            )}


                            {/* BOTÓN */}
                            <button
                                type="submit"
                                className="login-button"
                                disabled={loading}
                            >
                                {loading
                                    ? "INGRESANDO..."
                                    : "INGRESAR AL SISTEMA"}
                            </button>

                        </form>


                        <div className="form-footer">

                            <p>
                                ¿No tienes una cuenta?
                            </p>

                            <span>
                                Solicita tus credenciales a la
                                institución educativa.
                            </span>

                        </div>

                    </div>

                </div>

            </section>

        </main>
    );
}

export default Login;

