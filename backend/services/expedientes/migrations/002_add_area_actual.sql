BEGIN;

-- Area interna responsable del expediente en este momento (a quien le
-- corresponde atenderlo). Todo expediente nuevo entra por Secretaria (Mesa
-- de Partes) — coincide con el flujo F2 del analisis funcional: "Sistema ->
-- Secretaria (tramite recibido)", y solo pasa a Direccion/Subdireccion/etc.
-- cuando Secretaria (o quien tenga el expediente) registra una derivacion
-- real hacia esa area (ver Derivaciones Service, tipo = 'DERIVACION').
--
-- Los valores son los mismos codigos de rol que ya usa el JWT/Auth Service
-- (ADMIN no es un "area" de destino real, pero se incluye por si alguna vez
-- se deriva explicitamente ahi).
ALTER TABLE expedientes
    ADD COLUMN IF NOT EXISTS area_actual VARCHAR(20) NOT NULL DEFAULT 'SECRETARIA';

ALTER TABLE expedientes
    ADD CONSTRAINT chk_expedientes_area_actual
    CHECK (area_actual IN ('SECRETARIA', 'DIRECTOR', 'SUBDIRECTOR', 'DOCENTE', 'AUXILIAR', 'ADMIN'));

CREATE INDEX IF NOT EXISTS idx_expedientes_area_actual
    ON expedientes (area_actual);

COMMIT;

-- Down migration:
-- ALTER TABLE expedientes DROP CONSTRAINT IF EXISTS chk_expedientes_area_actual;
-- DROP INDEX IF EXISTS idx_expedientes_area_actual;
-- ALTER TABLE expedientes DROP COLUMN IF EXISTS area_actual;
