BEGIN;

-- Psicología Service — FASE 6N-3 (C1): elimina las columnas de contenido
-- clínico en TEXTO CLARO. Solo el bloque cifrado queda en la base.
--
-- APLICAR con psicologia_migrator actuando como dueño, y SOLO después de que
-- cmd/cifrar-contenido --verificar informe 0 pendientes y 100 % descifrable:
--   psql -U psicologia_migrator -d psicologia_db -c "SET ROLE psicologia_owner" -f 004_eliminar_texto_plano.sql
-- Luego, fuera de la transacción: VACUUM FULL (ver al final).
--
-- IRREVERSIBLE: después de esta migración, cmd/cifrar-contenido --revertir ya
-- no aplica; la única vuelta atrás es restaurar el backup previo.

-- Salvaguarda: nada pendiente de cifrar ni texto claro residual.
DO $$
DECLARE
    pendientes bigint;
BEGIN
    SELECT
        (SELECT COUNT(*) FROM atencion_versiones WHERE contenido_cifrado IS NULL OR cifrado_key_id IS NULL
            OR realizada_por_otros IS NOT NULL OR motivo_otros IS NOT NULL OR descripcion_motivo IS NOT NULL
            OR analisis_integral IS NOT NULL OR estrategias IS NOT NULL OR derivacion_otro IS NOT NULL
            OR seguimiento_resultados IS NOT NULL OR acuerdos IS NOT NULL OR compromiso_estudiante IS NOT NULL
            OR compromiso_familia IS NOT NULL OR motivo_correccion IS NOT NULL)
      + (SELECT COUNT(*) FROM atenciones_psicologicas WHERE motivo_anulacion IS NOT NULL
            OR (estado = 'ANULADA' AND motivo_anulacion_cifrado IS NULL))
      + (SELECT COUNT(*) FROM expedientes_psicologicos WHERE resumen_caso IS NOT NULL OR motivo_cierre IS NOT NULL
            OR (estado = 'CERRADO' AND motivo_cierre_cifrado IS NULL))
    INTO pendientes;
    IF pendientes > 0 THEN
        RAISE EXCEPTION '004 abortada: % fila(s) con contenido sin cifrar o texto claro residual (ejecutar cmd/cifrar-contenido)', pendientes;
    END IF;
END
$$;

-- Reglas que referencian columnas en claro: se reemplazan por su versión
-- solo-cifrada ANTES de eliminar las columnas.
ALTER TABLE atenciones_psicologicas DROP CONSTRAINT IF EXISTS chk_atenciones_psicologicas_anulacion;
ALTER TABLE atenciones_psicologicas ADD CONSTRAINT chk_atenciones_psicologicas_anulacion CHECK (
    estado <> 'ANULADA'
    OR (anulada_por IS NOT NULL AND anulada_en IS NOT NULL AND motivo_anulacion_cifrado IS NOT NULL)
);

ALTER TABLE expedientes_psicologicos DROP CONSTRAINT IF EXISTS chk_expedientes_psicologicos_cierre;
ALTER TABLE expedientes_psicologicos ADD CONSTRAINT chk_expedientes_psicologicos_cierre CHECK (
    estado <> 'CERRADO' OR (fecha_cierre IS NOT NULL AND motivo_cierre_cifrado IS NOT NULL)
);

ALTER TABLE atencion_versiones DROP CONSTRAINT IF EXISTS chk_atencion_versiones_correccion;
ALTER TABLE atencion_versiones ADD CONSTRAINT chk_atencion_versiones_correccion CHECK (
    numero_version <= 1 OR version_anterior_id IS NOT NULL
);

-- Inmutabilidad ESTRICTA otra vez (sin la transición de cifrado de la 003).
-- Una futura rotación de claves necesitará su propia transición controlada.
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

-- Eliminación del texto claro.
ALTER TABLE atencion_versiones
    DROP COLUMN realizada_por_otros,
    DROP COLUMN motivo_otros,
    DROP COLUMN descripcion_motivo,
    DROP COLUMN analisis_integral,
    DROP COLUMN estrategias,
    DROP COLUMN derivacion_otro,
    DROP COLUMN seguimiento_resultados,
    DROP COLUMN acuerdos,
    DROP COLUMN compromiso_estudiante,
    DROP COLUMN compromiso_familia,
    DROP COLUMN motivo_correccion;

ALTER TABLE atenciones_psicologicas DROP COLUMN motivo_anulacion;

ALTER TABLE expedientes_psicologicos
    DROP COLUMN resumen_caso,
    DROP COLUMN motivo_cierre;

-- Toda versión tiene su bloque cifrado.
ALTER TABLE atencion_versiones
    ALTER COLUMN contenido_cifrado SET NOT NULL,
    ALTER COLUMN cifrado_key_id SET NOT NULL;

COMMIT;

-- Después, fuera de la transacción (como psicologia_owner), para reescribir las
-- tablas sin las tuplas muertas que aún contienen el texto claro anterior:
--   VACUUM FULL atencion_versiones, atenciones_psicologicas, expedientes_psicologicos;
-- Los backups tomados ANTES de esta migración contienen texto claro: se
-- custodian y destruyen según el procedimiento institucional aprobado.
