BEGIN;

-- Psicología Service — FASE 6N-3 (C1): columnas para el contenido clínico
-- CIFRADO (AES-256-GCM en la aplicación, ver internal/clinicalcrypto).
--
-- APLICAR con psicologia_migrator actuando como dueño:
--   psql -U psicologia_migrator -d psicologia_db -c "SET ROLE psicologia_owner" -f 003_cifrado_contenido.sql
--
-- Esta migración NO borra nada: agrega las columnas cifradas junto a las de
-- texto claro. Los datos existentes se cifran después con
-- cmd/cifrar-contenido; las columnas en claro se eliminan recién en la 004,
-- cuando el 100 % esté cifrado y verificado.

-- ---------------------------------------------------------------------------
-- Columnas cifradas. Un bloque por fila de versión (JSON de todos los textos
-- libres de la hoja + motivo de corrección) y un bloque por campo en la
-- cabecera de la atención y en el expediente. cifrado_key_id identifica la
-- clave (rotación).
-- ---------------------------------------------------------------------------
ALTER TABLE atencion_versiones
    ADD COLUMN IF NOT EXISTS contenido_cifrado BYTEA NULL,
    ADD COLUMN IF NOT EXISTS cifrado_key_id VARCHAR(32) NULL;

ALTER TABLE atenciones_psicologicas
    ADD COLUMN IF NOT EXISTS motivo_anulacion_cifrado BYTEA NULL,
    ADD COLUMN IF NOT EXISTS cifrado_key_id VARCHAR(32) NULL;

ALTER TABLE expedientes_psicologicos
    ADD COLUMN IF NOT EXISTS resumen_caso_cifrado BYTEA NULL,
    ADD COLUMN IF NOT EXISTS motivo_cierre_cifrado BYTEA NULL,
    ADD COLUMN IF NOT EXISTS cifrado_key_id VARCHAR(32) NULL;

-- Coherencia bloque ↔ key_id.
ALTER TABLE atencion_versiones DROP CONSTRAINT IF EXISTS chk_atencion_versiones_cifrado;
ALTER TABLE atencion_versiones ADD CONSTRAINT chk_atencion_versiones_cifrado
    CHECK ((contenido_cifrado IS NULL) = (cifrado_key_id IS NULL));

ALTER TABLE atenciones_psicologicas DROP CONSTRAINT IF EXISTS chk_atenciones_psicologicas_cifrado;
ALTER TABLE atenciones_psicologicas ADD CONSTRAINT chk_atenciones_psicologicas_cifrado
    CHECK (motivo_anulacion_cifrado IS NULL OR cifrado_key_id IS NOT NULL);

ALTER TABLE expedientes_psicologicos DROP CONSTRAINT IF EXISTS chk_expedientes_psicologicos_cifrado;
ALTER TABLE expedientes_psicologicos ADD CONSTRAINT chk_expedientes_psicologicos_cifrado
    CHECK ((resumen_caso_cifrado IS NULL AND motivo_cierre_cifrado IS NULL) OR cifrado_key_id IS NOT NULL);

-- Las reglas que exigían texto (motivo de anulación, de cierre y de
-- corrección) aceptan ahora el texto claro (datos aún no migrados) o el
-- cifrado. La aplicación valida que el motivo no esté vacío antes de cifrarlo.
ALTER TABLE atenciones_psicologicas DROP CONSTRAINT IF EXISTS chk_atenciones_psicologicas_anulacion;
ALTER TABLE atenciones_psicologicas ADD CONSTRAINT chk_atenciones_psicologicas_anulacion CHECK (
    estado <> 'ANULADA'
    OR (anulada_por IS NOT NULL AND anulada_en IS NOT NULL
        AND (motivo_anulacion_cifrado IS NOT NULL
             OR length(TRIM(BOTH FROM COALESCE(motivo_anulacion, ''))) > 0))
);

ALTER TABLE expedientes_psicologicos DROP CONSTRAINT IF EXISTS chk_expedientes_psicologicos_cierre;
ALTER TABLE expedientes_psicologicos ADD CONSTRAINT chk_expedientes_psicologicos_cierre CHECK (
    estado <> 'CERRADO'
    OR (fecha_cierre IS NOT NULL
        AND (motivo_cierre_cifrado IS NOT NULL
             OR length(TRIM(BOTH FROM COALESCE(motivo_cierre, ''))) > 0))
);

