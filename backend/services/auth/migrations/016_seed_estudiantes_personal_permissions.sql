BEGIN;

-- Estudiantes Service (FASE E-08): permisos del dominio de DATOS PERSONALES
-- del estudiante (fecha de nacimiento, sexo, país y ubigeo de nacimiento,
-- lenguas). Separados de estudiantes.view/.update por mínimo privilegio.
-- SOLO se registran en el catálogo. NO se asignan a ningún rol (misma
-- política que 012, 014 y 015): la asignación la decide ADMIN desde
-- Configuración -> Roles y permisos.
-- Segura de re-ejecutar (ON CONFLICT DO NOTHING); no modifica filas existentes.

INSERT INTO permissions (codigo, nombre, descripcion, modulo, estado) VALUES
    ('estudiantes.personal.view',   'Ver datos personales de estudiantes',    'Consultar fecha de nacimiento, sexo, lugar de nacimiento y lenguas (cada consulta se audita)', 'estudiantes', TRUE),
    ('estudiantes.personal.update', 'Editar datos personales de estudiantes', 'Registrar o modificar fecha de nacimiento, sexo, lugar de nacimiento y lenguas',             'estudiantes', TRUE)
ON CONFLICT (codigo) DO NOTHING;

COMMIT;

-- Down migration:
-- DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE codigo IN ('estudiantes.personal.view', 'estudiantes.personal.update'));
-- DELETE FROM permissions WHERE codigo IN ('estudiantes.personal.view', 'estudiantes.personal.update');
