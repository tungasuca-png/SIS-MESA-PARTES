BEGIN;

-- Sistemas consumidores (client credentials): identidades de SISTEMA, no de
-- usuario. Nunca tienen rol ni contraseña: obtienen un JWT de servicio de
-- corta duración con client_id + client_secret.
--   - secret_hash: SHA-256 del client_secret (32 bytes). El secreto en texto
--     plano se muestra una sola vez al crearlo y nunca se guarda.
--   - Un cliente REVOCADO no obtiene tokens nuevos (los emitidos vencen solos,
--     ServiceTokenDuration ≤ 15 min).
--   - Sin DELETE: la fila queda como registro de quién tuvo acceso.
CREATE TABLE IF NOT EXISTS service_clients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nombre VARCHAR(120) NOT NULL,
    client_id VARCHAR(64) NOT NULL,
    secret_hash BYTEA NOT NULL,
    estado VARCHAR(20) NOT NULL DEFAULT 'ACTIVO',
    created_by UUID NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMPTZ NULL,
    revoked_at TIMESTAMPTZ NULL,
    CONSTRAINT uq_service_clients_client_id UNIQUE (client_id),
    CONSTRAINT chk_service_clients_nombre CHECK (length(TRIM(BOTH FROM nombre)) > 0),
    CONSTRAINT chk_service_clients_client_id CHECK (client_id ~ '^cli_[0-9a-f]{32}$'),
    CONSTRAINT chk_service_clients_secret_hash CHECK (octet_length(secret_hash) = 32),
    CONSTRAINT chk_service_clients_estado CHECK (estado IN ('ACTIVO', 'REVOCADO')),
    CONSTRAINT chk_service_clients_revocacion CHECK ((estado = 'REVOCADO') = (revoked_at IS NOT NULL)),
    CONSTRAINT fk_service_clients_created_by
        FOREIGN KEY (created_by) REFERENCES usuarios(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT
);

-- Permisos de cada sistema: códigos reales del catálogo (permissions), igual
-- que role_permissions pero sin pasar por roles humanos.
CREATE TABLE IF NOT EXISTS service_client_permissions (
    service_client_id UUID NOT NULL,
    permission_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (service_client_id, permission_id),
    CONSTRAINT fk_service_client_permissions_client
        FOREIGN KEY (service_client_id) REFERENCES service_clients(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT,
    CONSTRAINT fk_service_client_permissions_permission
        FOREIGN KEY (permission_id) REFERENCES permissions(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT
);

-- Catálogo (misma política que 012 y 014–018: se registran, NO se asignan a
-- ningún rol; la asignación se hace desde la pantalla de Roles).
--   - estudiantes.basic.view: solo datos básicos (nombre, código, documento)
--     para sistemas que no necesitan el registro completo.
--   - integraciones.manage: registrar, listar y revocar sistemas consumidores.
INSERT INTO permissions (codigo, nombre, descripcion, modulo, estado) VALUES
    ('estudiantes.basic.view', 'Ver datos básicos de estudiantes', 'Consultar solo los datos básicos de estudiantes (resolución por documento y consulta por lote)', 'estudiantes', TRUE),
    ('integraciones.manage', 'Administrar sistemas consumidores', 'Registrar, listar y revocar sistemas que consumen la API de Estudiantes', 'integraciones', TRUE)
ON CONFLICT (codigo) DO NOTHING;

COMMIT;

-- Down migration:
-- DROP TABLE service_client_permissions, service_clients;
-- DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE codigo IN ('estudiantes.basic.view', 'integraciones.manage'));
-- DELETE FROM permissions WHERE codigo IN ('estudiantes.basic.view', 'integraciones.manage');
