BEGIN;

-- Módulo de Psicología: rol de sistema PSICOLOGO y catálogo de permisos.
--
-- IMPORTANTE: esta migración SOLO REGISTRA el rol y los permisos. NO inserta
-- nada en role_permissions: ni ADMIN ni PSICOLOGO reciben permisos aquí. La
-- asignación la hace ADMIN desde Administración -> Roles y Permisos. El rol
-- solo agrupa usuarios; ninguna autorización depende de su nombre.
--
-- Segura de re-ejecutar (ON CONFLICT DO NOTHING); no modifica filas existentes.

INSERT INTO roles (nombre, descripcion, estado) VALUES
    ('PSICOLOGO', 'Profesional de Psicología', TRUE)
ON CONFLICT (nombre) DO NOTHING;

INSERT INTO permissions (codigo, nombre, descripcion, modulo, estado) VALUES
    ('psicologia.view',                'Acceder a Psicología',            'Entrar al módulo de Psicología',                                        'psicologia', TRUE),
    ('psicologia.estudiantes.view',    'Ver estudiantes (Psicología)',    'Buscar estudiantes y ver su ficha desde Psicología',                   'psicologia', TRUE),
    ('psicologia.expedientes.view',    'Ver expedientes psicológicos',    'Consultar todos los expedientes psicológicos (alcance general)',       'psicologia', TRUE),
    ('psicologia.expedientes.view_own','Ver mis expedientes psicológicos','Consultar solo los expedientes psicológicos donde es responsable',      'psicologia', TRUE),
    ('psicologia.expedientes.create',  'Abrir expediente psicológico',    'Abrir el expediente psicológico de un estudiante',                     'psicologia', TRUE),
    ('psicologia.expedientes.update',  'Modificar expediente psicológico','Modificar, cerrar y reabrir expedientes psicológicos dentro de su alcance', 'psicologia', TRUE)
ON CONFLICT (codigo) DO NOTHING;

COMMIT;

-- Down migration:
-- DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE modulo = 'psicologia');
-- DELETE FROM permissions WHERE modulo = 'psicologia';
-- DELETE FROM roles WHERE nombre = 'PSICOLOGO';
