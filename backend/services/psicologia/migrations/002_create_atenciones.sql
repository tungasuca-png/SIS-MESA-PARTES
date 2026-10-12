BEGIN;

-- Psicología Service: atenciones psicológicas (Hoja de Atención).
--
-- APLICAR CON UN ROL ADMINISTRADOR (dueño de la base), igual que la 001.
--
-- Modelo: expediente psicológico 1 ── N atenciones (SIN límite: los dos
-- bloques de la hoja impresa son solo formato de papel). Cada atención tiene
-- versiones: la versión 0 es el borrador (lo único modificable); al registrar
-- pasa a ser la versión 1 y desde ahí toda corrección INSERTA una versión
-- nueva. Nada se borra físicamente: la aplicación no recibe DELETE.
--
-- AVISO DE SEGURIDAD (BLOQUEO VIGENTE, ver docs/modulo-psicologia-diseno.md):
-- el contenido de la hoja se guarda en texto plano. NO cargar datos clínicos
-- reales hasta resolver el cifrado en reposo (gestión de claves) y la
-- revisión legal (Ley N.º 29733).

-- ---------------------------------------------------------------------------
-- Catálogos (configurables: nunca se borran, se desactivan con activo=false).
-- Los valores iniciales son los de la Hoja de Atención institucional.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS catalogo_motivos_atencion (
    codigo VARCHAR(40) PRIMARY KEY,
    etiqueta VARCHAR(100) NOT NULL,
    activo BOOLEAN NOT NULL DEFAULT TRUE,
    orden INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT chk_catalogo_motivos_codigo CHECK (codigo ~ '^[A-Z_]+$'),
    CONSTRAINT chk_catalogo_motivos_etiqueta CHECK (length(TRIM(BOTH FROM etiqueta)) > 0)
);

INSERT INTO catalogo_motivos_atencion (codigo, etiqueta, orden) VALUES
    ('ACADEMICO',      'Académico',      1),
    ('CONDUCTUAL',     'Conductual',     2),
    ('SOCIOEMOCIONAL', 'Socioemocional', 3),
    ('INCLUSION',      'Inclusión',      4),
    ('FAMILIAR',       'Familiar',       5)
ON CONFLICT (codigo) DO NOTHING;

CREATE TABLE IF NOT EXISTS catalogo_destinos_derivacion (
    codigo VARCHAR(40) PRIMARY KEY,
    etiqueta VARCHAR(100) NOT NULL,
    activo BOOLEAN NOT NULL DEFAULT TRUE,
    orden INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT chk_catalogo_destinos_codigo CHECK (codigo ~ '^[A-Z_]+$'),
    CONSTRAINT chk_catalogo_destinos_etiqueta CHECK (length(TRIM(BOTH FROM etiqueta)) > 0)
);

-- OTRO exige el texto libre "derivacion_otro" (validado en la aplicación).
INSERT INTO catalogo_destinos_derivacion (codigo, etiqueta, orden) VALUES
    ('TUTORIA',       'Tutoría',         1),
    ('PSICOLOGIA',    'Psicología',      2),
    ('SAANEE',        'SAANEE',          3),
    ('DEMUNA',        'DEMUNA',          4),
    ('CENTRO_SALUD',  'Centro de Salud', 5),
    ('OTRO',          'Otro',            6)
ON CONFLICT (codigo) DO NOTHING;

-- ---------------------------------------------------------------------------
-- Correlativo de atención por expediente (mismo patrón que
-- expediente_codigo_counters): INSERT ... ON CONFLICT DO UPDATE ... RETURNING
-- dentro de la transacción que registra la atención. Seguro ante
-- concurrencia; UNIQUE(expediente_id, numero) es la garantía final.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS atencion_numero_counters (
    expediente_id UUID PRIMARY KEY REFERENCES expedientes_psicologicos (id) ON DELETE RESTRICT,
    ultimo INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT chk_atencion_numero_counters_ultimo CHECK (ultimo >= 0)
);

