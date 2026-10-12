BEGIN;

-- FASE 5E (auditoría y saneamiento del catálogo de roles y permisos):
-- restaura una única fila de role_permissions que desapareció por error
-- durante las pruebas E2E de FASE 5D.
--
-- EVIDENCIA (ver informe completo de FASE 5E):
--   1. migrations/005_seed_permissions_and_role_permissions.sql, línea 124,
--      declara explícitamente ('SOLICITANTE', 'dashboard.view') como parte
--      del catálogo original — nunca hubo una migración posterior que la
--      quitara intencionalmente.
--   2. El estado real de PostgreSQL (verificado en esta fase) muestra que
--      SOLICITANTE tiene hoy exactamente 5 permisos —
--      documentos.create, documentos.view_own, expedientes.view_own,
--      seguimiento.view_own, solicitudes.create — y NO dashboard.view.
--   3. Causa identificada con precisión: durante FASE 5D, un script de
--      prueba temporal (fase5d_setup.go) otorgó 6 permisos a SOLICITANTE
--      para simular un usuario autorizado en el selector de estudiantes de
--      Psicología; uno de esos 6 (dashboard.view) YA estaba asignado desde
--      la migración 005, así que ese INSERT fue un no-op
--      (ON CONFLICT DO NOTHING). Al reasignar ese usuario de prueba al rol
--      PSICOLOGO, el script de limpieza (fase5d_fixrole.go) hizo DELETE de
--      los mismos 6 permisos sobre SOLICITANTE de forma indiscriminada, sin
--      distinguir cuáles había creado él mismo de cuál ya existía desde
--      antes — y removió por error la fila legítima. Los otros 5 permisos
--      de esa prueba sí eran nuevos para SOLICITANTE y su limpieza fue
--      correcta (no requieren restauración).
--
-- Clasificación: eliminación ACCIDENTAL (Caso 1 de FASE 5E — "falta una
-- relación que claramente debe existir"), no una decisión de producto.
--
-- Segura de re-ejecutar (ON CONFLICT DO NOTHING). No crea ningún permiso
-- nuevo, no toca ningún otro rol ni relación, no modifica ninguna migración
-- anterior.

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM (VALUES
    ('SOLICITANTE', 'dashboard.view')
) AS seed(rol_nombre, permiso_codigo)
JOIN roles r ON r.nombre = seed.rol_nombre
JOIN permissions p ON p.codigo = seed.permiso_codigo
ON CONFLICT (role_id, permission_id) DO NOTHING;

COMMIT;
