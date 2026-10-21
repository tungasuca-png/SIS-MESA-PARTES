import { useEffect, useRef, useState } from "react";
import { Link, Navigate, useNavigate } from "react-router-dom";
import DashboardLayout from "../../components/dashboard/DashboardLayout";
import StatCard from "../../components/dashboard/StatCard";
import RecentExpedientes from "../../components/dashboard/RecentExpedientes";
import ModuleCards from "../../components/dashboard/ModuleCards";
import LineChart from "../../components/dashboard/charts/LineChart";
import BarList from "../../components/dashboard/charts/BarList";
import Icon from "../../components/dashboard/Icon";
import "../../components/dashboard/forms.css";
import { useAuth } from "../../hooks/useAuth";
import { usePermissions } from "../../hooks/usePermissions";
import { useActualizacionEnVivo } from "../../hooks/useActualizacionEnVivo";
import { friendlyErrorMessage } from "../../utils/apiErrors";
import { roleLabel } from "../../constants/roles";
import { cargarPorTipo, cargarResumen, cargarSerieDiaria, PERIODOS } from "./dashboardData";
import "./dashboard.css";

// Este es el Dashboard ADMINISTRATIVO: personal interno (ADMIN, DIRECTOR,
// SUBDIRECTOR, SECRETARIA, DOCENTE, AUXILIAR). Un SOLICITANTE nunca debe
// entrar aquí — tiene su propio portal sin sidebar administrativo (ver
// PortalLayout / /mesa-de-partes) — así que se lo redirige antes de cargar
// nada de este componente.
//
// Todas las cifras salen del listado real de expedientes (ver
// dashboardData.js). No se muestran variaciones (%) contra periodos
// anteriores ni tiempos de atención: el backend no entrega historia de
// estados ni una fecha de atención, y mostrarlos sería inventar números.
// Los indicadores y gráficos de Mesa de Partes solo aparecen con
// "expedientes.view"; el backend además acota qué expedientes cuenta.

// Tarjetas por estado: enlazan a la bandeja real con ese filtro.
const STAT_ESTADOS = [
    { estado: "PENDIENTE", label: "Por revisar", icon: "inbox", tone: "pendiente" },
    { estado: "EN_PROCESO", label: "En proceso", icon: "loader", tone: "proceso" },
    { estado: "OBSERVADO", label: "Observados", icon: "eye", tone: "observado" },
    { estado: "ATENDIDO", label: "Atendidos", icon: "check", tone: "atendido" },
];

// Los gráficos son más costosos (varias consultas): se refrescan cada
// 6 ciclos en vivo (~1 minuto), no en cada uno como los indicadores.
const CICLOS_POR_REFRESCO_GRAFICOS = 6;

// Mismo criterio que el backend (authorization.CanViewAll / AreaDelRol),
// solo para explicar el alcance de las cifras; quien acota es el backend.
function textoAlcance(role) {
    if (role === "ADMIN" || role === "SECRETARIA") return "Expedientes activos de toda la institución.";
    if (["DIRECTOR", "SUBDIRECTOR", "DOCENTE", "AUXILIAR"].includes(role)) {
        return `Expedientes derivados actualmente a ${roleLabel(role)}.`;
    }
    return "Expedientes a los que tienes acceso.";
}

const porcentaje = (valor, total) => (total > 0 ? `${Math.round((valor * 100) / total)}% del total` : "Sin registros");

const formatoDia = new Intl.DateTimeFormat("es-PE", { day: "2-digit", month: "short" });
const formatoDiaLargo = new Intl.DateTimeFormat("es-PE", { weekday: "short", day: "2-digit", month: "long" });
const formatoHora = new Intl.DateTimeFormat("es-PE", { hour: "2-digit", minute: "2-digit" });

function EstadoPanel({ cargando, error, vacio, children }) {
    if (cargando) return <div className="db-skeleton" aria-busy="true" aria-label="Cargando" />;
    if (error) return <p className="db-panel-msg db-panel-msg--error">No se pudo cargar el gráfico. {error}</p>;
    if (vacio) return <p className="db-panel-msg">{vacio}</p>;
    return children;
}

