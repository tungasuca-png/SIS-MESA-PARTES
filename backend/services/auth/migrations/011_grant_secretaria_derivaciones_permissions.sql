BEGIN;

-- SECRETARIA ya debía tener estos permisos desde la migración 005,
-- pero las filas no existen actualmente en la BD.
-- Esta migración sincroniza únicamente esas dos asignaciones.

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM (VALUES
    ('SECRETARIA', 'derivaciones.view'),
    ('SECRETARIA', 'derivaciones.create')
) AS seed(rol_nombre, permiso_codigo)
JOIN roles r ON r.nombre = seed.rol_nombre
JOIN permissions p ON p.codigo = seed.permiso_codigo
ON CONFLICT (role_id, permission_id) DO NOTHING;

COMMIT;
