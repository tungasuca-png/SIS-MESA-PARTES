import "./login.css";

// Tarjeta con la identidad institucional de la pantalla de acceso, para las
// pantallas del acceso con Google (mismo diseño que el login).
function TarjetaAcceso({ titulo, descripcion, ancha = false, children }) {
    return (
        <main className="login-page">
            <section className={`login-card${ancha ? " login-card--registro" : ""}`} aria-labelledby="acceso-titulo">
                <header className="login-header">
                    <img className="login-logo" src="/logo-tungasuca.png" alt="Logo de la I.E. Tungasuca" />
                    <div className="login-brand-text">
                        <span className="login-brand-name">I.E. Tungasuca</span>
                        <span className="login-brand-system">SIS-MESA-PARTES</span>
                    </div>
                </header>
                <div className="login-title">
                    <h1 id="acceso-titulo">{titulo}</h1>
                    {descripcion && <p>{descripcion}</p>}
                </div>
                {children}
            </section>
            <footer className="login-footer">© 2026 I.E. Tungasuca</footer>
        </main>
    );
}

export default TarjetaAcceso;
