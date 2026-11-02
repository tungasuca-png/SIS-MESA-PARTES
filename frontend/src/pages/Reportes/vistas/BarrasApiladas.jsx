import { useEffect, useRef, useState } from "react";
import "../../../components/dashboard/charts/charts.css";

// Barras apiladas en SVG para la evolución de expedientes por estado (Reportes
// Fase 2). Mismo estilo que los gráficos del Dashboard, sin librerías: leyenda
// arriba, tooltip por barra, eje Y entero y tabla oculta para lectores de
// pantalla (la identidad de cada estado nunca depende solo del color).
//
// periodos: [{ key, label, labelLargo }]
// series:   [{ key, label }]   (orden de apilado, de abajo hacia arriba)
// valores:  Map "periodo|serie" -> cantidad

const PAD = { top: 12, right: 12, bottom: 26, left: 34 };
const RADIO = 4;
const SEPARACION = 2;

// Máximo "redondo" del eje Y con 4 divisiones enteras.
function escalaY(max) {
    if (max <= 4) return { max: 4, paso: 1 };
    const crudo = max / 4;
    const potencia = 10 ** Math.floor(Math.log10(crudo));
    const paso = [1, 2, 5, 10].map((f) => f * potencia).find((p) => p >= crudo);
    return { max: paso * 4, paso };
}

// Rectángulo con las esquinas superiores redondeadas (extremo de la barra).
function rectRedondeado(x, y, ancho, alto, r) {
    const rr = Math.min(r, ancho / 2, alto);
    return `M${x},${y + alto} V${y + rr} Q${x},${y} ${x + rr},${y} H${x + ancho - rr} Q${x + ancho},${y} ${x + ancho},${y + rr} V${y + alto} Z`;
}

