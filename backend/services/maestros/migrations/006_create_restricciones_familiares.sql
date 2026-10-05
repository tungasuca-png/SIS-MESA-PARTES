BEGIN;

CREATE TABLE IF NOT EXISTS restricciones_familiares (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    relacion_familiar_id UUID NOT NULL,

    tipo VARCHAR(30) NOT NULL,
    descripcion TEXT NOT NULL,

    fecha_inicio DATE NOT NULL DEFAULT CURRENT_DATE,
    fecha_fin DATE,

    estado VARCHAR(20) NOT NULL DEFAULT 'ACTIVA',

    registrado_por UUID,
    motivo TEXT NOT NULL DEFAULT '',

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_restricciones_relacion
        FOREIGN KEY (relacion_familiar_id)
        REFERENCES relaciones_familiares(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT,

    CONSTRAINT chk_restricciones_tipo
        CHECK (
            tipo IN (
                'CONTACTO',
                'RECOJO',
                'INFORMACION',
                'LEGAL',
                'OTRA'
            )
        ),

    CONSTRAINT chk_restricciones_descripcion
        CHECK (
            length(TRIM(BOTH FROM descripcion)) > 0
        ),

    CONSTRAINT chk_restricciones_estado
        CHECK (
            estado IN (
                'ACTIVA',
                'LEVANTADA',
                'VENCIDA'
            )
        ),

    CONSTRAINT chk_restricciones_fechas
        CHECK (
            fecha_fin IS NULL
            OR fecha_fin >= fecha_inicio
        )
);

CREATE INDEX IF NOT EXISTS idx_restricciones_relacion
    ON restricciones_familiares (relacion_familiar_id);

CREATE INDEX IF NOT EXISTS idx_restricciones_estado
    ON restricciones_familiares (estado);

COMMIT;

-- Down migration:
-- DROP TABLE IF EXISTS restricciones_familiares;