-- ---------------------------------------------------------------------------
-- Cabecera de la atención.
--   BORRADOR   -> editable solo por su autor, sin número.
--   REGISTRADA -> oficial, con número; el contenido solo cambia por versión nueva.
--   ANULADA    -> sigue visible, con motivo; nunca se borra.
--   DESCARTADA -> borrador abandonado por su autor (no se borra físicamente;
--                 no aparece en los listados).
-- FIRMADA NO existe todavía: depende de una decisión institucional sobre la firma.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS atenciones_psicologicas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    expediente_id UUID NOT NULL REFERENCES expedientes_psicologicos (id) ON DELETE RESTRICT,
    numero INTEGER NULL,
    estado VARCHAR(20) NOT NULL DEFAULT 'BORRADOR',
    autor_id UUID NOT NULL,
    registrada_por UUID NULL,
    registrada_en TIMESTAMPTZ NULL,
    version_vigente INTEGER NOT NULL DEFAULT 0,
    revision INTEGER NOT NULL DEFAULT 0,
    anulada_por UUID NULL,
    anulada_en TIMESTAMPTZ NULL,
    motivo_anulacion TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_atenciones_psicologicas_numero UNIQUE (expediente_id, numero),
    CONSTRAINT chk_atenciones_psicologicas_estado CHECK (estado IN ('BORRADOR', 'REGISTRADA', 'ANULADA', 'DESCARTADA')),
    CONSTRAINT chk_atenciones_psicologicas_numero CHECK (
        (estado IN ('BORRADOR', 'DESCARTADA')) = (numero IS NULL)
    ),
    CONSTRAINT chk_atenciones_psicologicas_numero_positivo CHECK (numero IS NULL OR numero > 0),
    CONSTRAINT chk_atenciones_psicologicas_registro CHECK (
        estado NOT IN ('REGISTRADA', 'ANULADA')
        OR (registrada_por IS NOT NULL AND registrada_en IS NOT NULL AND version_vigente >= 1)
    ),
    CONSTRAINT chk_atenciones_psicologicas_anulacion CHECK (
        estado <> 'ANULADA'
        OR (anulada_por IS NOT NULL AND anulada_en IS NOT NULL
            AND length(TRIM(BOTH FROM COALESCE(motivo_anulacion, ''))) > 0)
    ),
    CONSTRAINT chk_atenciones_psicologicas_revision CHECK (revision >= 0),
    CONSTRAINT chk_atenciones_psicologicas_version CHECK (version_vigente >= 0)
);

CREATE INDEX IF NOT EXISTS idx_atenciones_psicologicas_expediente_estado
    ON atenciones_psicologicas (expediente_id, estado);

CREATE INDEX IF NOT EXISTS idx_atenciones_psicologicas_autor_estado
    ON atenciones_psicologicas (autor_id, estado);

