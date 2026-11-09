-- Vuelta atrás a los binarios anteriores (f04b9b3) en derivaciones_db.
-- Ejecutar como derivaciones_migrator tras SET ROLE derivaciones_owner.
-- Vuelve a permitir el INSERT de tipo DERIVACION (lo usa el flujo anterior);
-- UPDATE, DELETE y TRUNCATE siguen bloqueados. No toca filas ni la proyección.
-- Al volver a desplegar, reaplicar 002 (restablece la función completa).
BEGIN;
CREATE OR REPLACE FUNCTION derivaciones_proteger() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'derivaciones es append-only: % no permitido', TG_OP USING ERRCODE = '42501';
END
$$;
COMMIT;
