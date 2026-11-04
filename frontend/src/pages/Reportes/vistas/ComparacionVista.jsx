import { getReporteComparacion } from "../../../services/reportesService";
import {
    MINIMO_COMPARACION,
    MODOS_COMPARACION,
    etiquetaArea,
    etiquetaEstado,
    etiquetaPrioridad,
    etiquetaTipo,
    filtrosResumen,
    tendencia,
    textoDiferencia,
    textoFiltros,
    textoPeriodo,
    textoVariacion,
    validarComparacion,
} from "../reportesData";
import { EncabezadoImpresion, EstadoCarga, InfoVista, Panel } from "./comun";
import { textoAlcanceDe, useReporte } from "./reporteHooks";

// Comparación de periodos (Fase 2). Regla aprobada: las variaciones solo se
// muestran si AMBOS periodos tienen al menos 10 expedientes; si no, «Datos
// insuficientes para comparar». No se estiman datos faltantes.

const GRUPOS = {
    RESUMEN: { titulo: "Resumen", etiqueta: (k) => ({ TOTAL: "Total de expedientes", PENDIENTES: "Pendientes (Pendiente + En proceso)" })[k] ?? k },
    ESTADO: { titulo: "Por estado", etiqueta: etiquetaEstado },
    TIPO: { titulo: "Por tipo de trámite", etiqueta: etiquetaTipo },
    PRIORIDAD: { titulo: "Por prioridad", etiqueta: etiquetaPrioridad },
    AREA: { titulo: "Por área actual", etiqueta: etiquetaArea },
};

function textoCelda(fila, comparable) {
    if (fila.tiene_variacion) return textoVariacion(fila.variacion);
    if (!comparable) return "—";
    return "Sin base";
}

// Diferencia absoluta con indicador de sentido. Solo con periodos
// comparables: con datos insuficientes no se sugiere ningún cambio.
function Diferencia({ actual, anterior, comparable }) {
    if (!comparable) return <span className="rp-cero">—</span>;
    const t = tendencia(actual, anterior);
    return (
        <span className={`rp-tendencia rp-tendencia--${t.clave}`}>
            <span aria-hidden="true" className="rp-tendencia-simbolo">{t.simbolo}</span>
            {textoDiferencia(actual - anterior)}
            <span className="dp-sr-only"> ({t.texto})</span>
        </span>
    );
}

