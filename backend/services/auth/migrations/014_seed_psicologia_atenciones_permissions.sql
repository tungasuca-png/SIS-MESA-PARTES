BEGIN;

-- Atenciones psicológicas (Hoja de Atención) dentro del expediente
-- psicológico. Permisos de ACCIÓN: el alcance (qué expedientes) sigue
-- saliendo de psicologia.expedientes.view / view_own — no hay "_own" por
-- atención.
--
-- IMPORTANTE: esta migración SOLO REGISTRA los permisos. NO inserta nada en
-- role_permissions (ni ADMIN ni PSICOLOGO los reciben aquí): la asignación
-- es una acción explícita de ADMIN desde Configuración -> Roles y Permisos.
--
-- Segura de re-ejecutar (ON CONFLICT DO NOTHING).

INSERT INTO permissions (codigo, nombre, descripcion, modulo, estado) VALUES
    ('psicologia.atenciones.view',    'Ver atenciones psicológicas',     'Consultar atenciones, sus versiones y descargar su PDF (dentro del alcance del expediente)', 'psicologia', TRUE),
    ('psicologia.atenciones.create',  'Registrar atenciones psicológicas', 'Crear, guardar y registrar la Hoja de Atención (borradores propios)',                    'psicologia', TRUE),
    ('psicologia.atenciones.correct', 'Corregir atenciones psicológicas', 'Corregir una atención registrada mediante una nueva versión',                            'psicologia', TRUE),
    ('psicologia.atenciones.annul',   'Anular atenciones psicológicas',   'Anular una atención registrada (sin borrarla)',                                        'psicologia', TRUE)
ON CONFLICT (codigo) DO NOTHING;

COMMIT;

-- Down migration:
-- DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE codigo LIKE 'psicologia.atenciones.%');
-- DELETE FROM permissions WHERE codigo LIKE 'psicologia.atenciones.%';
