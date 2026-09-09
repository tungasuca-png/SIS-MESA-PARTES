import { Navigate, useNavigate } from "react-router-dom";
import PortalLayout from "../../components/portal/PortalLayout";
import { useAuth } from "../../hooks/useAuth";
import "./mesaDePartes.css";

// Acciones del solicitante — NO son tarjetas KPI (sin conteos globales, sin
// datos de otros usuarios): cada bloque es simplemente un atajo a una
// función real que ya existe (/fut, /expedientes, .../seguimiento), acotada
// al propio usuario por el backend.
// No hay una tarjeta "Documentos" aparte: sus documentos se consultan desde
// el detalle de cada expediente (panel de Documentos en ExpedienteDetail).
const ACCIONES = [
    {
        key: "registrar",
        titulo: "Registrar solicitud",
        descripcion: "Presente un nuevo documento o solicitud ante la institución.",
        boton: "Iniciar trámite",
        to: "/fut",
        icon: "plus",
    },
    {
        key: "expedientes",
        titulo: "Mis expedientes",
        descripcion: "Consulte los expedientes que ha presentado y su estado actual.",
        boton: "Consultar",
        to: "/expedientes",
        icon: "folder",
    },
    {
        key: "seguimiento",
        titulo: "Seguimiento",
        descripcion: "Consulte el proceso y los movimientos de sus solicitudes.",
        boton: "Ver seguimiento",
        to: "/mesa-de-partes/seguimiento",
        icon: "trending",
    },
];

function MesaPartesHome() {
    const { user } = useAuth();
    const navigate = useNavigate();

    // Esta página es el portal del SOLICITANTE; cualquier otro rol se
    // redirige a su propio Dashboard en vez de mostrarle un portal que no
    // es el suyo.
    if (user && user.role !== "SOLICITANTE") {
        return <Navigate to="/dashboard" replace />;
    }

    return (
        <PortalLayout title="Mesa de Partes Virtual">
            <p className="mdp-intro">
                Aquí puede presentar documentos y consultar el estado de sus solicitudes ante el I.E. TUNGASUCA.
            </p>

            <div className="mdp-acciones-grid">
                {ACCIONES.map((accion) => (
                    <article key={accion.key} className="mdp-accion-card">
                        <h2>{accion.titulo}</h2>
                        <p>{accion.descripcion}</p>
                        <button type="button" className="dp-btn-primary" onClick={() => navigate(accion.to)}>
                            {accion.boton}
                        </button>
                    </article>
                ))}
            </div>
        </PortalLayout>
    );
}

export default MesaPartesHome;
