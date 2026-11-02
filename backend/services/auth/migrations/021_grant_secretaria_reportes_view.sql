BEGIN;

-- Módulo de Reportes (Fase 1): SECRETARIA opera la Mesa de Partes y necesita
-- los reportes de expedientes. Decisión funcional aprobada: acceso para
-- ADMIN, DIRECTOR, SUBDIRECTOR (ya asignados en la migración 005) y
-- SECRETARIA. Ningún otro rol recibe reportes.view.
--
-- El permiso ya existe en el catálogo (migración 005): solo se agrega la
-- asignación. Idempotente.

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM (VALUES
    ('SECRETARIA', 'reportes.view')
) AS seed(rol_nombre, permiso_codigo)
JOIN roles r ON r.nombre = seed.rol_nombre
JOIN permissions p ON p.codigo = seed.permiso_codigo
ON CONFLICT (role_id, permission_id) DO NOTHING;

COMMIT;

-- Reversión (manual, solo si se decide retirar el acceso):
-- DELETE FROM role_permissions
-- WHERE role_id = (SELECT id FROM roles WHERE nombre = 'SECRETARIA')
--   AND permission_id = (SELECT id FROM permissions WHERE codigo = 'reportes.view');
