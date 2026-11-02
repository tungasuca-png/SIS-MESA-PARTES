import BarList from "../../../components/dashboard/charts/BarList";
import LineChart from "../../../components/dashboard/charts/LineChart";
import { getReporteDocumentos } from "../../../services/reportesService";
import { GRANULARIDADES, aItems, aPuntos, etiquetaArea, etiquetaTipo, etiquetaTipoDocumento, ordenarPorCantidad } from "../reportesData";
import { EncabezadoImpresion, EstadoCarga, InfoVista, Panel, SinDatos } from "./comun";
import { textoAlcanceDe, useReporte } from "./reporteHooks";

// Documentos (Fase 2): documentos vigentes de expedientes activos, por tipo
// documental, tipo de trámite y periodo.
function DocumentosVista({ aplicados }) {
    const filtros = { desde: aplicados.desde, hasta: aplicados.hasta, tipo: aplicados.tipo, granularidad: aplicados.granularidad };
    const { datos, error, cargando, reintentar } = useReporte(() => getReporteDocumentos(filtros), JSON.stringify(filtros));
    const unidad = (GRANULARIDADES.find((g) => g.value === (datos?.granularidad || filtros.granularidad))?.label ?? "Mes").toLowerCase();
    const total = datos?.total ?? 0;
    const alcance = textoAlcanceDe(datos, etiquetaArea);

    return (
        <>
            {datos && <InfoVista fechaUsada="Según la fecha de registro de cada documento." alcance={alcance} generado={datos.generado} />}
            <EncabezadoImpresion titulo="Reporte de documentos – Mesa de Partes" campoPeriodo="Periodo (fecha de registro del documento)" filtros={filtros}
                textoFiltros={filtros.tipo ? `Tipo: ${etiquetaTipo(filtros.tipo)}` : "Sin filtros adicionales"} extra={[`Agrupación: por ${unidad}`]}
                alcance={datos ? alcance : ""} generado={datos?.generado} />
            <EstadoCarga cargando={cargando} error={error} datos={datos} onReintentar={reintentar} etiqueta="Cargando documentos" />

            {datos && !error && (
                <div className={cargando ? "rp-actualizando" : ""} aria-busy={cargando}>
                    {total === 0 ? (
                        <SinDatos mensaje="No hay documentos que coincidan con los filtros aplicados." />
                    ) : (
                        <>
                            <div className="rp-grid">
                                <Panel id="rp-doc-tipo" titulo="Tipos documentales más frecuentes" subtitulo={`${total} documentos vigentes · de más a menos frecuente`}>
                                    <BarList items={aItems(ordenarPorCantidad(datos.por_tipo_documento), etiquetaTipoDocumento, { sinCeros: true })} total={total} descripcion="Documentos por tipo documental" />
                                </Panel>
                                <Panel id="rp-doc-tramite" titulo="Por tipo de trámite" subtitulo="Tipo del expediente al que pertenecen, de más a menos frecuente">
                                    <BarList items={aItems(ordenarPorCantidad(datos.por_tipo_tramite), etiquetaTipo, { sinCeros: true })} total={total} descripcion="Documentos por tipo de trámite" />
                                </Panel>
                            </div>
                            <Panel id="rp-doc-serie" titulo={`Documentos registrados por ${unidad}`} subtitulo="Fecha de registro del documento" className="rp-panel-ancho">
                                {datos.serie?.length ? (
                                    <LineChart puntos={aPuntos(datos.serie, datos.granularidad)} alto={220} descripcion={`Documentos registrados por ${unidad}`} unidad="documentos" />
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

export default DocumentosVista;
