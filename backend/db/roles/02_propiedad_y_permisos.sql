-- FASE 6N-3 (C2) — Bases, propiedad y privilegios mínimos.
--
-- Ejecutar con psql como tg_admin, conectado a "postgres", DESPUÉS de
-- 01_roles.sql:
--   psql -U tg_admin -d postgres -v ON_ERROR_STOP=1 -f 02_propiedad_y_permisos.sql
-- Idempotente. Sin contraseñas. Ejecutable en cualquier momento:
--
--   Instalación NUEVA:  01_roles.sql -> 02 -> migraciones de cada servicio -> 02
--   ACTUALIZACIÓN:      01_roles.sql -> migraciones nuevas -> 02
--
-- La primera pasada crea las bases que falten (ya con su dueño) y concede
-- CONNECT a cada rol de servicio por su nombre (nunca a PUBLIC), de modo que
-- <s>_migrator pueda aplicar las migraciones. La segunda pasada concede los
-- privilegios sobre las tablas que crearon esas migraciones: todo GRANT sobre
-- una tabla concreta es condicional (to_regclass).
--
-- Nota: las bases auth_db, psicologia_db y estudiantes_db pertenecían al
-- superusuario auth_user; las de Mesa de Partes, a su propio rol de aplicación.
-- Se cambia el dueño de TODAS las bases (objetos compartidos) y luego, dentro
-- de cada base, REASSIGN OWNED mueve tablas, secuencias, funciones y
-- extensiones al <s>_owner correspondiente.

\set ON_ERROR_STOP on

-- ===========================================================================
-- 1. Bases: se crean las que falten, ya con su dueño (CREATE DATABASE no admite
--    IF NOT EXISTS ni ejecutarse dentro de una función: \gexec).
-- ===========================================================================
SELECT format('CREATE DATABASE %I OWNER %I', s || '_db', s || '_owner')
  FROM unnest(ARRAY['auth', 'psicologia', 'estudiantes', 'expedientes', 'documentos', 'derivaciones', 'usuarios']) AS s
 WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = s || '_db')
\gexec

ALTER DATABASE auth_db         OWNER TO auth_owner;
ALTER DATABASE psicologia_db   OWNER TO psicologia_owner;
ALTER DATABASE estudiantes_db  OWNER TO estudiantes_owner;
ALTER DATABASE expedientes_db  OWNER TO expedientes_owner;
ALTER DATABASE documentos_db   OWNER TO documentos_owner;
ALTER DATABASE derivaciones_db OWNER TO derivaciones_owner;
ALTER DATABASE usuarios_db     OWNER TO usuarios_owner;

-- Ninguna base admite conexiones de PUBLIC: solo los roles de su servicio.
REVOKE ALL ON DATABASE postgres, auth_db, psicologia_db, estudiantes_db, expedientes_db,
    documentos_db, derivaciones_db, usuarios_db FROM PUBLIC;

-- ===========================================================================
-- 2. CONNECT, rol por rol, ANTES de cualquier migración (corrige H-E1-01: el
--    REVOKE de arriba dejaba a los migradores sin acceso en una base nueva).
-- ===========================================================================
GRANT CONNECT ON DATABASE auth_db         TO auth_app, auth_tester, auth_migrator;
GRANT CONNECT ON DATABASE psicologia_db   TO psicologia_user, psicologia_tester, psicologia_migrator;
GRANT CONNECT ON DATABASE estudiantes_db  TO estudiantes_user, estudiantes_tester, estudiantes_migrator;
GRANT CONNECT ON DATABASE expedientes_db  TO expedientes_user, expedientes_tester, expedientes_migrator,
                                             expedientes_publicador, reconciliador_movimientos;
GRANT CONNECT ON DATABASE documentos_db   TO documentos_user, documentos_tester, documentos_migrator;
GRANT CONNECT ON DATABASE derivaciones_db TO derivaciones_user, derivaciones_tester, derivaciones_migrator,
                                             derivaciones_proyector, reconciliador_movimientos;
GRANT CONNECT ON DATABASE usuarios_db     TO usuarios_user, usuarios_tester, usuarios_migrator;

-- ===========================================================================
-- auth_db — Auth Service (auth_app). Privilegios derivados del SQL real de
-- ejecución de Auth (sin pruebas): nada de DDL, TRUNCATE ni acceso a otras bases.
-- ===========================================================================
\connect auth_db
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'auth_user') THEN
        EXECUTE 'REASSIGN OWNED BY auth_user TO auth_owner';
    END IF;
