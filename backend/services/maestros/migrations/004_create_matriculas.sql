BEGIN;

CREATE TABLE IF NOT EXISTS matriculas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    estudiante_id UUID NOT NULL,
    periodo_lectivo_id UUID NOT NULL,
    seccion_id UUID NOT NULL,

    estado VARCHAR(20) NOT NULL DEFAULT 'ACTIVA',
    fecha_registro TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_retiro TIMESTAMPTZ,

    observaciones TEXT NOT NULL DEFAULT '',

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_matriculas_estudiante
        FOREIGN KEY (estudiante_id)
        REFERENCES estudiantes(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT,

    CONSTRAINT fk_matriculas_periodo
        FOREIGN KEY (periodo_lectivo_id)
        REFERENCES periodos_lectivos(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT,

    CONSTRAINT fk_matriculas_seccion
        FOREIGN KEY (seccion_id)
        REFERENCES secciones(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT,

    CONSTRAINT uq_matriculas_estudiante_periodo
        UNIQUE (estudiante_id, periodo_lectivo_id),

    CONSTRAINT chk_matriculas_estado
        CHECK (
            estado IN (
                'ACTIVA',
                'PROMOVIDA',
                'RETIRADA',
                'TRASLADADA',
                'CULMINADA'
            )
        )
);

CREATE INDEX IF NOT EXISTS idx_matriculas_estudiante
    ON matriculas (estudiante_id);

CREATE INDEX IF NOT EXISTS idx_matriculas_periodo
    ON matriculas (periodo_lectivo_id);

CREATE INDEX IF NOT EXISTS idx_matriculas_seccion
    ON matriculas (seccion_id);

CREATE INDEX IF NOT EXISTS idx_matriculas_estado
    ON matriculas (estado);

COMMIT;

-- Down migration:
-- DROP TABLE IF EXISTS matriculas;