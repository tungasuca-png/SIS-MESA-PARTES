BEGIN;

-- Regla de producto: 1 usuario = 1 rol. Hasta ahora solo la lógica de la
-- aplicación la cumplía: la PK (usuario_id, rol_id) permitía varias filas por
-- usuario. Esta migración hace que PostgreSQL también la garantice.
--
-- usuario_roles no tiene historial ni borrado lógico (columnas: usuario_id,
-- rol_id, created_at), así que corresponde un UNIQUE simple sobre usuario_id.
--
-- Seguridad: si ya existiera algún usuario con más de un rol, la migración se
-- ABORTA con el detalle — no borra ni elige filas automáticamente. Hay que
-- resolver esos datos a mano y volver a aplicarla.
DO $$
DECLARE
    conflictos TEXT;
BEGIN
    SELECT string_agg(usuario_id::text || ' (' || cantidad || ' roles)', ', ')
      INTO conflictos
      FROM (
          SELECT usuario_id, COUNT(*) AS cantidad
            FROM usuario_roles
           GROUP BY usuario_id
          HAVING COUNT(*) > 1
      ) t;

    IF conflictos IS NOT NULL THEN
        RAISE EXCEPTION 'No se puede imponer 1 usuario = 1 rol: usuarios con mas de un rol: %', conflictos;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'uq_usuario_roles_usuario'
    ) THEN
        ALTER TABLE usuario_roles
            ADD CONSTRAINT uq_usuario_roles_usuario UNIQUE (usuario_id);
    END IF;
END
$$;

COMMIT;

-- Down migration:
-- ALTER TABLE usuario_roles DROP CONSTRAINT IF EXISTS uq_usuario_roles_usuario;
