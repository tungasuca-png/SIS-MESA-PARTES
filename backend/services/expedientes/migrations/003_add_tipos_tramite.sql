BEGIN;

-- Amplia el catalogo de tipos de expediente para distinguir los tramites
-- reales de Mesa de Partes (antes todo entraba como "SOLICITUD" generica
-- desde el FUT Digital -- ver docs/etapa-2-tipos-tramite.md). Los 5 valores
-- nuevos vienen de docs/analisis-5-flujos-completo.md (FLUJO 2: Certificado/
-- Constancia de Estudios; FLUJO 4: Permiso/Justificacion de Falta/
-- Justificacion de Tardanza) -- no se inventan.
--
-- Se mantienen SOLICITUD/TRAMITE/OFICIO/OTRO: expedientes existentes con
-- esos valores siguen siendo validos (no se migran/reinterpretan), y el
-- registro interno generico (ExpedienteFormModal.jsx, para el personal
-- interno) sigue pudiendo usarlos.
--
-- "JUSTIFICACION_TARDANZA" (22 caracteres) no entra en VARCHAR(20); se
-- amplia la columna a VARCHAR(30). Ampliar un varchar nunca trunca ni
-- afecta los valores ya guardados.
ALTER TABLE expedientes ALTER COLUMN tipo TYPE VARCHAR(30);

ALTER TABLE expedientes DROP CONSTRAINT chk_expedientes_tipo;

ALTER TABLE expedientes
    ADD CONSTRAINT chk_expedientes_tipo
    CHECK (tipo IN (
        'SOLICITUD', 'TRAMITE', 'OFICIO', 'OTRO',
        'CERTIFICADO', 'CONSTANCIA', 'PERMISO',
        'JUSTIFICACION_FALTA', 'JUSTIFICACION_TARDANZA'
    ));

COMMIT;

-- Down migration:
-- ALTER TABLE expedientes DROP CONSTRAINT IF EXISTS chk_expedientes_tipo;
-- ALTER TABLE expedientes ADD CONSTRAINT chk_expedientes_tipo CHECK (tipo IN ('SOLICITUD', 'TRAMITE', 'OFICIO', 'OTRO'));
-- ALTER TABLE expedientes ALTER COLUMN tipo TYPE VARCHAR(20);
