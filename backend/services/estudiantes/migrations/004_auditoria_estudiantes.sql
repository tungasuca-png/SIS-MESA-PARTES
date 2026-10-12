BEGIN;

-- Estudiantes Service — FASE E-05: auditoría de la ESCRITURA del maestro
-- (CreateEstudiante, UpdateEstudiante, ChangeEstadoEstudiante).
--
-- APLICAR con estudiantes_migrator actuando como dueño:
--   (echo "SET ROLE estudiantes_owner;"; cat 004_auditoria_estudiantes.sql) | \
--     psql -U estudiantes_migrator -d estudiantes_db -v ON_ERROR_STOP=1
-- No destructiva e idempotente.
--
-- Nota de numeración: la migración de Matrícula (003 en E-02, 004 tras E-04)
-- pasa a ser 005.
--
-- Estrategia (E-05): la escribe la APLICACIÓN, en la MISMA transacción que el
-- cambio (mismo patrón que auditoria_psicologica). PostgreSQL no conoce al
-- usuario de la aplicación —todas las conexiones son estudiantes_user—, así que
-- un trigger no podría registrar "quién" sin que la aplicación se lo pase; el
-- usuario sale del JWT ya validado por el servicio, nunca del cliente.
--
-- Qué guarda: quién, cuándo, qué estudiante, qué operación, la versión
-- resultante y los NOMBRES de los campos modificados. Nunca valores (ni DNI,
-- ni fecha de nacimiento, ni nombres). Para CHANGE_ESTADO guarda además el
-- estado anterior y el nuevo (ACTIVO/INACTIVO, no son datos personales).
CREATE TABLE IF NOT EXISTS auditoria_estudiantes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    usuario_id UUID NOT NULL,
    accion VARCHAR(20) NOT NULL,
    estudiante_id UUID NOT NULL,
    fecha_hora TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version_resultante INTEGER NOT NULL,
    campos TEXT[] NOT NULL DEFAULT '{}',
    estado_anterior VARCHAR(20) NULL,
    estado_nuevo VARCHAR(20) NULL,
    CONSTRAINT fk_auditoria_estudiantes_estudiante
        FOREIGN KEY (estudiante_id) REFERENCES estudiantes (id) ON DELETE RESTRICT,
    CONSTRAINT chk_auditoria_estudiantes_accion CHECK (accion IN ('CREATE', 'UPDATE', 'CHANGE_ESTADO')),
    CONSTRAINT chk_auditoria_estudiantes_version CHECK (version_resultante >= 1),
    -- Solo nombres de columnas del maestro: impide que un valor termine aquí
    -- por error de programación.
    CONSTRAINT chk_auditoria_estudiantes_campos CHECK (campos <@ ARRAY[
        'codigo', 'codigo_siagie', 'tipo_documento', 'numero_documento', 'nombres',
        'apellido_paterno', 'apellido_materno', 'apellidos', 'fecha_nacimiento', 'sexo',
        'pais_nacimiento', 'ubigeo_nacimiento', 'lengua_materna', 'segunda_lengua',
        'foto_ref', 'estado'
    ]::TEXT[]),
    CONSTRAINT chk_auditoria_estudiantes_estados CHECK (
        (accion = 'CHANGE_ESTADO' AND estado_anterior IN ('ACTIVO', 'INACTIVO')
            AND estado_nuevo IN ('ACTIVO', 'INACTIVO') AND estado_anterior <> estado_nuevo)
        OR (accion <> 'CHANGE_ESTADO' AND estado_anterior IS NULL AND estado_nuevo IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_auditoria_estudiantes_estudiante_fecha
    ON auditoria_estudiantes (estudiante_id, fecha_hora);

CREATE INDEX IF NOT EXISTS idx_auditoria_estudiantes_usuario_fecha
    ON auditoria_estudiantes (usuario_id, fecha_hora);

-- Append-only, capa 2 (la capa 1 son los privilegios: la aplicación solo
-- recibe SELECT/INSERT). El trigger rechaza UPDATE, DELETE y TRUNCATE incluso
-- para quien conserve esos privilegios.
CREATE OR REPLACE FUNCTION auditoria_estudiantes_append_only() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'auditoria_estudiantes es append-only: % no permitido', TG_OP;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_auditoria_estudiantes_no_mod ON auditoria_estudiantes;
CREATE TRIGGER trg_auditoria_estudiantes_no_mod
    BEFORE UPDATE OR DELETE ON auditoria_estudiantes
    FOR EACH ROW EXECUTE FUNCTION auditoria_estudiantes_append_only();

DROP TRIGGER IF EXISTS trg_auditoria_estudiantes_no_truncate ON auditoria_estudiantes;
CREATE TRIGGER trg_auditoria_estudiantes_no_truncate
    BEFORE TRUNCATE ON auditoria_estudiantes
    FOR EACH STATEMENT EXECUTE FUNCTION auditoria_estudiantes_append_only();

-- Privilegios mínimos de la aplicación: SELECT/INSERT, sin UPDATE ni DELETE.
-- estudiantes (INSERT/UPDATE, sin DELETE) ya viene de la migración 001.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'estudiantes_user') THEN
        GRANT SELECT, INSERT ON auditoria_estudiantes TO estudiantes_user;
    END IF;
END
$$;

COMMIT;

-- Down migration (solo entornos de desarrollo; exige desactivar el trigger):
-- DROP TABLE IF EXISTS auditoria_estudiantes;
-- DROP FUNCTION IF EXISTS auditoria_estudiantes_append_only();