END $$;
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM auth_app;
-- Tablas de las migraciones 000-018 (condicional: en una base nueva la primera
-- pasada de este script corre antes de que existan).
DO $$ BEGIN
    IF to_regclass('public.usuarios') IS NOT NULL THEN
        GRANT SELECT, INSERT, UPDATE ON public.usuarios TO auth_app;
    END IF;
    IF to_regclass('public.usuario_roles') IS NOT NULL THEN
        GRANT SELECT, INSERT, DELETE ON public.usuario_roles TO auth_app;
    END IF;
    IF to_regclass('public.role_permissions') IS NOT NULL THEN
        GRANT SELECT, INSERT, DELETE ON public.role_permissions TO auth_app;
    END IF;
    IF to_regclass('public.refresh_tokens') IS NOT NULL THEN
        GRANT SELECT, INSERT, UPDATE ON public.refresh_tokens TO auth_app;
    END IF;
    IF to_regclass('public.permissions') IS NOT NULL THEN
        GRANT SELECT, INSERT, UPDATE ON public.permissions TO auth_app;
    END IF;
    IF to_regclass('public.roles') IS NOT NULL THEN
        GRANT SELECT ON public.roles TO auth_app;
    END IF;
END $$;
-- Tablas creadas por migraciones posteriores a la FASE 6N-3. El REVOKE de
-- arriba también las alcanza, así que se vuelven a conceder aquí (mismo
-- mínimo que cada migración) para que reejecutar este script no rompa Auth.
--   019 + 020: service_clients / service_client_permissions (DELETE solo en
--              la relación de permisos de un sistema).
--   022:       user_auth_providers (nunca DELETE).
--   023:       oauth_login_codes (nunca DELETE); auth_eventos solo INSERT
--              (auditoría de solo escritura: auth_app no la lee).
--              cancelar_registro_google_pendiente(uuid): única vía para
--              compensar un autorregistro con Google (auth_app no tiene
--              DELETE sobre usuarios); EXECUTE solo para auth_app.
--   027:       service_clients.audiencia queda cubierta por el GRANT de tabla;
--              su inmutabilidad la garantiza un trigger de la migración.
DO $$ BEGIN
    IF to_regclass('public.service_clients') IS NOT NULL THEN
        GRANT SELECT, INSERT, UPDATE ON public.service_clients TO auth_app;
    END IF;
    IF to_regclass('public.service_client_permissions') IS NOT NULL THEN
        GRANT SELECT, INSERT, DELETE ON public.service_client_permissions TO auth_app;
    END IF;
    IF to_regclass('public.user_auth_providers') IS NOT NULL THEN
        GRANT SELECT, INSERT, UPDATE ON public.user_auth_providers TO auth_app;
    END IF;
    IF to_regclass('public.oauth_login_codes') IS NOT NULL THEN
        GRANT SELECT, INSERT, UPDATE ON public.oauth_login_codes TO auth_app;
    END IF;
    IF to_regclass('public.auth_eventos') IS NOT NULL THEN
        GRANT INSERT ON public.auth_eventos TO auth_app;
    END IF;
    IF to_regprocedure('public.cancelar_registro_google_pendiente(uuid)') IS NOT NULL THEN
        ALTER FUNCTION public.cancelar_registro_google_pendiente(uuid) OWNER TO auth_owner;
        REVOKE ALL ON FUNCTION public.cancelar_registro_google_pendiente(uuid) FROM PUBLIC;
        REVOKE ALL ON FUNCTION public.cancelar_registro_google_pendiente(uuid) FROM auth_tester;
        GRANT EXECUTE ON FUNCTION public.cancelar_registro_google_pendiente(uuid) TO auth_app;
    END IF;
    -- 024: registro manual con verificación del correo (nunca DELETE) y su
    -- compensación, con el mismo criterio que la de Google.
    IF to_regclass('public.registros_correo') IS NOT NULL THEN
        GRANT SELECT, INSERT, UPDATE ON public.registros_correo TO auth_app;
    END IF;
    -- 025: identidad declarada de los solicitantes públicos (nunca DELETE).
    IF to_regclass('public.identidad_solicitantes') IS NOT NULL THEN
        GRANT SELECT, INSERT, UPDATE ON public.identidad_solicitantes TO auth_app;
    END IF;
    IF to_regprocedure('public.cancelar_registro_correo_pendiente(uuid)') IS NOT NULL THEN
        ALTER FUNCTION public.cancelar_registro_correo_pendiente(uuid) OWNER TO auth_owner;
        REVOKE ALL ON FUNCTION public.cancelar_registro_correo_pendiente(uuid) FROM PUBLIC;
        REVOKE ALL ON FUNCTION public.cancelar_registro_correo_pendiente(uuid) FROM auth_tester;
        GRANT EXECUTE ON FUNCTION public.cancelar_registro_correo_pendiente(uuid) TO auth_app;
    END IF;
