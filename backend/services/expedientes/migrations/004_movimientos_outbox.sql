-- 004 — Movimientos del expediente (append-only) y Transactional Outbox.
--
-- Ejecutar como expedientes_migrator actuando como dueño:
--   psql -U expedientes_migrator -d expedientes_db -v ON_ERROR_STOP=1 \
--        -c "SET ROLE expedientes_owner" -f 004_movimientos_outbox.sql
-- Requiere db/roles/01_roles.sql actualizado (rol expedientes_publicador).
-- Idempotente. No modifica ni borra datos existentes: agrega una columna con
-- valor por defecto 0 (solo metadatos) y objetos nuevos.
--
-- Garantías en la base (PostgreSQL NO verifica la autenticidad criptográfica
-- del actor: actor_id/actor_rol los aporta Expedientes Service tras validar
-- JWT, permiso efectivo y área; la base solo garantiza coherencia estructural):
--   * un movimiento no se modifica ni se borra (triggers append-only);
--   * cada movimiento confirmado tiene su evento y coincide con el estado
--     final del expediente (constraint trigger diferido);
--   * la hora del movimiento la pone la base (no se puede antedatar);
--   * las derivaciones F2/F4 respetan la matriz institucional (paridad con
--     estados.CanDerivar, verificada por prueba);
--   * el rol de aplicación solo crea eventos PENDIENTE; solo
--     expedientes_publicador los reclama y cierra, con fencing por reclamo_id
--     y sin poder robar un reclamo vigente.

BEGIN;

ALTER TABLE expedientes ADD COLUMN IF NOT EXISTS ultima_secuencia INTEGER NOT NULL DEFAULT 0;
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                    WHERE conrelid = 'public.expedientes'::regclass
                      AND conname  = 'chk_expedientes_ultima_secuencia') THEN
        ALTER TABLE expedientes
            ADD CONSTRAINT chk_expedientes_ultima_secuencia CHECK (ultima_secuencia >= 0);
    END IF;
END
$$;

CREATE TABLE IF NOT EXISTS expediente_movimientos (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    expediente_id    UUID NOT NULL,
    secuencia        INTEGER NOT NULL,
    tipo             VARCHAR(20) NOT NULL,
    actor_id         UUID NOT NULL,
    actor_rol        VARCHAR(30) NOT NULL,
    area_origen      VARCHAR(20) NOT NULL,
    area_destino     VARCHAR(20) NOT NULL,
    estado_anterior  VARCHAR(20) NOT NULL,
    estado_nuevo     VARCHAR(20) NOT NULL,
    motivo           VARCHAR(500) NOT NULL DEFAULT '',
    condicion        VARCHAR(255) NOT NULL DEFAULT '',
    idempotency_key  UUID NOT NULL,
    huella_solicitud CHAR(64) NOT NULL,
    ocurrido_en      TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT fk_mov_expediente   FOREIGN KEY (expediente_id) REFERENCES expedientes (id) ON DELETE RESTRICT,
    CONSTRAINT uq_mov_secuencia    UNIQUE (expediente_id, secuencia),
    CONSTRAINT uq_mov_idempotencia UNIQUE (actor_id, idempotency_key),
    CONSTRAINT uq_mov_id_exp_sec   UNIQUE (id, expediente_id, secuencia),
    CONSTRAINT chk_mov_secuencia   CHECK (secuencia > 0),
    CONSTRAINT chk_mov_tipo        CHECK (tipo IN ('DERIVACION', 'RECHAZO', 'CAMBIO_ESTADO', 'RESOLUCION', 'CORRECCION')),
    CONSTRAINT chk_mov_actor_rol   CHECK (actor_rol IN ('ADMIN', 'DIRECTOR', 'SUBDIRECTOR', 'SECRETARIA', 'DOCENTE', 'AUXILIAR', 'SOLICITANTE')),
    CONSTRAINT chk_mov_areas       CHECK (
            area_origen  IN ('SECRETARIA', 'DIRECTOR', 'SUBDIRECTOR', 'DOCENTE', 'AUXILIAR', 'ADMIN')
        AND area_destino IN ('SECRETARIA', 'DIRECTOR', 'SUBDIRECTOR', 'DOCENTE', 'AUXILIAR', 'ADMIN')),
    CONSTRAINT chk_mov_estados     CHECK (
            estado_anterior IN ('PENDIENTE', 'EN_PROCESO', 'ATENDIDO', 'OBSERVADO')
        AND estado_nuevo    IN ('PENDIENTE', 'EN_PROCESO', 'ATENDIDO', 'OBSERVADO')),
    CONSTRAINT chk_mov_regla_tipo  CHECK (
           (tipo = 'DERIVACION' AND area_origen <> area_destino
              AND (estado_anterior = estado_nuevo
                   OR (estado_anterior, estado_nuevo) IN (('PENDIENTE', 'EN_PROCESO'), ('EN_PROCESO', 'ATENDIDO'))))
        -- RECHAZO: siempre devuelve a SECRETARIA y pasa EN_PROCESO -> OBSERVADO. El
        -- origen puede ser la propia SECRETARIA (regla vigente: Secretaría puede
        -- rechazar un F4 que ya tiene en proceso); el cambio de estado basta.
        OR (tipo = 'RECHAZO' AND area_destino = 'SECRETARIA'
              AND estado_anterior = 'EN_PROCESO' AND estado_nuevo = 'OBSERVADO')
        OR (tipo = 'CAMBIO_ESTADO' AND area_origen = area_destino
              AND (estado_anterior, estado_nuevo) IN (('PENDIENTE', 'EN_PROCESO'), ('PENDIENTE', 'OBSERVADO'), ('EN_PROCESO', 'ATENDIDO')))
        OR (tipo = 'RESOLUCION' AND area_origen = 'SECRETARIA' AND area_destino = 'SECRETARIA'
              AND estado_anterior = 'EN_PROCESO' AND estado_nuevo = 'ATENDIDO')
        -- CORRECCION: la hace el dueño del expediente (solicitante_id), que puede
        -- ser personal interno que lo registró; la propiedad la verifica
        -- Expedientes Service sobre la fila bloqueada, no el rol.
        OR (tipo = 'CORRECCION' AND area_origen = area_destino
              AND estado_anterior = 'OBSERVADO' AND estado_nuevo = 'PENDIENTE')),
    CONSTRAINT chk_mov_motivo      CHECK (tipo NOT IN ('DERIVACION', 'RECHAZO') OR length(btrim(motivo)) > 0),
    CONSTRAINT chk_mov_huella      CHECK (huella_solicitud ~ '^[0-9a-f]{64}$')
);

