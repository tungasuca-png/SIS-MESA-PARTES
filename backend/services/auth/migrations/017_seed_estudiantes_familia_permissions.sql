BEGIN;

-- Estudiantes Service (modelo maestro, etapa 4): permisos del dominio
-- FAMILIAR (personas relacionadas, parentesco, apoderado). Propuestos en
-- E-02 (D-M5, permisos por dominio). Separados de estudiantes.view y de
-- estudiantes.personal.* por mínimo privilegio.
-- SOLO se registran en el catálogo. NO se asignan a ningún rol (misma
-- política que 012, 014, 015 y 016).
-- Segura de re-ejecutar (ON CONFLICT DO NOTHING); no modifica filas existentes.

INSERT INTO permissions (codigo, nombre, descripcion, modulo, estado) VALUES
    ('estudiantes.familia.view',   'Ver familiares de estudiantes',    'Consultar las personas relacionadas con un estudiante, su parentesco y apoderado (cada consulta se audita)', 'estudiantes', TRUE),
    ('estudiantes.familia.update', 'Editar familiares de estudiantes', 'Vincular o desvincular personas relacionadas con un estudiante',                                      'estudiantes', TRUE)
ON CONFLICT (codigo) DO NOTHING;

COMMIT;

-- Down migration:
-- DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE codigo IN ('estudiantes.familia.view', 'estudiantes.familia.update'));
-- DELETE FROM permissions WHERE codigo IN ('estudiantes.familia.view', 'estudiantes.familia.update');
