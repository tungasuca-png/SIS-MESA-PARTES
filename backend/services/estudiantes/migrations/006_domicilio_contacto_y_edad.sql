BEGIN;

-- Estudiantes Service — Modelo maestro, etapa 2: DOMICILIO y CONTACTO.
--
-- APLICAR con estudiantes_migrator actuando como dueño:
--   (echo "SET ROLE estudiantes_owner;"; cat 006_domicilio_contacto_y_edad.sql) | \
--     psql -U estudiantes_migrator -d estudiantes_db -v ON_ERROR_STOP=1
-- Aditiva e idempotente: solo agrega columnas NULL y restricciones sobre
-- ellas; ninguna fila existente cambia (todas quedan en NULL).
--
-- Decisión (E-02 §6, aprobada): domicilio y teléfono pertenecen al dominio
-- PERSONAL. Se reutilizan su endpoint (GetEstudiantePersonal /
-- UpdateEstudiantePersonal), sus permisos (estudiantes.personal.view/.update)
-- y su auditoría. Un solo domicilio VIGENTE por estudiante: no se identificó
-- una necesidad de historial de domicilios (se puede agregar después sin
-- romper nada). Nunca se exponen en listados ni en GetEstudiante.
--
-- Una sola fuente de verdad: estas columnas son el único lugar donde se
-- guarda el domicilio y el contacto del estudiante.
--   - departamento/provincia/distrito: texto tal como viene de la fuente
--     institucional (no hay todavía catálogo territorial oficial).
--   - domicilio_ubigeo: opcional; NO se deduce de los nombres ni se inventa.
--     Cuando exista el catálogo de ubigeos, se validará la coherencia.
--   - correo/telefono: canal de contacto registrado para el estudiante (en la
--     práctica puede ser el de su familia). Se guardan como se reciben
--     (recortados); no se reescriben para no cambiar su significado.
ALTER TABLE estudiantes
    ADD COLUMN IF NOT EXISTS domicilio_direccion VARCHAR(200) NULL,
    ADD COLUMN IF NOT EXISTS domicilio_departamento VARCHAR(60) NULL,
    ADD COLUMN IF NOT EXISTS domicilio_provincia VARCHAR(60) NULL,
    ADD COLUMN IF NOT EXISTS domicilio_distrito VARCHAR(60) NULL,
    ADD COLUMN IF NOT EXISTS domicilio_ubigeo CHAR(6) NULL,
    ADD COLUMN IF NOT EXISTS correo VARCHAR(254) NULL,
    ADD COLUMN IF NOT EXISTS telefono VARCHAR(20) NULL;

-- Ausencia = NULL (nunca cadena vacía).
ALTER TABLE estudiantes DROP CONSTRAINT IF EXISTS chk_estudiantes_domicilio;
ALTER TABLE estudiantes ADD CONSTRAINT chk_estudiantes_domicilio CHECK (
    (domicilio_direccion IS NULL OR length(TRIM(BOTH FROM domicilio_direccion)) > 0)
    AND (domicilio_departamento IS NULL OR length(TRIM(BOTH FROM domicilio_departamento)) > 0)
    AND (domicilio_provincia IS NULL OR length(TRIM(BOTH FROM domicilio_provincia)) > 0)
    AND (domicilio_distrito IS NULL OR length(TRIM(BOTH FROM domicilio_distrito)) > 0)
    AND (domicilio_ubigeo IS NULL OR domicilio_ubigeo ~ '^[0-9]{6}$')
);

ALTER TABLE estudiantes DROP CONSTRAINT IF EXISTS chk_estudiantes_contacto;
ALTER TABLE estudiantes ADD CONSTRAINT chk_estudiantes_contacto CHECK (
    (correo IS NULL OR correo ~ '^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$')
    AND (telefono IS NULL OR telefono ~ '^\+?[0-9][0-9 -]{5,18}$')
);

-- Auditoría: los nuevos nombres de campo pasan a la lista blanca (sigue
-- sin admitir valores).
ALTER TABLE auditoria_estudiantes DROP CONSTRAINT IF EXISTS chk_auditoria_estudiantes_campos;
ALTER TABLE auditoria_estudiantes ADD CONSTRAINT chk_auditoria_estudiantes_campos CHECK (campos <@ ARRAY[
    'codigo', 'codigo_siagie', 'tipo_documento', 'numero_documento', 'nombres',
    'apellido_paterno', 'apellido_materno', 'apellidos', 'fecha_nacimiento', 'sexo',
    'pais_nacimiento', 'ubigeo_nacimiento', 'lengua_materna', 'segunda_lengua',
    'foto_ref', 'estado',
    'domicilio_direccion', 'domicilio_departamento', 'domicilio_provincia',
    'domicilio_distrito', 'domicilio_ubigeo', 'correo', 'telefono'
]::TEXT[]);

-- EDAD: nunca se almacena. Se calcula a una fecha de referencia EXPLÍCITA
-- (p. ej. la fecha de corte de un reporte histórico). NULL si falta la fecha
-- de nacimiento o si la referencia es anterior al nacimiento.
CREATE OR REPLACE FUNCTION estudiantes_edad(nacimiento date, referencia date) RETURNS integer
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
AS $$
    SELECT CASE WHEN referencia < nacimiento THEN NULL
                ELSE date_part('year', age(referencia, nacimiento))::integer END
$$;

COMMIT;

-- Down migration (solo desarrollo):
-- ALTER TABLE estudiantes DROP CONSTRAINT chk_estudiantes_domicilio, DROP CONSTRAINT chk_estudiantes_contacto,
--   DROP COLUMN domicilio_direccion, DROP COLUMN domicilio_departamento, DROP COLUMN domicilio_provincia,
--   DROP COLUMN domicilio_distrito, DROP COLUMN domicilio_ubigeo, DROP COLUMN correo, DROP COLUMN telefono;
-- DROP FUNCTION estudiantes_edad(date, date);
-- (restaurar chk_auditoria_estudiantes_campos de 004)
