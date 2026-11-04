import LineChart from "../../../components/dashboard/charts/LineChart";
import { getReporteExpedientes } from "../../../services/reportesService";
import {
    AREAS,
    ESTADOS_ORDEN,
    GRANULARIDADES,
    aPuntos,
    cargaPendientePorArea,
    etiquetaArea,
    etiquetaEstado,
    etiquetaTipo,
    filtrosResumen,
    porcentaje,
    textoAlcance,
    textoFiltros,
} from "../reportesData";
import { TIPOS } from "../../../constants/expedientes";
import BarrasApiladas from "./BarrasApiladas";
import { EncabezadoImpresion, EstadoCarga, InfoVista, Panel, SinDatos, TablaCruzada } from "./comun";
import { useReporte } from "./reporteHooks";

const SERIES_ESTADO = ESTADOS_ORDEN.map((key) => ({ key, label: etiquetaEstado(key) }));

// Expedientes y áreas (Fase 2): evolución de registros por día, semana o mes
// separada por estado ACTUAL, carga pendiente por área y tablas cruzadas
// área × estado y tipo × estado (estado y área ACTUALES).
function ExpedientesVista({ aplicados }) {
    const filtros = { ...filtrosResumen(aplicados), granularidad: aplicados.granularidad };
    const { datos, error, cargando, reintentar } = useReporte(() => getReporteExpedientes(filtros), JSON.stringify(filtros));
    const unidad = (GRANULARIDADES.find((g) => g.value === (datos?.granularidad || filtros.granularidad))?.label ?? "Mes").toLowerCase();
    const puntos = aPuntos(datos?.serie, datos?.granularidad);
    // Evolución por estado: si el servicio aún no la entrega, se muestra la serie simple.
    const porEstado = Array.isArray(datos?.serie_estado);
    const valores = new Map((datos?.serie_estado ?? []).map((c) => [`${c.fila}|${c.columna}`, c.cantidad]));
    const carga = cargaPendientePorArea(datos?.area_estado);
    const totalCarga = carga.reduce((s, a) => s + a.total, 0);

    return (
        <>
            {datos && <InfoVista fechaUsada="Según la fecha de registro de cada expediente." alcance={textoAlcance(datos)} generado={datos.generado} />}
            <EncabezadoImpresion
                titulo="Expedientes por periodo y área – Mesa de Partes"
                campoPeriodo="Periodo (fecha de registro)"
                filtros={filtros}
                textoFiltros={textoFiltros(filtros)}
                extra={[`Agrupación: por ${unidad}`]}
                alcance={datos ? textoAlcance(datos) : ""}
                generado={datos?.generado}
            />
            <EstadoCarga cargando={cargando} error={error} datos={datos} onReintentar={reintentar} etiqueta="Cargando expedientes por periodo" />

            {datos && !error && (
                <div className={cargando ? "rp-actualizando" : ""} aria-busy={cargando}>
                    {datos.total === 0 ? (
                        <SinDatos mensaje="No hay expedientes que coincidan con los filtros aplicados." />
                    ) : (
                        <>
                            <p className="rp-nota" role="note">
                                Los estados y las áreas son los <strong>actuales</strong> de cada expediente registrado en el periodo; no reflejan su
                                situación en fechas pasadas.
                            </p>
                            <Panel
                                id="rp-exp-serie"
                                titulo={`Expedientes registrados por ${unidad}`}
                                subtitulo={`${datos.total} en el periodo · fecha de registro${porEstado ? " · color: estado actual" : ""}`}
                                className="rp-panel-ancho"
                            >
                                {puntos.length === 0 ? (
                                    <p className="db-panel-msg">Sin datos para el periodo.</p>
                                ) : porEstado ? (
                                    <BarrasApiladas
                                        periodos={puntos.map((p) => ({ key: p.key, label: p.label, labelLargo: p.labelLargo }))}
                                        series={SERIES_ESTADO}
                                        valores={valores}
                                        descripcion={`Expedientes registrados por ${unidad}, según su estado actual`}
                                    />
                                ) : (
                                    <LineChart puntos={puntos} alto={240} descripcion={`Expedientes registrados por ${unidad}`} />
                                )}
                            </Panel>

                            <Panel
                                id="rp-carga"
                                titulo="Áreas con mayor carga pendiente"
                                subtitulo="Pendiente + En proceso según el área en la que está cada expediente hoy. No mide plazos ni demoras."
                                className="rp-panel-ancho"
                            >
                                {carga.length === 0 ? (
                                    <p className="db-panel-msg">Ningún expediente del periodo está pendiente o en proceso.</p>
                                ) : (
                                    <div className="dp-table-wrapper">
                                        <table className="dp-table rp-tabla" aria-label="Carga pendiente por área actual">
                                            <thead>
                                                <tr>
                                                    <th scope="col">Área actual</th>
                                                    <th scope="col" className="rp-num">Pendiente</th>
                                                    <th scope="col" className="rp-num">En proceso</th>
                                                    <th scope="col" className="rp-num">Carga pendiente</th>
                                                    <th scope="col" className="rp-num">% de la carga</th>
                                                </tr>
                                            </thead>
                                            <tbody>
                                                {carga.map((a) => (
                                                    <tr key={a.area}>
                                                        <td>{etiquetaArea(a.area)}</td>
                                                        <td className="rp-num">{a.pendiente}</td>
                                                        <td className="rp-num">{a.enProceso}</td>
                                                        <td className="rp-num rp-total-fila">{a.total}</td>
                                                        <td className="rp-num rp-pct">{porcentaje(a.total, totalCarga)}</td>
                                                    </tr>
                                                ))}
                                            </tbody>
                                            <tfoot>
                                                <tr>
                                                    <th scope="row">Total</th>
                                                    <td className="rp-num">{carga.reduce((s, a) => s + a.pendiente, 0)}</td>
                                                    <td className="rp-num">{carga.reduce((s, a) => s + a.enProceso, 0)}</td>
                                                    <td className="rp-num">{totalCarga}</td>
                                                    <td className="rp-num">{porcentaje(totalCarga, totalCarga)}</td>
                                                </tr>
                                            </tfoot>
                                        </table>
                                    </div>
                                )}
                            </Panel>

                            <Panel id="rp-area-estado" titulo="Área actual × estado" subtitulo="Dónde está cada expediente hoy y en qué estado" className="rp-panel-ancho">
                                <TablaCruzada celdas={datos.area_estado} orden={AREAS.map((a) => a.value)} etiquetaFila={etiquetaArea} columnaFila="Área" descripcion="Expedientes por área actual y estado" />
                            </Panel>
                            <Panel id="rp-tipo-estado" titulo="Tipo de trámite × estado" subtitulo="De más a menos frecuente · estado actual de cada expediente" className="rp-panel-ancho">
                                <TablaCruzada
                                    celdas={datos.tipo_estado}
                                    orden={TIPOS.map((t) => t.value)}
                                    etiquetaFila={etiquetaTipo}
                                    columnaFila="Tipo de trámite"
                                    descripcion="Expedientes por tipo de trámite y estado"
                                    porTotal
                                />
                            </Panel>
                        </>
                    )}
                </div>
            )}
        </>
    );
}

export default ExpedientesVista;