ALTER TABLE atencion_versiones DROP CONSTRAINT IF EXISTS chk_atencion_versiones_correccion;
ALTER TABLE atencion_versiones ADD CONSTRAINT chk_atencion_versiones_correccion CHECK (
    numero_version <= 1
    OR (version_anterior_id IS NOT NULL
        AND (contenido_cifrado IS NOT NULL
             OR length(TRIM(BOTH FROM COALESCE(motivo_correccion, ''))) > 0))
);

-- ---------------------------------------------------------------------------
-- Canario: un valor conocido cifrado con cada clave. Psicología lo descifra al
-- arrancar y NO inicia si falla (clave ausente, incorrecta o no aprovisionada).
-- Lo aprovisiona cmd/cifrar-contenido --inicializar-canario (nunca el servicio).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS cifrado_canario (
    key_id VARCHAR(32) PRIMARY KEY,
    canario BYTEA NOT NULL,
    creado_en TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ---------------------------------------------------------------------------
-- Inmutabilidad de las versiones oficiales. Igual que antes, salvo UNA
-- transición controlada, necesaria para cifrar los datos existentes (y para
-- revertir ese cifrado mientras existan las columnas en claro): solo el dueño
-- (psicologia_owner, vía psicologia_migrator) puede reemplazar el texto claro
-- por el bloque cifrado —o viceversa— SIN cambiar ningún otro dato de la fila.
-- La 004 restaura la versión estricta al eliminar las columnas en claro.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION atencion_versiones_inmutables() RETURNS trigger AS $$
DECLARE
    misma_fila boolean;
    claro_nuevo_vacio boolean;
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'atencion_versiones: DELETE no permitido';
    END IF;
    IF OLD.numero_version = 0 THEN
        RETURN NEW; -- el borrador (y su paso a versión 1) sigue siendo editable
    END IF;

    misma_fila := (NEW.id, NEW.atencion_id, NEW.numero_version, NEW.version_anterior_id,
                   NEW.fecha_atencion, NEW.hora_atencion, NEW.realizada_por, NEW.motivos,
                   NEW.derivaciones, NEW.creada_por, NEW.creada_en)
        IS NOT DISTINCT FROM
                  (OLD.id, OLD.atencion_id, OLD.numero_version, OLD.version_anterior_id,
                   OLD.fecha_atencion, OLD.hora_atencion, OLD.realizada_por, OLD.motivos,
                   OLD.derivaciones, OLD.creada_por, OLD.creada_en);
    claro_nuevo_vacio := NEW.realizada_por_otros IS NULL AND NEW.motivo_otros IS NULL
        AND NEW.descripcion_motivo IS NULL AND NEW.analisis_integral IS NULL
        AND NEW.estrategias IS NULL AND NEW.derivacion_otro IS NULL
        AND NEW.seguimiento_resultados IS NULL AND NEW.acuerdos IS NULL
        AND NEW.compromiso_estudiante IS NULL AND NEW.compromiso_familia IS NULL
        AND NEW.motivo_correccion IS NULL;

    IF current_user = 'psicologia_owner' AND misma_fila THEN
        -- Cifrado de datos existentes: texto claro -> bloque cifrado.
        IF OLD.contenido_cifrado IS NULL AND NEW.contenido_cifrado IS NOT NULL
           AND NEW.cifrado_key_id IS NOT NULL AND claro_nuevo_vacio THEN
            RETURN NEW;
        END IF;
        -- Reversión (solo antes de la 004): bloque cifrado -> texto claro.
        IF OLD.contenido_cifrado IS NOT NULL AND NEW.contenido_cifrado IS NULL
           AND NEW.cifrado_key_id IS NULL THEN
            RETURN NEW;
        END IF;
    END IF;

    RAISE EXCEPTION 'atencion_versiones: la versión % es inmutable', OLD.numero_version;
END;
$$ LANGUAGE plpgsql;

-- ---------------------------------------------------------------------------
-- Privilegios: la aplicación solo lee el canario.
-- ---------------------------------------------------------------------------
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'psicologia_user') THEN
        GRANT SELECT ON cifrado_canario TO psicologia_user;
    END IF;
END
$$;

COMMIT;
