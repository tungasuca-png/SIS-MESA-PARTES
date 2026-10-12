BEGIN;

-- Estudiantes Service (FASE 5B): registra el permiso de consulta.
-- SOLO se registra el código en el catálogo. NO se asigna a ningún rol
-- (ni PSICOLOGO ni ADMIN): la asignación es una decisión de negocio que se
-- toma desde Administración -> Roles y Permisos, no en una migración.
-- Segura de re-ejecutar (ON CONFLICT DO NOTHING); no modifica filas existentes.

INSERT INTO permissions (codigo, nombre, descripcion, modulo, estado) VALUES
    ('estudiantes.view', 'Ver estudiantes', 'Consultar estudiantes para operaciones autorizadas', 'estudiantes', TRUE)
ON CONFLICT (codigo) DO NOTHING;

COMMIT;

-- Down migration:
-- DELETE FROM role_permissions WHERE permission_id = (SELECT id FROM permissions WHERE codigo = 'estudiantes.view');
-- DELETE FROM permissions WHERE codigo = 'estudiantes.view';
