BEGIN;

-- Estudiantes Service — FASE E-03: identidad del maestro institucional de
-- estudiantes y catálogo base de tipos de documento (diseño aprobado en E-02).
--
-- APLICAR con estudiantes_migrator actuando como dueño:
--   (echo "SET ROLE estudiantes_owner;"; cat 002_identidad_y_catalogos_base.sql) | \
--     psql -U estudiantes_migrator -d estudiantes_db -v ON_ERROR_STOP=1
--
-- NO destructiva: no elimina columnas ni filas. nivel/grado/seccion/turno/
-- estado_matricula se CONSERVAN hasta la fase de Matrícula (periodos,
-- secciones, matrículas) y hasta que ningún consumidor las lea.
-- Idempotente: puede reaplicarse sin efecto.

-- ---------------------------------------------------------------------------
-- 1. Catálogo de tipos de documento (único catálogo necesario en esta fase;
--    países, ubigeo y lenguas se cargan en la migración 008).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS cat_tipos_documento (
    codigo VARCHAR(20) PRIMARY KEY,
    nombre VARCHAR(60) NOT NULL,
    activo BOOLEAN NOT NULL DEFAULT TRUE,
    orden SMALLINT NOT NULL DEFAULT 0,
    CONSTRAINT chk_cat_tipos_documento_codigo CHECK (codigo ~ '^[A-Z_]{2,20}$')
);

INSERT INTO cat_tipos_documento (codigo, nombre, orden) VALUES
    ('DNI',           'Documento Nacional de Identidad', 1),
    ('CE',            'Carné de Extranjería',            2),
    ('PASAPORTE',     'Pasaporte',                       3),
    ('SIN_DOCUMENTO', 'Sin documento',                   4)
ON CONFLICT (codigo) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 2. Datos existentes: documento vacío -> NULL + SIN_DOCUMENTO; tipo vacío con
--    un número de 8 dígitos -> DNI. Cualquier otra combinación no se adivina:
--    la migración se detiene para revisarla a mano.
-- ---------------------------------------------------------------------------
ALTER TABLE estudiantes ALTER COLUMN numero_documento DROP NOT NULL;
ALTER TABLE estudiantes ALTER COLUMN numero_documento DROP DEFAULT;

UPDATE estudiantes SET numero_documento = NULL
    WHERE numero_documento IS NOT NULL AND TRIM(numero_documento) = '';
UPDATE estudiantes SET numero_documento = UPPER(TRIM(numero_documento))
    WHERE numero_documento IS NOT NULL AND numero_documento <> UPPER(TRIM(numero_documento));
UPDATE estudiantes SET tipo_documento = 'SIN_DOCUMENTO'
    WHERE numero_documento IS NULL AND tipo_documento IN ('', 'SIN_DOCUMENTO');
UPDATE estudiantes SET tipo_documento = 'DNI'
    WHERE tipo_documento = '' AND numero_documento ~ '^[0-9]{8}$';

DO $$
DECLARE
    pendientes bigint;
BEGIN
    SELECT COUNT(*) INTO pendientes FROM estudiantes
    WHERE tipo_documento NOT IN (SELECT codigo FROM cat_tipos_documento)
       OR (tipo_documento = 'SIN_DOCUMENTO') <> (numero_documento IS NULL);
    IF pendientes > 0 THEN
        RAISE EXCEPTION '002 detenida: % estudiante(s) con tipo/número de documento que no se puede clasificar automáticamente', pendientes;
    END IF;
END
$$;

ALTER TABLE estudiantes ALTER COLUMN tipo_documento SET DEFAULT 'SIN_DOCUMENTO';

ALTER TABLE estudiantes DROP CONSTRAINT IF EXISTS fk_estudiantes_tipo_documento;
ALTER TABLE estudiantes ADD CONSTRAINT fk_estudiantes_tipo_documento
    FOREIGN KEY (tipo_documento) REFERENCES cat_tipos_documento (codigo) ON DELETE RESTRICT;

