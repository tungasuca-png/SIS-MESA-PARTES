BEGIN;

-- Estudiantes Service — Modelo maestro, etapa 3: MATRÍCULA (diseño E-02 §B,
-- estados D-M4 aprobados). Separa lo académico de la identidad.
--
-- APLICAR con estudiantes_migrator actuando como dueño:
--   (echo "SET ROLE estudiantes_owner;"; cat 007_matricula.sql) | \
--     psql -U estudiantes_migrator -d estudiantes_db -v ON_ERROR_STOP=1
-- Aditiva e idempotente: crea tablas nuevas; NO modifica ni borra las
-- columnas académicas heredadas de "estudiantes" (nivel, grado, seccion,
-- turno, estado_matricula), que siguen siendo leídas por los consumidores
-- actuales durante la transición.
--
-- Historial: una matrícula por estudiante y periodo (el historial de años);
-- los cambios dentro del periodo quedan en matricula_movimientos
-- (append-only). Un cambio de sección o de periodo nunca sobrescribe lo
-- anterior.
--
-- CARGA DE LOS DATOS HEREDADOS: NO se realiza en esta migración. Requiere
-- decidir a qué periodo académico corresponden los valores actuales (no se
-- inventan periodos). Al final se informa cuántos estudiantes tienen datos
-- académicos heredados; al aplicar esta migración eran 0.

-- updated_at genérico para tablas sin columna version.
CREATE OR REPLACE FUNCTION tg_actualizar_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at := CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE IF NOT EXISTS periodos_academicos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    anio SMALLINT NOT NULL,
    nombre VARCHAR(60) NOT NULL,
    fecha_inicio DATE NULL,
    fecha_fin DATE NULL,
    estado VARCHAR(20) NOT NULL DEFAULT 'PLANIFICADO',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_periodos_academicos_anio UNIQUE (anio),
    CONSTRAINT chk_periodos_academicos_anio CHECK (anio BETWEEN 2000 AND 2100),
    CONSTRAINT chk_periodos_academicos_nombre CHECK (length(TRIM(BOTH FROM nombre)) > 0),
    CONSTRAINT chk_periodos_academicos_fechas CHECK (fecha_inicio IS NULL OR fecha_fin IS NULL OR fecha_fin >= fecha_inicio),
    CONSTRAINT chk_periodos_academicos_estado CHECK (estado IN ('PLANIFICADO', 'EN_CURSO', 'CERRADO'))
);

DROP TRIGGER IF EXISTS trg_periodos_academicos_updated_at ON periodos_academicos;
CREATE TRIGGER trg_periodos_academicos_updated_at
    BEFORE UPDATE ON periodos_academicos
    FOR EACH ROW EXECUTE FUNCTION tg_actualizar_updated_at();

-- Sección (aula) de un periodo. turno/aula/vacantes: opcionales (NULL si no
-- se conocen; nunca cadena vacía).
CREATE TABLE IF NOT EXISTS secciones (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    periodo_id UUID NOT NULL,
    nivel VARCHAR(20) NOT NULL,
    grado VARCHAR(20) NOT NULL,
    seccion VARCHAR(10) NOT NULL,
    turno VARCHAR(20) NULL,
    aula VARCHAR(40) NULL,
    vacantes SMALLINT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_secciones_periodo FOREIGN KEY (periodo_id) REFERENCES periodos_academicos (id) ON DELETE RESTRICT,
    -- (id, periodo_id) único: permite que la FK compuesta de matriculas
    -- garantice que la sección pertenece al MISMO periodo de la matrícula.
    CONSTRAINT uq_secciones_id_periodo UNIQUE (id, periodo_id),
    CONSTRAINT uq_secciones_periodo_grupo UNIQUE NULLS NOT DISTINCT (periodo_id, nivel, grado, seccion, turno),
    CONSTRAINT chk_secciones_nivel CHECK (nivel IN ('INICIAL', 'PRIMARIA', 'SECUNDARIA')),
    CONSTRAINT chk_secciones_textos CHECK (
        length(TRIM(BOTH FROM grado)) > 0 AND length(TRIM(BOTH FROM seccion)) > 0
        AND (turno IS NULL OR length(TRIM(BOTH FROM turno)) > 0)
        AND (aula IS NULL OR length(TRIM(BOTH FROM aula)) > 0)
    ),
    CONSTRAINT chk_secciones_vacantes CHECK (vacantes IS NULL OR vacantes >= 0)
);

DROP TRIGGER IF EXISTS trg_secciones_updated_at ON secciones;
CREATE TRIGGER trg_secciones_updated_at
    BEFORE UPDATE ON secciones
    FOR EACH ROW EXECUTE FUNCTION tg_actualizar_updated_at();

