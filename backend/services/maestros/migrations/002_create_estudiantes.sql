BEGIN;

CREATE TABLE IF NOT EXISTS estudiantes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    persona_id UUID NOT NULL,

    codigo_institucional VARCHAR(30) NOT NULL,
    codigo_siagie VARCHAR(30),

    estado_academico VARCHAR(20) NOT NULL DEFAULT 'MATRICULADO',

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_estudiantes_persona
        FOREIGN KEY (persona_id)
        REFERENCES personas(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT,

    CONSTRAINT uq_estudiantes_persona
        UNIQUE (persona_id),

    CONSTRAINT uq_estudiantes_codigo_institucional
        UNIQUE (codigo_institucional),

    CONSTRAINT uq_estudiantes_codigo_siagie
        UNIQUE (codigo_siagie),

    CONSTRAINT chk_estudiantes_estado
        CHECK (
            estado_academico IN (
                'MATRICULADO',
                'RETIRADO',
                'TRASLADADO',
                'EGRESADO'
            )
        )
);

CREATE INDEX IF NOT EXISTS idx_estudiantes_persona_id
    ON estudiantes (persona_id);

CREATE INDEX IF NOT EXISTS idx_estudiantes_estado
    ON estudiantes (estado_academico);

COMMIT;

-- Down migration:
-- DROP TABLE IF EXISTS estudiantes;