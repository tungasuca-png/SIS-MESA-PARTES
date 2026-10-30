// Pestaña de un módulo que todavía no existe en el sistema. No muestra datos
// de ejemplo ni botones que no hagan nada: solo explica qué falta.
function ProximamenteTab({ titulo, descripcion, requiere }) {
    return (
        <section className="dp-panel est-soon">
            <div className="est-soon-head">
                <h3 className="dp-panel-title">{titulo}</h3>
                <span className="est-tab-badge est-tab-badge--soon">Próximamente</span>
            </div>
            <p>{descripcion}</p>
            <p className="est-form-note">{requiere}</p>
        </section>
    );
}

export default ProximamenteTab;
