// Mapa de rol -> nombre legible en español. Las claves son EXACTAMENTE el
// claim "role" del JWT (los mismos códigos que usa cada microservicio en su
// internal/authorization) — no inventar valores nuevos acá.
export const ROLE_LABELS = {
    ADMIN: "Administración",
    DIRECTOR: "Dirección",
    SUBDIRECTOR: "Subdirección",
    SECRETARIA: "Secretaría",
    DOCENTE: "Docente",
    AUXILIAR: "Auxiliar",
    SOLICITANTE: "Solicitante",
};

export function roleLabel(role) {
    return ROLE_LABELS[role] ?? role ?? "";
}

// Posibles destinos de una derivación: los roles del sistema + "Sistema"
// (movimientos automáticos, tal como aparece repetido en la sección 15 del
// análisis funcional). El origen YA NO se elige de una lista: es siempre
// quien está logueado (ver DerivacionesPanel).
//
// NO incluye "Padre de familia" ni "Trabajador del Colegio" (F3): esos
// actores todavía no son un rol del sistema (F3-02 sigue pendiente de
// confirmar con la institución, ver docs/etapa-12-especificacion-funcional.md
// sección 23) — agregarlos acá sería resolver esa pregunta en silencio.
export const ACTORES_DESTINO = [
    { value: "Sistema", label: "Sistema" },
    ...Object.values(ROLE_LABELS).map((label) => ({ value: label, label })),
];