function Contenido({ aplicados }) {
    const filtros = { ...filtrosResumen(aplicados), modo: aplicados.modo, desdeB: aplicados.desdeB, hastaB: aplicados.hastaB };
    const { datos, error, cargando, reintentar } = useReporte(() => getReporteComparacion(filtros), JSON.stringify(filtros));
    const alcance = textoAlcanceDe(datos, etiquetaArea);
    const modo = MODOS_COMPARACION.find((m) => m.value === filtros.modo)?.label ?? "";
    const filaTotal = datos?.filas?.find((f) => f.grupo === "RESUMEN" && f.clave === "TOTAL");
    const grupos = [];
    for (const fila of datos?.filas ?? []) {
        if (!grupos.length || grupos[grupos.length - 1].grupo !== fila.grupo) grupos.push({ grupo: fila.grupo, filas: [] });
        grupos[grupos.length - 1].filas.push(fila);
    }

    return (
        <>
            {datos && <InfoVista fechaUsada="Según la fecha de registro de cada expediente." alcance={alcance} generado={datos.generado} />}
            <EncabezadoImpresion titulo="Comparación de periodos – Mesa de Partes" campoPeriodo="Periodo actual (fecha de registro)" filtros={filtros}
                textoFiltros={textoFiltros(filtros)}
                extra={datos ? [`Periodo de referencia: ${textoPeriodo({ desde: datos.desde_b, hasta: datos.hasta_b })} (${modo})`] : []}
                alcance={datos ? alcance : ""} generado={datos?.generado} />
            <EstadoCarga cargando={cargando} error={error} datos={datos} onReintentar={reintentar} etiqueta="Cargando comparación" />

            {datos && !error && (
                <div className={cargando ? "rp-actualizando" : ""} aria-busy={cargando}>
                    <div className="rp-comparacion-cabecera">
                        <div className="dp-stat-card">
                            <p className="dp-stat-label">Periodo actual</p>
                            <p className="dp-stat-value">{datos.total_a}</p>
                            <p className="dp-stat-hint">{textoPeriodo({ desde: datos.desde_a, hasta: datos.hasta_a })}</p>
                        </div>
                        <div className="dp-stat-card">
                            <p className="dp-stat-label">Periodo de referencia</p>
                            <p className="dp-stat-value">{datos.total_b}</p>
                            <p className="dp-stat-hint">{textoPeriodo({ desde: datos.desde_b, hasta: datos.hasta_b })} · {modo}</p>
                        </div>
                        <div className="dp-stat-card">
                            <p className="dp-stat-label">Cambio en el total</p>
                            <p className="dp-stat-value">
                                <Diferencia actual={datos.total_a} anterior={datos.total_b} comparable={datos.comparable} />
                            </p>
                            <p className="dp-stat-hint">
                                {!datos.comparable
                                    ? "No comparable: datos insuficientes"
                                    : filaTotal?.tiene_variacion
                                      ? `${textoVariacion(filaTotal.variacion)} respecto de la referencia`
                                      : "Sin base: 0 en el periodo de referencia"}
                            </p>
                        </div>
                    </div>

                    {!datos.comparable && (
                        <div className="db-alert db-alert--warn rp-insuficiente" role="status">
                            <strong>Datos insuficientes para comparar.</strong> Se requieren al menos {datos.minimo || MINIMO_COMPARACION} expedientes en cada
                            periodo (actual: {datos.total_a}; referencia: {datos.total_b}). Se muestran solo las cantidades, sin variaciones.
                        </div>
                    )}

                    {grupos.map((g) => (
                        <Panel key={g.grupo} id={`rp-comp-${g.grupo}`} titulo={GRUPOS[g.grupo]?.titulo ?? g.grupo} className="rp-panel-ancho">
                            <div className="dp-table-wrapper">
                                <table className="dp-table rp-tabla" aria-label={`Comparación ${GRUPOS[g.grupo]?.titulo ?? g.grupo}`}>
                                    <thead>
                                        <tr>
                                            <th scope="col">Concepto</th>
                                            <th scope="col" className="rp-num">Periodo actual</th>
                                            <th scope="col" className="rp-num">Periodo de referencia</th>
                                            <th scope="col" className="rp-num">Diferencia</th>
                                            <th scope="col" className="rp-num">Variación</th>
                                        </tr>
                                    </thead>
                                    <tbody>
                                        {g.filas.map((f) => (
                                            <tr key={f.clave}>
                                                <td>{(GRUPOS[f.grupo]?.etiqueta ?? ((k) => k))(f.clave)}</td>
                                                <td className="rp-num">{f.actual}</td>
                                                <td className="rp-num">{f.anterior}</td>
                                                <td className="rp-num">
                                                    <Diferencia actual={f.actual} anterior={f.anterior} comparable={datos.comparable} />
                                                </td>
                                                <td className={`rp-num ${f.tiene_variacion ? (f.variacion > 0 ? "rp-sube" : f.variacion < 0 ? "rp-baja" : "") : "rp-cero"}`}>
                                                    {textoCelda(f, datos.comparable)}
                                                </td>
                                            </tr>
                                        ))}
                                    </tbody>
                                </table>
                            </div>
                        </Panel>
                    ))}
                </div>
            )}
        </>
    );
}

function ComparacionVista({ aplicados }) {
    // Sin periodo completo no se consulta: se explica qué falta.
    const falta = validarComparacion(aplicados);
    if (falta) return <p className="dp-panel rp-vacio">{falta} Usa los filtros de arriba y pulsa «Aplicar filtros».</p>;
    return <Contenido aplicados={aplicados} />;
}

export default ComparacionVista;
