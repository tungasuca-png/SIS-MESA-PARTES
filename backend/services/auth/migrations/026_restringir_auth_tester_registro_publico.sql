BEGIN;

-- APLICAR después de Auth 025 (orden: Usuarios 003 -> Auth 024 -> Auth 025 ->
-- Auth 026; ver docs/despliegue/registro-publico-identidad.md), con
-- auth_migrator actuando como dueño:
--   docker cp 026_restringir_auth_tester_registro_publico.sql tg-postgres:/tmp/026.sql
--   docker exec tg-postgres psql -U auth_migrator -d auth_db -v ON_ERROR_STOP=1 \
--     -c "SET ROLE auth_owner" -f /tmp/026.sql
--
-- auth_tester (solo pruebas de integración en desarrollo; no debe existir en
-- producción) recibe SELECT, INSERT, UPDATE y DELETE sobre TODA tabla nueva de
-- auth_owner por los privilegios por defecto de
-- backend/db/roles/02_propiedad_y_permisos.sql. registros_correo (datos del
-- formulario y hash de la clave hasta completar el registro) e
-- identidad_solicitantes (DNI declarado y decisión de Secretaría) no deben
-- poder borrarse con esa cuenta: las pruebas solo siembran y leen
-- (SELECT, INSERT, UPDATE). Las acciones ON DELETE CASCADE / SET NULL de sus
-- claves foráneas se ejecutan con los privilegios del dueño de la tabla, así
-- que auth_tester puede seguir eliminando sus usuarios de prueba.
--
-- Solo retira DELETE de esas dos tablas: no cambia los privilegios de ninguna
-- otra tabla, ni los privilegios por defecto, ni los de auth_app. Idempotente
-- (REVOKE de un privilegio inexistente no hace nada).
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'auth_tester') THEN
        IF to_regclass('public.registros_correo') IS NOT NULL THEN
            REVOKE DELETE ON public.registros_correo FROM auth_tester;
        END IF;
        IF to_regclass('public.identidad_solicitantes') IS NOT NULL THEN
            REVOKE DELETE ON public.identidad_solicitantes FROM auth_tester;
        END IF;
    END IF;
END
$$;

COMMIT;

-- Down migration (restaura la política anterior):
-- GRANT DELETE ON public.registros_correo TO auth_tester;
-- GRANT DELETE ON public.identidad_solicitantes TO auth_tester;