function Dashboard() {
    const { user } = useAuth();
    const { can } = usePermissions();
    const navigate = useNavigate();
    const esSolicitante = user?.role === "SOLICITANTE";
    const verMetricas = can("expedientes.view") && !esSolicitante;

    const [resumen, setResumen] = useState(null);
    const [errorResumen, setErrorResumen] = useState("");
    const [actualizado, setActualizado] = useState(null);
    const [tipos, setTipos] = useState({ cargando: true, error: "", items: [] });
    const [periodo, setPeriodo] = useState(30);
    const [serie, setSerie] = useState({ cargando: true, error: "", puntos: [], completo: true });

    // En vivo: indicadores y recientes en cada ciclo; gráficos cada N ciclos.
    const [vivo, setVivo] = useState(0);
    const [refrescoGraficos, setRefrescoGraficos] = useState(0);
    const ciclos = useRef(0);
    useActualizacionEnVivo(() => {
        setVivo((n) => n + 1);
        ciclos.current += 1;
        if (ciclos.current % CICLOS_POR_REFRESCO_GRAFICOS === 0) setRefrescoGraficos((n) => n + 1);
    }, verMetricas);

    useEffect(() => {
        if (!verMetricas) return undefined;
        let activo = true;
        cargarResumen()
            .then((datos) => {
                if (!activo) return;
                setResumen(datos);
                setErrorResumen("");
                setActualizado(new Date());
            })
            .catch((err) => {
                if (activo) setErrorResumen(friendlyErrorMessage(err));
            });
        return () => {
            activo = false;
        };
    }, [vivo, verMetricas]);

    useEffect(() => {
        if (!verMetricas) return undefined;
        let activo = true;
        cargarPorTipo()
            .then((items) => activo && setTipos({ cargando: false, error: "", items }))
            .catch((err) => activo && setTipos((prev) => ({ ...prev, cargando: false, error: friendlyErrorMessage(err) })));
        return () => {
            activo = false;
        };
    }, [refrescoGraficos, verMetricas]);

    useEffect(() => {
        if (!verMetricas) return undefined;
        let activo = true;
        cargarSerieDiaria(periodo)
            .then(({ puntos, completo }) => activo && setSerie({ cargando: false, error: "", puntos, completo }))
            .catch((err) => activo && setSerie((prev) => ({ ...prev, cargando: false, error: friendlyErrorMessage(err) })));
        return () => {
            activo = false;
        };
    }, [periodo, refrescoGraficos, verMetricas]);

    if (esSolicitante) {
        return <Navigate to="/mesa-de-partes" replace />;
    }

    const cambiarPeriodo = (dias) => {
        if (dias === periodo) return;
        setSerie((prev) => ({ ...prev, cargando: true, error: "" }));
        setPeriodo(dias);
    };

    const actualizarAhora = () => {
        setVivo((n) => n + 1);
        setRefrescoGraficos((n) => n + 1);
    };

    if (!verMetricas) {
        return (
            <DashboardLayout title="Dashboard">
                <div className="db-intro">
                    <div>
                        <h2 className="db-intro-title">Panel principal</h2>
                        <p className="db-intro-text">Accede a los módulos habilitados para tu rol.</p>
                    </div>
                </div>
                <div className="db-solo-accesos">
                    <ModuleCards title="Tus módulos" />
                </div>
            </DashboardLayout>
        );
    }

    const total = resumen?.total ?? 0;
    const puntosSerie = serie.puntos.map((p) => ({
        key: p.fecha,
        label: formatoDia.format(p.dia),
        labelLargo: formatoDiaLargo.format(p.dia),
        value: p.total,
    }));
    const registradosPeriodo = serie.puntos.reduce((suma, p) => suma + p.total, 0);

    return (
        <DashboardLayout title="Dashboard">
            <div className="db-intro">
                <div>
                    <h2 className="db-intro-title">Resumen de Mesa de Partes</h2>
                    <p className="db-intro-text">
                        {textoAlcance(user?.role)}
                        {actualizado && <span className="db-intro-time"> · Actualizado {formatoHora.format(actualizado)}</span>}
                    </p>
                </div>
                <div className="db-intro-actions">
                    <button type="button" className="dp-btn-secondary db-btn" onClick={actualizarAhora}>
                        Actualizar
                    </button>
                    {can("expedientes.create") && (
                        <button
                            type="button"
                            className="dp-btn-primary db-btn"
                            onClick={() => navigate("/expedientes", { state: { openCreate: true } })}
                        >
                            <Icon name="plus" size={15} />
                            Nuevo expediente
                        </button>
                    )}
                </div>
            </div>

            {!resumen && !errorResumen && (
                <div className="db-kpis" aria-busy="true" aria-label="Cargando indicadores">
                    {[0, 1, 2, 3, 4].map((i) => (
                        <div key={i} className="db-skeleton db-skeleton--kpi" />
                    ))}
                </div>
            )}

            {!resumen && errorResumen && (
                <div className="db-alert db-alert--error" role="alert">
                    <span>No se pudieron cargar los indicadores. {errorResumen}</span>
                    <button type="button" className="dp-btn-secondary db-btn" onClick={actualizarAhora}>
                        Reintentar
                    </button>
                </div>
            )}

            {resumen && (
                <>
                    {errorResumen && (
                        <p className="db-alert db-alert--warn" role="status">
                            No se pudo actualizar; se muestran los últimos datos obtenidos.
                        </p>
                    )}

                    <div className="db-kpis">
                        <StatCard
                            icon="folder"
                            label="Total de expedientes"
                            value={total}
                            hint={
                                resumen.urgentesAbiertos > 0
                                    ? `${resumen.urgentesAbiertos} urgente${resumen.urgentesAbiertos === 1 ? "" : "s"} sin atender`
                                    : "Sin urgentes pendientes"
                            }
                            to="/expedientes"
                        />
                        {STAT_ESTADOS.map((def) => (
                            <StatCard
                                key={def.estado}
                                icon={def.icon}
                                label={def.label}
                                tone={def.tone}
                                value={resumen.porEstado[def.estado] ?? 0}
                                hint={porcentaje(resumen.porEstado[def.estado] ?? 0, total)}
                                to={`/expedientes?estado=${def.estado}`}
                            />
                        ))}
                    </div>
                </>
            )}

            <div className="db-charts">
                <section className="dp-panel db-chart-panel" aria-labelledby="db-serie-title">
                    <div className="dp-panel-head">
                        <div>
                            <h2 id="db-serie-title" className="dp-panel-title">Expedientes registrados</h2>
                            <p className="db-panel-sub">
                                {serie.cargando || serie.error
                                    ? "Por día de registro"
                                    : `${registradosPeriodo} en los últimos ${periodo} días`}
                            </p>
                        </div>
                        <div className="db-segmented" role="group" aria-label="Periodo">
                            {PERIODOS.map((p) => (
                                <button
                                    key={p.dias}
                                    type="button"
                                    className={p.dias === periodo ? "is-active" : ""}
                                    aria-pressed={p.dias === periodo}
                                    onClick={() => cambiarPeriodo(p.dias)}
                                >
                                    {p.label}
                                </button>
                            ))}
                        </div>
                    </div>
                    <EstadoPanel cargando={serie.cargando} error={serie.error}>
                        <LineChart
                            puntos={puntosSerie}
                            alto={250}
                            descripcion={`Expedientes registrados por día en los últimos ${periodo} días`}
                        />
                        {!serie.completo && (
                            <p className="db-chart-note">
                                Serie parcial: el periodo supera los 1000 registros que se consultan como máximo.
                            </p>
                        )}
                    </EstadoPanel>
                </section>

                <section className="dp-panel db-chart-panel" aria-labelledby="db-tipos-title">
                    <div className="dp-panel-head">
                        <div>
                            <h2 id="db-tipos-title" className="dp-panel-title">Por tipo de trámite</h2>
                            <p className="db-panel-sub">Del total de expedientes</p>
                        </div>
                    </div>
                    <EstadoPanel
                        cargando={tipos.cargando}
                        error={tipos.error}
                        vacio={tipos.items.length === 0 ? "Todavía no hay expedientes registrados." : ""}
                    >
                        <BarList items={tipos.items} total={total} descripcion="Expedientes por tipo de trámite" />
                    </EstadoPanel>
                </section>
            </div>

            <div className="db-bottom">
                <RecentExpedientes
                    items={resumen?.recientes ?? []}
                    emptyMessage={resumen ? "No hay expedientes registrados." : errorResumen ? "No disponible por el momento." : "Cargando..."}
                    action={
                        <Link to="/expedientes" className="db-link">
                            Ver todos
                        </Link>
                    }
                />
                <ModuleCards />
            </div>
        </DashboardLayout>
    );
}

export default Dashboard;
