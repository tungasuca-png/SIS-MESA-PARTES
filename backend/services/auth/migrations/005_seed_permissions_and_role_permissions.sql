BEGIN;

-- Siembra los permisos reales que hoy solo existen como mock en
-- frontend/src/permissions/permissions.js (ROLE_PERMISSIONS), para que
-- AuthorizationService/HasPermission (ya implementado y probado, ver
-- internal/authorization/authorization.go) tenga datos reales contra los
-- que operar. No se inventa ningún código nuevo: son exactamente los 20
-- códigos que ya usa ese mock. No se modifica ninguna tabla ni lógica
-- existente — solo se insertan filas.
--
-- Segura de re-ejecutar: ON CONFLICT (codigo) / ON CONFLICT (role_id,
-- permission_id) evitan duplicados. No borra ni actualiza filas existentes.

INSERT INTO permissions (codigo, nombre, descripcion, modulo, estado) VALUES
    ('dashboard.view',            'Ver dashboard',                    'Acceso a la pantalla principal del panel administrativo', 'dashboard',     TRUE),
    ('expedientes.view',          'Ver todos los expedientes',        'Listar y ver el detalle de cualquier expediente',         'expedientes',   TRUE),
    ('expedientes.view_own',      'Ver expedientes propios',          'Listar y ver solo los expedientes propios del solicitante', 'expedientes', TRUE),
    ('expedientes.create',       'Crear expediente',                 'Registrar un nuevo expediente',                           'expedientes',   TRUE),
    ('expedientes.update',        'Editar expediente',                'Editar asunto, descripción o prioridad de un expediente', 'expedientes',  TRUE),
    ('expedientes.change_estado', 'Cambiar estado del expediente',    'Mover un expediente entre estados',                       'expedientes',   TRUE),
    ('expedientes.delete',        'Eliminar expediente',              'Dar de baja (lógica) un expediente',                      'expedientes',   TRUE),
    ('documentos.view',           'Ver todos los documentos',         'Listar y ver documentos de cualquier expediente',         'documentos',    TRUE),
    ('documentos.view_own',       'Ver documentos propios',           'Listar y ver solo los documentos propios del solicitante', 'documentos',   TRUE),
    ('documentos.create',         'Subir documento',                  'Adjuntar un documento a un expediente',                   'documentos',    TRUE),
    ('documentos.delete',         'Eliminar documento',                'Dar de baja (lógica) un documento',                       'documentos',    TRUE),
    ('derivaciones.view',         'Ver derivaciones',                  'Listar el historial de derivaciones',                     'derivaciones',  TRUE),
    ('derivaciones.create',       'Registrar derivación',              'Registrar una nueva derivación',                          'derivaciones',  TRUE),
    ('seguimiento.view',          'Ver seguimiento',                   'Ver el seguimiento de cualquier expediente',              'seguimiento',   TRUE),
    ('seguimiento.view_own',      'Ver seguimiento propio',            'Ver el seguimiento de los expedientes propios',           'seguimiento',   TRUE),
    ('reportes.view',             'Ver reportes',                      'Acceso a reportes',                                       'reportes',      TRUE),
    ('solicitudes.create',        'Crear solicitud',                   'Registrar una nueva solicitud (Mesa de Partes Virtual)',  'solicitudes',   TRUE),
    ('usuarios.view',             'Ver usuarios',                      'Acceso al listado de usuarios',                           'usuarios',      TRUE),
    ('roles.view',                'Ver roles y permisos',              'Acceso a la administración de roles y permisos',          'roles',         TRUE),
    ('configuracion.view',        'Ver configuración',                 'Acceso a la configuración del sistema',                   'configuracion', TRUE)
ON CONFLICT (codigo) DO NOTHING;