function BarrasApiladas({ periodos, series, valores, alto = 240, descripcion, unidad = "expedientes" }) {
    const contenedor = useRef(null);
    const [ancho, setAncho] = useState(640);
    const [activo, setActivo] = useState(null);

    useEffect(() => {
        const el = contenedor.current;
        if (!el || typeof ResizeObserver === "undefined") return undefined;
        const observer = new ResizeObserver(([entry]) => {
            const w = Math.round(entry.contentRect.width);
            if (w > 0) setAncho(w);
        });
        observer.observe(el);
        return () => observer.disconnect();
    }, []);

    const valor = (p, s) => valores.get(`${p}|${s}`) ?? 0;
    const totales = periodos.map((p) => series.reduce((suma, s) => suma + valor(p.key, s.key), 0));
    const n = periodos.length;
    const { max, paso } = escalaY(Math.max(0, ...totales));
    const w = Math.max(ancho - PAD.left - PAD.right, 10);
    const h = alto - PAD.top - PAD.bottom;
    const banda = n > 0 ? w / n : w;
    const anchoBarra = Math.max(2, Math.min(40, banda * 0.7));
    const xBarra = (i) => PAD.left + i * banda + (banda - anchoBarra) / 2;
    const alturaDe = (v) => (v / max) * h;

    const marcas = [];
    for (let v = 0; v <= max; v += paso) marcas.push(v);

    // Etiquetas del eje X: ~1 cada 72px (máx. 8), siempre la última.
    const maxEtiquetas = Math.max(2, Math.min(8, Math.floor(w / 72)));
    const cada = Math.max(1, Math.ceil(n / maxEtiquetas));
    const indicesX = [];
    for (let i = 0; i < n; i += cada) indicesX.push(i);
    if (n > 0 && indicesX[indicesX.length - 1] !== n - 1) {
        if (n - 1 - indicesX[indicesX.length - 1] < cada / 2) indicesX.pop();
        indicesX.push(n - 1);
    }

    const alMover = (event) => {
        if (n === 0) return;
        const rect = event.currentTarget.getBoundingClientRect();
        const px = ((event.clientX - rect.left) / rect.width) * ancho;
        setActivo(Math.min(n - 1, Math.max(0, Math.floor((px - PAD.left) / banda))));
    };

    const p = activo !== null ? periodos[activo] : null;
    const tooltipIzq = p ? Math.min(Math.max(xBarra(activo) + anchoBarra / 2, 90), ancho - 90) : 0;
    const textoUnidad = (v) => (v === 1 ? unidad.replace(/s$/, "") : unidad);

    return (
        <div className="rp-barras">
            <ul className="rp-leyenda" aria-label="Estados">
                {series.map((s) => (
                    <li key={s.key}>
                        <span className={`rp-leyenda-marca rp-estado--${s.key.toLowerCase()}`} aria-hidden="true" />
                        {s.label}
                    </li>
                ))}
            </ul>
            <div className="dp-chart" ref={contenedor}>
                <svg
                    width={ancho}
                    height={alto}
                    viewBox={`0 0 ${ancho} ${alto}`}
                    role="img"
                    aria-label={descripcion}
                    onMouseMove={alMover}
                    onMouseLeave={() => setActivo(null)}
                >
                    {marcas.map((v) => (
                        <g key={v}>
                            <line className="dp-chart-grid" x1={PAD.left} x2={PAD.left + w} y1={PAD.top + h - alturaDe(v)} y2={PAD.top + h - alturaDe(v)} />
                            <text className="dp-chart-axis" x={PAD.left - 8} y={PAD.top + h - alturaDe(v)} textAnchor="end" dominantBaseline="middle">
                                {v}
                            </text>
                        </g>
                    ))}
                    {indicesX.map((i) => (
                        <text key={periodos[i].key} className="dp-chart-axis" x={xBarra(i) + anchoBarra / 2} y={alto - 6}
                            textAnchor={i === 0 && n > 1 ? "start" : i === n - 1 && n > 1 ? "end" : "middle"}>
                            {periodos[i].label}
                        </text>
                    ))}
                    {activo !== null && <rect className="rp-barras-foco" x={PAD.left + activo * banda} y={PAD.top} width={banda} height={h} />}
                    {periodos.map((per, i) => {
                        const presentes = series.filter((s) => valor(per.key, s.key) > 0);
                        let base = PAD.top + h;
                        return (
                            <g key={per.key}>
                                {presentes.map((s, j) => {
                                    const alturaBruta = alturaDe(valor(per.key, s.key));
                                    const ultimo = j === presentes.length - 1;
                                    // Separación de 2px entre segmentos (no en el extremo superior).
                                    const altura = Math.max(1, ultimo ? alturaBruta : alturaBruta - SEPARACION);
                                    const y = base - alturaBruta + (ultimo ? 0 : SEPARACION);
                                    base -= alturaBruta;
                                    const clase = `rp-barra rp-estado--${s.key.toLowerCase()}`;
                                    return ultimo ? (
                                        <path key={s.key} className={clase} d={rectRedondeado(xBarra(i), y, anchoBarra, altura, RADIO)} />
                                    ) : (
                                        <rect key={s.key} className={clase} x={xBarra(i)} y={y} width={anchoBarra} height={altura} />
                                    );
                                })}
                            </g>
                        );
                    })}
                </svg>

                {p && (
                    <div className="dp-chart-tooltip rp-barras-tooltip" style={{ left: tooltipIzq, top: 0 }} aria-hidden="true">
                        <span className="dp-chart-tooltip-label">{p.labelLargo ?? p.label}</span>
                        {series.map((s) => (
                            <span key={s.key} className="rp-barras-tooltip-fila">
                                <span className={`rp-leyenda-marca rp-estado--${s.key.toLowerCase()}`} />
                                {s.label}
                                <strong>{valor(p.key, s.key)}</strong>
                            </span>
                        ))}
                        <strong>
                            Total: {totales[activo]} {textoUnidad(totales[activo])}
                        </strong>
                    </div>
                )}

                <table className="dp-sr-only">
                    <caption>{descripcion}</caption>
                    <thead>
                        <tr>
                            <th scope="col">Periodo</th>
                            {series.map((s) => (
                                <th key={s.key} scope="col">{s.label}</th>
                            ))}
                            <th scope="col">Total</th>
                        </tr>
                    </thead>
                    <tbody>
                        {periodos.map((per, i) => (
                            <tr key={per.key}>
                                <th scope="row">{per.labelLargo ?? per.label}</th>
                                {series.map((s) => (
                                    <td key={s.key}>{valor(per.key, s.key)}</td>
                                ))}
                                <td>{totales[i]}</td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>
        </div>
    );
}

export default BarrasApiladas;
