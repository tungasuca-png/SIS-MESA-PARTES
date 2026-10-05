BEGIN;

CREATE TABLE IF NOT EXISTS historial_cambios (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    entidad VARCHAR(100) NOT NULL,
    entidad_id UUID NOT NULL,

    accion VARCHAR(10) NOT NULL,

    usuario_id UUID,
    servicio_origen VARCHAR(100) NOT NULL,

    valor_anterior JSONB,
    valor_nuevo JSONB,

    motivo TEXT NOT NULL DEFAULT '',

    fecha_cambio TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT chk_historial_entidad
        CHECK (
            length(TRIM(BOTH FROM entidad)) > 0
        ),

    CONSTRAINT chk_historial_accion
        CHECK (
            accion IN (
                'INSERT',
                'UPDATE',
                'DELETE'
            )
        ),

    CONSTRAINT chk_historial_servicio
        CHECK (
            length(TRIM(BOTH FROM servicio_origen)) > 0
        )
);

CREATE INDEX IF NOT EXISTS idx_historial_entidad
    ON historial_cambios (entidad, entidad_id);

CREATE INDEX IF NOT EXISTS idx_historial_usuario
    ON historial_cambios (usuario_id);

CREATE INDEX IF NOT EXISTS idx_historial_fecha
    ON historial_cambios (fecha_cambio);

CREATE INDEX IF NOT EXISTS idx_historial_accion
    ON historial_cambios (accion);

COMMIT;

-- Down migration:
-- DROP TABLE IF EXISTS historial_cambios;