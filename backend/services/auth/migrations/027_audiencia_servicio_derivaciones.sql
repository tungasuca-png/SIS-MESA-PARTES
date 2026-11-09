-- 027 — Audiencia por cliente de servicio y permiso exclusivo del publicador.
--
-- Ejecutar como auth_migrator tras SET ROLE auth_owner. Idempotente.
-- auth_app ya tiene SELECT/INSERT/UPDATE a nivel de tabla sobre
-- service_clients (verificado en pg_class.relacl), así que la columna nueva
-- queda cubierta sin otro GRANT; su inmutabilidad la asegura un trigger.
-- Los clientes existentes reciben 'estudiantes', que es la audiencia con la
-- que ya operan (único valor aceptado hasta hoy).

BEGIN;

ALTER TABLE service_clients ADD COLUMN IF NOT EXISTS audiencia VARCHAR(30) NOT NULL DEFAULT 'estudiantes';
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                    WHERE conrelid = 'public.service_clients'::regclass
                      AND conname  = 'chk_service_clients_audiencia') THEN
        ALTER TABLE service_clients ADD CONSTRAINT chk_service_clients_audiencia
            CHECK (audiencia IN ('estudiantes', 'derivaciones'));
    END IF;
END
$$;

CREATE OR REPLACE FUNCTION service_clients_audiencia_inmutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.audiencia IS DISTINCT FROM OLD.audiencia THEN
        RAISE EXCEPTION 'la audiencia de un cliente de servicio no se puede cambiar' USING ERRCODE = '42501';
    END IF;
    RETURN NEW;
END
$$;
DROP TRIGGER IF EXISTS trg_service_clients_audiencia ON service_clients;
CREATE TRIGGER trg_service_clients_audiencia BEFORE UPDATE OF audiencia ON service_clients
    FOR EACH ROW EXECUTE FUNCTION service_clients_audiencia_inmutable();

INSERT INTO permissions (codigo, nombre, descripcion, modulo, estado)
VALUES ('derivaciones.proyectar', 'Proyectar movimientos',
        'Exclusivo del publicador de Expedientes mediante token de servicio. Nunca para roles humanos.',
        'derivaciones', TRUE)
ON CONFLICT (codigo) DO NOTHING;

-- Ningún rol humano puede recibir el permiso del publicador.
CREATE OR REPLACE FUNCTION role_permissions_sin_permisos_de_servicio() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM permissions p
                WHERE p.id = NEW.permission_id AND p.codigo = 'derivaciones.proyectar') THEN
        RAISE EXCEPTION 'derivaciones.proyectar no se asigna a roles humanos' USING ERRCODE = '42501';
    END IF;
    RETURN NEW;
END
$$;
DROP TRIGGER IF EXISTS trg_role_permissions_servicio ON role_permissions;
CREATE TRIGGER trg_role_permissions_servicio BEFORE INSERT OR UPDATE ON role_permissions
    FOR EACH ROW EXECUTE FUNCTION role_permissions_sin_permisos_de_servicio();

COMMIT;
