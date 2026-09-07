BEGIN;

CREATE TABLE IF NOT EXISTS user_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    nombres VARCHAR(100) NOT NULL,
    apellidos VARCHAR(100) NOT NULL,
    tipo_documento VARCHAR(20) NOT NULL,
    numero_documento VARCHAR(30) NOT NULL,
    telefono VARCHAR(20),
    direccion TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_user_profiles_user_id UNIQUE (user_id),
    CONSTRAINT uq_user_profiles_document UNIQUE (tipo_documento, numero_documento),
    CONSTRAINT fk_user_profiles_user
        FOREIGN KEY (user_id) REFERENCES usuarios(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    CONSTRAINT chk_user_profiles_nombres
        CHECK (length(TRIM(BOTH FROM nombres)) > 0),
    CONSTRAINT chk_user_profiles_apellidos
        CHECK (length(TRIM(BOTH FROM apellidos)) > 0),
    CONSTRAINT chk_user_profiles_tipo_documento
        CHECK (length(TRIM(BOTH FROM tipo_documento)) > 0),
    CONSTRAINT chk_user_profiles_numero_documento
        CHECK (length(TRIM(BOTH FROM numero_documento)) > 0)
);

COMMIT;

-- Down migration:
-- DROP TABLE IF EXISTS user_profiles;