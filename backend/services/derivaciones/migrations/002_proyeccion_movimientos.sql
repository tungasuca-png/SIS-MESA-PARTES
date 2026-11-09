-- 002 — Proyección de movimientos e historial protegido.
--
-- Ejecutar como derivaciones_migrator tras SET ROLE derivaciones_owner.
-- Requiere db/roles/01_roles.sql actualizado (rol derivaciones_proyector).
-- Idempotente. No hace DML sobre derivaciones: las filas existentes quedan
-- intactas (id y contenido); solo se agregan índice, triggers y una vista.

BEGIN;

CREATE TABLE IF NOT EXISTS movimientos_proyeccion (
    movimiento_id   UUID PRIMARY KEY,
    expediente_id   UUID NOT NULL,
    secuencia       INTEGER NOT NULL,
    tipo            VARCHAR(20) NOT NULL,
    actor_id        UUID NOT NULL,
    actor_rol       VARCHAR(30) NOT NULL,
    area_origen     VARCHAR(20) NOT NULL,
    area_destino    VARCHAR(20) NOT NULL,
    estado_anterior VARCHAR(20) NOT NULL,
    estado_nuevo    VARCHAR(20) NOT NULL,
    motivo          VARCHAR(500) NOT NULL,
    condicion       VARCHAR(255) NOT NULL,
    ocurrido_en     TIMESTAMPTZ NOT NULL,
    proyectado_en   TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT uq_proy_secuencia  UNIQUE (expediente_id, secuencia),
    CONSTRAINT chk_proy_secuencia CHECK (secuencia > 0),
    CONSTRAINT chk_proy_tipo      CHECK (tipo IN ('DERIVACION', 'RECHAZO', 'CAMBIO_ESTADO', 'RESOLUCION', 'CORRECCION')),
    CONSTRAINT chk_proy_areas     CHECK (
            area_origen  IN ('SECRETARIA', 'DIRECTOR', 'SUBDIRECTOR', 'DOCENTE', 'AUXILIAR', 'ADMIN')
        AND area_destino IN ('SECRETARIA', 'DIRECTOR', 'SUBDIRECTOR', 'DOCENTE', 'AUXILIAR', 'ADMIN')),
    CONSTRAINT chk_proy_estados   CHECK (
            estado_anterior IN ('PENDIENTE', 'EN_PROCESO', 'ATENDIDO', 'OBSERVADO')
        AND estado_nuevo    IN ('PENDIENTE', 'EN_PROCESO', 'ATENDIDO', 'OBSERVADO'))
);
CREATE INDEX IF NOT EXISTS idx_proy_expediente_fecha         ON movimientos_proyeccion (expediente_id, ocurrido_en DESC);
CREATE INDEX IF NOT EXISTS idx_derivaciones_expediente_fecha ON derivaciones (expediente_id, fecha_registro DESC);

CREATE OR REPLACE FUNCTION derivaciones_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION '% es append-only: % no permitido', TG_TABLE_NAME, TG_OP USING ERRCODE = '42501';
END
$$;

-- derivaciones: inmutable y sin movimientos manuales nuevos (solo anotaciones).
CREATE OR REPLACE FUNCTION derivaciones_proteger() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.tipo = 'DERIVACION' THEN
            RAISE EXCEPTION 'los movimientos se registran solo desde Expedientes' USING ERRCODE = '42501';
        END IF;
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'derivaciones es append-only: % no permitido', TG_OP USING ERRCODE = '42501';
END
$$;
DROP TRIGGER IF EXISTS trg_derivaciones_proteger ON derivaciones;
CREATE TRIGGER trg_derivaciones_proteger BEFORE INSERT OR UPDATE OR DELETE ON derivaciones
    FOR EACH ROW EXECUTE FUNCTION derivaciones_proteger();
DROP TRIGGER IF EXISTS trg_derivaciones_no_truncate ON derivaciones;
CREATE TRIGGER trg_derivaciones_no_truncate BEFORE TRUNCATE ON derivaciones
    FOR EACH STATEMENT EXECUTE FUNCTION derivaciones_append_only();