-- ---------------------------------------------------------------------------
-- Contenido de la Hoja de Atención, versionado.
--   numero_version = 0 -> borrador (única fila modificable).
--   numero_version >= 1 -> versión oficial INMUTABLE (trigger abajo).
-- realizada_por: valores fijos de la hoja (Docente/Auxiliar/Directivo/Psicólogo).
-- motivos / derivaciones: [{"codigo": "...", "etiqueta": "..."}] — la etiqueta
-- queda congelada aunque el catálogo cambie después.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS atencion_versiones (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    atencion_id UUID NOT NULL REFERENCES atenciones_psicologicas (id) ON DELETE RESTRICT,
    numero_version INTEGER NOT NULL,
    version_anterior_id UUID NULL REFERENCES atencion_versiones (id) ON DELETE RESTRICT,
    fecha_atencion DATE NULL,
    hora_atencion TIME NULL,
    realizada_por TEXT[] NOT NULL DEFAULT '{}',
    realizada_por_otros TEXT NULL,
    motivos JSONB NOT NULL DEFAULT '[]',
    motivo_otros TEXT NULL,
    descripcion_motivo TEXT NULL,
    analisis_integral TEXT NULL,
    estrategias TEXT NULL,
    derivaciones JSONB NOT NULL DEFAULT '[]',
    derivacion_otro TEXT NULL,
    seguimiento_resultados TEXT NULL,
    acuerdos TEXT NULL,
    compromiso_estudiante TEXT NULL,
    compromiso_familia TEXT NULL,
    motivo_correccion TEXT NULL,
    creada_por UUID NOT NULL,
    creada_en TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_atencion_versiones_numero UNIQUE (atencion_id, numero_version),
    CONSTRAINT chk_atencion_versiones_numero CHECK (numero_version >= 0),
    CONSTRAINT chk_atencion_versiones_realizada_por CHECK (
        realizada_por <@ ARRAY['DOCENTE', 'AUXILIAR', 'DIRECTIVO', 'PSICOLOGO']::TEXT[]
    ),
    CONSTRAINT chk_atencion_versiones_motivos CHECK (jsonb_typeof(motivos) = 'array'),
    CONSTRAINT chk_atencion_versiones_derivaciones CHECK (jsonb_typeof(derivaciones) = 'array'),
    -- Una versión oficial siempre tiene fecha y hora de atención.
    CONSTRAINT chk_atencion_versiones_fecha_hora CHECK (
        numero_version = 0 OR (fecha_atencion IS NOT NULL AND hora_atencion IS NOT NULL)
    ),
    -- Toda corrección (v2 en adelante) lleva motivo y apunta a la versión anterior.
    CONSTRAINT chk_atencion_versiones_correccion CHECK (
        numero_version <= 1
        OR (length(TRIM(BOTH FROM COALESCE(motivo_correccion, ''))) > 0 AND version_anterior_id IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_atencion_versiones_atencion
    ON atencion_versiones (atencion_id, numero_version);

-- Inmutabilidad de las versiones oficiales (capa 2; la capa 1 es que la
-- aplicación no tiene DELETE). Solo el borrador (versión 0) admite UPDATE,
-- incluido el UPDATE que lo convierte en la versión 1 al registrar.
CREATE OR REPLACE FUNCTION atencion_versiones_inmutables() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'atencion_versiones: DELETE no permitido';
    END IF;
    IF OLD.numero_version > 0 THEN
        RAISE EXCEPTION 'atencion_versiones: la versión % es inmutable', OLD.numero_version;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_atencion_versiones_inmutables ON atencion_versiones;
CREATE TRIGGER trg_atencion_versiones_inmutables
    BEFORE UPDATE OR DELETE ON atencion_versiones
    FOR EACH ROW EXECUTE FUNCTION atencion_versiones_inmutables();

-- ---------------------------------------------------------------------------
-- Auditoría: nuevas acciones y recursos válidos.
-- ---------------------------------------------------------------------------
ALTER TABLE auditoria_psicologica DROP CONSTRAINT IF EXISTS chk_auditoria_psicologica_accion;
ALTER TABLE auditoria_psicologica ADD CONSTRAINT chk_auditoria_psicologica_accion CHECK (
    accion IN ('CREATE', 'VIEW', 'UPDATE', 'CLOSE', 'REOPEN', 'DENIED',
               'REGISTER', 'CORRECT', 'ANNUL', 'DISCARD', 'EXPORT')
);

ALTER TABLE auditoria_psicologica DROP CONSTRAINT IF EXISTS chk_auditoria_psicologica_recurso_valido;
ALTER TABLE auditoria_psicologica ADD CONSTRAINT chk_auditoria_psicologica_recurso_valido CHECK (
    recurso IN ('EXPEDIENTE_PSICOLOGICO', 'ATENCION_PSICOLOGICA')
);

-- ---------------------------------------------------------------------------
-- Privilegios mínimos de la aplicación: sin DELETE en ninguna tabla.
-- ---------------------------------------------------------------------------
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'psicologia_user') THEN
        GRANT SELECT ON catalogo_motivos_atencion TO psicologia_user;
        GRANT SELECT ON catalogo_destinos_derivacion TO psicologia_user;
        GRANT SELECT, INSERT, UPDATE ON atencion_numero_counters TO psicologia_user;
        GRANT SELECT, INSERT, UPDATE ON atenciones_psicologicas TO psicologia_user;
        GRANT SELECT, INSERT, UPDATE ON atencion_versiones TO psicologia_user;
    END IF;
END
$$;

COMMIT;

-- Down migration (solo entornos de desarrollo, sin datos reales):
-- DROP TABLE IF EXISTS atencion_versiones;
-- DROP FUNCTION IF EXISTS atencion_versiones_inmutables();
-- DROP TABLE IF EXISTS atenciones_psicologicas;
-- DROP TABLE IF EXISTS atencion_numero_counters;
-- DROP TABLE IF EXISTS catalogo_destinos_derivacion;
-- DROP TABLE IF EXISTS catalogo_motivos_atencion;
-- ALTER TABLE auditoria_psicologica DROP CONSTRAINT IF EXISTS chk_auditoria_psicologica_recurso_valido;
-- (y restaurar chk_auditoria_psicologica_accion a las 6 acciones originales)
