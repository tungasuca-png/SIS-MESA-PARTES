BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE IF NOT EXISTS personas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    tipo_documento VARCHAR(20) NOT NULL,
    numero_documento VARCHAR(20) NOT NULL,

    nombres VARCHAR(100) NOT NULL,
    apellido_paterno VARCHAR(100) NOT NULL,
    apellido_materno VARCHAR(100) NOT NULL DEFAULT '',

    fecha_nacimiento DATE,
    sexo VARCHAR(20),

    correo_personal CITEXT,
    telefono VARCHAR(20),

    activo BOOLEAN NOT NULL DEFAULT TRUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT uq_personas_documento
        UNIQUE (tipo_documento, numero_documento),

    CONSTRAINT chk_personas_tipo_documento
        CHECK (
            tipo_documento IN ('DNI', 'CE', 'PASAPORTE', 'OTRO')
        ),

    CONSTRAINT chk_personas_numero_documento
        CHECK (
            length(TRIM(BOTH FROM numero_documento)) > 0
        ),

    CONSTRAINT chk_personas_nombres
        CHECK (
            length(TRIM(BOTH FROM nombres)) > 0
        ),

    CONSTRAINT chk_personas_apellido_paterno
        CHECK (
            length(TRIM(BOTH FROM apellido_paterno)) > 0
        )
);

CREATE INDEX IF NOT EXISTS idx_personas_numero_documento
    ON personas (numero_documento);

CREATE INDEX IF NOT EXISTS idx_personas_apellidos
    ON personas (apellido_paterno, apellido_materno);

CREATE INDEX IF NOT EXISTS idx_personas_activo
    ON personas (activo);

COMMIT;

-- Down migration:
-- DROP TABLE IF EXISTS personas;