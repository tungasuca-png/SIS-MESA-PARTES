BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Estudiantes Service (FASE 5A): fuente básica de estudiantes. NO contiene
-- historial académico, notas, asistencia, apoderados ni datos clínicos —
-- eso pertenece a otros servicios (p. ej. Psicología, que solo guarda
-- estudiante_id como UUID, sin FK hacia esta base).
--
-- id se autogenera aquí (a diferencia de usuarios_db, donde el id lo asigna
-- Auth Service): el estudiante no tiene cuenta en Auth, así que Estudiantes
-- es dueño de su propia identidad.
--
-- Esta fase NO implementa ningún RPC de alta: la tabla queda vacía hasta que
-- una fase posterior agregue esa capacidad (o una importación institucional).
CREATE TABLE IF NOT EXISTS estudiantes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    codigo VARCHAR(20) NOT NULL,
    nombres VARCHAR(100) NOT NULL,
    apellidos VARCHAR(100) NOT NULL,
    -- Documento e datos de nacimiento: se guardan (útiles para una futura
    -- ficha administrativa) pero NINGÚN RPC de esta fase los expone (ver
    -- proto: Estudiante/EstudianteBasico no los incluyen) — minimiza lo que
    -- sale del servicio, no lo que guarda.
    tipo_documento VARCHAR(20) NOT NULL DEFAULT '',
    numero_documento VARCHAR(30) NOT NULL DEFAULT '',
    fecha_nacimiento DATE,
    nivel VARCHAR(20) NOT NULL DEFAULT '',
    grado VARCHAR(20) NOT NULL DEFAULT '',
    seccion VARCHAR(10) NOT NULL DEFAULT '',
    turno VARCHAR(20) NOT NULL DEFAULT '',
    estado_matricula VARCHAR(20) NOT NULL DEFAULT 'ACTIVO',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_estudiantes_codigo CHECK (length(TRIM(BOTH FROM codigo)) > 0),
    CONSTRAINT chk_estudiantes_nombres CHECK (length(TRIM(BOTH FROM nombres)) > 0),
    CONSTRAINT chk_estudiantes_apellidos CHECK (length(TRIM(BOTH FROM apellidos)) > 0),
    -- Solo ACTIVO/RETIRADO (mínimo pedido para esta fase): no se inventan
    -- estados académicos adicionales sin una regla institucional confirmada.
    CONSTRAINT chk_estudiantes_estado_matricula CHECK (estado_matricula IN ('ACTIVO', 'RETIRADO'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_estudiantes_codigo
    ON estudiantes (codigo);

CREATE INDEX IF NOT EXISTS idx_estudiantes_estado_matricula
    ON estudiantes (estado_matricula);

CREATE INDEX IF NOT EXISTS idx_estudiantes_nivel_grado_seccion
    ON estudiantes (nivel, grado, seccion);

CREATE INDEX IF NOT EXISTS idx_estudiantes_apellidos
    ON estudiantes (apellidos);

-- Privilegios mínimos para la aplicación: sin DELETE (no hay baja física
-- prevista; RETIRADO es la baja lógica) mientras no exista un RPC de
-- escritura. Se agrega igual UPDATE/INSERT para que la fase que implemente
-- el alta no necesite otra migración de permisos.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'estudiantes_user') THEN
        GRANT SELECT, INSERT, UPDATE ON estudiantes TO estudiantes_user;
    END IF;
END
$$;

COMMIT;

-- Down migration (solo entornos de desarrollo):
-- DROP TABLE IF EXISTS estudiantes;