-- Matrícula: una por estudiante y periodo. seccion_id puede ser NULL (p. ej.
-- matriculado sin sección asignada todavía). version + updated_at los
-- mantiene el mismo trigger que "estudiantes" (concurrencia optimista).
CREATE TABLE IF NOT EXISTS matriculas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    estudiante_id UUID NOT NULL,
    periodo_id UUID NOT NULL,
    seccion_id UUID NULL,
    estado_matricula VARCHAR(20) NOT NULL DEFAULT 'MATRICULADO',
    fecha_matricula DATE NULL,
    fecha_retiro DATE NULL,
    motivo_retiro VARCHAR(200) NULL,
    -- Trazabilidad del origen del dato (REGISTRO = operación de la API;
    -- MIGRACION_LEGADO = cargado desde las columnas heredadas).
    origen VARCHAR(20) NOT NULL DEFAULT 'REGISTRO',
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_matriculas_estudiante FOREIGN KEY (estudiante_id) REFERENCES estudiantes (id) ON DELETE RESTRICT,
    CONSTRAINT fk_matriculas_periodo FOREIGN KEY (periodo_id) REFERENCES periodos_academicos (id) ON DELETE RESTRICT,
    CONSTRAINT fk_matriculas_seccion_mismo_periodo FOREIGN KEY (seccion_id, periodo_id)
        REFERENCES secciones (id, periodo_id) ON DELETE RESTRICT,
    CONSTRAINT uq_matriculas_estudiante_periodo UNIQUE (estudiante_id, periodo_id),
    CONSTRAINT chk_matriculas_estado CHECK (estado_matricula IN ('MATRICULADO', 'RETIRADO', 'TRASLADADO', 'CONCLUIDO')),
    CONSTRAINT chk_matriculas_retiro CHECK (
        (estado_matricula IN ('RETIRADO', 'TRASLADADO')) OR (fecha_retiro IS NULL AND motivo_retiro IS NULL)
    ),
    CONSTRAINT chk_matriculas_fechas CHECK (fecha_retiro IS NULL OR fecha_matricula IS NULL OR fecha_retiro >= fecha_matricula),
    CONSTRAINT chk_matriculas_motivo CHECK (motivo_retiro IS NULL OR length(TRIM(BOTH FROM motivo_retiro)) > 0),
    CONSTRAINT chk_matriculas_origen CHECK (origen IN ('REGISTRO', 'MIGRACION_LEGADO')),
    CONSTRAINT chk_matriculas_version CHECK (version >= 1)
);

-- Listado de una sección/periodo (la búsqueda por estudiante ya la cubre
-- uq_matriculas_estudiante_periodo, que empieza por estudiante_id).
CREATE INDEX IF NOT EXISTS idx_matriculas_periodo_seccion ON matriculas (periodo_id, seccion_id);

DROP TRIGGER IF EXISTS trg_matriculas_actualizar_version ON matriculas;
CREATE TRIGGER trg_matriculas_actualizar_version
    BEFORE UPDATE ON matriculas
    FOR EACH ROW EXECUTE FUNCTION estudiantes_actualizar_version();

-- Historial de movimientos dentro de un periodo (append-only).
CREATE TABLE IF NOT EXISTS matricula_movimientos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    matricula_id UUID NOT NULL,
    tipo VARCHAR(20) NOT NULL,
    seccion_anterior_id UUID NULL,
    seccion_nueva_id UUID NULL,
    fecha DATE NOT NULL,
    motivo VARCHAR(200) NULL,
    registrado_por UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_movimientos_matricula FOREIGN KEY (matricula_id) REFERENCES matriculas (id) ON DELETE RESTRICT,
    CONSTRAINT fk_movimientos_seccion_anterior FOREIGN KEY (seccion_anterior_id) REFERENCES secciones (id) ON DELETE RESTRICT,
    CONSTRAINT fk_movimientos_seccion_nueva FOREIGN KEY (seccion_nueva_id) REFERENCES secciones (id) ON DELETE RESTRICT,
    CONSTRAINT chk_movimientos_tipo CHECK (tipo IN ('CAMBIO_SECCION', 'CAMBIO_TURNO', 'RETIRO', 'REINGRESO', 'TRASLADO_SALIDA')),
    CONSTRAINT chk_movimientos_motivo CHECK (motivo IS NULL OR length(TRIM(BOTH FROM motivo)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_matricula_movimientos_matricula ON matricula_movimientos (matricula_id, created_at);

CREATE OR REPLACE FUNCTION matricula_movimientos_append_only() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'matricula_movimientos es append-only: % no permitido', TG_OP;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_matricula_movimientos_no_mod ON matricula_movimientos;
CREATE TRIGGER trg_matricula_movimientos_no_mod
    BEFORE UPDATE OR DELETE ON matricula_movimientos
    FOR EACH ROW EXECUTE FUNCTION matricula_movimientos_append_only();

DROP TRIGGER IF EXISTS trg_matricula_movimientos_no_truncate ON matricula_movimientos;
CREATE TRIGGER trg_matricula_movimientos_no_truncate
    BEFORE TRUNCATE ON matricula_movimientos
    FOR EACH STATEMENT EXECUTE FUNCTION matricula_movimientos_append_only();

-- Privilegios mínimos: esta etapa solo LEE matrículas desde la aplicación
-- (las operaciones de escritura de matrícula son una fase posterior).
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'estudiantes_user') THEN
        GRANT SELECT ON periodos_academicos, secciones, matriculas, matricula_movimientos TO estudiantes_user;
    END IF;
END
$$;

-- Informe de la transición (no modifica nada).
DO $$
DECLARE
    heredados bigint;
BEGIN
    SELECT count(*) INTO heredados FROM estudiantes
    WHERE btrim(nivel) <> '' OR btrim(grado) <> '' OR btrim(seccion) <> '' OR btrim(turno) <> '';
    RAISE NOTICE '007: estudiantes con datos académicos heredados pendientes de carga: %', heredados;
END
$$;

COMMIT;

-- Down migration (solo desarrollo; exige desactivar el trigger append-only):
-- DROP TABLE matricula_movimientos, matriculas, secciones, periodos_academicos;
-- DROP FUNCTION matricula_movimientos_append_only(); DROP FUNCTION tg_actualizar_updated_at();
