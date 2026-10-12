BEGIN;

-- Estudiantes Service — FASE E-08: auditoría de los DATOS PERSONALES
-- (fecha de nacimiento, sexo, país y ubigeo de nacimiento, lenguas).
--
-- APLICAR con estudiantes_migrator actuando como dueño:
--   (echo "SET ROLE estudiantes_owner;"; cat 005_auditoria_datos_personales.sql) | \
--     psql -U estudiantes_migrator -d estudiantes_db -v ON_ERROR_STOP=1
-- No destructiva e idempotente. Nota de numeración: Matrícula pasa a 006.
--
-- Solo amplía las acciones permitidas; las existentes no cambian:
--   PERSONAL_VIEW   lectura de datos personales (GetEstudiantePersonal).
--                   version_resultante = versión OBSERVADA (la lectura no
--                   crea versión); campos = nombres de los campos leídos.
--   PERSONAL_UPDATE edición de datos personales (UpdateEstudiantePersonal).
--                   version_resultante = versión nueva; campos = nombres de
--                   los campos modificados.
-- Igual que en 004: nunca valores. La lista blanca de "campos" ya incluye
-- los seis nombres personales y sigue impidiendo que un valor llegue aquí.
ALTER TABLE auditoria_estudiantes DROP CONSTRAINT IF EXISTS chk_auditoria_estudiantes_accion;
ALTER TABLE auditoria_estudiantes ADD CONSTRAINT chk_auditoria_estudiantes_accion
    CHECK (accion IN ('CREATE', 'UPDATE', 'CHANGE_ESTADO', 'PERSONAL_VIEW', 'PERSONAL_UPDATE'));

COMMIT;

-- Down migration (solo desarrollo; falla si ya hay filas PERSONAL_*):
-- ALTER TABLE auditoria_estudiantes DROP CONSTRAINT chk_auditoria_estudiantes_accion;
-- ALTER TABLE auditoria_estudiantes ADD CONSTRAINT chk_auditoria_estudiantes_accion
--     CHECK (accion IN ('CREATE', 'UPDATE', 'CHANGE_ESTADO'));