CREATE TABLE IF NOT EXISTS expediente_outbox (
    id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    movimiento_id      UUID NOT NULL,
    expediente_id      UUID NOT NULL,
    secuencia          INTEGER NOT NULL,
    estado             VARCHAR(10) NOT NULL DEFAULT 'PENDIENTE',
    intentos           INTEGER NOT NULL DEFAULT 0,
    proximo_intento_en TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    reclamo_id         UUID,
    reclamado_hasta    TIMESTAMPTZ,
    ultimo_error       VARCHAR(500),
    creado_en          TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    publicado_en       TIMESTAMPTZ,
    CONSTRAINT uq_outbox_movimiento    UNIQUE (movimiento_id),
    CONSTRAINT fk_outbox_movimiento    FOREIGN KEY (movimiento_id, expediente_id, secuencia)
                                       REFERENCES expediente_movimientos (id, expediente_id, secuencia),
    CONSTRAINT chk_outbox_estado       CHECK (estado IN ('PENDIENTE', 'PUBLICADO', 'FALLIDO')),
    CONSTRAINT chk_outbox_intentos     CHECK (intentos >= 0),
    CONSTRAINT chk_outbox_publicado    CHECK ((estado = 'PUBLICADO') = (publicado_en IS NOT NULL)),
    CONSTRAINT chk_outbox_reclamo      CHECK ((reclamo_id IS NULL) = (reclamado_hasta IS NULL)),
    CONSTRAINT chk_outbox_reclamo_pend CHECK (estado = 'PENDIENTE' OR reclamo_id IS NULL)
);
CREATE INDEX IF NOT EXISTS idx_outbox_cabezas ON expediente_outbox (expediente_id, secuencia) WHERE estado <> 'PUBLICADO';
CREATE INDEX IF NOT EXISTS idx_outbox_purga   ON expediente_outbox (publicado_en)            WHERE estado = 'PUBLICADO';
ALTER TABLE expediente_outbox SET (autovacuum_vacuum_scale_factor = 0.02, autovacuum_analyze_scale_factor = 0.02);

-- Append-only genérico (UPDATE/DELETE por fila, TRUNCATE por sentencia).
CREATE OR REPLACE FUNCTION expedientes_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION '% es append-only: % no permitido', TG_TABLE_NAME, TG_OP USING ERRCODE = '42501';
END
$$;

