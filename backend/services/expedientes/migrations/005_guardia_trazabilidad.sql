-- 005 — Guardia de trazabilidad de expedientes.
--
-- Ejecutar como expedientes_migrator tras SET ROLE expedientes_owner, después
-- de 004 y en la MISMA liberación que los cinco flujos (Derivar, Rechazar,
-- ChangeEstado, Resolver, Corregir) migrados a RegistrarMovimiento: desde
-- aquí, ningún cambio de area_actual o estado se confirma sin su movimiento,
-- y el rol de aplicación ya no puede borrar físicamente. Idempotente. No
-- modifica datos.

BEGIN;

-- Todo expediente nace PENDIENTE, en SECRETARIA y sin movimientos (lo que ya
-- hace ExpedienteRepository.Create).
CREATE OR REPLACE FUNCTION expedientes_guardia_insert() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.ultima_secuencia <> 0 OR NEW.estado <> 'PENDIENTE' OR NEW.area_actual <> 'SECRETARIA' THEN
        RAISE EXCEPTION 'un expediente nuevo nace PENDIENTE en SECRETARIA y sin movimientos' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END
$$;
DROP TRIGGER IF EXISTS trg_expedientes_guardia_insert ON expedientes;
CREATE TRIGGER trg_expedientes_guardia_insert BEFORE INSERT ON expedientes
    FOR EACH ROW EXECUTE FUNCTION expedientes_guardia_insert();

-- Cambiar área o estado exige avanzar exactamente una secuencia; la secuencia
-- no se mueve sin un cambio de área o estado.
CREATE OR REPLACE FUNCTION expedientes_exigir_secuencia() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.area_actual IS DISTINCT FROM OLD.area_actual OR NEW.estado IS DISTINCT FROM OLD.estado THEN
        IF NEW.ultima_secuencia <> OLD.ultima_secuencia + 1 THEN
            RAISE EXCEPTION 'cambio de área/estado sin movimiento (expediente %)', OLD.id USING ERRCODE = '23514';
        END IF;
    ELSIF NEW.ultima_secuencia <> OLD.ultima_secuencia THEN
        RAISE EXCEPTION 'ultima_secuencia solo cambia con un movimiento (expediente %)', OLD.id USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END
$$;
DROP TRIGGER IF EXISTS trg_expedientes_exigir_secuencia ON expedientes;
CREATE TRIGGER trg_expedientes_exigir_secuencia
    BEFORE UPDATE OF area_actual, estado, ultima_secuencia ON expedientes
    FOR EACH ROW EXECUTE FUNCTION expedientes_exigir_secuencia();

-- Al confirmar: existe el movimiento de esa secuencia y describe exactamente
-- el cambio (área y estado anteriores y nuevos).
CREATE OR REPLACE FUNCTION expedientes_verificar_movimiento() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.ultima_secuencia <> OLD.ultima_secuencia AND NOT EXISTS (
           SELECT 1 FROM expediente_movimientos m
            WHERE m.expediente_id = NEW.id AND m.secuencia = NEW.ultima_secuencia
              AND m.area_origen = OLD.area_actual AND m.area_destino = NEW.area_actual
              AND m.estado_anterior = OLD.estado  AND m.estado_nuevo = NEW.estado) THEN
        RAISE EXCEPTION 'falta el movimiento % del expediente %', NEW.ultima_secuencia, NEW.id USING ERRCODE = '23514';
    END IF;
    RETURN NULL;
END
$$;
DROP TRIGGER IF EXISTS trg_expedientes_movimiento_registrado ON expedientes;
CREATE CONSTRAINT TRIGGER trg_expedientes_movimiento_registrado
    AFTER UPDATE OF ultima_secuencia ON expedientes
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION expedientes_verificar_movimiento();

-- Sin borrado físico: Expedientes solo hace bajas lógicas (activo = FALSE).
-- Se retira DELETE al rol de aplicación sobre todas las tablas existentes y
-- sobre las que creen migraciones futuras (privilegios por defecto del dueño).
-- db/roles/02 aplica el mismo criterio al reejecutarse. El rol de pruebas
-- (solo desarrollo) conserva DELETE.
REVOKE DELETE ON ALL TABLES IN SCHEMA public FROM expedientes_user;
ALTER DEFAULT PRIVILEGES FOR ROLE expedientes_owner IN SCHEMA public
    REVOKE DELETE ON TABLES FROM expedientes_user;

COMMIT;
