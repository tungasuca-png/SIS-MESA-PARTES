// Actores del sistema, en español legible — se usan como lista cerrada para
// "origen"/"destino" de una derivación (antes eran texto libre). Son los
// roles reales de Auth Service más "Sistema" (movimientos automáticos, tal
// como aparece repetido en la sección 15 del análisis funcional).
//
// NO incluye "Padre de familia" ni "Trabajador del Colegio" (F3): esos
// actores todavía no son un rol del sistema (F3-02 sigue pendiente de
// confirmar con la institución, ver docs/etapa-12-especificacion-funcional.md
// sección 23) — agregarlos acá sería resolver esa pregunta en silencio.
export const ACTORES = [
    { value: "Sistema", label: "Sistema" },
    { value: "Solicitante", label: "Solicitante" },
    { value: "Secretaría", label: "Secretaría" },
    { value: "Subdirección", label: "Subdirección" },
    { value: "Dirección", label: "Dirección" },
    { value: "Docente", label: "Docente" },
    { value: "Auxiliar", label: "Auxiliar" },
    { value: "Administración", label: "Administración" },
];
