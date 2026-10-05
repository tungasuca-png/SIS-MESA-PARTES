BEGIN;

-- Periodos lectivos de la institución.
-- Ejemplo: Año Escolar 2026.
CREATE TABLE IF NOT EXISTS periodos_lectivos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nombre VARCHAR(100) NOT NULL,
    anio INTEGER NOT NULL,
    fecha_inicio DATE NOT NULL,
    fecha_fin DATE NOT NULL,
    estado VARCHAR(20) NOT NULL DEFAULT 'PLANIFICADO',

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT uq_periodos_lectivos_anio UNIQUE (anio),

    CONSTRAINT chk_periodos_lectivos_anio
        CHECK (anio >= 2000 AND anio <= 2100),

    CONSTRAINT chk_periodos_lectivos_fechas
        CHECK (fecha_fin >= fecha_inicio),

    CONSTRAINT chk_periodos_lectivos_estado
        CHECK (estado IN ('PLANIFICADO', 'ACTIVO', 'CERRADO'))
);


-- Niveles educativos.
CREATE TABLE IF NOT EXISTS niveles_educativos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nombre VARCHAR(30) NOT NULL,
    estado BOOLEAN NOT NULL DEFAULT TRUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT uq_niveles_educativos_nombre UNIQUE (nombre),

    CONSTRAINT chk_niveles_educativos_nombre
        CHECK (
            nombre IN ('INICIAL', 'PRIMARIA', 'SECUNDARIA')
        )
);


-- Grados pertenecientes a un nivel educativo.
CREATE TABLE IF NOT EXISTS grados (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    nivel_id UUID NOT NULL,
    numero INTEGER NOT NULL,
    nombre VARCHAR(50) NOT NULL,

    estado BOOLEAN NOT NULL DEFAULT TRUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_grados_nivel
        FOREIGN KEY (nivel_id)
        REFERENCES niveles_educativos(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT,

    CONSTRAINT uq_grados_nivel_numero
        UNIQUE (nivel_id, numero),

    CONSTRAINT chk_grados_numero
        CHECK (numero >= 1 AND numero <= 6)
);


-- Secciones de cada grado.
CREATE TABLE IF NOT EXISTS secciones (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    grado_id UUID NOT NULL,
    nombre VARCHAR(10) NOT NULL,

    estado BOOLEAN NOT NULL DEFAULT TRUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_secciones_grado
        FOREIGN KEY (grado_id)
        REFERENCES grados(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT,

    CONSTRAINT uq_secciones_grado_nombre
        UNIQUE (grado_id, nombre),

    CONSTRAINT chk_secciones_nombre
        CHECK (length(TRIM(BOTH FROM nombre)) > 0)
);


CREATE INDEX IF NOT EXISTS idx_grados_nivel_id
    ON grados (nivel_id);

CREATE INDEX IF NOT EXISTS idx_secciones_grado_id
    ON secciones (grado_id);

CREATE INDEX IF NOT EXISTS idx_periodos_lectivos_estado
    ON periodos_lectivos (estado);

COMMIT;

-- Down migration:
-- DROP TABLE IF EXISTS secciones;
-- DROP TABLE IF EXISTS grados;
-- DROP TABLE IF EXISTS niveles_educativos;
-- DROP TABLE IF EXISTS periodos_lectivos;