BEGIN;

-- Estudiantes Service — FASE E-04: búsqueda normalizada (sin tildes, sin
-- distinguir mayúsculas, en cualquier orden de nombres/apellidos) con índice
-- de trigramas. Decisión D-M7 (E-02).
--
-- APLICAR con estudiantes_migrator actuando como dueño:
--   (echo "SET ROLE estudiantes_owner;"; cat 003_busqueda_normalizada.sql) | \
--     psql -U estudiantes_migrator -d estudiantes_db -v ON_ERROR_STOP=1
-- pg_trgm y unaccent son extensiones "trusted": el dueño de la base puede
-- instalarlas sin superusuario. No destructiva e idempotente.
--
-- Nota de numeración: la migración de Matrícula prevista en E-02 como 003
-- pasa a ser 004 (y las siguientes se desplazan una posición).

CREATE EXTENSION IF NOT EXISTS unaccent;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Normalización única para el texto guardado y para la consulta: sin tildes
-- (unaccent con diccionario explícito, requisito para declararla IMMUTABLE),
-- en minúsculas y con espacios colapsados. Así "José  PÉREZ" y "jose perez"
-- producen lo mismo.
CREATE OR REPLACE FUNCTION estudiantes_normalizar(texto text) RETURNS text
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
AS $$
    SELECT btrim(regexp_replace(lower(public.unaccent('public.unaccent'::regdictionary, texto)), '[[:space:]]+', ' ', 'g'))
$$;

-- Texto de búsqueda mantenido por la base (columna generada): identificadores
-- y todas las variantes del nombre. La aplicación nunca lo escribe; el
-- listado no lo devuelve.
ALTER TABLE estudiantes ADD COLUMN IF NOT EXISTS texto_busqueda TEXT
    GENERATED ALWAYS AS (estudiantes_normalizar(
        codigo || ' ' || COALESCE(codigo_siagie, '') || ' ' || COALESCE(numero_documento, '') || ' ' ||
        nombres || ' ' || apellidos || ' ' ||
        COALESCE(apellido_paterno, '') || ' ' || COALESCE(apellido_materno, '')
    )) STORED;

-- Índice GIN de trigramas: permite que cada condición
--   texto_busqueda LIKE '%' || estudiantes_normalizar($n) || '%'
-- (una por palabra buscada, combinadas con AND) use el índice en lugar de
-- recorrer toda la tabla. Útil para términos de 3 o más caracteres.
CREATE INDEX IF NOT EXISTS idx_estudiantes_texto_busqueda_trgm
    ON estudiantes USING gin (texto_busqueda gin_trgm_ops);

-- Filtros: nivel/grado/seccion ya tienen idx_estudiantes_nivel_grado_seccion
-- (001). No se indexa "estado" (solo ACTIVO/INACTIVO: un índice de tan baja
-- selectividad casi nunca lo usaría el planificador).

COMMIT;

-- Down (solo desarrollo): DROP INDEX idx_estudiantes_texto_busqueda_trgm;
-- ALTER TABLE estudiantes DROP COLUMN
-- texto_busqueda; DROP FUNCTION estudiantes_normalizar(text); las extensiones
-- pueden quedar instaladas.
