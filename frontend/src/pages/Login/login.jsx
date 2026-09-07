
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../../hooks/useAuth";
import "./login.css";

function Login() {
    const navigate = useNavigate();
    const { login: loginUser } = useAuth();

    const [username, setUsername] = useState("");
    const [password, setPassword] = useState("");
    const [error, setError] = useState("");
    const [loading, setLoading] = useState(false);

    const handleSubmit = async (e) => {
        e.preventDefault();
        setError("");

        if (!username.trim() || !password.trim()) {
            setError("Ingresa tu usuario y contraseña");
            return;
        }

        setLoading(true);

        try {
            await loginUser(username, password);
            navigate("/dashboard");
        } catch (err) {
            console.error(err);
            setError("Usuario o contraseña incorrectos");
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

                            {/* USUARIO */}
                            <div className="form-group">

                                <label htmlFor="username">
                                    Usuario
                                </label>

                                <div className="input-box">

                                    <span className="input-icon">
                                        👤
                                    </span>

                                    <input
                                        id="username"
                                        type="text"
                                        value={username}
                                        onChange={(e) =>
                                            setUsername(e.target.value)
                                        }
                                        placeholder="Ingrese su usuario"
                                        autoComplete="username"
                                        disabled={loading}
                                    />

                                </div>

                            </div>


                            {/* CONTRASEÑA */}
                            <div className="form-group">

                                <label htmlFor="password">
                                    Contraseña
                                </label>

                                <div className="input-box">

                                    <span className="input-icon">
                                        🔒
                                    </span>

                                    <input
                                        id="password"
                                        type="password"
                                        value={password}
                                        onChange={(e) =>
                                            setPassword(e.target.value)
                                        }
                                        placeholder="Ingrese su contraseña"
                                        autoComplete="current-password"
                                        disabled={loading}
                                    />

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