-- Proyección: hora puesta por la base y prefijo contiguo por expediente
-- (TG001 = falta la secuencia anterior; el publicador reintenta).
CREATE OR REPLACE FUNCTION movimientos_proyeccion_antes_insertar() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.proyectado_en := clock_timestamp();
    IF NEW.secuencia > 1 AND NOT EXISTS (
           SELECT 1 FROM movimientos_proyeccion p
            WHERE p.expediente_id = NEW.expediente_id AND p.secuencia = NEW.secuencia - 1) THEN
        RAISE EXCEPTION 'falta la secuencia % del expediente %', NEW.secuencia - 1, NEW.expediente_id
            USING ERRCODE = 'TG001';
    END IF;
    RETURN NEW;
END
$$;
DROP TRIGGER IF EXISTS trg_proy_antes_insertar ON movimientos_proyeccion;
CREATE TRIGGER trg_proy_antes_insertar BEFORE INSERT ON movimientos_proyeccion
    FOR EACH ROW EXECUTE FUNCTION movimientos_proyeccion_antes_insertar();
DROP TRIGGER IF EXISTS trg_proy_no_mod ON movimientos_proyeccion;
CREATE TRIGGER trg_proy_no_mod BEFORE UPDATE OR DELETE ON movimientos_proyeccion
    FOR EACH ROW EXECUTE FUNCTION derivaciones_append_only();
DROP TRIGGER IF EXISTS trg_proy_no_truncate ON movimientos_proyeccion;
CREATE TRIGGER trg_proy_no_truncate BEFORE TRUNCATE ON movimientos_proyeccion
    FOR EACH STATEMENT EXECUTE FUNCTION derivaciones_append_only();

-- Historial unificado: LEGADO (derivaciones tipo DERIVACION, todas anteriores
-- a esta migración), ANOTACION (resto de derivaciones) y MOVIMIENTO (proyección).
CREATE OR REPLACE VIEW historial_expediente WITH (security_invoker = true) AS
SELECT d.id, d.expediente_id,
       CASE WHEN d.tipo = 'DERIVACION' THEN 'LEGADO' ELSE 'ANOTACION' END AS clase,
       d.tipo, d.origen, d.destino,
       NULL::varchar AS estado_anterior, NULL::varchar AS estado_nuevo,
       d.motivo, d.condicion, d.registrado_por, NULL::varchar AS actor_rol,
       d.fecha_registro AS fecha, NULL::integer AS secuencia
  FROM derivaciones d
UNION ALL
SELECT m.movimiento_id, m.expediente_id, 'MOVIMIENTO', m.tipo, m.area_origen, m.area_destino,
       m.estado_anterior, m.estado_nuevo, m.motivo, m.condicion, m.actor_id, m.actor_rol,
       m.ocurrido_en, m.secuencia
  FROM movimientos_proyeccion m;

-- Privilegios mínimos (D3): solo derivaciones_proyector inserta en la proyección.
-- reconciliador_movimientos (D8) solo la lee; no tiene acceso a derivaciones.
-- Requiere db/roles/01_roles.sql actualizado (roles derivaciones_proyector y reconciliador_movimientos).
GRANT CONNECT ON DATABASE derivaciones_db TO derivaciones_proyector, reconciliador_movimientos;
REVOKE ALL ON movimientos_proyeccion FROM derivaciones_user, derivaciones_tester, derivaciones_proyector, reconciliador_movimientos;
GRANT SELECT         ON movimientos_proyeccion TO derivaciones_user, derivaciones_tester, reconciliador_movimientos;
GRANT SELECT, INSERT ON movimientos_proyeccion TO derivaciones_proyector;
REVOKE UPDATE, DELETE ON derivaciones FROM derivaciones_user, derivaciones_tester;
REVOKE ALL ON historial_expediente FROM derivaciones_user, derivaciones_tester, derivaciones_proyector;
GRANT SELECT ON historial_expediente TO derivaciones_user, derivaciones_tester;

COMMIT;
