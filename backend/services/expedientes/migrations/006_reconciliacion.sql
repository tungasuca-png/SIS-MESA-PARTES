-- 006 — Reconciliación diaria entre la fuente de verdad y la proyección (D8).
--
-- Ejecutar como expedientes_migrator tras SET ROLE expedientes_owner, después
-- de 004 y 005. Requiere db/roles/01_roles.sql actualizado (rol
-- reconciliador_movimientos). Idempotente. No modifica datos existentes.
--
-- El proceso reconciliador (rol reconciliador_movimientos):
--   * LEE expediente_movimientos, expediente_outbox, expedientes (solo id,
--     área, estado, activo y última secuencia) y, en derivaciones_db,
--     movimientos_proyeccion;
--   * REGISTRA cada ejecución y cada discrepancia (append-only, con la hora y
--     el usuario de sesión puestos por la base);
--   * REPUBLICA solo a través de reconciliacion_republicar(): únicamente
--     movimientos detectados SIN PROYECCIÓN en una ejecución de modo
--     REPUBLICACION en curso, una vez por movimiento y ejecución, reencolando o
--     recreando su evento de outbox. Nunca modifica movimientos, la proyección
--     ni el historial: las proyecciones divergentes o huérfanas y los eventos
--     FALLIDO quedan para revisión humana.

BEGIN;

CREATE TABLE IF NOT EXISTS reconciliacion_ejecuciones (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    modo                    VARCHAR(15) NOT NULL,
    corte_hasta             TIMESTAMPTZ NOT NULL,
    iniciada_en             TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    finalizada_en           TIMESTAMPTZ,
    estado                  VARCHAR(20) NOT NULL DEFAULT 'EN_CURSO',
    expedientes_revisados   INTEGER,
    movimientos_origen      INTEGER,
    movimientos_proyectados INTEGER,
    discrepancias           INTEGER,
    huella_informe          CHAR(64),
    version_reconciliador   VARCHAR(40) NOT NULL,
    ejecutada_por           VARCHAR(63) NOT NULL DEFAULT session_user,
    CONSTRAINT chk_rec_modo      CHECK (modo IN ('DETECCION', 'REPUBLICACION')),
    CONSTRAINT chk_rec_estado    CHECK (estado IN ('EN_CURSO', 'SIN_DISCREPANCIAS', 'CON_DISCREPANCIAS', 'ERROR')),
    CONSTRAINT chk_rec_final     CHECK ((estado = 'EN_CURSO') = (finalizada_en IS NULL)),
    CONSTRAINT chk_rec_conteo    CHECK (estado <> 'SIN_DISCREPANCIAS' OR discrepancias = 0),
    CONSTRAINT chk_rec_conteo2   CHECK (estado <> 'CON_DISCREPANCIAS' OR discrepancias > 0),
    CONSTRAINT chk_rec_huella    CHECK (huella_informe IS NULL OR huella_informe ~ '^[0-9a-f]{64}$'),
    CONSTRAINT chk_rec_corte     CHECK (corte_hasta <= iniciada_en)
);
CREATE INDEX IF NOT EXISTS idx_rec_ejecuciones_finalizada ON reconciliacion_ejecuciones (finalizada_en DESC);