-- Formato por tipo. SIN_DOCUMENTO <-> número NULL (varios estudiantes sin
-- documento son válidos). Un tipo nuevo en el catálogo exige ampliar este CHECK.
-- "IS NOT NULL" explícito: sin él, un número NULL con tipo DNI haría que la
-- expresión valga NULL y el CHECK la aceptaría.
ALTER TABLE estudiantes DROP CONSTRAINT IF EXISTS chk_estudiantes_documento;
ALTER TABLE estudiantes ADD CONSTRAINT chk_estudiantes_documento CHECK (
    (tipo_documento = 'SIN_DOCUMENTO' AND numero_documento IS NULL)
    OR (tipo_documento = 'DNI' AND numero_documento IS NOT NULL AND numero_documento ~ '^[0-9]{8}$')
    OR (tipo_documento IN ('CE', 'PASAPORTE') AND numero_documento IS NOT NULL
        AND numero_documento ~ '^[A-Z0-9]{4,12}$')
);

-- Unicidad del documento: (tipo, número) solo cuando hay número. El mismo
-- número con distinto tipo (p. ej. DNI y CE) es un documento distinto.
CREATE UNIQUE INDEX IF NOT EXISTS uq_estudiantes_documento
    ON estudiantes (tipo_documento, numero_documento)
    WHERE numero_documento IS NOT NULL;

-- ---------------------------------------------------------------------------
-- 3. Nuevos campos de identidad (todos opcionales salvo estado y version).
-- ---------------------------------------------------------------------------
ALTER TABLE estudiantes
    ADD COLUMN IF NOT EXISTS codigo_siagie VARCHAR(14) NULL,
    ADD COLUMN IF NOT EXISTS apellido_paterno VARCHAR(60) NULL,
    ADD COLUMN IF NOT EXISTS apellido_materno VARCHAR(60) NULL,
    ADD COLUMN IF NOT EXISTS sexo CHAR(1) NULL,
    ADD COLUMN IF NOT EXISTS pais_nacimiento CHAR(2) NULL,
    ADD COLUMN IF NOT EXISTS ubigeo_nacimiento CHAR(6) NULL,
    ADD COLUMN IF NOT EXISTS lengua_materna VARCHAR(10) NULL,
    ADD COLUMN IF NOT EXISTS segunda_lengua VARCHAR(10) NULL,
    ADD COLUMN IF NOT EXISTS foto_ref VARCHAR(200) NULL,
    ADD COLUMN IF NOT EXISTS estado VARCHAR(20) NOT NULL DEFAULT 'ACTIVO',
    ADD COLUMN IF NOT EXISTS version INTEGER NOT NULL DEFAULT 1;

ALTER TABLE estudiantes DROP CONSTRAINT IF EXISTS chk_estudiantes_codigo_siagie;
ALTER TABLE estudiantes ADD CONSTRAINT chk_estudiantes_codigo_siagie
    CHECK (codigo_siagie IS NULL OR codigo_siagie ~ '^[0-9]{14}$');

-- apellidos se mantiene por compatibilidad (RPC y consumidores actuales).
-- Cuando se registran los apellidos separados, apellidos debe coincidir con
-- su concatenación: así nunca divergen. No se separan automáticamente los
-- datos existentes (apellidos compuestos como "DE LA CRUZ" no se adivinan).
ALTER TABLE estudiantes DROP CONSTRAINT IF EXISTS chk_estudiantes_apellidos_separados;
ALTER TABLE estudiantes ADD CONSTRAINT chk_estudiantes_apellidos_separados CHECK (
    apellido_paterno IS NULL
    OR (length(TRIM(BOTH FROM apellido_paterno)) > 0
        AND apellidos = TRIM(BOTH FROM apellido_paterno || ' ' || COALESCE(apellido_materno, '')))
);

ALTER TABLE estudiantes DROP CONSTRAINT IF EXISTS chk_estudiantes_sexo;
ALTER TABLE estudiantes ADD CONSTRAINT chk_estudiantes_sexo CHECK (sexo IS NULL OR sexo IN ('M', 'F'));

