import { ESTADOS_ORDEN, etiquetaEstado, porcentaje, textoPeriodo } from "../reportesData";
import { fechaGenerado } from "./reporteHooks";

// Piezas comunes de las pestañas de Reportes (Fase 2).

export function Panel({ id, titulo, subtitulo, children, className = "" }) {
    return (
        <section className={`dp-panel rp-panel ${className}`} aria-labelledby={id}>
            <div className="dp-panel-head">
                <div>
                    <h2 id={id} className="dp-panel-title">{titulo}</h2>
                    {subtitulo && <p className="db-panel-sub">{subtitulo}</p>}
                </div>
            </div>
            {children}
        </section>
    );
}

// Carga inicial (esqueleto) y error con reintento. La ausencia de datos se
// muestra aparte con <SinDatos>, para no confundirla con un fallo.
export function EstadoCarga({ cargando, error, datos, onReintentar, etiqueta }) {
    if (error) {
        return (
            <div className="db-alert db-alert--error" role="alert">
                <span>No se pudo cargar el reporte. {error}</span>
                <button type="button" className="dp-btn-secondary db-btn rp-acciones" onClick={onReintentar}>
                    Reintentar
                </button>
            </div>
        );
    }
    if (cargando && !datos) {
        return (
            <div className="db-kpis rp-kpis" aria-busy="true" aria-label={etiqueta}>
                {[0, 1, 2, 3].map((i) => (
                    <div key={i} className="db-skeleton db-skeleton--kpi" />
                ))}
            </div>
        );
    }
    return null;
}

// Consulta correcta sin resultados: mismo aspecto en todas las pestañas.
export function SinDatos({ mensaje, ayuda = "Prueba con un periodo más amplio o quita algún filtro." }) {
    return (
        <div className="dp-panel rp-vacio" role="status">
            <p className="rp-vacio-titulo">{mensaje}</p>
            {ayuda && <p className="rp-vacio-ayuda">{ayuda}</p>}
        </div>
    );
}

// Línea informativa (fecha usada, alcance y momento de los datos).
export function InfoVista({ fechaUsada, alcance, generado }) {
    return (
        <p className="rp-info">
            {fechaUsada} {alcance}
            {generado && <span className="db-intro-time"> · Datos al {fechaGenerado(generado)}</span>}
        </p>
    );
}

// Encabezado que solo aparece al imprimir.
export function EncabezadoImpresion({ titulo, campoPeriodo, filtros, textoFiltros, alcance, generado, extra = [] }) {
    return (
        <div className="rp-print-only rp-print-head">
            <p className="rp-print-title">{titulo}</p>
            <p>Colegio Tungasuca · SIS-MESA-PARTES</p>
            <p>
                {campoPeriodo}: {textoPeriodo(filtros)}
            </p>
            <p>Filtros: {textoFiltros}</p>
            {extra.map((linea) => (
                <p key={linea}>{linea}</p>
            ))}
            {alcance && <p>Alcance: {alcance}</p>}
            {generado && <p>Generado: {fechaGenerado(generado)}</p>}
        </div>
    );
}

// Mismo orden de estados que el Resumen y los archivos exportados.
const ESTADOS = ESTADOS_ORDEN.map((value) => ({ value, label: etiquetaEstado(value) }));

// Tabla cruzada (filas × estados) con totales por fila y por columna y la
// participación de cada fila en el total. Solo muestra las filas con valores;
// `porTotal` las ordena de mayor a menor (empates: orden del catálogo).
export function TablaCruzada({ celdas = [], orden, etiquetaFila, columnaFila, descripcion, porTotal = false }) {
    const valor = new Map();
    const presentes = new Set();
    for (const c of celdas) {
        valor.set(`${c.fila}|${c.columna}`, (valor.get(`${c.fila}|${c.columna}`) ?? 0) + c.cantidad);
        presentes.add(c.fila);
    }
    const totalFila = (f) => ESTADOS.reduce((s, e) => s + (valor.get(`${f}|${e.value}`) ?? 0), 0);
    const filas = [...orden.filter((f) => presentes.has(f)), ...[...presentes].filter((f) => !orden.includes(f)).sort()];
    if (porTotal) filas.sort((a, b) => totalFila(b) - totalFila(a));
    const totalCol = ESTADOS.map((e) => filas.reduce((s, f) => s + (valor.get(`${f}|${e.value}`) ?? 0), 0));
    const general = totalCol.reduce((a, b) => a + b, 0);
    if (filas.length === 0) return <p className="db-panel-msg">Sin datos para los filtros aplicados.</p>;
    return (
        <div className="dp-table-wrapper">
            <table className="dp-table rp-tabla rp-cruce" aria-label={descripcion}>
                <thead>
                    <tr>
                        <th scope="col">{columnaFila}</th>
                        {ESTADOS.map((e) => (
                            <th key={e.value} scope="col" className="rp-num">{e.label}</th>
                        ))}
                        <th scope="col" className="rp-num">Total</th>
                        <th scope="col" className="rp-num">% del total</th>
                    </tr>
                </thead>
                <tbody>
                    {filas.map((f) => {
                        const vals = ESTADOS.map((e) => valor.get(`${f}|${e.value}`) ?? 0);
                        return (
                            <tr key={f}>
                                <td>{etiquetaFila(f)}</td>
                                {vals.map((v, i) => (
                                    <td key={ESTADOS[i].value} className={`rp-num ${v === 0 ? "rp-cero" : ""}`}>{v}</td>
                                ))}
                                <td className="rp-num rp-total-fila">{totalFila(f)}</td>
                                <td className="rp-num rp-pct">{porcentaje(totalFila(f), general)}</td>
                            </tr>
                        );
                    })}
                </tbody>
                <tfoot>
                    <tr>
                        <th scope="row">Total</th>
                        {totalCol.map((v, i) => (
                            <td key={ESTADOS[i].value} className="rp-num">{v}</td>
                        ))}
                        <td className="rp-num">{general}</td>
                        <td className="rp-num">{porcentaje(general, general)}</td>
                    </tr>
                </tfoot>
            </table>
        </div>
    );
}
