import { useState } from "react";
import { useSearchParams } from "react-router-dom";
import DashboardLayout from "../../components/dashboard/DashboardLayout";
import "../../components/dashboard/forms.css";
import "../../components/dashboard/RecentExpedientes.css";
import "../Dashboard/dashboard.css";
import "./reportes.css";
import { ESTADOS, PRIORIDADES, TIPOS } from "../../constants/expedientes";
import {
    exportarReporteActividad,
    exportarReporteComparacion,
    exportarReporteDerivaciones,
    exportarReporteDocumentos,
    exportarReporteExpedientes,
} from "../../services/reportesService";
import { descargarBase64 } from "../../utils/descargas";
import {
    AREAS,
    FILTROS_INICIALES,
    GRANULARIDADES,
    MODOS_COMPARACION,
    VISTAS,
    atajosPeriodo,
    etiquetaTipo,
    fechaISO,
    filtrosResumen,
    mensajeError,
    textoFiltros,
    textoPeriodo,
    validarComparacion,
    validarFiltros,
    vistaPorId,
} from "./reportesData";
import ResumenVista from "./vistas/ResumenVista";
import ExpedientesVista from "./vistas/ExpedientesVista";
import DerivacionesVista from "./vistas/DerivacionesVista";
import DocumentosVista from "./vistas/DocumentosVista";
import ActividadVista from "./vistas/ActividadVista";
import ComparacionVista from "./vistas/ComparacionVista";

// Reportes: pestañas sobre un panel de filtros compartido. El Resumen es el
// de la Fase 1 (sin cambios); el resto es la Fase 2. Cada pestaña consulta
// su propio endpoint y exporta con exactamente los filtros aplicados que le
// corresponden. No se muestran tiempos de atención ni cumplimiento de plazos:
// el sistema todavía no guarda el historial de estados ni plazos.

const COMPONENTES = {
    resumen: ResumenVista,
    expedientes: ExpedientesVista,
    derivaciones: DerivacionesVista,
    documentos: DocumentosVista,
    actividad: ActividadVista,
    comparacion: ComparacionVista,
};

const periodoTipo = (f) => ({ desde: f.desde, hasta: f.hasta, tipo: f.tipo });

// Exportación de cada pestaña: función y filtros que le aplican.
const EXPORTAR = {
    resumen: (f, formato) => exportarReporteExpedientes(filtrosResumen(f), formato),
    expedientes: (f, formato) => exportarReporteExpedientes({ ...filtrosResumen(f), vista: "EXPEDIENTES", granularidad: f.granularidad }, formato),
    derivaciones: (f, formato) => exportarReporteDerivaciones({ ...periodoTipo(f), granularidad: f.granularidad }, formato),
    documentos: (f, formato) => exportarReporteDocumentos({ ...periodoTipo(f), granularidad: f.granularidad }, formato),
    actividad: (f, formato) => exportarReporteActividad(periodoTipo(f), formato),
    comparacion: (f, formato) => exportarReporteComparacion({ ...filtrosResumen(f), modo: f.modo, desdeB: f.desdeB, hastaB: f.hastaB }, formato),
};

function textoFiltrosVista(vista, f) {
    if (vista.filtros.includes("estado")) return textoFiltros(f);
    return f.tipo ? `Tipo: ${etiquetaTipo(f.tipo)}` : "Sin filtros adicionales";
}

