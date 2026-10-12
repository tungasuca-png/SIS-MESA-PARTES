BEGIN;

-- Psicología Service: base del módulo (FASE 1).
--
-- APLICAR CON UN ROL ADMINISTRADOR (dueño de la base), NO con psicologia_user:
-- las tablas deben pertenecer al administrador para que la aplicación reciba
-- solo los privilegios mínimos (ver GRANT al final) y no pueda alterar ni
-- borrar la bitácora de auditoría. Ver README.md.

-- Contador atómico por año: codigo = PSI-<año>-<consecutivo de 6 dígitos>.
-- Se incrementa con INSERT ... ON CONFLICT DO UPDATE dentro de la misma
-- transacción que crea el expediente, así que es seguro ante concurrencia y
-- no deja huecos si el INSERT del expediente falla (todo se revierte junto).
CREATE TABLE IF NOT EXISTS expediente_codigo_counters (
    anio INTEGER PRIMARY KEY,
    ultimo INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT chk_expediente_codigo_counters_ultimo CHECK (ultimo >= 0)
);

-- Expediente psicológico: NO es el expediente administrativo de Mesa de
-- Partes. Un estudiante = un expediente (UNIQUE físico en PostgreSQL).
CREATE TABLE IF NOT EXISTS expedientes_psicologicos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- estudiante_id NO es una FK: el dueño de los datos del estudiante será un
    -- Estudiantes Service (DEPENDENCIA PENDIENTE). Aquí solo se guarda la
    -- referencia; nada de nombre, DNI, grado ni sección del estudiante.
    estudiante_id UUID NOT NULL,
    codigo VARCHAR(20) NOT NULL,
    estado VARCHAR(20) NOT NULL DEFAULT 'ABIERTO',
    -- Profesional responsable y quien abrió el expediente: UUID de Auth, sin FK
    -- (bases aisladas por microservicio).
    responsable_id UUID NOT NULL,
    abierto_por UUID NOT NULL,
    fecha_apertura TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- fecha_cierre/motivo_cierre conservan el ÚLTIMO cierre aunque el
    -- expediente se reabra; el estado vigente lo define "estado".
    fecha_cierre TIMESTAMPTZ NULL,
    motivo_cierre TEXT NULL,
    resumen_caso TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_expedientes_psicologicos_estudiante UNIQUE (estudiante_id),
    CONSTRAINT uq_expedientes_psicologicos_codigo UNIQUE (codigo),
    CONSTRAINT chk_expedientes_psicologicos_codigo CHECK (codigo ~ '^PSI-[0-9]{4}-[0-9]{6}$'),
    CONSTRAINT chk_expedientes_psicologicos_estado CHECK (estado IN ('ABIERTO', 'EN_SEGUIMIENTO', 'CERRADO')),
    CONSTRAINT chk_expedientes_psicologicos_cierre CHECK (
        estado <> 'CERRADO'
        OR (fecha_cierre IS NOT NULL AND length(TRIM(BOTH FROM COALESCE(motivo_cierre, ''))) > 0)
    )
);

CREATE INDEX IF NOT EXISTS idx_expedientes_psicologicos_responsable
    ON expedientes_psicologicos (responsable_id);

CREATE INDEX IF NOT EXISTS idx_expedientes_psicologicos_estado
    ON expedientes_psicologicos (estado);

CREATE INDEX IF NOT EXISTS idx_expedientes_psicologicos_fecha_apertura
    ON expedientes_psicologicos (fecha_apertura);

-- Bitácora de auditoría append-only. NUNCA guarda contenido clínico, JWT,
-- claves ni documentos: solo quién, qué, sobre qué recurso y cuándo. "motivo"
-- es un código corto del sistema (p. ej. PERMISO_INSUFICIENTE), no texto libre
-- del usuario.
CREATE TABLE IF NOT EXISTS auditoria_psicologica (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    usuario_id UUID NOT NULL,
    accion VARCHAR(20) NOT NULL,
    recurso VARCHAR(50) NOT NULL,
    recurso_id UUID NULL,
    estudiante_id UUID NULL,
    fecha_hora TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resultado VARCHAR(20) NOT NULL,
    motivo VARCHAR(100) NULL,
    CONSTRAINT chk_auditoria_psicologica_accion CHECK (accion IN ('CREATE', 'VIEW', 'UPDATE', 'CLOSE', 'REOPEN', 'DENIED')),
    CONSTRAINT chk_auditoria_psicologica_resultado CHECK (resultado IN ('PERMITIDO', 'DENEGADO')),
    CONSTRAINT chk_auditoria_psicologica_recurso CHECK (length(TRIM(BOTH FROM recurso)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_auditoria_psicologica_recurso
    ON auditoria_psicologica (recurso, recurso_id);

CREATE INDEX IF NOT EXISTS idx_auditoria_psicologica_usuario_fecha
    ON auditoria_psicologica (usuario_id, fecha_hora);

CREATE INDEX IF NOT EXISTS idx_auditoria_psicologica_fecha
    ON auditoria_psicologica (fecha_hora);

-- Inmutabilidad, capa 2 (la capa 1 son los privilegios: la aplicación solo
-- recibe SELECT/INSERT). El trigger rechaza UPDATE, DELETE y TRUNCATE incluso
-- para quien conserve esos privilegios.
CREATE OR REPLACE FUNCTION auditoria_psicologica_append_only() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'auditoria_psicologica es append-only: % no permitido', TG_OP;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_auditoria_psicologica_no_mod ON auditoria_psicologica;
CREATE TRIGGER trg_auditoria_psicologica_no_mod
    BEFORE UPDATE OR DELETE ON auditoria_psicologica
    FOR EACH ROW EXECUTE FUNCTION auditoria_psicologica_append_only();

DROP TRIGGER IF EXISTS trg_auditoria_psicologica_no_truncate ON auditoria_psicologica;
CREATE TRIGGER trg_auditoria_psicologica_no_truncate
    BEFORE TRUNCATE ON auditoria_psicologica
    FOR EACH STATEMENT EXECUTE FUNCTION auditoria_psicologica_append_only();

-- Privilegios mínimos para la aplicación. Sin DELETE en ninguna tabla (la
-- aplicación no borra físicamente) y sin UPDATE sobre la auditoría.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'psicologia_user') THEN
        GRANT SELECT, INSERT, UPDATE ON expedientes_psicologicos TO psicologia_user;
        GRANT SELECT, INSERT, UPDATE ON expediente_codigo_counters TO psicologia_user;
        GRANT SELECT, INSERT ON auditoria_psicologica TO psicologia_user;
    END IF;
END
$$;

COMMIT;

-- Down migration (solo entornos de desarrollo):
-- DROP TABLE IF EXISTS auditoria_psicologica;
-- DROP FUNCTION IF EXISTS auditoria_psicologica_append_only();
-- DROP TABLE IF EXISTS expedientes_psicologicos;
-- DROP TABLE IF EXISTS expediente_codigo_counters;
