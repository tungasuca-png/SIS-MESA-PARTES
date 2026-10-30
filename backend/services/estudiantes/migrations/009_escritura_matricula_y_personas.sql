BEGIN;

-- Estudiantes Service — Modelo maestro, etapa 5: ESCRITURA de matrícula y
-- gestión de PERSONAS (padres y apoderados).
--
-- APLICAR con estudiantes_migrator actuando como dueño:
--   (echo "SET ROLE estudiantes_owner;"; cat 009_escritura_matricula_y_personas.sql) | \
--     psql -U estudiantes_migrator -d estudiantes_db -v ON_ERROR_STOP=1
-- Aditiva e idempotente: no borra ni reescribe filas. Solo:
--   1. Movimiento CONCLUSION en matricula_movimientos (el estado CONCLUIDO
--      ya existía en matriculas desde 007; faltaba su movimiento).
--   2. Auditoría de PERSONAS: auditoria_estudiantes admite filas de una
--      persona (persona_id) sin estudiante. La tabla es la MISMA (no se crea
--      otra auditoría); las filas existentes no cambian (todas tienen
--      estudiante_id).
--   3. Acciones nuevas de auditoría y nombres de campos permitidos.
--   4. Privilegios de escritura de la aplicación sobre matrícula, sin DELETE.
--
-- Las columnas académicas heredadas de "estudiantes" (nivel, grado, seccion,
-- turno, estado_matricula) NO se modifican ni se sincronizan aquí: la
-- decisión sobre su carga y retiro sigue pendiente (ADR-08).

-- 1. Movimiento de conclusión del periodo.
ALTER TABLE matricula_movimientos DROP CONSTRAINT IF EXISTS chk_movimientos_tipo;
ALTER TABLE matricula_movimientos ADD CONSTRAINT chk_movimientos_tipo
    CHECK (tipo IN ('CAMBIO_SECCION', 'CAMBIO_TURNO', 'RETIRO', 'REINGRESO', 'TRASLADO_SALIDA', 'CONCLUSION'));

-- 2. Auditoría de personas en la misma tabla.
ALTER TABLE auditoria_estudiantes ADD COLUMN IF NOT EXISTS persona_id UUID NULL;
ALTER TABLE auditoria_estudiantes ALTER COLUMN estudiante_id DROP NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_auditoria_estudiantes_persona') THEN
        ALTER TABLE auditoria_estudiantes ADD CONSTRAINT fk_auditoria_estudiantes_persona
            FOREIGN KEY (persona_id) REFERENCES personas (id) ON DELETE RESTRICT;
    END IF;
END
$$;

-- Toda fila identifica al estudiante o a la persona afectada.
ALTER TABLE auditoria_estudiantes DROP CONSTRAINT IF EXISTS chk_auditoria_estudiantes_sujeto;
ALTER TABLE auditoria_estudiantes ADD CONSTRAINT chk_auditoria_estudiantes_sujeto
    CHECK (estudiante_id IS NOT NULL OR persona_id IS NOT NULL);

CREATE INDEX IF NOT EXISTS idx_auditoria_estudiantes_persona_fecha
    ON auditoria_estudiantes (persona_id, fecha_hora) WHERE persona_id IS NOT NULL;

-- 3. Acciones: MATRICULA_UPDATE (alta y cambios de matrícula, sobre el
-- estudiante), PERSONA_VIEW y PERSONA_UPDATE (sobre la persona).
ALTER TABLE auditoria_estudiantes DROP CONSTRAINT IF EXISTS chk_auditoria_estudiantes_accion;
ALTER TABLE auditoria_estudiantes ADD CONSTRAINT chk_auditoria_estudiantes_accion
    CHECK (accion IN ('CREATE', 'UPDATE', 'CHANGE_ESTADO', 'PERSONAL_VIEW', 'PERSONAL_UPDATE',
                      'FAMILIA_VIEW', 'FAMILIA_UPDATE',
                      'MATRICULA_UPDATE', 'PERSONA_VIEW', 'PERSONA_UPDATE'));

ALTER TABLE auditoria_estudiantes DROP CONSTRAINT IF EXISTS chk_auditoria_estudiantes_campos;
ALTER TABLE auditoria_estudiantes ADD CONSTRAINT chk_auditoria_estudiantes_campos CHECK (campos <@ ARRAY[
    'codigo', 'codigo_siagie', 'tipo_documento', 'numero_documento', 'nombres',
    'apellido_paterno', 'apellido_materno', 'apellidos', 'fecha_nacimiento', 'sexo',
    'pais_nacimiento', 'ubigeo_nacimiento', 'lengua_materna', 'segunda_lengua',
    'foto_ref', 'estado',
    'domicilio_direccion', 'domicilio_departamento', 'domicilio_provincia',
    'domicilio_distrito', 'domicilio_ubigeo', 'correo', 'telefono',
    'familiares',
    'matricula', 'seccion', 'estado_matricula', 'es_apoderado'
]::TEXT[]);

-- 4. Privilegios mínimos: la aplicación registra periodos, secciones y
-- matrículas y actualiza su estado; los movimientos son solo INSERT
-- (append-only). Sin DELETE en ninguna tabla.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'estudiantes_user') THEN
        GRANT SELECT, INSERT, UPDATE ON periodos_academicos, secciones, matriculas TO estudiantes_user;
        GRANT SELECT, INSERT ON matricula_movimientos TO estudiantes_user;
    END IF;
END
$$;

COMMIT;

-- Down migration (solo desarrollo; exige que no existan filas de auditoría
-- de personas ni movimientos CONCLUSION):
-- REVOKE INSERT, UPDATE ON periodos_academicos, secciones, matriculas FROM estudiantes_user;
-- REVOKE INSERT ON matricula_movimientos FROM estudiantes_user;
-- (restaurar chk_auditoria_estudiantes_accion y _campos de 008)
-- ALTER TABLE auditoria_estudiantes DROP CONSTRAINT chk_auditoria_estudiantes_sujeto;
-- DROP INDEX idx_auditoria_estudiantes_persona_fecha;
-- ALTER TABLE auditoria_estudiantes DROP CONSTRAINT fk_auditoria_estudiantes_persona;
-- ALTER TABLE auditoria_estudiantes ALTER COLUMN estudiante_id SET NOT NULL;
-- ALTER TABLE auditoria_estudiantes DROP COLUMN persona_id;
-- (restaurar chk_movimientos_tipo de 007)