function Reportes() {
    const [searchParams, setSearchParams] = useSearchParams();
    const vista = vistaPorId(searchParams.get("vista"));
    const [borrador, setBorrador] = useState(FILTROS_INICIALES);
    const [aplicados, setAplicados] = useState(FILTROS_INICIALES);
    const [errorFiltros, setErrorFiltros] = useState("");
    const [exportando, setExportando] = useState("");
    const [errorExport, setErrorExport] = useState("");
    const hoy = fechaISO(new Date());
    const aplica = (campo) => vista.filtros.includes(campo);
    const VistaActiva = COMPONENTES[vista.id];

    const cambiar = (campo) => (event) => setBorrador((prev) => ({ ...prev, [campo]: event.target.value }));

    const consultar = (filtros) => {
        const error = validarFiltros(filtros);
        setErrorFiltros(error);
        if (error) return;
        setErrorExport("");
        setAplicados({ ...filtros });
    };

    const aplicar = (event) => {
        event.preventDefault();
        consultar(borrador);
    };

    const limpiar = () => {
        setBorrador(FILTROS_INICIALES);
        consultar(FILTROS_INICIALES);
    };

    const cambiarVista = (id) => {
        setErrorExport("");
        setSearchParams(id === "resumen" ? {} : { vista: id }, { replace: true });
    };

    const usarAtajo = (atajo) => setBorrador((prev) => ({ ...prev, desde: atajo.desde, hasta: atajo.hasta }));

    const exportar = async (formato) => {
        setExportando(formato);
        setErrorExport("");
        try {
            const archivo = await EXPORTAR[vista.id](aplicados, formato);
            descargarBase64(archivo.contenido_base64, archivo.nombre, archivo.mime);
        } catch (err) {
            setErrorExport(`No se pudo exportar el reporte. ${mensajeError(err)}`);
        } finally {
            setExportando("");
        }
    };

    const sinPeriodoComparable = vista.id === "comparacion" && Boolean(validarComparacion(aplicados));
    const puedeExportar = !exportando && !sinPeriodoComparable;
    const noAplica = "No aplica a esta pestaña";

    return (
        <DashboardLayout title="Reportes">
            <div className="db-intro rp-intro">
                <div>
                    <h2 className="db-intro-title">Reportes de Mesa de Partes</h2>
                    <p className="db-intro-text">Estadísticas agregadas de expedientes, derivaciones y documentos. Cada pestaña indica qué fecha utiliza.</p>
                </div>
                <div className="db-intro-actions rp-acciones">
                    <button type="button" className="dp-btn-secondary db-btn" disabled={!puedeExportar} onClick={() => exportar("xlsx")}>
                        {exportando === "xlsx" ? "Generando..." : "Exportar Excel"}
                    </button>
                    <button type="button" className="dp-btn-secondary db-btn" disabled={!puedeExportar} onClick={() => exportar("csv")}>
                        {exportando === "csv" ? "Generando..." : "Exportar CSV"}
                    </button>
                    <button type="button" className="dp-btn-secondary db-btn" disabled={!puedeExportar} onClick={() => exportar("pdf")}>
                        {exportando === "pdf" ? "Generando..." : "Exportar PDF"}
                    </button>
                    <button type="button" className="dp-btn-primary db-btn" onClick={() => window.print()}>
                        Imprimir
                    </button>
                </div>
            </div>

            <div className="rp-pestanas" role="tablist" aria-label="Secciones de reportes">
                {VISTAS.map((v) => (
                    <button
                        key={v.id}
                        type="button"
                        role="tab"
                        id={`rp-tab-${v.id}`}
                        aria-selected={v.id === vista.id}
                        aria-controls="rp-panel-vista"
                        className={`rp-pestana ${v.id === vista.id ? "is-active" : ""}`}
                        onClick={() => cambiarVista(v.id)}
                    >
                        {v.label}
                    </button>
                ))}
            </div>

            <form className="dp-panel rp-filtros" onSubmit={aplicar} aria-label="Filtros del reporte" noValidate>
                <div className="rp-filtros-grid">
                    <label className="rp-campo">
                        <span>Desde</span>
                        <input type="date" value={borrador.desde} max={hoy} onChange={cambiar("desde")} />
                    </label>
                    <label className="rp-campo">
                        <span>Hasta</span>
                        <input type="date" value={borrador.hasta} onChange={cambiar("hasta")} />
                    </label>
                    <label className="rp-campo" title={aplica("estado") ? undefined : noAplica}>
                        <span>Estado</span>
                        <select value={borrador.estado} onChange={cambiar("estado")} disabled={!aplica("estado")}>
                            <option value="">Todos</option>
                            {ESTADOS.map((e) => (
                                <option key={e.value} value={e.value}>{e.label}</option>
                            ))}
                        </select>
                    </label>
                    <label className="rp-campo">
                        <span>Tipo de trámite</span>
                        <select value={borrador.tipo} onChange={cambiar("tipo")}>
                            <option value="">Todos</option>
                            {TIPOS.map((t) => (
                                <option key={t.value} value={t.value}>{t.label}</option>
                            ))}
                        </select>
                    </label>
                    <label className="rp-campo" title={aplica("prioridad") ? undefined : noAplica}>
                        <span>Prioridad</span>
                        <select value={borrador.prioridad} onChange={cambiar("prioridad")} disabled={!aplica("prioridad")}>
                            <option value="">Todas</option>
                            {PRIORIDADES.map((p) => (
                                <option key={p.value} value={p.value}>{p.label}</option>
                            ))}
                        </select>
                    </label>
                    <label className="rp-campo" title={aplica("area") ? undefined : noAplica}>
                        <span>Área actual</span>
                        <select value={borrador.area} onChange={cambiar("area")} disabled={!aplica("area")}>
                            <option value="">Todas</option>
                            {AREAS.map((a) => (
                                <option key={a.value} value={a.value}>{a.label}</option>
                            ))}
                        </select>
                    </label>
                    {aplica("granularidad") && (
                        <label className="rp-campo">
                            <span>Agrupar por</span>
                            <select value={borrador.granularidad} onChange={cambiar("granularidad")}>
                                {GRANULARIDADES.map((g) => (
                                    <option key={g.value} value={g.value}>{g.label}</option>
                                ))}
                            </select>
                        </label>
                    )}
                    {aplica("modo") && (
                        <label className="rp-campo rp-campo--doble">
                            <span>Comparar con</span>
                            <select value={borrador.modo} onChange={cambiar("modo")}>
                                {MODOS_COMPARACION.map((m) => (
                                    <option key={m.value} value={m.value}>{m.label}</option>
                                ))}
                            </select>
                        </label>
                    )}
                    {aplica("modo") && borrador.modo === "PERSONALIZADO" && (
                        <>
                            <label className="rp-campo">
                                <span>Referencia desde</span>
                                <input type="date" value={borrador.desdeB} max={hoy} onChange={cambiar("desdeB")} />
                            </label>
                            <label className="rp-campo">
                                <span>Referencia hasta</span>
                                <input type="date" value={borrador.hastaB} onChange={cambiar("hastaB")} />
                            </label>
                        </>
                    )}
                </div>
                <div className="rp-filtros-pie">
                    <div className="rp-atajos" role="group" aria-label="Periodos rápidos">
                        {atajosPeriodo().map((a) => (
                            <button key={a.id} type="button" className="rp-atajo" onClick={() => usarAtajo(a)} title={textoPeriodo(a)}>
                                {a.label}
                            </button>
                        ))}
                    </div>
                    <div className="rp-filtros-botones">
                        <button type="button" className="dp-btn-secondary db-btn" onClick={limpiar}>
                            Limpiar filtros
                        </button>
                        <button type="submit" className="dp-btn-primary db-btn">
                            Aplicar filtros
                        </button>
                    </div>
                </div>
                {errorFiltros && <p className="dp-form-error" role="alert">{errorFiltros}</p>}
                {!aplica("estado") && (
                    <p className="rp-aplicados">En esta pestaña solo aplican el periodo{aplica("granularidad") ? ", la agrupación" : ""} y el tipo de trámite.</p>
                )}
                <p className="rp-aplicados">
                    Mostrando: <strong>{textoPeriodo(aplicados)}</strong> · {textoFiltrosVista(vista, aplicados)}
                </p>
            </form>

            {errorExport && (
                <p className="db-alert db-alert--error rp-acciones" role="alert">{errorExport}</p>
            )}

            <div id="rp-panel-vista" role="tabpanel" aria-labelledby={`rp-tab-${vista.id}`}>
                <VistaActiva key={vista.id} aplicados={aplicados} />
            </div>
        </DashboardLayout>
    );
}

export default Reportes;
