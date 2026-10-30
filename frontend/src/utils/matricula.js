// Presentación de matrículas y secciones (solo formato; las reglas son del
// backend).
export const seccionLabel = (s) =>
    s ? [s.nivel, `${s.grado}°`, `«${s.seccion}»`, s.turno].filter(Boolean).join(" · ") : "Sin sección asignada";

// Matrícula del periodo EN_CURSO (la vigente), si existe.
export const matriculaVigente = (matriculas) => (matriculas || []).find((m) => m.periodo?.estado === "EN_CURSO") || null;

// Fecha de la primera matrícula registrada (no es la "fecha de ingreso"
// oficial: el sistema no guarda ese dato todavía).
export const primeraMatricula = (matriculas) =>
    (matriculas || [])
        .map((m) => m.fecha_matricula)
        .filter(Boolean)
        .sort()[0] || "";
