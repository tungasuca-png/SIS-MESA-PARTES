-- FASE 6N-3 (C2) — Estructura de roles de PostgreSQL por servicio.
--
-- Ejecutar como el superusuario de ADMINISTRACIÓN (tg_admin) conectado a la
-- base "postgres". Idempotente. NO contiene contraseñas: se asignan aparte
-- (ver README.md de esta carpeta) y nunca se versionan.
--
-- Por cada servicio <s>:
--   <s>_owner    NOLOGIN. Dueño de la base y de sus objetos. Nadie inicia
--                sesión con él.
--   <s>_migrator LOGIN. Solo migraciones: miembro de <s>_owner SIN herencia
--                (debe hacer SET ROLE <s>_owner explícitamente), así que fuera
--                de una migración no tiene privilegios sobre los objetos.
--   <s>_tester   LOGIN. Solo pruebas de integración en DESARROLLO (DML completo
--                para preparar y limpiar datos). No debe existir en producción.
--   aplicación   LOGIN. Solo DML mínimo (auth_app, psicologia_user,
--                estudiantes_user, expedientes_user, ...).
-- Ningún rol de aplicación, migración o prueba es superusuario ni puede crear
-- bases, roles o saltarse RLS.

DO $$
DECLARE
    s text;
    app text;
BEGIN
    FOREACH s IN ARRAY ARRAY['auth', 'psicologia', 'estudiantes', 'expedientes', 'documentos', 'derivaciones', 'usuarios'] LOOP
        IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = s || '_owner') THEN
            EXECUTE format('CREATE ROLE %I NOLOGIN', s || '_owner');
        END IF;
        IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = s || '_migrator') THEN
            EXECUTE format('CREATE ROLE %I LOGIN', s || '_migrator');
        END IF;
        IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = s || '_tester') THEN
            EXECUTE format('CREATE ROLE %I LOGIN', s || '_tester');
        END IF;

        -- Rol de aplicación: auth usa auth_app (antes el superusuario auth_user);
        -- los demás conservan su nombre (<s>_user).
        app := CASE WHEN s = 'auth' THEN 'auth_app' ELSE s || '_user' END;
        IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = app) THEN
            EXECUTE format('CREATE ROLE %I LOGIN', app);
        END IF;

        EXECUTE format('ALTER ROLE %I NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS', s || '_owner');
        EXECUTE format('ALTER ROLE %I LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS', s || '_migrator');
        -- Sin LOGIN aquí: el rol de pruebas nace con LOGIN (desarrollo), pero si en
        -- producción se le retiró (ALTER ROLE ... NOLOGIN, M-02), reejecutar este
        -- script no se lo devuelve.
        EXECUTE format('ALTER ROLE %I NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS', s || '_tester');
        EXECUTE format('ALTER ROLE %I LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS', app);

        -- El migrador actúa como dueño solo tras SET ROLE (INHERIT FALSE).
        EXECUTE format('GRANT %I TO %I WITH INHERIT FALSE, SET TRUE', s || '_owner', s || '_migrator');
    END LOOP;
END
$$;

-- Limpieza de datos de PRUEBA en psicologia_db: la bitácora es append-only
-- por trigger; las pruebas la limpian con session_replication_role=replica,
-- que normalmente exige superusuario. Se concede solo ese parámetro al rol de
-- pruebas (solo desarrollo), en lugar de usar un superusuario.
GRANT SET ON PARAMETER session_replication_role TO psicologia_tester;
-- FASE E-05: mismo caso en estudiantes_db (auditoria_estudiantes es append-only).
GRANT SET ON PARAMETER session_replication_role TO estudiantes_tester;

-- ===========================================================================
-- Mesa de Partes A + B (transacción local + Transactional Outbox).
-- Roles de UNA sola función, separados del rol de aplicación:
--   expedientes_publicador LOGIN. Solo el publicador del Outbox de Expedientes:
--       reclama, reprograma y cierra eventos (siempre con fencing por
--       reclamo_id) y purga los ya publicados. Lee los movimientos; no puede
--       modificar expedientes ni movimientos, ni crear eventos.
--   derivaciones_proyector LOGIN. Solo el consumidor AplicarMovimiento de
--       Derivaciones: inserta en movimientos_proyeccion. La API de Derivaciones
--       (derivaciones_user) solo la lee.
--   reconciliador_movimientos LOGIN. Solo la reconciliación diaria (D8): lee
--       movimientos, outbox y proyección; registra ejecuciones y discrepancias;
--       republica únicamente mediante reconciliacion_republicar(). No modifica
--       movimientos, proyección ni historial.
-- Sin contraseñas aquí (se asignan aparte, ver README.md).
-- ===========================================================================
DO $$
DECLARE
    r text;
BEGIN
    FOREACH r IN ARRAY ARRAY['expedientes_publicador', 'derivaciones_proyector', 'reconciliador_movimientos'] LOOP
        IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = r) THEN
            EXECUTE format('CREATE ROLE %I LOGIN', r);
        END IF;
        EXECUTE format('ALTER ROLE %I LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS', r);
    END LOOP;
END
$$;