END $$;
-- user_profiles: sin privilegios (Auth no la usa en ejecución).
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO auth_tester;
ALTER DEFAULT PRIVILEGES FOR ROLE auth_owner IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO auth_tester;
-- 026: auth_tester no borra registros del registro público ni identidades
-- declaradas (las pruebas solo siembran y leen). El GRANT de arriba se lo
-- devolvería al reejecutar este script; se retira de nuevo aquí.
DO $$ BEGIN
    IF to_regclass('public.registros_correo') IS NOT NULL THEN
        REVOKE DELETE ON public.registros_correo FROM auth_tester;
    END IF;
    IF to_regclass('public.identidad_solicitantes') IS NOT NULL THEN
        REVOKE DELETE ON public.identidad_solicitantes FROM auth_tester;
    END IF;
END $$;

-- ===========================================================================
-- psicologia_db — los privilegios de psicologia_user ya son mínimos y los
-- definen sus migraciones (sin DELETE); solo cambia el dueño.
-- ===========================================================================
\connect psicologia_db
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'auth_user') THEN
        EXECUTE 'REASSIGN OWNED BY auth_user TO psicologia_owner';
    END IF;
END $$;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO psicologia_tester;
ALTER DEFAULT PRIVILEGES FOR ROLE psicologia_owner IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO psicologia_tester;

-- ===========================================================================
-- estudiantes_db — idem (los privilegios de estudiantes_user los define su
-- migración).
-- ===========================================================================
\connect estudiantes_db
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'auth_user') THEN
        EXECUTE 'REASSIGN OWNED BY auth_user TO estudiantes_owner';
    END IF;
END $$;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO estudiantes_tester;
ALTER DEFAULT PRIVILEGES FOR ROLE estudiantes_owner IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO estudiantes_tester;

-- ===========================================================================
-- Mesa de Partes (expedientes, documentos, derivaciones, usuarios): el rol de
-- aplicación era DUEÑO de su base (podía crear, alterar y borrar tablas). Pasa
-- a tener solo DML sobre sus tablas y secuencias, sin DDL.
-- ===========================================================================
\connect expedientes_db
REASSIGN OWNED BY expedientes_user TO expedientes_owner;
-- Expedientes solo hace bajas LÓGICAS (activo = FALSE): su rol de aplicación no
-- tiene DELETE sobre ninguna tabla, tampoco las que creen migraciones futuras.
-- El rol de pruebas (solo desarrollo) conserva DELETE para limpiar sus datos.
REVOKE DELETE ON ALL TABLES IN SCHEMA public FROM expedientes_user;
GRANT SELECT, INSERT, UPDATE ON ALL TABLES IN SCHEMA public TO expedientes_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO expedientes_tester;
GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public TO expedientes_user, expedientes_tester;
ALTER DEFAULT PRIVILEGES FOR ROLE expedientes_owner IN SCHEMA public
    REVOKE DELETE ON TABLES FROM expedientes_user;
ALTER DEFAULT PRIVILEGES FOR ROLE expedientes_owner IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE ON TABLES TO expedientes_user;
ALTER DEFAULT PRIVILEGES FOR ROLE expedientes_owner IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO expedientes_tester;
ALTER DEFAULT PRIVILEGES FOR ROLE expedientes_owner IN SCHEMA public
    GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO expedientes_user, expedientes_tester;
