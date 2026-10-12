BEGIN;

-- Estudiantes Service (FASE E-05): permisos de ESCRITURA del maestro.
-- SOLO se registran en el catálogo. NO se asignan a ningún rol (ni ADMIN):
-- misma política que 012 y 014 — la asignación es una decisión de negocio que
-- ADMIN toma desde Configuración -> Roles y permisos, no una migración.
-- No existe estudiantes.delete: el maestro no tiene borrado físico por API.
-- Segura de re-ejecutar (ON CONFLICT DO NOTHING); no modifica filas existentes.

INSERT INTO permissions (codigo, nombre, descripcion, modulo, estado) VALUES
    ('estudiantes.create',        'Registrar estudiantes',        'Registrar estudiantes en el maestro institucional',                  'estudiantes', TRUE),
    ('estudiantes.update',        'Editar estudiantes',           'Editar la identidad y los datos básicos del maestro de estudiantes', 'estudiantes', TRUE),
    ('estudiantes.change_estado', 'Cambiar estado de estudiantes', 'Activar o inactivar el registro de un estudiante (sin borrarlo)',   'estudiantes', TRUE)
ON CONFLICT (codigo) DO NOTHING;

COMMIT;

-- Down migration:
-- DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE codigo IN ('estudiantes.create', 'estudiantes.update', 'estudiantes.change_estado'));
-- DELETE FROM permissions WHERE codigo IN ('estudiantes.create', 'estudiantes.update', 'estudiantes.change_estado');
