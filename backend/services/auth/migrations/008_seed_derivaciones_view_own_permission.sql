BEGIN;

-- Nuevo permiso de alcance propio para consultar derivaciones:
--   derivaciones.view      -> alcance general (sujeto al alcance de datos del expediente)
--   derivaciones.view_own  -> solo derivaciones de expedientes propios
-- Segura de re-ejecutar: ON CONFLICT evita duplicados; no borra ni actualiza
-- ninguna fila existente y NO lo asigna a ningún rol (ni siquiera a
-- SOLICITANTE): ADMIN decide qué rol lo recibe desde Configuración ->
-- Roles y permisos.

INSERT INTO permissions (codigo, nombre, descripcion, modulo, estado) VALUES
    ('derivaciones.view_own', 'Ver mis derivaciones', 'Consultar únicamente las derivaciones de expedientes propios', 'derivaciones', TRUE)
ON CONFLICT (codigo) DO NOTHING;

COMMIT;

-- Down migration:
-- DELETE FROM role_permissions WHERE permission_id = (SELECT id FROM permissions WHERE codigo = 'derivaciones.view_own');
-- DELETE FROM permissions WHERE codigo = 'derivaciones.view_own';