CREATE TABLE IF NOT EXISTS reconciliacion_discrepancias (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ejecucion_id  UUID NOT NULL REFERENCES reconciliacion_ejecuciones (id),
    tipo          VARCHAR(40) NOT NULL,
    expediente_id UUID NOT NULL,
    secuencia     INTEGER,
    movimiento_id UUID,
    detalle       VARCHAR(500) NOT NULL,
    detectada_en  TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT chk_disc_tipo CHECK (tipo IN (
        'MOVIMIENTO_SIN_PROYECCION',       -- sin proyección y sin evento (purgado o perdido)
        'PUBLICADO_SIN_PROYECCION',        -- evento PUBLICADO pero sin proyección (incluido el último movimiento)
        'EVENTO_RETRASADO',                -- evento PENDIENTE más antiguo que la ventana de gracia
        'EVENTO_FALLIDO',                  -- evento FALLIDO: revisión humana
        'PROYECCION_DIVERGENTE',           -- proyección con algún campo distinto del origen
        'PROYECCION_HUERFANA',             -- proyección sin movimiento de origen
        'SECUENCIA_ORIGEN_INCOMPLETA',     -- huecos en la secuencia de origen o última secuencia incoherente
        'SECUENCIA_PROYECCION_INCOMPLETA', -- huecos en la secuencia proyectada
        'EXPEDIENTE_INCOHERENTE',          -- área/estado del expediente distintos del último movimiento
        'CADENA_DISCONTINUA')),            -- origen de un movimiento distinto del destino del anterior (cambio no trazado)
    CONSTRAINT uq_disc UNIQUE NULLS NOT DISTINCT (ejecucion_id, tipo, expediente_id, secuencia, movimiento_id)
);
CREATE INDEX IF NOT EXISTS idx_disc_ejecucion  ON reconciliacion_discrepancias (ejecucion_id);
CREATE INDEX IF NOT EXISTS idx_disc_movimiento ON reconciliacion_discrepancias (movimiento_id) WHERE movimiento_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS reconciliacion_acciones (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ejecucion_id  UUID NOT NULL REFERENCES reconciliacion_ejecuciones (id),
    movimiento_id UUID NOT NULL REFERENCES expediente_movimientos (id),
    accion        VARCHAR(30) NOT NULL,
    realizada_en  TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    realizada_por VARCHAR(63) NOT NULL DEFAULT session_user,
    CONSTRAINT chk_acc_accion CHECK (accion IN ('REENCOLADO', 'EVENTO_RECREADO', 'SIN_CAMBIO_YA_PENDIENTE', 'RECHAZADO_FALLIDO')),
    CONSTRAINT uq_acc_ejecucion_movimiento UNIQUE (ejecucion_id, movimiento_id)
);

-- Ejecuciones: la base fija horas y usuario; una ejecución solo se finaliza una
-- vez (EN_CURSO -> estado final) y nunca se borra.
CREATE OR REPLACE FUNCTION reconciliacion_ejecucion_proteger() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        NEW.iniciada_en   := clock_timestamp();
        NEW.finalizada_en := NULL;
        NEW.estado        := 'EN_CURSO';
        NEW.ejecutada_por := session_user;
        IF NEW.corte_hasta > NEW.iniciada_en THEN
            NEW.corte_hasta := NEW.iniciada_en;
        END IF;
        RETURN NEW;
    END IF;
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'reconciliacion_ejecuciones es append-only: DELETE no permitido' USING ERRCODE = '42501';
    END IF;
    IF OLD.estado <> 'EN_CURSO' THEN
        RAISE EXCEPTION 'la ejecución % ya está finalizada (%)', OLD.id, OLD.estado USING ERRCODE = '42501';
    END IF;
    IF NEW.estado = 'EN_CURSO' THEN
        RAISE EXCEPTION 'una ejecución solo se actualiza para finalizarla' USING ERRCODE = '42501';
    END IF;
    IF NEW.id <> OLD.id OR NEW.modo <> OLD.modo OR NEW.corte_hasta <> OLD.corte_hasta
       OR NEW.iniciada_en <> OLD.iniciada_en OR NEW.version_reconciliador <> OLD.version_reconciliador
       OR NEW.ejecutada_por <> OLD.ejecutada_por THEN
        RAISE EXCEPTION 'datos de inicio de la ejecución inmutables' USING ERRCODE = '42501';
    END IF;
    NEW.finalizada_en := clock_timestamp();
    RETURN NEW;
END
$$;
DROP TRIGGER IF EXISTS trg_rec_ejecucion_proteger ON reconciliacion_ejecuciones;
CREATE TRIGGER trg_rec_ejecucion_proteger BEFORE INSERT OR UPDATE OR DELETE ON reconciliacion_ejecuciones
    FOR EACH ROW EXECUTE FUNCTION reconciliacion_ejecucion_proteger();
DROP TRIGGER IF EXISTS trg_rec_ejecucion_no_truncate ON reconciliacion_ejecuciones;
CREATE TRIGGER trg_rec_ejecucion_no_truncate BEFORE TRUNCATE ON reconciliacion_ejecuciones
    FOR EACH STATEMENT EXECUTE FUNCTION expedientes_append_only();

