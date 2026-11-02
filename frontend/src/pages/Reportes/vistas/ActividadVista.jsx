import { getReporteActividad } from "../../../services/reportesService";
import { etiquetaArea, etiquetaRol, etiquetaTipo, porcentaje } from "../reportesData";
import { EncabezadoImpresion, EstadoCarga, InfoVista, Panel, SinDatos } from "./comun";
import { textoAlcanceDe, useReporte } from "./reporteHooks";

// Actividad institucional por ROL (Fase 2). Decisión aprobada: nunca por
// persona ni como ranking; el orden de los roles es fijo.
//
// Movimientos registrados = TODOS los movimientos (derivaciones,
// notificaciones, asignaciones, recepciones y aprobaciones). «De ellos,
// derivaciones» es la parte de tipo DERIVACION: ya está dentro de los
// movimientos y no se suma al total.
function normalizar(r) {
    // Respuesta anterior a la mejora: su "derivaciones_registradas" contaba todos los movimientos.
    if (r.movimientos_registrados === undefined) return { ...r, movimientos_registrados: r.derivaciones_registradas, derivaciones_registradas: null };
    return r;
}

function ActividadVista({ aplicados }) {
    const filtros = { desde: aplicados.desde, hasta: aplicados.hasta, tipo: aplicados.tipo };
    const { datos, error, cargando, reintentar } = useReporte(() => getReporteActividad(filtros), JSON.stringify(filtros));
    const roles = (datos?.roles ?? []).map(normalizar);
    const suma = (campo) => roles.reduce((s, r) => s + (r[campo] ?? 0), 0);
    const totalAcciones = suma("total");
    const conDerivaciones = roles.every((r) => r.derivaciones_registradas !== null);
    const alcance = textoAlcanceDe(datos, etiquetaArea);

    return (
        <>
            {datos && <InfoVista fechaUsada="Según la fecha de cada acción." alcance={alcance} generado={datos.generado} />}
            <EncabezadoImpresion titulo="Actividad institucional por rol – Mesa de Partes" campoPeriodo="Periodo (fecha de cada acción)" filtros={filtros}
                textoFiltros={filtros.tipo ? `Tipo: ${etiquetaTipo(filtros.tipo)}` : "Sin filtros adicionales"} alcance={datos ? alcance : ""} generado={datos?.generado} />
            <EstadoCarga cargando={cargando} error={error} datos={datos} onReintentar={reintentar} etiqueta="Cargando actividad" />

            {datos && !error && (
                <div className={cargando ? "rp-actualizando" : ""} aria-busy={cargando}>
                    {roles.length === 0 ? (
                        <SinDatos mensaje="Sin actividad registrada para los filtros aplicados." />
                    ) : (
                        <Panel
                            id="rp-actividad"
                            titulo="Actividad por rol"
                            subtitulo="Agrupada por el rol actual de cada usuario. No se muestran personas. Son volúmenes, no productividad: no reflejan complejidad ni tiempos."
                            className="rp-panel-ancho"
                        >
                            <div className="dp-table-wrapper">
                                <table className="dp-table rp-tabla" aria-label="Actividad por rol">
                                    <thead>
                                        <tr>
                                            <th scope="col">Rol</th>
                                            <th scope="col" className="rp-num">Expedientes registrados</th>
                                            <th scope="col" className="rp-num">Movimientos registrados</th>
                                            {conDerivaciones && <th scope="col" className="rp-num">De ellos, derivaciones</th>}
                                            <th scope="col" className="rp-num">Documentos subidos</th>
                                            <th scope="col" className="rp-num">Total de acciones</th>
                                            <th scope="col" className="rp-num">% del total</th>
                                        </tr>
                                    </thead>
                                    <tbody>
                                        {roles.map((r) => (
                                            <tr key={r.rol}>
                                                <td>{etiquetaRol(r.rol)}</td>
                                                <td className="rp-num">{r.expedientes_registrados}</td>
                                                <td className="rp-num">{r.movimientos_registrados}</td>
                                                {conDerivaciones && <td className="rp-num rp-sub">{r.derivaciones_registradas}</td>}
                                                <td className="rp-num">{r.documentos_subidos}</td>
                                                <td className="rp-num rp-total-fila">{r.total}</td>
                                                <td className="rp-num rp-pct">{porcentaje(r.total, totalAcciones)}</td>
                                            </tr>
                                        ))}
                                    </tbody>
                                    <tfoot>
                                        <tr>
                                            <th scope="row">Total</th>
                                            <td className="rp-num">{suma("expedientes_registrados")}</td>
                                            <td className="rp-num">{suma("movimientos_registrados")}</td>
                                            {conDerivaciones && <td className="rp-num">{suma("derivaciones_registradas")}</td>}
                                            <td className="rp-num">{suma("documentos_subidos")}</td>
                                            <td className="rp-num">{totalAcciones}</td>
                                            <td className="rp-num">{porcentaje(totalAcciones, totalAcciones)}</td>
                                        </tr>
                                    </tfoot>
                                </table>
                            </div>
                            <p className="rp-pie">
                                <strong>Movimientos registrados</strong>: derivaciones, notificaciones, asignaciones, recepciones y aprobaciones.
                                {conDerivaciones && " «De ellos, derivaciones» ya está incluido en los movimientos."} Total de acciones = expedientes +
                                movimientos + documentos.
                            </p>
                        </Panel>
                    )}
                </div>
            )}
        </>
    );
}

export default ActividadVista;
