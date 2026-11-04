import StatCard from "../../../components/dashboard/StatCard";
import BarList from "../../../components/dashboard/charts/BarList";
import LineChart from "../../../components/dashboard/charts/LineChart";
import { getReporteExpedientes } from "../../../services/reportesService";
import {
    RANGOS_ANTIGUEDAD,
    aItems,
    cantidadDe,
    etiquetaArea,
    etiquetaEstado,
    etiquetaMes,
    etiquetaPrioridad,
    etiquetaTipo,
    filtrosResumen,
    porcentaje,
    textoAlcance,
    textoFiltros,
    textoPeriodo,
} from "../reportesData";
import { EncabezadoImpresion, EstadoCarga, InfoVista, Panel, SinDatos } from "./comun";
import { useReporte } from "./reporteHooks";

// Resumen (Fase 1, sin cambios de contenido): estadísticas de expedientes
// por fecha de registro. Usa solo los filtros de la Fase 1.
function ResumenVista({ aplicados }) {
    const filtros = filtrosResumen(aplicados);
    const { datos: reporte, error, cargando, reintentar } = useReporte(() => getReporteExpedientes(filtros), JSON.stringify(filtros));

    const total = reporte?.total ?? 0;
    const pendientes = reporte?.pendientes ?? 0;
    const atendidos = cantidadDe(reporte?.por_estado, "ATENDIDO");
    const observados = cantidadDe(reporte?.por_estado, "OBSERVADO");
    const sinResultados = reporte && total === 0;
    const serieMensual = (reporte?.por_mes ?? []).map((m) => {
        const mes = etiquetaMes(m.clave);
        return { key: m.clave, label: mes.corta, labelLargo: mes.larga, value: m.cantidad };
    });

    return (
        <>
            {reporte && <InfoVista fechaUsada="Según la fecha de registro de cada expediente." alcance={textoAlcance(reporte)} generado={reporte.generado} />}
            <EncabezadoImpresion
                titulo="Reporte de expedientes – Mesa de Partes"
                campoPeriodo="Periodo (fecha de registro)"
                filtros={filtros}
                textoFiltros={textoFiltros(filtros)}
                alcance={reporte ? textoAlcance(reporte) : ""}
                generado={reporte?.generado}
            />
            <EstadoCarga cargando={cargando} error={error} datos={reporte} onReintentar={reintentar} etiqueta="Cargando reporte" />

            {reporte && !error && (
                <div className={cargando ? "rp-actualizando" : ""} aria-busy={cargando}>
                    <div className="db-kpis rp-kpis">
                        <StatCard icon="folder" label="Total de expedientes" value={total} hint={textoPeriodo(filtros)} />
                        <StatCard icon="inbox" label="Pendientes" tone="pendiente" value={pendientes}
                            hint={total ? `${porcentaje(pendientes, total)} · Pendiente + En proceso` : "Pendiente + En proceso"} />
                        <StatCard icon="check" label="Atendidos" tone="atendido" value={atendidos} hint={porcentaje(atendidos, total)} />
                        <StatCard icon="eye" label="Observados" tone="observado" value={observados}
                            hint={total ? `${porcentaje(observados, total)} · esperan corrección` : "Esperan corrección del solicitante"} />
                    </div>

                    <p className="rp-pie rp-pie--kpis">Pendientes, atendidos y observados: estado actual de los expedientes registrados en el periodo.</p>

                    {sinResultados ? (
                        <SinDatos mensaje="No hay expedientes que coincidan con los filtros aplicados." />
                    ) : (
                        <>
                            <div className="rp-grid">
                                <Panel id="rp-estado" titulo="Por estado" subtitulo="Estado actual de cada expediente">
                                    <BarList items={aItems(reporte.por_estado, etiquetaEstado)} total={total} descripcion="Expedientes por estado" />
                                </Panel>
                                <Panel id="rp-prioridad" titulo="Por prioridad">
                                    <BarList items={aItems(reporte.por_prioridad, etiquetaPrioridad)} total={total} descripcion="Expedientes por prioridad" />
                                </Panel>
                                <Panel id="rp-tipo" titulo="Por tipo de trámite" subtitulo="Solo los tipos con registros">
                                    <BarList items={aItems(reporte.por_tipo, etiquetaTipo, { sinCeros: true })} total={total} descripcion="Expedientes por tipo de trámite" />
                                </Panel>
                                <Panel id="rp-area" titulo="Por área actual" subtitulo="Área en la que se encuentra cada expediente hoy">
                                    <BarList items={aItems(reporte.por_area, etiquetaArea, { sinCeros: true })} total={total} descripcion="Expedientes por área actual" />
                                </Panel>
                            </div>

                            <Panel id="rp-mes" titulo="Tendencia mensual" subtitulo="Expedientes registrados por mes" className="rp-panel-ancho">
                                {serieMensual.length > 0 ? (
                                    <LineChart puntos={serieMensual} alto={240} descripcion="Expedientes registrados por mes" />
                                ) : (
                                    <p className="db-panel-msg">Sin datos para el periodo.</p>
                                )}
                            </Panel>
                        </>
                    )}

                    <Panel
                        id="rp-antiguedad"
                        titulo="Pendientes por antigüedad"
                        subtitulo="Días transcurridos desde la fecha de registro hasta hoy. Rangos descriptivos: no representan plazos institucionales."
                        className="rp-panel-ancho"
                    >
                        <div className="dp-table-wrapper">
                            <table className="dp-table rp-tabla">
                                <thead>
                                    <tr>
                                        <th scope="col">Antigüedad</th>
                                        <th scope="col" className="rp-num">Pendientes</th>
                                        <th scope="col" className="rp-num">% de pendientes</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {RANGOS_ANTIGUEDAD.map((r) => {
                                        const n = cantidadDe(reporte.pendientes_por_antiguedad, r.clave);
                                        return (
                                            <tr key={r.clave}>
                                                <td>{r.label}</td>
                                                <td className="rp-num">{n}</td>
                                                <td className="rp-num">{porcentaje(n, pendientes)}</td>
                                            </tr>
                                        );
                                    })}
                                </tbody>
                                <tfoot>
                                    <tr>
                                        <th scope="row">Total pendientes</th>
                                        <td className="rp-num">{pendientes}</td>
                                        <td className="rp-num">{pendientes ? "100,0 %" : "—"}</td>
                                    </tr>
                                </tfoot>
                            </table>
                        </div>
                    </Panel>
                </div>
            )}
        </>
    );
}

export default ResumenVista;