-- Discrepancias: solo en una ejecución EN_CURSO; hora fijada por la base; append-only.
CREATE OR REPLACE FUNCTION reconciliacion_discrepancia_antes_insertar() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM reconciliacion_ejecuciones e WHERE e.id = NEW.ejecucion_id AND e.estado = 'EN_CURSO') THEN
        RAISE EXCEPTION 'la ejecución % no está en curso', NEW.ejecucion_id USING ERRCODE = '42501';
    END IF;
    NEW.detectada_en := clock_timestamp();
    RETURN NEW;
END
$$;
DROP TRIGGER IF EXISTS trg_disc_antes_insertar ON reconciliacion_discrepancias;
CREATE TRIGGER trg_disc_antes_insertar BEFORE INSERT ON reconciliacion_discrepancias
    FOR EACH ROW EXECUTE FUNCTION reconciliacion_discrepancia_antes_insertar();
DROP TRIGGER IF EXISTS trg_disc_no_mod ON reconciliacion_discrepancias;
CREATE TRIGGER trg_disc_no_mod BEFORE UPDATE OR DELETE ON reconciliacion_discrepancias
    FOR EACH ROW EXECUTE FUNCTION expedientes_append_only();
DROP TRIGGER IF EXISTS trg_disc_no_truncate ON reconciliacion_discrepancias;
CREATE TRIGGER trg_disc_no_truncate BEFORE TRUNCATE ON reconciliacion_discrepancias
    FOR EACH STATEMENT EXECUTE FUNCTION expedientes_append_only();
DROP TRIGGER IF EXISTS trg_acc_no_mod ON reconciliacion_acciones;
CREATE TRIGGER trg_acc_no_mod BEFORE UPDATE OR DELETE ON reconciliacion_acciones
    FOR EACH ROW EXECUTE FUNCTION expedientes_append_only();
DROP TRIGGER IF EXISTS trg_acc_no_truncate ON reconciliacion_acciones;
CREATE TRIGGER trg_acc_no_truncate BEFORE TRUNCATE ON reconciliacion_acciones
    FOR EACH STATEMENT EXECUTE FUNCTION expedientes_append_only();

-- Republicación controlada e idempotente. SECURITY DEFINER (dueño
-- expedientes_owner) con search_path fijo: el reconciliador no tiene UPDATE ni
-- INSERT sobre el outbox; solo puede pedir esta acción acotada.
CREATE OR REPLACE FUNCTION reconciliacion_republicar(p_ejecucion UUID, p_movimiento UUID)
RETURNS VARCHAR
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
    v_exp    UUID;
    v_seq    INTEGER;
    v_estado VARCHAR(10);
    v_accion VARCHAR(30);