-- A + B y reconciliación (migraciones 004 y 006): los GRANT generales de
-- arriba les devolverían privilegios al reejecutar este script; aquí se
-- restablece el mínimo de cada rol. Los triggers de las migraciones son la
-- garantía final aunque este bloque no se ejecutara.
DO $$ BEGIN
    IF to_regclass('public.expediente_movimientos') IS NOT NULL THEN
        REVOKE ALL ON public.expediente_movimientos FROM expedientes_user, expedientes_tester, expedientes_publicador;
        GRANT SELECT, INSERT ON public.expediente_movimientos TO expedientes_user, expedientes_tester;
        GRANT SELECT ON public.expediente_movimientos TO expedientes_publicador, reconciliador_movimientos;
    END IF;
    IF to_regclass('public.expediente_outbox') IS NOT NULL THEN
        REVOKE ALL ON public.expediente_outbox FROM expedientes_user, expedientes_tester, expedientes_publicador;
        GRANT SELECT, INSERT ON public.expediente_outbox TO expedientes_user, expedientes_tester;
        GRANT SELECT, DELETE ON public.expediente_outbox TO expedientes_publicador;
        GRANT UPDATE (estado, intentos, proximo_intento_en, reclamo_id, reclamado_hasta, ultimo_error, publicado_en)
            ON public.expediente_outbox TO expedientes_publicador;
        GRANT SELECT ON public.expediente_outbox TO reconciliador_movimientos;
    END IF;
    IF to_regclass('public.expedientes') IS NOT NULL THEN
        GRANT SELECT (id, area_actual, estado, activo) ON public.expedientes TO reconciliador_movimientos;
        IF EXISTS (SELECT 1 FROM information_schema.columns
                    WHERE table_schema = 'public' AND table_name = 'expedientes' AND column_name = 'ultima_secuencia') THEN
            GRANT SELECT (ultima_secuencia) ON public.expedientes TO reconciliador_movimientos;
        END IF;
    END IF;
    IF to_regclass('public.reconciliacion_ejecuciones') IS NOT NULL THEN
        REVOKE ALL ON public.reconciliacion_ejecuciones, public.reconciliacion_discrepancias, public.reconciliacion_acciones
            FROM expedientes_user, expedientes_tester, expedientes_publicador, reconciliador_movimientos;
        GRANT SELECT ON public.reconciliacion_ejecuciones, public.reconciliacion_discrepancias, public.reconciliacion_acciones
            TO expedientes_tester, reconciliador_movimientos;
        GRANT INSERT ON public.reconciliacion_ejecuciones, public.reconciliacion_discrepancias TO reconciliador_movimientos;
        GRANT UPDATE (estado, expedientes_revisados, movimientos_origen, movimientos_proyectados, discrepancias, huella_informe)
            ON public.reconciliacion_ejecuciones TO reconciliador_movimientos;
    END IF;
    IF to_regclass('public.reconciliacion_vigilancia') IS NOT NULL THEN
        REVOKE ALL ON public.reconciliacion_vigilancia
            FROM expedientes_user, expedientes_tester, expedientes_publicador, reconciliador_movimientos;
        GRANT SELECT ON public.reconciliacion_vigilancia TO expedientes_tester, reconciliador_movimientos;
    END IF;
END $$;
ALTER ROLE expedientes_user          IN DATABASE expedientes_db SET idle_in_transaction_session_timeout = '5s';
ALTER ROLE expedientes_publicador    IN DATABASE expedientes_db SET idle_in_transaction_session_timeout = '5s';
ALTER ROLE reconciliador_movimientos IN DATABASE expedientes_db SET idle_in_transaction_session_timeout = '60s';

\connect documentos_db
REASSIGN OWNED BY documentos_user TO documentos_owner;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO documentos_user, documentos_tester;
GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public TO documentos_user, documentos_tester;
ALTER DEFAULT PRIVILEGES FOR ROLE documentos_owner IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO documentos_user, documentos_tester;
ALTER DEFAULT PRIVILEGES FOR ROLE documentos_owner IN SCHEMA public
    GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO documentos_user, documentos_tester;

\connect derivaciones_db
REASSIGN OWNED BY derivaciones_user TO derivaciones_owner;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO derivaciones_user, derivaciones_tester;
GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public TO derivaciones_user, derivaciones_tester;
ALTER DEFAULT PRIVILEGES FOR ROLE derivaciones_owner IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO derivaciones_user, derivaciones_tester;
ALTER DEFAULT PRIVILEGES FOR ROLE derivaciones_owner IN SCHEMA public
    GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO derivaciones_user, derivaciones_tester;
-- A + B (migración 002): historial append-only y proyección.
DO $$ BEGIN
    IF to_regclass('public.derivaciones') IS NOT NULL THEN
        REVOKE UPDATE, DELETE ON public.derivaciones FROM derivaciones_user, derivaciones_tester;
    END IF;
    IF to_regclass('public.movimientos_proyeccion') IS NOT NULL THEN
        REVOKE ALL ON public.movimientos_proyeccion FROM derivaciones_user, derivaciones_tester, derivaciones_proyector;
        GRANT SELECT ON public.movimientos_proyeccion TO derivaciones_user, derivaciones_tester, reconciliador_movimientos;
        GRANT SELECT, INSERT ON public.movimientos_proyeccion TO derivaciones_proyector;
    END IF;
    IF to_regclass('public.historial_expediente') IS NOT NULL THEN
        REVOKE ALL ON public.historial_expediente FROM derivaciones_user, derivaciones_tester, derivaciones_proyector;
        GRANT SELECT ON public.historial_expediente TO derivaciones_user, derivaciones_tester;
    END IF;
END $$;

\connect usuarios_db
REASSIGN OWNED BY usuarios_user TO usuarios_owner;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO usuarios_user, usuarios_tester;
GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public TO usuarios_user, usuarios_tester;
ALTER DEFAULT PRIVILEGES FOR ROLE usuarios_owner IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO usuarios_user, usuarios_tester;
ALTER DEFAULT PRIVILEGES FOR ROLE usuarios_owner IN SCHEMA public
    GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO usuarios_user, usuarios_tester;
