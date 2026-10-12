BEGIN;

-- Estudiantes Service — Modelo maestro, etapa 4: FAMILIA (diseño E-02 §D,
-- permisos por dominio D-M5: estudiantes.familia.view/.update).
--
-- APLICAR con estudiantes_migrator actuando como dueño:
--   (echo "SET ROLE estudiantes_owner;"; cat 008_familia.sql) | \
--     psql -U estudiantes_migrator -d estudiantes_db -v ON_ERROR_STOP=1
-- Aditiva e idempotente.
--
-- Modelo normalizado (no 8 columnas de padre/madre en "estudiantes"):
--   personas               identidad de una persona relacionada; UNA fila
--                          aunque sea madre de varios estudiantes.
--   estudiante_familiares  relación estudiante-persona con parentesco,
--                          apoderado y vigencia (historial: no se borra,
--                          se cierra con "hasta").
-- Minimización: de la persona solo se guardan los datos que trae la fuente
-- institucional (documento, nombres y apellidos). Otros datos (ocupación,
-- contacto, cuenta en Auth) se agregarán cuando exista una necesidad
-- aprobada; nunca contraseñas.

-- Parentescos aprobados en E-02. El apoderado es un ROL de la relación
-- (es_apoderado), no un parentesco.
CREATE TABLE IF NOT EXISTS cat_parentescos (
    codigo VARCHAR(20) PRIMARY KEY,
    nombre VARCHAR(60) NOT NULL,
    activo BOOLEAN NOT NULL DEFAULT TRUE,
    orden SMALLINT NOT NULL DEFAULT 0,
    CONSTRAINT chk_cat_parentescos_codigo CHECK (codigo ~ '^[A-Z_]{2,20}$')
);

INSERT INTO cat_parentescos (codigo, nombre, orden) VALUES
    ('PADRE',       'Padre',       1),
    ('MADRE',       'Madre',       2),
    ('TUTOR_LEGAL', 'Tutor legal', 3),
    ('ABUELO_A',    'Abuelo(a)',   4),
    ('HERMANO_A',   'Hermano(a)',  5),
    ('TIO_A',       'Tío(a)',      6),
    ('OTRO',        'Otro',        7)
ON CONFLICT (codigo) DO NOTHING;

-- Persona: mismas reglas de documento que el estudiante (catálogo
-- cat_tipos_documento; SIN_DOCUMENTO = número NULL). La unicidad por
-- documento evita duplicar a una persona identificada; sin documento no se
-- puede deduplicar automáticamente (no se fusiona por nombres).
CREATE TABLE IF NOT EXISTS personas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tipo_documento VARCHAR(20) NOT NULL DEFAULT 'SIN_DOCUMENTO',
    numero_documento VARCHAR(12) NULL,
    nombres VARCHAR(100) NOT NULL,
    apellido_paterno VARCHAR(60) NULL,
    apellido_materno VARCHAR(60) NULL,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_personas_tipo_documento FOREIGN KEY (tipo_documento) REFERENCES cat_tipos_documento (codigo) ON DELETE RESTRICT,
    CONSTRAINT chk_personas_documento CHECK (
        (tipo_documento = 'SIN_DOCUMENTO' AND numero_documento IS NULL)
        OR (tipo_documento = 'DNI' AND numero_documento IS NOT NULL AND numero_documento ~ '^[0-9]{8}$')
        OR (tipo_documento IN ('CE', 'PASAPORTE') AND numero_documento IS NOT NULL
            AND numero_documento ~ '^[A-Z0-9]{4,12}$')
    ),
    CONSTRAINT chk_personas_textos CHECK (
        length(TRIM(BOTH FROM nombres)) > 0
        AND (apellido_paterno IS NULL OR length(TRIM(BOTH FROM apellido_paterno)) > 0)
        AND (apellido_materno IS NULL OR length(TRIM(BOTH FROM apellido_materno)) > 0)
    ),
    CONSTRAINT chk_personas_version CHECK (version >= 1)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_personas_documento
    ON personas (tipo_documento, numero_documento)
    WHERE numero_documento IS NOT NULL;

DROP TRIGGER IF EXISTS trg_personas_actualizar_version ON personas;
CREATE TRIGGER trg_personas_actualizar_version
    BEFORE UPDATE ON personas
    FOR EACH ROW EXECUTE FUNCTION estudiantes_actualizar_version();

