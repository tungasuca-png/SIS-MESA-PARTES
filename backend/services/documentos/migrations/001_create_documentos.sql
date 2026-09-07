BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS documentos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- expediente_id NO es una FK hacia Expedientes Service: Documentos es un
    -- microservicio independiente, sin tablas compartidas. Solo guarda el
    -- identificador; la existencia real del expediente se asume validada
    -- por quien originó la carga (frontend/Gateway ya lo consultó antes).
    expediente_id UUID NOT NULL,
    nombre VARCHAR(255) NOT NULL,
    tipo_documento VARCHAR(20) NOT NULL,
    extension VARCHAR(10) NOT NULL,
    tamano_bytes BIGINT NOT NULL,
    contenido BYTEA NOT NULL,
    subido_por UUID NOT NULL,
    estado VARCHAR(20) NOT NULL DEFAULT 'ACTIVO',
    fecha_registro TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_documentos_nombre CHECK (length(TRIM(BOTH FROM nombre)) > 0),
    CONSTRAINT chk_documentos_tipo CHECK (tipo_documento IN ('ADJUNTO', 'PROVEIDO', 'ACTA', 'INFORME', 'FORMATO', 'OTRO')),
    CONSTRAINT chk_documentos_extension CHECK (extension IN ('pdf', 'jpg', 'jpeg', 'png', 'doc', 'docx')),
    CONSTRAINT chk_documentos_tamano CHECK (tamano_bytes > 0),
    CONSTRAINT chk_documentos_estado CHECK (estado IN ('ACTIVO', 'ELIMINADO'))
);

CREATE INDEX IF NOT EXISTS idx_documentos_expediente_id
    ON documentos (expediente_id);

CREATE INDEX IF NOT EXISTS idx_documentos_estado
    ON documentos (estado);

COMMIT;

-- Down migration:
-- DROP TABLE IF EXISTS documentos;
