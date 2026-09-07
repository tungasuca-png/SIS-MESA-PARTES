BEGIN;

CREATE EXTENSION IF NOT EXISTS citext;

-- Perfil de negocio del usuario. El id NO se genera aquí: es el mismo UUID
-- que Auth Service asignó al usuario (por eso no hay DEFAULT gen_random_uuid).
-- No existe FK hacia auth_db: son bases aisladas por microservicio y la
-- relación se resuelve por API/gRPC, nunca por SQL entre bases.
CREATE TABLE IF NOT EXISTS usuarios (
    id UUID PRIMARY KEY,
    nombres VARCHAR(100) NOT NULL,
    apellidos VARCHAR(100) NOT NULL,
    dni VARCHAR(20) NOT NULL DEFAULT '',
    telefono VARCHAR(20) NOT NULL DEFAULT '',
    correo CITEXT NOT NULL DEFAULT '',
    direccion TEXT NOT NULL DEFAULT '',
    tipo_usuario VARCHAR(20) NOT NULL,
    activo BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_usuarios_nombres CHECK (length(TRIM(BOTH FROM nombres)) > 0),
    CONSTRAINT chk_usuarios_apellidos CHECK (length(TRIM(BOTH FROM apellidos)) > 0),
    CONSTRAINT chk_usuarios_tipo CHECK (
        tipo_usuario IN ('ADMIN', 'DIRECTOR', 'SUBDIRECTOR', 'SECRETARIA', 'DOCENTE', 'AUXILIAR', 'SOLICITANTE')
    )
);

-- DNI único solo cuando está informado (cadena vacía = dato aún no capturado).
CREATE UNIQUE INDEX IF NOT EXISTS uq_usuarios_dni
    ON usuarios (dni)
    WHERE dni <> '';

CREATE INDEX IF NOT EXISTS idx_usuarios_tipo_usuario
    ON usuarios (tipo_usuario);

CREATE INDEX IF NOT EXISTS idx_usuarios_apellidos
    ON usuarios (apellidos);

COMMIT;

-- Down migration:
-- DROP TABLE IF EXISTS usuarios;
