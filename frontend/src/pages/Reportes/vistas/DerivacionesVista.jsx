import StatCard from "../../../components/dashboard/StatCard";
import BarList from "../../../components/dashboard/charts/BarList";
import LineChart from "../../../components/dashboard/charts/LineChart";
import { getReporteDerivaciones } from "../../../services/reportesService";
import {
    GRANULARIDADES,
    aItems,
    aPuntos,
    cantidadDe,
    etiquetaArea,
    etiquetaAreaHistorica,
    etiquetaMovimiento,
    etiquetaTipo,
    ordenarPorCantidad,
    porcentaje,
} from "../reportesData";
import { EncabezadoImpresion, EstadoCarga, InfoVista, Panel, SinDatos } from "./comun";
import { textoAlcanceDe, useReporte } from "./reporteHooks";

// Derivaciones (Fase 2): solo derivaciones entre áreas de expedientes ACTIVOS
// (sin bajas lógicas), con origen/destino normalizados. Lo irreconocible es
// "Sin clasificar". Otros movimientos solo se cuentan.
function DerivacionesVista({ aplicados }) {
    const filtros = { desde: aplicados.desde, hasta: aplicados.hasta, tipo: aplicados.tipo, granularidad: aplicados.granularidad };
    const { datos, error, cargando, reintentar } = useReporte(() => getReporteDerivaciones(filtros), JSON.stringify(filtros));
    const unidad = (GRANULARIDADES.find((g) => g.value === (datos?.granularidad || filtros.granularidad))?.label ?? "Mes").toLowerCase();
    const total = datos?.total ?? 0;
    const otros = (datos?.otros_movimientos ?? []).reduce((s, c) => s + c.cantidad, 0);
    const movimientos = total + otros;
    const aprobaciones = cantidadDe(datos?.otros_movimientos, "APROBACION");
    const alcance = textoAlcanceDe(datos, etiquetaArea);
    const filtrosTexto = filtros.tipo ? `Tipo: ${etiquetaTipo(filtros.tipo)}` : "Sin filtros adicionales";

    return (
        <>
            {datos && <InfoVista fechaUsada="Según la fecha de cada derivación." alcance={alcance} generado={datos.generado} />}
            <EncabezadoImpresion titulo="Reporte de derivaciones – Mesa de Partes" campoPeriodo="Periodo (fecha de derivación)" filtros={filtros}
                textoFiltros={filtrosTexto} extra={[`Agrupación: por ${unidad}`]} alcance={datos ? alcance : ""} generado={datos?.generado} />
            <EstadoCarga cargando={cargando} error={error} datos={datos} onReintentar={reintentar} etiqueta="Cargando derivaciones" />

            {datos && !error && (
                <div className={cargando ? "rp-actualizando" : ""} aria-busy={cargando}>
                    <div className="db-kpis rp-kpis">
                        <StatCard icon="inbox" label="Movimientos registrados" value={movimientos} hint="Derivaciones y otros movimientos" />
                        <StatCard icon="share" label="Derivaciones entre áreas" value={total}
                            hint={movimientos ? `${porcentaje(total, movimientos)} de los movimientos · incluye devoluciones` : "Incluye devoluciones a Secretaría"} />
                        <StatCard icon="check" label="Aprobaciones" tone="atendido" value={aprobaciones}
                            hint={movimientos ? `${porcentaje(aprobaciones, movimientos)} de los movimientos` : "Registradas como movimiento"} />
                        <StatCard icon="eye" label="Sin clasificar" tone="observado" value={datos.sin_clasificar}
                            hint={total ? `${porcentaje(datos.sin_clasificar, total)} de las derivaciones · registros históricos` : "Registros históricos"} />
                    </div>
                    <p className="rp-nota" role="note">
                        Las devoluciones a Secretaría se cuentan como derivaciones: hoy no se registran rechazos diferenciados. «Sin clasificar» agrupa
                        registros antiguos cuyo origen o destino no corresponde a un área reconocible.
                    </p>

                    {total === 0 && otros === 0 ? (
                        <SinDatos mensaje="No hay derivaciones ni movimientos que coincidan con los filtros aplicados." />
                    ) : (
                        <>
                            <div className="rp-grid">
                                <Panel id="rp-der-destino" titulo="Por área de destino">
                                    <BarList items={aItems(datos.por_destino, etiquetaAreaHistorica, { sinCeros: true })} total={total} descripcion="Derivaciones por área de destino" />
                                </Panel>
                                <Panel id="rp-der-origen" titulo="Por área de origen">
                                    <BarList items={aItems(datos.por_origen, etiquetaAreaHistorica, { sinCeros: true })} total={total} descripcion="Derivaciones por área de origen" />
                                </Panel>
                                <Panel id="rp-der-tipo" titulo="Por tipo de trámite" subtitulo="Tipo del expediente derivado, de más a menos frecuente">
                                    <BarList items={aItems(ordenarPorCantidad(datos.por_tipo_tramite), etiquetaTipo, { sinCeros: true })} total={total} descripcion="Derivaciones por tipo de trámite" />
                                </Panel>
                                <Panel id="rp-der-otros" titulo="Otros movimientos" subtitulo={`${otros} en total · solo cantidad: su origen y destino son texto libre`}>
                                    <BarList items={aItems(datos.otros_movimientos, etiquetaMovimiento)} total={otros} descripcion="Otros movimientos por tipo" />
                                </Panel>
                            </div>
                            <Panel id="rp-der-flujo" titulo="Flujo entre áreas" subtitulo="Origen → destino" className="rp-panel-ancho">
                                <div className="dp-table-wrapper">
                                    <table className="dp-table rp-tabla" aria-label="Flujo de derivaciones entre áreas">
                                        <thead>
                                            <tr>
                                                <th scope="col">Origen → destino</th>
                                                <th scope="col" className="rp-num">Derivaciones</th>
                                                <th scope="col" className="rp-num">% del total</th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            {(datos.flujo ?? []).map((f) => (
                                                <tr key={`${f.origen}-${f.destino}`}>
                                                    <td>{etiquetaAreaHistorica(f.origen)} → {etiquetaAreaHistorica(f.destino)}</td>
                                                    <td className="rp-num">{f.cantidad}</td>
                                                    <td className="rp-num">{porcentaje(f.cantidad, total)}</td>
                                                </tr>
                                            ))}
                                        </tbody>
                                        <tfoot>
                                            <tr>
                                                <th scope="row">Total</th>
                                                <td className="rp-num">{total}</td>
                                                <td className="rp-num">{total ? "100,0 %" : "—"}</td>
                                            </tr>
                                        </tfoot>
                                    </table>
                                </div>
                            </Panel>
                            <Panel id="rp-der-serie" titulo={`Derivaciones por ${unidad}`} subtitulo="Fecha de la derivación" className="rp-panel-ancho">
                                {datos.serie?.length ? (
                                    <LineChart puntos={aPuntos(datos.serie, datos.granularidad)} alto={220} descripcion={`Derivaciones por ${unidad}`} unidad="derivaciones" />
                                ) : (
                                    <p className="db-panel-msg">Sin datos para el periodo.</p>
                                )}
                            </Panel>
                        </>
                    )}
                </div>
            )}
        </>
    );
}

export default DerivacionesVista;
