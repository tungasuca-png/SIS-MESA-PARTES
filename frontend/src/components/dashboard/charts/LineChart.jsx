import { useEffect, useRef, useState } from "react";
import "./charts.css";

// Gráfico de líneas liviano en SVG (el proyecto no tiene librería de
// gráficos y no hace falta una para una sola serie). Mide su contenedor
// para que el texto no se deforme al escalar; muestra un tooltip con el
// punto más cercano al puntero y una tabla oculta para lectores de pantalla.
//
// puntos: [{ key, label, labelLargo, value }]

const PAD = { top: 48, right: 12, bottom: 26, left: 34 };

// Máximo "redondo" del eje Y con 4 divisiones enteras.
function escalaY(max) {
    if (max <= 4) return { max: 4, paso: 1 };
    const crudo = max / 4;
    const potencia = 10 ** Math.floor(Math.log10(crudo));
    const paso = [1, 2, 5, 10].map((f) => f * potencia).find((p) => p >= crudo);
    return { max: paso * 4, paso };
}

function LineChart({ puntos, alto = 200, descripcion, unidad = "expedientes" }) {
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

    const n = puntos.length;
    const { max, paso } = escalaY(Math.max(0, ...puntos.map((p) => p.value)));
    const w = Math.max(ancho - PAD.left - PAD.right, 10);
    const h = alto - PAD.top - PAD.bottom;
    const x = (i) => PAD.left + (n <= 1 ? w / 2 : (i * w) / (n - 1));
    const y = (v) => PAD.top + h - (v / max) * h;

    const linea = puntos.map((p, i) => `${i === 0 ? "M" : "L"}${x(i).toFixed(1)},${y(p.value).toFixed(1)}`).join(" ");
    const area = n > 0 ? `${linea} L${x(n - 1).toFixed(1)},${PAD.top + h} L${x(0).toFixed(1)},${PAD.top + h} Z` : "";

    const marcas = [];
    for (let v = 0; v <= max; v += paso) marcas.push(v);

    // Etiquetas del eje X: según el ancho (~1 cada 72px, máx. 6), siempre la última.
    const maxEtiquetas = Math.max(2, Math.min(6, Math.floor(w / 72)));
    const cada = Math.max(1, Math.ceil(n / maxEtiquetas));
    const indicesX = [];
    for (let i = 0; i < n; i += cada) indicesX.push(i);
    if (n > 0 && indicesX[indicesX.length - 1] !== n - 1) {
        // La última etiqueta reemplaza a la anterior si quedarían encimadas.
        if (n - 1 - indicesX[indicesX.length - 1] < cada / 2) indicesX.pop();
        indicesX.push(n - 1);
    }
    const etiquetasX = indicesX.map((i) => ({ ...puntos[i], i }));

    const mostrarMarcadores = n <= 31;

    const alMover = (event) => {
        if (n === 0) return;
        const rect = event.currentTarget.getBoundingClientRect();
        const px = ((event.clientX - rect.left) / rect.width) * ancho;
        const i = n <= 1 ? 0 : Math.round(((px - PAD.left) / w) * (n - 1));
        setActivo(Math.min(n - 1, Math.max(0, i)));
    };

    const p = activo !== null ? puntos[activo] : null;
    const tooltipIzq = p ? Math.min(Math.max(x(activo), 70), ancho - 70) : 0;

    return (
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
                        <line className="dp-chart-grid" x1={PAD.left} x2={PAD.left + w} y1={y(v)} y2={y(v)} />
                        <text className="dp-chart-axis" x={PAD.left - 8} y={y(v)} textAnchor="end" dominantBaseline="middle">
                            {v}
                        </text>
                    </g>
                ))}
                {etiquetasX.map((e) => (
                    <text key={e.key} className="dp-chart-axis" x={x(e.i)} y={alto - 6} textAnchor={e.i === 0 ? "start" : e.i === n - 1 ? "end" : "middle"}>
                        {e.label}
                    </text>
                ))}
                {n > 0 && <path className="dp-chart-area" d={area} />}
                {n > 0 && <path className="dp-chart-line" d={linea} />}
                {mostrarMarcadores &&
                    puntos.map((pt, i) => <circle key={pt.key} className="dp-chart-dot" cx={x(i)} cy={y(pt.value)} r={2.5} />)}
                {p && (
                    <g>
                        <line className="dp-chart-guide" x1={x(activo)} x2={x(activo)} y1={PAD.top} y2={PAD.top + h} />
                        <circle className="dp-chart-dot dp-chart-dot--active" cx={x(activo)} cy={y(p.value)} r={4} />
                    </g>
                )}
            </svg>

            {p && (
                <div className="dp-chart-tooltip" style={{ left: tooltipIzq, top: 0 }} aria-hidden="true">
                    <span className="dp-chart-tooltip-label">{p.labelLargo ?? p.label}</span>
                    <strong>
                        {p.value} {p.value === 1 ? unidad.replace(/s$/, "") : unidad}
                    </strong>
                </div>
            )}

            <table className="dp-sr-only">
                <caption>{descripcion}</caption>
                <tbody>
                    {puntos.map((pt) => (
                        <tr key={pt.key}>
                            <th scope="row">{pt.labelLargo ?? pt.label}</th>
                            <td>{pt.value}</td>
                        </tr>
                    ))}
                </tbody>
            </table>
        </div>
    );
}

export default LineChart;
