BEGIN;

-- El login pasa a ser DNI + clave. Hasta ahora el DNI solo existía en Usuarios
-- Service (perfil); Auth, dueño de las credenciales, no lo guardaba.
--
-- La columna es NULLABLE a propósito: los usuarios existentes no tienen DNI en
-- Auth y NO se les inventa ni se les deduce uno. Mientras no tengan DNI no
-- pueden iniciar sesión (el login busca por DNI); asignarlo es una operación
-- explícita. Los usuarios nuevos (Register / CreateUser) lo exigen a nivel de
-- aplicación.
--
-- Formato: DNI peruano, exactamente 8 dígitos (texto, nunca numérico).
-- Unicidad: índice único parcial (solo filas con DNI). La migración es
-- idempotente y no toca contraseñas, roles ni permisos.
ALTER TABLE usuarios ADD COLUMN IF NOT EXISTS dni VARCHAR(8);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_usuarios_dni'
    ) THEN
        ALTER TABLE usuarios
            ADD CONSTRAINT chk_usuarios_dni CHECK (dni IS NULL OR dni ~ '^[0-9]{8}$');
    END IF;
END
$$;

-- Si ya existieran DNI duplicados (no pueden existir en una columna recién
-- creada, pero la migración se puede reaplicar tras cargas manuales), el
-- CREATE UNIQUE INDEX falla y toda la migración se revierte: no se elimina ni
-- se elige ningún usuario.
CREATE UNIQUE INDEX IF NOT EXISTS uq_usuarios_dni ON usuarios (dni) WHERE dni IS NOT NULL;

COMMIT;

-- Down migration:
-- DROP INDEX IF EXISTS uq_usuarios_dni;
-- ALTER TABLE usuarios DROP CONSTRAINT IF EXISTS chk_usuarios_dni;
-- ALTER TABLE usuarios DROP COLUMN IF EXISTS dni;
