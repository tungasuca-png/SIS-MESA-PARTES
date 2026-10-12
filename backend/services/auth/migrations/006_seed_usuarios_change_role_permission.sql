BEGIN;

-- Nuevo permiso para la operación administrativa "cambiar el rol de un
-- usuario existente" (Usuario -> Rol), separado a propósito de
-- "roles.view" (que protege Rol -> Permisos, un dominio distinto).
-- Segura de re-ejecutar: ON CONFLICT evita duplicados; no borra ni
-- actualiza ninguna fila existente, no toca ningún otro permiso ni rol.

INSERT INTO permissions (codigo, nombre, descripcion, modulo, estado) VALUES
    ('usuarios.change_role', 'Cambiar rol de usuario', 'Reemplazar el único rol asignado a un usuario existente', 'usuarios', TRUE)
ON CONFLICT (codigo) DO NOTHING;

-- Solo ADMIN lo tiene inicialmente — no se asigna a ningún otro rol.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM (VALUES
    ('ADMIN', 'usuarios.change_role')
) AS seed(rol_nombre, permiso_codigo)
JOIN roles r ON r.nombre = seed.rol_nombre
JOIN permissions p ON p.codigo = seed.permiso_codigo
ON CONFLICT (role_id, permission_id) DO NOTHING;

COMMIT;

-- Down migration:
-- DELETE FROM role_permissions WHERE permission_id = (SELECT id FROM permissions WHERE codigo = 'usuarios.change_role');
-- DELETE FROM permissions WHERE codigo = 'usuarios.change_role';
