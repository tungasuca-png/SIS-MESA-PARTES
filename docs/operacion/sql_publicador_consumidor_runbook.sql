-- SQL operativo validado en E1 (no es una migración). Parámetros pgx: $n.
-- Lo ejecutan, respectivamente: expedientes_user (productor), expedientes_publicador
-- (publicador), derivaciones_proyector (consumidor) y expedientes_owner (runbook).

------------------------------------------------------------------------------
-- 1. Productor: RegistrarMovimiento (expedientes_user), UNA transacción.
--    Antes: JWT, rol, permiso efectivo (Auth.HasPermission) y reglas de área
--    validados en Expedientes Service, fuera de la transacción.
------------------------------------------------------------------------------
-- BEGIN;
SET LOCAL lock_timeout = '500ms';
SET LOCAL statement_timeout = '1000ms';           -- en código: min(1000ms, deadline restante - 300ms)
SELECT area_actual, estado, ultima_secuencia, tipo, solicitante_id
  FROM expedientes WHERE id = $1 AND activo FOR UPDATE;                     -- 55P03 => 409 "ocupado"
SELECT id, huella_solicitud, secuencia
  FROM expediente_movimientos WHERE actor_id = $1 AND idempotency_key = $2; -- tras el bloqueo:
--   misma huella => devolver ESE movimiento (200); otra huella => 409 "clave reutilizada"
--   área/estado esperados distintos => 409; destino = origen => 400; reglas de negocio => 400/403
UPDATE expedientes SET area_actual = $2, estado = $3, ultima_secuencia = $4, fecha_actualizacion = now()
 WHERE id = $1;                                    -- $4 = ultima_secuencia + 1 (y descripcion en Corregir)
INSERT INTO expediente_movimientos (expediente_id, secuencia, tipo, actor_id, actor_rol, area_origen, area_destino,
       estado_anterior, estado_nuevo, motivo, condicion, idempotency_key, huella_solicitud)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13) RETURNING id, ocurrido_en;
INSERT INTO expediente_outbox (movimiento_id, expediente_id, secuencia) VALUES ($1, $2, $3);
-- si ctx vencido => ROLLBACK; si no => COMMIT
-- 23505 en uq_mov_idempotencia al confirmar => nueva lectura por (actor, clave) como arriba.

------------------------------------------------------------------------------
-- 2. Publicador (expedientes_publicador): reclamo de cabezas por expediente.
------------------------------------------------------------------------------
WITH cabezas AS (
    SELECT DISTINCT ON (expediente_id) id, estado, proximo_intento_en, reclamado_hasta
      FROM expediente_outbox WHERE estado <> 'PUBLICADO'
     ORDER BY expediente_id, secuencia),
elegibles AS (
    SELECT id FROM cabezas
     WHERE estado = 'PENDIENTE' AND proximo_intento_en <= clock_timestamp()
       AND (reclamado_hasta IS NULL OR reclamado_hasta < clock_timestamp())
     ORDER BY proximo_intento_en, id LIMIT $1),
bloqueados AS (
    SELECT o.id FROM expediente_outbox o
     WHERE o.id IN (SELECT id FROM elegibles) AND o.estado = 'PENDIENTE'
       AND (o.reclamado_hasta IS NULL OR o.reclamado_hasta < clock_timestamp())
     FOR UPDATE SKIP LOCKED)
UPDATE expediente_outbox o
   SET reclamo_id = gen_random_uuid(), reclamado_hasta = clock_timestamp() + $2::interval, intentos = o.intentos + 1
  FROM bloqueados b WHERE o.id = b.id
RETURNING o.id, o.reclamo_id, o.movimiento_id, o.intentos;

SELECT id, expediente_id, secuencia, tipo, actor_id, actor_rol, area_origen, area_destino,
       estado_anterior, estado_nuevo, motivo, condicion, ocurrido_en
  FROM expediente_movimientos WHERE id = $1;

-- Cierres: SIEMPRE id + reclamo_id + estado. 0 filas = reclamo perdido: no tocar más ese evento.
UPDATE expediente_outbox SET estado = 'PUBLICADO', reclamo_id = NULL, reclamado_hasta = NULL, ultimo_error = NULL
 WHERE id = $1 AND reclamo_id = $2 AND estado = 'PENDIENTE';
UPDATE expediente_outbox SET proximo_intento_en = clock_timestamp() + $3::interval, reclamo_id = NULL,
       reclamado_hasta = NULL, ultimo_error = $4
 WHERE id = $1 AND reclamo_id = $2 AND estado = 'PENDIENTE';
UPDATE expediente_outbox SET estado = 'FALLIDO', reclamo_id = NULL, reclamado_hasta = NULL, ultimo_error = $3
 WHERE id = $1 AND reclamo_id = $2 AND estado = 'PENDIENTE';

-- Purga (retención 30 días), en lotes.
DELETE FROM expediente_outbox
 WHERE id IN (SELECT id FROM expediente_outbox
               WHERE estado = 'PUBLICADO' AND publicado_en < clock_timestamp() - interval '30 days'
               ORDER BY id LIMIT 500);

------------------------------------------------------------------------------
-- 3. Consumidor AplicarMovimiento (derivaciones_proyector). CORREGIDO en E1:
--    primero busca y compara; solo si no existe, inserta.
------------------------------------------------------------------------------
-- Paso 1: ¿existe? => DUPLICADO (true) / DIVERGENTE (false); sin fila => paso 2.
SELECT (expediente_id, secuencia, tipo, actor_id, actor_rol, area_origen, area_destino,
        estado_anterior, estado_nuevo, motivo, condicion, ocurrido_en)
       IS NOT DISTINCT FROM
       ($2::uuid, $3::int, $4::varchar, $5::uuid, $6::varchar, $7::varchar, $8::varchar,
        $9::varchar, $10::varchar, $11::varchar, $12::varchar, $13::timestamptz) AS identico
  FROM movimientos_proyeccion WHERE movimiento_id = $1;
-- Paso 2: insertar. TG001 => Aborted (falta la anterior, reintentable);
--         23505 en uq_proy_secuencia => DIVERGENTE (permanente);
--         0 filas (carrera en la PK) => repetir el paso 1.
INSERT INTO movimientos_proyeccion (movimiento_id, expediente_id, secuencia, tipo, actor_id, actor_rol,
       area_origen, area_destino, estado_anterior, estado_nuevo, motivo, condicion, ocurrido_en)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
ON CONFLICT (movimiento_id) DO NOTHING;

------------------------------------------------------------------------------
-- 4. Runbook (expedientes_migrator + SET ROLE expedientes_owner).
------------------------------------------------------------------------------
-- Volver a encolar un evento FALLIDO (tras corregir la causa):
UPDATE expediente_outbox SET estado = 'PENDIENTE', intentos = 0, proximo_intento_en = clock_timestamp(),
       reclamo_id = NULL, reclamado_hasta = NULL, ultimo_error = NULL
 WHERE id = $1 AND estado = 'FALLIDO';
-- Republicar desde la secuencia N (proyección perdida o evento suprimido):
INSERT INTO expediente_outbox (movimiento_id, expediente_id, secuencia)
SELECT m.id, m.expediente_id, m.secuencia FROM expediente_movimientos m
 WHERE m.expediente_id = $1 AND m.secuencia >= $2
ON CONFLICT (movimiento_id) DO UPDATE SET estado = 'PENDIENTE', intentos = 0, proximo_intento_en = clock_timestamp(),
       reclamo_id = NULL, reclamado_hasta = NULL, publicado_en = NULL, ultimo_error = NULL;