-- País (ISO 3166-1 alfa-2), ubigeo INEI (6 dígitos) y lengua: formato ahora;
-- la FK a cat_paises/cat_ubigeo/cat_lenguas se agrega en la migración 008,
-- cuando esos catálogos oficiales se carguen.
ALTER TABLE estudiantes DROP CONSTRAINT IF EXISTS chk_estudiantes_pais_nacimiento;
ALTER TABLE estudiantes ADD CONSTRAINT chk_estudiantes_pais_nacimiento
    CHECK (pais_nacimiento IS NULL OR pais_nacimiento ~ '^[A-Z]{2}$');

ALTER TABLE estudiantes DROP CONSTRAINT IF EXISTS chk_estudiantes_ubigeo_nacimiento;
ALTER TABLE estudiantes ADD CONSTRAINT chk_estudiantes_ubigeo_nacimiento CHECK (
    ubigeo_nacimiento IS NULL
    OR (ubigeo_nacimiento ~ '^[0-9]{6}$' AND COALESCE(pais_nacimiento, '') = 'PE')
);

ALTER TABLE estudiantes DROP CONSTRAINT IF EXISTS chk_estudiantes_lenguas;
ALTER TABLE estudiantes ADD CONSTRAINT chk_estudiantes_lenguas CHECK (
    (lengua_materna IS NULL OR lengua_materna ~ '^[A-Z0-9_]{1,10}$')
    AND (segunda_lengua IS NULL OR segunda_lengua ~ '^[A-Z0-9_]{1,10}$')
    AND (segunda_lengua IS NULL OR segunda_lengua IS DISTINCT FROM lengua_materna)
);

-- Fotografía: solo una referencia opaca a un almacenamiento privado. Nunca
-- una URL pública ni el archivo en la base.
ALTER TABLE estudiantes DROP CONSTRAINT IF EXISTS chk_estudiantes_foto_ref;
ALTER TABLE estudiantes ADD CONSTRAINT chk_estudiantes_foto_ref CHECK (
    foto_ref IS NULL OR (length(TRIM(BOTH FROM foto_ref)) > 0 AND foto_ref !~* '^[a-z][a-z0-9+.-]*://')
);

-- Estado del REGISTRO del estudiante (no de su matrícula, que sigue en
-- estado_matricula hasta la fase de Matrícula).
ALTER TABLE estudiantes DROP CONSTRAINT IF EXISTS chk_estudiantes_estado;
ALTER TABLE estudiantes ADD CONSTRAINT chk_estudiantes_estado CHECK (estado IN ('ACTIVO', 'INACTIVO'));

ALTER TABLE estudiantes DROP CONSTRAINT IF EXISTS chk_estudiantes_version;
ALTER TABLE estudiantes ADD CONSTRAINT chk_estudiantes_version CHECK (version >= 1);

CREATE UNIQUE INDEX IF NOT EXISTS uq_estudiantes_codigo_siagie
    ON estudiantes (codigo_siagie)
    WHERE codigo_siagie IS NOT NULL;

-- ---------------------------------------------------------------------------
-- 4. updated_at y version mantenidos por la base en TODO UPDATE: la aplicación
--    no debe tocarlos. El control optimista futuro (UpdateEstudiante) solo
--    compara: UPDATE ... WHERE id = $1 AND version = $esperada.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION estudiantes_actualizar_version() RETURNS trigger AS $$
BEGIN
    NEW.updated_at := CURRENT_TIMESTAMP;
    NEW.version := OLD.version + 1;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_estudiantes_actualizar_version ON estudiantes;
CREATE TRIGGER trg_estudiantes_actualizar_version
    BEFORE UPDATE ON estudiantes
    FOR EACH ROW EXECUTE FUNCTION estudiantes_actualizar_version();

-- ---------------------------------------------------------------------------
-- 5. Privilegios mínimos: la aplicación solo lee el catálogo.
-- ---------------------------------------------------------------------------
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'estudiantes_user') THEN
        GRANT SELECT ON cat_tipos_documento TO estudiantes_user;
    END IF;
END
$$;

COMMIT;

-- Down (solo desarrollo, sin datos reales): quitar trigger, constraints e
-- índices nuevos, las columnas nuevas y cat_tipos_documento; volver
-- numero_documento a NOT NULL DEFAULT '' (NULL -> '') y tipo_documento DEFAULT ''.
