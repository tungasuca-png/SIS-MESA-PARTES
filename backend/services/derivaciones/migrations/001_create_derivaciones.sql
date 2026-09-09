BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS derivaciones (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- expediente_id NO es una FK hacia Expedientes Service: Derivaciones es
    -- un microservicio independiente, sin tablas compartidas (mismo patrón
    -- ya usado en Documentos). Solo guarda el identificador; la existencia
    -- real del expediente se asume validada por quien originó el registro.
    expediente_id UUID NOT NULL,
    tipo VARCHAR(20) NOT NULL,
    origen VARCHAR(100) NOT NULL,
    destino VARCHAR(100) NOT NULL,
    motivo VARCHAR(500) NOT NULL,
    condicion VARCHAR(255) NOT NULL DEFAULT '',
    -- Quién registró el movimiento (usuario autenticado), no necesariamente
    -- el "origen" en sí (que es texto libre: puede ser un área/rol, o
    -- "Sistema" para movimientos automáticos).
    registrado_por UUID NOT NULL,
    fecha_registro TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_derivaciones_tipo CHECK (tipo IN ('DERIVACION', 'NOTIFICACION', 'ASIGNACION', 'RECEPCION', 'APROBACION')),
    CONSTRAINT chk_derivaciones_origen CHECK (length(TRIM(BOTH FROM origen)) > 0),
    CONSTRAINT chk_derivaciones_destino CHECK (length(TRIM(BOTH FROM destino)) > 0),
    CONSTRAINT chk_derivaciones_motivo CHECK (length(TRIM(BOTH FROM motivo)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_derivaciones_expediente_id
    ON derivaciones (expediente_id);

CREATE INDEX IF NOT EXISTS idx_derivaciones_tipo
    ON derivaciones (tipo);

COMMIT;

-- Down migration:
-- DROP TABLE IF EXISTS derivaciones;