BEGIN
    IF NOT EXISTS (SELECT 1 FROM public.reconciliacion_ejecuciones e
                    WHERE e.id = p_ejecucion AND e.modo = 'REPUBLICACION' AND e.estado = 'EN_CURSO') THEN
        RAISE EXCEPTION 'la ejecución % no es una republicación en curso', p_ejecucion USING ERRCODE = '42501';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM public.reconciliacion_discrepancias d
                    WHERE d.ejecucion_id = p_ejecucion AND d.movimiento_id = p_movimiento
                      AND d.tipo IN ('MOVIMIENTO_SIN_PROYECCION', 'PUBLICADO_SIN_PROYECCION')) THEN
        RAISE EXCEPTION 'solo se republican movimientos detectados sin proyección en esta ejecución' USING ERRCODE = '42501';
    END IF;
    SELECT m.expediente_id, m.secuencia INTO v_exp, v_seq
      FROM public.expediente_movimientos m WHERE m.id = p_movimiento;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'movimiento % inexistente', p_movimiento USING ERRCODE = '42501';
    END IF;
    IF EXISTS (SELECT 1 FROM public.reconciliacion_acciones a
                WHERE a.ejecucion_id = p_ejecucion AND a.movimiento_id = p_movimiento) THEN
        RETURN 'YA_PROCESADO_EN_ESTA_EJECUCION';
    END IF;

    SELECT o.estado INTO v_estado FROM public.expediente_outbox o WHERE o.movimiento_id = p_movimiento FOR UPDATE;
    IF NOT FOUND THEN
        INSERT INTO public.expediente_outbox (movimiento_id, expediente_id, secuencia) VALUES (p_movimiento, v_exp, v_seq);
        v_accion := 'EVENTO_RECREADO';
    ELSIF v_estado = 'PUBLICADO' THEN
        UPDATE public.expediente_outbox
           SET estado = 'PENDIENTE', intentos = 0, proximo_intento_en = clock_timestamp(), reclamo_id = NULL,
               reclamado_hasta = NULL, publicado_en = NULL,
               ultimo_error = left('republicado por reconciliación ' || p_ejecucion::text, 500)
         WHERE movimiento_id = p_movimiento;
        v_accion := 'REENCOLADO';
    ELSIF v_estado = 'PENDIENTE' THEN
        v_accion := 'SIN_CAMBIO_YA_PENDIENTE';
    ELSE
        v_accion := 'RECHAZADO_FALLIDO';       -- un FALLIDO lo revisa una persona (runbook)
    END IF;
    INSERT INTO public.reconciliacion_acciones (ejecucion_id, movimiento_id, accion) VALUES (p_ejecucion, p_movimiento, v_accion);
    RETURN v_accion;
END
$$;
REVOKE ALL ON FUNCTION reconciliacion_republicar(UUID, UUID) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION reconciliacion_republicar(UUID, UUID) TO reconciliador_movimientos;

-- Vigilancia ("dead man's switch"): alerta si no hubo una ejecución finalizada
-- correctamente en las últimas 26 horas o si la última encontró discrepancias.
CREATE OR REPLACE VIEW reconciliacion_vigilancia WITH (security_invoker = true) AS
SELECT max(finalizada_en) FILTER (WHERE estado IN ('SIN_DISCREPANCIAS', 'CON_DISCREPANCIAS')) AS ultima_ejecucion_correcta,
       (SELECT e.estado FROM reconciliacion_ejecuciones e WHERE e.estado <> 'EN_CURSO'
         ORDER BY e.finalizada_en DESC LIMIT 1)                                            AS ultimo_estado,
       (SELECT e.discrepancias FROM reconciliacion_ejecuciones e WHERE e.estado <> 'EN_CURSO'
         ORDER BY e.finalizada_en DESC LIMIT 1)                                            AS ultimas_discrepancias,
       coalesce(max(finalizada_en) FILTER (WHERE estado IN ('SIN_DISCREPANCIAS', 'CON_DISCREPANCIAS'))
                < now() - interval '26 hours', true)                                       AS alerta_sin_ejecucion
  FROM reconciliacion_ejecuciones;

-- Privilegios mínimos. DEFAULT PRIVILEGES del dueño concedió S/I/U a
-- expedientes_user y S/I/U/D a expedientes_tester: se revoca todo y se concede lo justo.
GRANT CONNECT ON DATABASE expedientes_db TO reconciliador_movimientos;
REVOKE ALL ON reconciliacion_ejecuciones, reconciliacion_discrepancias, reconciliacion_acciones, reconciliacion_vigilancia
    FROM expedientes_user, expedientes_tester, expedientes_publicador, reconciliador_movimientos;
GRANT SELECT ON reconciliacion_ejecuciones, reconciliacion_discrepancias, reconciliacion_acciones, reconciliacion_vigilancia
    TO reconciliador_movimientos, expedientes_tester;
GRANT INSERT ON reconciliacion_ejecuciones, reconciliacion_discrepancias TO reconciliador_movimientos;
GRANT UPDATE (estado, expedientes_revisados, movimientos_origen, movimientos_proyectados, discrepancias, huella_informe)
    ON reconciliacion_ejecuciones TO reconciliador_movimientos;
GRANT SELECT ON expediente_movimientos, expediente_outbox TO reconciliador_movimientos;
GRANT SELECT (id, area_actual, estado, activo, ultima_secuencia) ON expedientes TO reconciliador_movimientos;

COMMIT;
