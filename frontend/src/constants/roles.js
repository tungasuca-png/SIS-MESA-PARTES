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

// Inverso de ROLE_LABELS: a partir de un "destino" elegido en una
// derivación (que es el label en español, ver ACTORES_DESTINO), encuentra
// el código de rol/área real. Devuelve null para "Sistema" o cualquier
// label que no corresponda a un rol interno (no hay un área que actualizar
// en esos casos — ver DerivacionesPanel).
export function roleForLabel(label) {
    const entry = Object.entries(ROLE_LABELS).find(([, value]) => value === label);
    return entry ? entry[0] : null;
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

// Áreas reales a las que se puede DERIVAR un expediente (Etapa 3): mismo
// vocabulario que expedientes.area_actual y internal/estados.IsValidArea
// del backend (value = código de área, no el label) — a diferencia de
// ACTORES_DESTINO (que es texto libre para el resto de tipos de
// derivación), acá el backend valida la transición real, así que el value
// debe ser el código exacto que espera DerivarExpediente. No incluye
// SOLICITANTE (no es un área interna a la que se derive un expediente).
export const AREAS_DESTINO = Object.entries(ROLE_LABELS)
    .filter(([role]) => role !== "SOLICITANTE")
    .map(([value, label]) => ({ value, label }));
