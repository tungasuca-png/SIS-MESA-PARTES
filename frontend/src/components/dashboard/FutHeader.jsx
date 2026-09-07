import "./futHeader.css";

// Reproduce el encabezado del FUT físico de la I.E. Tungasuca. Los textos
// (dirección, teléfono, la frase del año) son fieles al documento físico
// provisto — no se cambian ni se resumen.
function FutHeader({ numero }) {
    return (
        <div className="fut-header">
            <div className="fut-header-institucion">
                <p className="fut-header-tipo">INSTITUCIÓN EDUCATIVA</p>
                <p className="fut-header-nombre">TUNGASUCA</p>
                <p className="fut-header-linea">UGEL 04 – CARABAYLLO</p>
                <p className="fut-header-linea">Av. Mariano Condorcanqui S/N</p>
                <p className="fut-header-linea">Urb. Tungasuca – Carabayllo</p>
                <p className="fut-header-linea">5441531</p>
            </div>

            <div className="fut-header-anio">
                <p>
                    “Año de la
                    <br />
                    Esperanza y el
                    <br />
                    Fortalecimiento de
                    <br />
                    la Democracia”
                </p>
            </div>

            <div className="fut-header-numero">
                <span className="fut-header-numero-label">N°</span>
                <span className={`fut-header-numero-valor ${numero ? "" : "fut-header-numero-pendiente"}`}>
                    {numero || "Se genera al enviar"}
                </span>
            </div>
        </div>
    );
}

export default FutHeader;