DROP TRIGGER IF EXISTS trg_mov_no_mod ON expediente_movimientos;
CREATE TRIGGER trg_mov_no_mod BEFORE UPDATE OR DELETE ON expediente_movimientos
    FOR EACH ROW EXECUTE FUNCTION expedientes_append_only();
DROP TRIGGER IF EXISTS trg_mov_no_truncate ON expediente_movimientos;
CREATE TRIGGER trg_mov_no_truncate BEFORE TRUNCATE ON expediente_movimientos
    FOR EACH STATEMENT EXECUTE FUNCTION expedientes_append_only();
DROP TRIGGER IF EXISTS trg_outbox_no_truncate ON expediente_outbox;
CREATE TRIGGER trg_outbox_no_truncate BEFORE TRUNCATE ON expediente_outbox
    FOR EACH STATEMENT EXECUTE FUNCTION expedientes_append_only();

-- Antes de insertar un movimiento: hora puesta por la base y matriz F2/F4 (D1).
CREATE OR REPLACE FUNCTION expediente_movimientos_antes_insertar() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    v_tipo_tramite VARCHAR(30);
BEGIN
    NEW.ocurrido_en := clock_timestamp();
    IF NEW.tipo = 'DERIVACION' THEN
        SELECT e.tipo INTO v_tipo_tramite FROM expedientes e WHERE e.id = NEW.expediente_id;
        IF v_tipo_tramite IN ('CERTIFICADO', 'CONSTANCIA')
           AND (NEW.area_origen, NEW.area_destino) NOT IN (('SECRETARIA', 'DIRECTOR'), ('DIRECTOR', 'SECRETARIA')) THEN
            RAISE EXCEPTION 'derivación % -> % no permitida para %', NEW.area_origen, NEW.area_destino, v_tipo_tramite
                USING ERRCODE = '23514';
        ELSIF v_tipo_tramite IN ('PERMISO', 'JUSTIFICACION_FALTA', 'JUSTIFICACION_TARDANZA')
           AND (NEW.area_origen, NEW.area_destino) NOT IN (('SECRETARIA', 'DIRECTOR'), ('DIRECTOR', 'SUBDIRECTOR'), ('SUBDIRECTOR', 'DOCENTE')) THEN
            RAISE EXCEPTION 'derivación % -> % no permitida para %', NEW.area_origen, NEW.area_destino, v_tipo_tramite
                USING ERRCODE = '23514';
        END IF;
    END IF;
    RETURN NEW;
END
$$;
DROP TRIGGER IF EXISTS trg_mov_antes_insertar ON expediente_movimientos;
CREATE TRIGGER trg_mov_antes_insertar BEFORE INSERT ON expediente_movimientos
    FOR EACH ROW EXECUTE FUNCTION expediente_movimientos_antes_insertar();

-- Al confirmar: el movimiento tiene evento y coincide con el estado final del expediente.
CREATE OR REPLACE FUNCTION expediente_movimientos_verificar() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM expediente_outbox o WHERE o.movimiento_id = NEW.id) THEN
        RAISE EXCEPTION 'movimiento % sin evento de outbox', NEW.id USING ERRCODE = '23514';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM expedientes e
                    WHERE e.id = NEW.expediente_id AND e.ultima_secuencia = NEW.secuencia
                      AND e.area_actual = NEW.area_destino AND e.estado = NEW.estado_nuevo) THEN
        RAISE EXCEPTION 'movimiento % no coincide con el expediente', NEW.id USING ERRCODE = '23514';
    END IF;
    RETURN NULL;
END
$$;
DROP TRIGGER IF EXISTS trg_mov_verificar ON expediente_movimientos;
CREATE CONSTRAINT TRIGGER trg_mov_verificar AFTER INSERT ON expediente_movimientos
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION expediente_movimientos_verificar();