-- Relación estudiante-persona. Vigencia: vigente = sin "hasta"; al
-- desvincular se cierra (vigente=false, hasta=fecha) y se conserva.
CREATE TABLE IF NOT EXISTS estudiante_familiares (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    estudiante_id UUID NOT NULL,
    persona_id UUID NOT NULL,
    parentesco VARCHAR(20) NOT NULL,
    es_apoderado BOOLEAN NOT NULL DEFAULT FALSE,
    vigente BOOLEAN NOT NULL DEFAULT TRUE,
    desde DATE NULL,
    hasta DATE NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_estudiante_familiares_estudiante FOREIGN KEY (estudiante_id) REFERENCES estudiantes (id) ON DELETE RESTRICT,
    CONSTRAINT fk_estudiante_familiares_persona FOREIGN KEY (persona_id) REFERENCES personas (id) ON DELETE RESTRICT,
    CONSTRAINT fk_estudiante_familiares_parentesco FOREIGN KEY (parentesco) REFERENCES cat_parentescos (codigo) ON DELETE RESTRICT,
    CONSTRAINT chk_estudiante_familiares_vigencia CHECK (
        (vigente AND hasta IS NULL) OR (NOT vigente AND hasta IS NOT NULL)
    ),
    CONSTRAINT chk_estudiante_familiares_fechas CHECK (hasta IS NULL OR desde IS NULL OR hasta >= desde)
);

-- Una relación vigente por par estudiante-persona (el historial se conserva).
CREATE UNIQUE INDEX IF NOT EXISTS uq_estudiante_familiares_vigente
    ON estudiante_familiares (estudiante_id, persona_id) WHERE vigente;
-- Un solo PADRE y una sola MADRE vigentes por estudiante.
CREATE UNIQUE INDEX IF NOT EXISTS uq_estudiante_familiares_progenitor
    ON estudiante_familiares (estudiante_id, parentesco) WHERE vigente AND parentesco IN ('PADRE', 'MADRE');
-- Un solo apoderado vigente por estudiante (regla aprobada en E-02).
CREATE UNIQUE INDEX IF NOT EXISTS uq_estudiante_familiares_apoderado
    ON estudiante_familiares (estudiante_id) WHERE vigente AND es_apoderado;
-- Una persona relacionada con varios estudiantes (búsqueda inversa).
CREATE INDEX IF NOT EXISTS idx_estudiante_familiares_persona ON estudiante_familiares (persona_id);

DROP TRIGGER IF EXISTS trg_estudiante_familiares_updated_at ON estudiante_familiares;
CREATE TRIGGER trg_estudiante_familiares_updated_at
    BEFORE UPDATE ON estudiante_familiares
    FOR EACH ROW EXECUTE FUNCTION tg_actualizar_updated_at();

-- Auditoría: nuevas acciones del dominio familiar (sin valores; el campo
-- registrado es "familiares").
ALTER TABLE auditoria_estudiantes DROP CONSTRAINT IF EXISTS chk_auditoria_estudiantes_accion;
ALTER TABLE auditoria_estudiantes ADD CONSTRAINT chk_auditoria_estudiantes_accion
    CHECK (accion IN ('CREATE', 'UPDATE', 'CHANGE_ESTADO', 'PERSONAL_VIEW', 'PERSONAL_UPDATE',
                      'FAMILIA_VIEW', 'FAMILIA_UPDATE'));

ALTER TABLE auditoria_estudiantes DROP CONSTRAINT IF EXISTS chk_auditoria_estudiantes_campos;
ALTER TABLE auditoria_estudiantes ADD CONSTRAINT chk_auditoria_estudiantes_campos CHECK (campos <@ ARRAY[
    'codigo', 'codigo_siagie', 'tipo_documento', 'numero_documento', 'nombres',
    'apellido_paterno', 'apellido_materno', 'apellidos', 'fecha_nacimiento', 'sexo',
    'pais_nacimiento', 'ubigeo_nacimiento', 'lengua_materna', 'segunda_lengua',
    'foto_ref', 'estado',
    'domicilio_direccion', 'domicilio_departamento', 'domicilio_provincia',
    'domicilio_distrito', 'domicilio_ubigeo', 'correo', 'telefono',
    'familiares'
]::TEXT[]);

-- Privilegios mínimos: la aplicación lee el catálogo, crea personas y
-- relaciones y cierra relaciones (UPDATE). Sin DELETE.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'estudiantes_user') THEN
        GRANT SELECT ON cat_parentescos TO estudiantes_user;
        GRANT SELECT, INSERT, UPDATE ON personas, estudiante_familiares TO estudiantes_user;
    END IF;
END
$$;

COMMIT;

-- Down migration (solo desarrollo):
-- DROP TABLE estudiante_familiares, personas, cat_parentescos;
-- (restaurar chk_auditoria_estudiantes_accion y _campos de 006)