-- Relaciones rol -> permiso, tal como las define hoy ROLE_PERMISSIONS del
-- frontend (mismo mock, sin cambiar ninguna regla). role_id/permission_id
-- se resuelven por nombre/codigo, nunca por UUID fijo.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM (VALUES
    -- ADMIN
    ('ADMIN', 'dashboard.view'),
    ('ADMIN', 'expedientes.view'),
    ('ADMIN', 'expedientes.create'),
    ('ADMIN', 'expedientes.update'),
    ('ADMIN', 'expedientes.change_estado'),
    ('ADMIN', 'expedientes.delete'),
    ('ADMIN', 'documentos.view'),
    ('ADMIN', 'documentos.create'),
    ('ADMIN', 'documentos.delete'),
    ('ADMIN', 'derivaciones.view'),
    ('ADMIN', 'derivaciones.create'),
    ('ADMIN', 'seguimiento.view'),
    ('ADMIN', 'reportes.view'),
    ('ADMIN', 'usuarios.view'),
    ('ADMIN', 'roles.view'),
    ('ADMIN', 'configuracion.view'),
    -- DIRECTOR
    ('DIRECTOR', 'dashboard.view'),
    ('DIRECTOR', 'expedientes.view'),
    ('DIRECTOR', 'expedientes.update'),
    ('DIRECTOR', 'expedientes.change_estado'),
    ('DIRECTOR', 'expedientes.delete'),
    ('DIRECTOR', 'documentos.view'),
    ('DIRECTOR', 'documentos.create'),
    ('DIRECTOR', 'documentos.delete'),
    ('DIRECTOR', 'derivaciones.view'),
    ('DIRECTOR', 'derivaciones.create'),
    ('DIRECTOR', 'seguimiento.view'),
    ('DIRECTOR', 'reportes.view'),
    -- SUBDIRECTOR (idéntico a DIRECTOR en el mock actual)
    ('SUBDIRECTOR', 'dashboard.view'),
    ('SUBDIRECTOR', 'expedientes.view'),
    ('SUBDIRECTOR', 'expedientes.update'),
    ('SUBDIRECTOR', 'expedientes.change_estado'),
    ('SUBDIRECTOR', 'expedientes.delete'),
    ('SUBDIRECTOR', 'documentos.view'),
    ('SUBDIRECTOR', 'documentos.create'),
    ('SUBDIRECTOR', 'documentos.delete'),
    ('SUBDIRECTOR', 'derivaciones.view'),
    ('SUBDIRECTOR', 'derivaciones.create'),
    ('SUBDIRECTOR', 'seguimiento.view'),
    ('SUBDIRECTOR', 'reportes.view'),
    -- SECRETARIA (sin reportes.view, con expedientes.create)
    ('SECRETARIA', 'dashboard.view'),
    ('SECRETARIA', 'expedientes.view'),
    ('SECRETARIA', 'expedientes.create'),
    ('SECRETARIA', 'expedientes.update'),
    ('SECRETARIA', 'expedientes.change_estado'),
    ('SECRETARIA', 'expedientes.delete'),
    ('SECRETARIA', 'documentos.view'),
    ('SECRETARIA', 'documentos.create'),
    ('SECRETARIA', 'documentos.delete'),
    ('SECRETARIA', 'derivaciones.view'),
    ('SECRETARIA', 'derivaciones.create'),
    ('SECRETARIA', 'seguimiento.view'),
    -- DOCENTE (sin reportes.view, sin expedientes.create)
    ('DOCENTE', 'dashboard.view'),
    ('DOCENTE', 'expedientes.view'),
    ('DOCENTE', 'expedientes.update'),
    ('DOCENTE', 'expedientes.change_estado'),
    ('DOCENTE', 'expedientes.delete'),
    ('DOCENTE', 'documentos.view'),
    ('DOCENTE', 'documentos.create'),
    ('DOCENTE', 'documentos.delete'),
    ('DOCENTE', 'derivaciones.view'),
    ('DOCENTE', 'derivaciones.create'),
    ('DOCENTE', 'seguimiento.view'),
    -- AUXILIAR (idéntico a DOCENTE en el mock actual)
    ('AUXILIAR', 'dashboard.view'),
    ('AUXILIAR', 'expedientes.view'),
    ('AUXILIAR', 'expedientes.update'),
    ('AUXILIAR', 'expedientes.change_estado'),
    ('AUXILIAR', 'expedientes.delete'),
    ('AUXILIAR', 'documentos.view'),
    ('AUXILIAR', 'documentos.create'),
    ('AUXILIAR', 'documentos.delete'),
    ('AUXILIAR', 'derivaciones.view'),
    ('AUXILIAR', 'derivaciones.create'),
    ('AUXILIAR', 'seguimiento.view'),
    -- SOLICITANTE
    ('SOLICITANTE', 'dashboard.view'),
    ('SOLICITANTE', 'solicitudes.create'),
    ('SOLICITANTE', 'expedientes.view_own'),
    ('SOLICITANTE', 'documentos.view_own'),
    ('SOLICITANTE', 'documentos.create'),
    ('SOLICITANTE', 'seguimiento.view_own')
) AS seed(rol_nombre, permiso_codigo)
JOIN roles r ON r.nombre = seed.rol_nombre
JOIN permissions p ON p.codigo = seed.permiso_codigo
ON CONFLICT (role_id, permission_id) DO NOTHING;

COMMIT;

-- Down migration:
-- (no se provee down: borrar permisos/relaciones sembrados aquí requeriría
-- distinguirlos de los que pudiera crear otro proceso más adelante; si se
-- necesita revertir, hacerlo por codigo explícito caso por caso).
