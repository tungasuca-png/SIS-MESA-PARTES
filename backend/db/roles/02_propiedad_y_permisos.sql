-- FASE 6N-3 (C2) — Propiedad de las bases y privilegios mínimos.
--
-- Ejecutar con psql como tg_admin, conectado a "postgres", DESPUÉS de
-- 01_roles.sql:
--   psql -U tg_admin -d postgres -v ON_ERROR_STOP=1 -f 02_propiedad_y_permisos.sql
-- Idempotente. Sin contraseñas.
--
-- Nota: las bases auth_db, psicologia_db y estudiantes_db pertenecían al
-- superusuario auth_user; las de Mesa de Partes, a su propio rol de aplicación.
-- Primero se cambia el dueño de TODAS las bases (objetos compartidos) y luego,
-- dentro de cada base, REASSIGN OWNED mueve tablas, secuencias, funciones y
-- extensiones al <s>_owner correspondiente.

\set ON_ERROR_STOP on

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
-- auth_db — Auth Service (auth_app). Privilegios derivados del SQL real de
-- ejecución de Auth (sin pruebas): nada de DDL, TRUNCATE ni acceso a otras bases.
-- ===========================================================================
\connect auth_db
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'auth_user') THEN
        EXECUTE 'REASSIGN OWNED BY auth_user TO auth_owner';
    END IF;
END $$;
GRANT CONNECT ON DATABASE auth_db TO auth_app, auth_tester, auth_migrator;
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM auth_app;
GRANT SELECT, INSERT, UPDATE         ON usuarios         TO auth_app;
GRANT SELECT, INSERT, DELETE         ON usuario_roles    TO auth_app;
GRANT SELECT, INSERT, DELETE         ON role_permissions TO auth_app;
GRANT SELECT, INSERT, UPDATE         ON refresh_tokens   TO auth_app;
GRANT SELECT, INSERT, UPDATE         ON permissions      TO auth_app;
GRANT SELECT                         ON roles            TO auth_app;
-- user_profiles: sin privilegios (Auth no la usa en ejecución).
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO auth_tester;
ALTER DEFAULT PRIVILEGES FOR ROLE auth_owner IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO auth_tester;

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
GRANT CONNECT ON DATABASE psicologia_db TO psicologia_user, psicologia_tester, psicologia_migrator;
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
GRANT CONNECT ON DATABASE estudiantes_db TO estudiantes_user, estudiantes_tester, estudiantes_migrator;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO estudiantes_tester;
ALTER DEFAULT PRIVILEGES FOR ROLE estudiantes_owner IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO estudiantes_tester;

-- ===========================================================================
-- Mesa de Partes (expedientes, documentos, derivaciones, usuarios): el rol de
-- aplicación era DUEÑO de su base (podía crear, alterar y borrar tablas). Pasa
-- a tener solo DML sobre sus tablas y secuencias — el mismo acceso a datos que
-- usa hoy, sin DDL. Sin cambios funcionales.
-- ===========================================================================
\connect expedientes_db
REASSIGN OWNED BY expedientes_user TO expedientes_owner;
GRANT CONNECT ON DATABASE expedientes_db TO expedientes_user, expedientes_tester, expedientes_migrator;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO expedientes_user, expedientes_tester;
GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public TO expedientes_user, expedientes_tester;
ALTER DEFAULT PRIVILEGES FOR ROLE expedientes_owner IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO expedientes_user, expedientes_tester;
ALTER DEFAULT PRIVILEGES FOR ROLE expedientes_owner IN SCHEMA public
    GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO expedientes_user, expedientes_tester;

\connect documentos_db
REASSIGN OWNED BY documentos_user TO documentos_owner;
GRANT CONNECT ON DATABASE documentos_db TO documentos_user, documentos_tester, documentos_migrator;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO documentos_user, documentos_tester;
GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public TO documentos_user, documentos_tester;
ALTER DEFAULT PRIVILEGES FOR ROLE documentos_owner IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO documentos_user, documentos_tester;
ALTER DEFAULT PRIVILEGES FOR ROLE documentos_owner IN SCHEMA public
    GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO documentos_user, documentos_tester;

\connect derivaciones_db
REASSIGN OWNED BY derivaciones_user TO derivaciones_owner;
GRANT CONNECT ON DATABASE derivaciones_db TO derivaciones_user, derivaciones_tester, derivaciones_migrator;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO derivaciones_user, derivaciones_tester;
GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public TO derivaciones_user, derivaciones_tester;
ALTER DEFAULT PRIVILEGES FOR ROLE derivaciones_owner IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO derivaciones_user, derivaciones_tester;
ALTER DEFAULT PRIVILEGES FOR ROLE derivaciones_owner IN SCHEMA public
    GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO derivaciones_user, derivaciones_tester;

\connect usuarios_db
REASSIGN OWNED BY usuarios_user TO usuarios_owner;
GRANT CONNECT ON DATABASE usuarios_db TO usuarios_user, usuarios_tester, usuarios_migrator;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO usuarios_user, usuarios_tester;
GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public TO usuarios_user, usuarios_tester;
ALTER DEFAULT PRIVILEGES FOR ROLE usuarios_owner IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO usuarios_user, usuarios_tester;
ALTER DEFAULT PRIVILEGES FOR ROLE usuarios_owner IN SCHEMA public
    GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO usuarios_user, usuarios_tester;
