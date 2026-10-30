import "./charts.css";

// Barras horizontales ordenadas (categorías con nombres largos, como los
// tipos de trámite). Cada barra es proporcional al mayor valor; el
// porcentaje es sobre `total` (el universo real de la métrica), así que no
// tiene por qué sumar 100 si se omiten categorías.
//
// items: [{ key, label, value }]
function BarList({ items, total, descripcion }) {
    const mayor = Math.max(1, ...items.map((item) => item.value));

    return (
        <ul className="dp-barlist" aria-label={descripcion}>
            {items.map((item) => {
                const porcentaje = total > 0 ? Math.round((item.value / total) * 100) : 0;
                return (
                    <li key={item.key} className="dp-barlist-item">
                        <div className="dp-barlist-head">
                            <span className="dp-barlist-label">{item.label}</span>
                            <span className="dp-barlist-value">
                                {item.value}
                                {total > 0 && <span className="dp-barlist-pct">{porcentaje}%</span>}
                            </span>
                        </div>
                        <div className="dp-barlist-track" aria-hidden="true">
                            <div className="dp-barlist-fill" style={{ width: `${(item.value / mayor) * 100}%` }} />
                        </div>
                    </li>
                );
            })}
        </ul>
    );
}

export default BarList;
