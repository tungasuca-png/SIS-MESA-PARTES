-- Vuelta atrás a los binarios anteriores (f04b9b3) en expedientes_db.
-- Ejecutar como expedientes_migrator tras SET ROLE expedientes_owner.
-- Solo retira las guardias de 005 (los binarios anteriores cambian área y
-- estado sin movimiento). NO borra tablas, filas, columnas ni triggers de 004:
-- movimientos y outbox quedan intactos y append-only. Reaplicar 005 al volver.
BEGIN;
DROP TRIGGER IF EXISTS trg_expedientes_guardia_insert       ON expedientes;
DROP TRIGGER IF EXISTS trg_expedientes_exigir_secuencia      ON expedientes;
DROP TRIGGER IF EXISTS trg_expedientes_movimiento_registrado ON expedientes;
COMMIT;
