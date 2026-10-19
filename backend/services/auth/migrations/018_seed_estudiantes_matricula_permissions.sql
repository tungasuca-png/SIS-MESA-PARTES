BEGIN;

-- Estudiantes Service (modelo maestro, etapa 5): permiso del dominio
-- MATRÍCULA (periodos, secciones, matrículas, cambios de sección, retiros,
-- traslados, reingresos y conclusión). La lectura sigue con estudiantes.view.
-- SOLO se registra en el catálogo. NO se asigna a ningún rol (misma política
-- que 012 y 014–017): la asignación se hace desde la pantalla de Roles.
-- Segura de re-ejecutar (ON CONFLICT DO NOTHING); no modifica filas existentes.

INSERT INTO permissions (codigo, nombre, descripcion, modulo, estado) VALUES
    ('estudiantes.matricula.update', 'Gestionar matrículas', 'Registrar periodos, secciones y matrículas, y cambiar sección, retirar, trasladar, reingresar o concluir una matrícula', 'estudiantes', TRUE)
ON CONFLICT (codigo) DO NOTHING;

COMMIT;

-- Down migration:
-- DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE codigo = 'estudiantes.matricula.update');
-- DELETE FROM permissions WHERE codigo = 'estudiantes.matricula.update';
