BEGIN;

-- APLICAR con auth_migrator actuando como dueño (mismo procedimiento que 019):
--   (echo "SET ROLE auth_owner;"; cat 020_seed_estudiantes_search_basic.sql) | \
--     psql -U auth_migrator -d auth_db -v ON_ERROR_STOP=1
-- GUARDE la salida: los avisos (NOTICE) "020: ..." indican qué existía antes y
-- qué creó esta migración; la reversión segura depende de ellos.
--
-- Idempotente: ejecutarla otra vez no duplica ni cambia nada (solo repite los
-- avisos, que entonces dirán "ya existía").

-- 1. Catálogo: estudiantes.search.basic, permiso para SISTEMAS consumidores
--    (búsqueda mínima de estudiantes para seleccionarlos: id, nombre, grado y
--    sección). Independiente de estudiantes.basic.view y de estudiantes.view.
--    Misma política que 012 y 014–019: se registra, NO se asigna a ningún rol
--    humano ni a ningún sistema. A un sistema se le otorga después con
--    `clientes-servicio otorgar` (RPC GrantServiceClientPermission).
--    La descripción exacta identifica la fila creada por esta migración (la
--    usa la reversión para no borrar un permiso preexistente).
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM permissions WHERE codigo = 'estudiantes.search.basic') THEN
        RAISE NOTICE '020: estudiantes.search.basic YA EXISTÍA; no se crea ni se modifica. La reversión NO debe eliminarlo.';
    ELSE
        RAISE NOTICE '020: se crea estudiantes.search.basic (sin asignarlo a roles ni sistemas).';
    END IF;
END
$$;

INSERT INTO permissions (codigo, nombre, descripcion, modulo, estado) VALUES
    ('estudiantes.search.basic', 'Buscar estudiantes (selección)', 'Buscar estudiantes por nombres, apellidos o DNI y obtener solo los datos mínimos para seleccionarlos; exclusivo de sistemas consumidores (migración 020)', 'estudiantes', TRUE)
ON CONFLICT (codigo) DO NOTHING;

-- 2. Retirar un permiso de un sistema (RPC WithdrawServiceClientPermission)
--    borra su fila en service_client_permissions: auth_app necesita DELETE
--    SOLO en esa tabla. role_permissions y el resto no cambian. Si auth_app ya
--    tenía DELETE, no se toca (y la reversión no debe revocarlo).
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'auth_app') THEN
        RAISE NOTICE '020: el rol auth_app no existe; no se concede DELETE.';
    ELSIF has_table_privilege('auth_app', 'service_client_permissions', 'DELETE') THEN
        RAISE NOTICE '020: auth_app YA TENÍA DELETE sobre service_client_permissions; no se modifica. La reversión NO debe revocarlo.';
    ELSE
        GRANT DELETE ON service_client_permissions TO auth_app;
        RAISE NOTICE '020: se concede DELETE sobre service_client_permissions a auth_app.';
    END IF;
END
$$;

COMMIT;

-- ---------------------------------------------------------------------------
-- Down migration (CONSERVADORA; como auth_owner, mismo procedimiento):
--
-- Nunca borra asignaciones (no las crea esta migración: las de sistemas se
-- hacen con el CLI y las de roles desde la pantalla de Roles) ni un permiso
-- preexistente. Si hay asignaciones, se detiene sin cambiar nada.
--
-- Paso previo: retirar el permiso de cada sistema que lo tenga
--   clientes-servicio retirar -cliente cli_... -permiso estudiantes.search.basic
-- (y de los roles, si alguien lo asignó, desde la pantalla de Roles).
--
-- BEGIN;
-- DO $$
-- DECLARE
--     pid UUID;
--     en_roles INTEGER;
--     en_sistemas INTEGER;
-- BEGIN
--     SELECT id INTO pid FROM permissions
--     WHERE codigo = 'estudiantes.search.basic'
--       AND descripcion = 'Buscar estudiantes por nombres, apellidos o DNI y obtener solo los datos mínimos para seleccionarlos; exclusivo de sistemas consumidores (migración 020)';
--     IF pid IS NULL THEN
--         RAISE NOTICE '020 down: el permiso no existe o no lo creó la 020; no se elimina nada.';
--         RETURN;
--     END IF;
--     SELECT count(*) INTO en_roles FROM role_permissions WHERE permission_id = pid;
--     SELECT count(*) INTO en_sistemas FROM service_client_permissions WHERE permission_id = pid;
--     IF en_roles > 0 OR en_sistemas > 0 THEN
--         RAISE EXCEPTION '020 down: el permiso sigue asignado (roles=%, sistemas=%); retírelo primero. No se eliminó nada.', en_roles, en_sistemas;
--     END IF;
--     DELETE FROM permissions WHERE id = pid;
--     RAISE NOTICE '020 down: estudiantes.search.basic eliminado del catálogo.';
-- END
-- $$;
-- -- SOLO si la salida de la aplicación dijo "se concede DELETE ... a auth_app"
-- -- (no si dijo "YA TENÍA DELETE"):
-- -- REVOKE DELETE ON service_client_permissions FROM auth_app;
-- COMMIT;
-- ---------------------------------------------------------------------------
