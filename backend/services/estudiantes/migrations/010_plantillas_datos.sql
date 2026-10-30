BEGIN;

-- Estudiantes Service — PLANTILLAS del motor de importación/exportación de
-- datos (configuraciones reutilizables: mapeo de columnas confirmado,
-- opciones de lectura, campos/encabezados/formato de exportación).
--
-- APLICAR con estudiantes_migrator actuando como dueño:
--   (echo "SET ROLE estudiantes_owner;"; cat 010_plantillas_datos.sql) | \
--     psql -U estudiantes_migrator -d estudiantes_db -v ON_ERROR_STOP=1
-- Aditiva e idempotente: crea UNA tabla nueva; no toca estudiantes,
-- matrículas ni auditoría. Necesaria porque el esquema no tenía dónde
-- guardar configuraciones (no son datos de estudiantes: no hay datos
-- personales en "configuracion", solo encabezados y nombres de campos).

CREATE TABLE IF NOT EXISTS plantillas_datos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    modulo VARCHAR(40) NOT NULL,
    tipo VARCHAR(20) NOT NULL,
    nombre VARCHAR(80) NOT NULL,
    formato VARCHAR(10) NULL,
    configuracion JSONB NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    creado_por UUID NOT NULL,
    actualizado_por UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_plantillas_datos_tipo CHECK (tipo IN ('IMPORTACION', 'EXPORTACION')),
    CONSTRAINT chk_plantillas_datos_nombre CHECK (length(TRIM(BOTH FROM nombre)) > 0),
    CONSTRAINT chk_plantillas_datos_modulo CHECK (modulo ~ '^[a-z_]{2,40}$'),
    CONSTRAINT chk_plantillas_datos_config CHECK (jsonb_typeof(configuracion) = 'object'
        AND pg_column_size(configuracion) <= 65536),
    CONSTRAINT chk_plantillas_datos_version CHECK (version >= 1)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_plantillas_datos_nombre
    ON plantillas_datos (modulo, tipo, lower(nombre));

DROP TRIGGER IF EXISTS trg_plantillas_datos_updated_at ON plantillas_datos;
CREATE TRIGGER trg_plantillas_datos_updated_at
    BEFORE UPDATE ON plantillas_datos
    FOR EACH ROW EXECUTE FUNCTION tg_actualizar_updated_at();

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'estudiantes_user') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON plantillas_datos TO estudiantes_user;
    END IF;
END
$$;

COMMIT;

-- Down migration (solo desarrollo):
-- DROP TABLE plantillas_datos;
