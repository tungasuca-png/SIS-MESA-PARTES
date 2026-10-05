BEGIN;

-- Catálogo institucional de parentescos.
CREATE TABLE IF NOT EXISTS tipos_parentesco (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    codigo VARCHAR(30) NOT NULL,
    nombre VARCHAR(50) NOT NULL,
    estado BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT uq_tipos_parentesco_codigo UNIQUE (codigo),
    CONSTRAINT uq_tipos_parentesco_nombre UNIQUE (nombre)
);


-- Relación entre una Persona (familiar) y un Estudiante.
CREATE TABLE IF NOT EXISTS relaciones_familiares (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    familiar_persona_id UUID NOT NULL,
    estudiante_id UUID NOT NULL,
    tipo_parentesco_id UUID NOT NULL,

    es_apoderado BOOLEAN NOT NULL DEFAULT FALSE,
    vive_con_estudiante BOOLEAN NOT NULL DEFAULT FALSE,

    prioridad_contacto INTEGER,

    estado_autorizacion VARCHAR(20) NOT NULL DEFAULT 'PENDIENTE',

    tiene_restriccion BOOLEAN NOT NULL DEFAULT FALSE,

    fecha_inicio DATE NOT NULL DEFAULT CURRENT_DATE,
    fecha_fin DATE,

    activo BOOLEAN NOT NULL DEFAULT TRUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_relaciones_familiar
        FOREIGN KEY (familiar_persona_id)
        REFERENCES personas(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT,

    CONSTRAINT fk_relaciones_estudiante
        FOREIGN KEY (estudiante_id)
        REFERENCES estudiantes(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT,

    CONSTRAINT fk_relaciones_parentesco
        FOREIGN KEY (tipo_parentesco_id)
        REFERENCES tipos_parentesco(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT,

    CONSTRAINT uq_relacion_familiar_estudiante
        UNIQUE (familiar_persona_id, estudiante_id, tipo_parentesco_id),

    CONSTRAINT chk_relaciones_prioridad
        CHECK (
            prioridad_contacto IS NULL
            OR prioridad_contacto >= 1
        ),

    CONSTRAINT chk_relaciones_autorizacion
        CHECK (
            estado_autorizacion IN (
                'PENDIENTE',
                'VALIDADA',
                'RECHAZADA',
                'REVOCADA'
            )
        ),

    CONSTRAINT chk_relaciones_fechas
        CHECK (
            fecha_fin IS NULL
            OR fecha_fin >= fecha_inicio
        )
);

CREATE INDEX IF NOT EXISTS idx_relaciones_familiar
    ON relaciones_familiares (familiar_persona_id);

CREATE INDEX IF NOT EXISTS idx_relaciones_estudiante
    ON relaciones_familiares (estudiante_id);

CREATE INDEX IF NOT EXISTS idx_relaciones_apoderado
    ON relaciones_familiares (estudiante_id, es_apoderado)
    WHERE activo = TRUE;

COMMIT;

-- Down migration:
-- DROP TABLE IF EXISTS relaciones_familiares;
-- DROP TABLE IF EXISTS tipos_parentesco;