-- Outbox: separación de funciones y fencing en la propia base.
--   * cualquier rol con INSERT (expedientes_user) solo crea eventos PENDIENTE sin procesar;
--   * solo expedientes_publicador modifica o purga; expedientes_owner administra (runbook);
--   * un reclamo nuevo solo sobre un evento libre o con lease vencido, y suma un intento;
--   * cerrar o liberar exige un reclamo previo y no altera intentos;
--   * un evento PUBLICADO o FALLIDO no lo modifica el publicador.
-- El fencing por reclamo_id lo aplica además cada UPDATE del publicador
-- (WHERE id = $1 AND reclamo_id = $2 AND estado = 'PENDIENTE').
CREATE OR REPLACE FUNCTION expediente_outbox_proteger() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF current_user = 'expedientes_owner' THEN
        RETURN COALESCE(NEW, OLD);
    END IF;

    IF TG_OP = 'INSERT' THEN
        IF NEW.estado <> 'PENDIENTE' OR NEW.intentos <> 0 OR NEW.reclamo_id IS NOT NULL
           OR NEW.reclamado_hasta IS NOT NULL OR NEW.publicado_en IS NOT NULL OR NEW.ultimo_error IS NOT NULL THEN
            RAISE EXCEPTION 'un evento nuevo debe nacer PENDIENTE y sin procesar' USING ERRCODE = '42501';
        END IF;
        NEW.creado_en := clock_timestamp();
        NEW.proximo_intento_en := clock_timestamp();
        RETURN NEW;
    END IF;

    IF current_user <> 'expedientes_publicador' THEN
        RAISE EXCEPTION 'solo el publicador modifica o purga eventos' USING ERRCODE = '42501';
    END IF;

    IF TG_OP = 'DELETE' THEN
        IF OLD.estado <> 'PUBLICADO' THEN
            RAISE EXCEPTION 'solo se purgan eventos publicados' USING ERRCODE = '42501';
        END IF;
        RETURN OLD;
    END IF;

    IF OLD.estado <> 'PENDIENTE' THEN
        RAISE EXCEPTION 'el evento % está % y no es modificable', OLD.id, OLD.estado USING ERRCODE = '42501';
    END IF;

    IF NEW.reclamo_id IS NOT NULL THEN
        IF NEW.reclamo_id IS NOT DISTINCT FROM OLD.reclamo_id THEN
            RAISE EXCEPTION 'un reclamo no se renueva: se cierra o se libera' USING ERRCODE = '42501';
        END IF;
        IF OLD.reclamo_id IS NOT NULL AND OLD.reclamado_hasta >= clock_timestamp() THEN
            RAISE EXCEPTION 'el evento % tiene un reclamo vigente', OLD.id USING ERRCODE = 'TG002';
        END IF;
        IF NEW.estado <> 'PENDIENTE' OR NEW.intentos <> OLD.intentos + 1
           OR NEW.reclamado_hasta > clock_timestamp() + interval '10 minutes' THEN
            RAISE EXCEPTION 'reclamo inválido del evento %', OLD.id USING ERRCODE = '42501';
        END IF;
        RETURN NEW;
    END IF;

    IF OLD.reclamo_id IS NULL THEN
        RAISE EXCEPTION 'el evento % no está reclamado', OLD.id USING ERRCODE = '42501';
    END IF;
    IF NEW.intentos <> OLD.intentos THEN
        RAISE EXCEPTION 'cerrar o liberar no altera los intentos' USING ERRCODE = '42501';
    END IF;
    IF NEW.estado = 'PUBLICADO' THEN
        NEW.publicado_en := clock_timestamp();
    END IF;
    RETURN NEW;
END
$$;
DROP TRIGGER IF EXISTS trg_outbox_proteger ON expediente_outbox;
CREATE TRIGGER trg_outbox_proteger BEFORE INSERT OR UPDATE OR DELETE ON expediente_outbox
    FOR EACH ROW EXECUTE FUNCTION expediente_outbox_proteger();

-- Privilegios mínimos. DEFAULT PRIVILEGES (roles/02) ya concedió S/I/U/D a
-- expedientes_user y expedientes_tester: se revoca todo y se concede lo justo.
-- REVOKE de tabla también retira los privilegios de columna: va primero.
GRANT CONNECT ON DATABASE expedientes_db TO expedientes_publicador;
REVOKE ALL ON expediente_movimientos FROM expedientes_user, expedientes_tester, expedientes_publicador;
REVOKE ALL ON expediente_outbox      FROM expedientes_user, expedientes_tester, expedientes_publicador;
GRANT SELECT, INSERT ON expediente_movimientos TO expedientes_user, expedientes_tester;
GRANT SELECT         ON expediente_movimientos TO expedientes_publicador;
GRANT SELECT, INSERT ON expediente_outbox      TO expedientes_user, expedientes_tester;
GRANT SELECT, DELETE ON expediente_outbox      TO expedientes_publicador;
GRANT UPDATE (estado, intentos, proximo_intento_en, reclamo_id, reclamado_hasta, ultimo_error, publicado_en)
    ON expediente_outbox TO expedientes_publicador;

COMMIT;
