BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS expedientes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    codigo VARCHAR(20) NOT NULL,
    tipo VARCHAR(20) NOT NULL,
    asunto VARCHAR(255) NOT NULL,
    descripcion TEXT NOT NULL DEFAULT '',
    -- solicitante_id NO es una FK hacia Auth Service: Expedientes es un
    -- microservicio independiente y no comparte tablas con Auth. Solo
    -- guarda el identificador del usuario; la identidad/autorización se
    -- valida mediante el JWT que llega vía Gateway.
    solicitante_id UUID NOT NULL,
    estado VARCHAR(20) NOT NULL DEFAULT 'PENDIENTE',
    prioridad VARCHAR(20) NOT NULL DEFAULT 'NORMAL',
    fecha_registro TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_actualizacion TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    activo BOOLEAN NOT NULL DEFAULT TRUE,
    CONSTRAINT uq_expedientes_codigo UNIQUE (codigo),
    CONSTRAINT chk_expedientes_asunto CHECK (length(TRIM(BOTH FROM asunto)) > 0),
    CONSTRAINT chk_expedientes_tipo CHECK (tipo IN ('SOLICITUD', 'TRAMITE', 'OFICIO', 'OTRO')),
    CONSTRAINT chk_expedientes_estado CHECK (estado IN ('PENDIENTE', 'EN_PROCESO', 'ATENDIDO', 'OBSERVADO')),
    CONSTRAINT chk_expedientes_prioridad CHECK (prioridad IN ('NORMAL', 'URGENTE'))
);

CREATE INDEX IF NOT EXISTS idx_expedientes_solicitante_id
    ON expedientes (solicitante_id);

CREATE INDEX IF NOT EXISTS idx_expedientes_estado
    ON expedientes (estado);

CREATE INDEX IF NOT EXISTS idx_expedientes_fecha_registro
    ON expedientes (fecha_registro);

-- Contador atómico por año usado para generar codigo = EXP-<anio>-<consecutivo>.
CREATE TABLE IF NOT EXISTS expediente_codigo_counters (
    anio INTEGER PRIMARY KEY,
    ultimo INTEGER NOT NULL DEFAULT 0
);

COMMIT;

-- Down migration:
-- DROP TABLE IF EXISTS expediente_codigo_counters;
-- DROP TABLE IF EXISTS expedientes;
