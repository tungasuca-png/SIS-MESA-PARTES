BEGIN;

-- APLICAR después de Usuarios 003 y de Auth 024, y seguida de Auth 026 (orden:
-- Usuarios 003 -> Auth 024 -> Auth 025 -> Auth 026; ver
-- docs/despliegue/registro-publico-identidad.md), con auth_migrator actuando
-- como dueño:
--   docker cp 025_create_identidad_solicitantes.sql tg-postgres:/tmp/025.sql
--   docker exec tg-postgres psql -U auth_migrator -d auth_db -v ON_ERROR_STOP=1 \
--     -c "SET ROLE auth_owner" -f /tmp/025.sql
--
-- Identidad de los SOLICITANTES registrados por el registro público (correo
-- verificado o Google). Verificar el correo NO prueba la titularidad del DNI:
-- el registro público guarda aquí solo el DNI DECLARADO (sin unicidad) y deja
-- usuarios.dni en NULL, así que nadie puede reservar un DNI ajeno y varias
-- declaraciones del mismo DNI pueden coexistir. Mientras tanto la cuenta
-- inicia sesión con su correo (o con Google).
--
-- Secretaría (permiso solicitantes.identidad.verify) coteja la identidad y la
-- marca VERIFICADA: recién entonces el DNI se escribe en usuarios.dni, cuyo
-- índice único (uq_usuarios_dni, migración 009) resuelve conflictos y
-- carreras; un DNI que ya pertenece a otra cuenta nunca se sobrescribe ni se
-- vincula. RECHAZADA deja la cuenta sin DNI.
--
-- Consistencia con Usuarios Service (otra base: no hay transacción común).
-- Auth es la fuente de verdad y guarda el estado de la operación:
--   sincronizacion = PERFIL_PENDIENTE: Auth ya verificó (usuarios.dni escrito)
--     y falta que Usuarios pase el DNI al perfil. El Gateway lo reintenta
--     (reconciliación); si Usuarios encuentra el DNI en OTRO perfil, Auth
--     revierte su verificación (compensación) y la identidad vuelve a
--     PENDIENTE sin DNI.
--   sincronizacion = COMPLETA: ambos servicios tienen el DNI.
-- sincronizacion_intentos / sincronizacion_error / sincronizacion_at dejan la
-- traza del último intento (código del error, nunca el DNI).
--
-- No guarda copias de documentos: solo el método de cotejo, quién verificó y
-- cuándo. Las cuentas institucionales y las cuentas públicas anteriores a
-- esta migración no tienen fila aquí: su comportamiento no cambia.
CREATE TABLE IF NOT EXISTS identidad_solicitantes (
    usuario_id      UUID PRIMARY KEY,
    dni_declarado   VARCHAR(8)   NOT NULL,
    origen          VARCHAR(10)  NOT NULL,
    estado          VARCHAR(12)  NOT NULL DEFAULT 'PENDIENTE',
    metodo          VARCHAR(20),
    verificado_por  UUID,
    verificado_at   TIMESTAMPTZ,
    motivo_rechazo  VARCHAR(200),
    sincronizacion          VARCHAR(20),
    sincronizacion_intentos INTEGER      NOT NULL DEFAULT 0,
    sincronizacion_error    VARCHAR(60),
    sincronizacion_at       TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_identidad_solicitantes_usuario
        FOREIGN KEY (usuario_id) REFERENCES usuarios(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    -- Quien verificó puede dejar de existir: el rastro se conserva sin él.
    CONSTRAINT fk_identidad_solicitantes_verificador
        FOREIGN KEY (verificado_por) REFERENCES usuarios(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,
    CONSTRAINT chk_identidad_solicitantes_dni CHECK (dni_declarado ~ '^[0-9]{8}$'),
    CONSTRAINT chk_identidad_solicitantes_origen CHECK (origen IN ('correo', 'google')),
    CONSTRAINT chk_identidad_solicitantes_estado CHECK (estado IN ('PENDIENTE', 'VERIFICADA', 'RECHAZADA')),
    CONSTRAINT chk_identidad_solicitantes_metodo CHECK (metodo IS NULL OR metodo IN ('PRESENCIAL', 'DOCUMENTO')),
    CONSTRAINT chk_identidad_solicitantes_verificada CHECK (
        estado <> 'VERIFICADA' OR (metodo IS NOT NULL AND verificado_at IS NOT NULL)
    ),
    CONSTRAINT chk_identidad_solicitantes_rechazada CHECK (
        estado <> 'RECHAZADA' OR (motivo_rechazo IS NOT NULL AND length(trim(motivo_rechazo)) > 0 AND verificado_at IS NOT NULL)
    ),
    CONSTRAINT chk_identidad_solicitantes_sincronizacion CHECK (
        (estado = 'VERIFICADA' AND sincronizacion IN ('PERFIL_PENDIENTE', 'COMPLETA'))
        OR (estado <> 'VERIFICADA' AND sincronizacion IS NULL)
    ),
    CONSTRAINT chk_identidad_solicitantes_sinc_intentos CHECK (sincronizacion_intentos >= 0)
);

-- Bandeja de Secretaría (pendientes primero, por antigüedad).
CREATE INDEX IF NOT EXISTS idx_identidad_solicitantes_estado
    ON identidad_solicitantes (estado, created_at);

-- Operaciones a reconciliar (verificadas en Auth, perfil pendiente).
CREATE INDEX IF NOT EXISTS idx_identidad_solicitantes_sincronizacion
    ON identidad_solicitantes (sincronizacion_at)
    WHERE sincronizacion = 'PERFIL_PENDIENTE';

-- Detección de conflictos para el personal (sin UNIQUE: declarar un DNI no
-- lo reserva).
CREATE INDEX IF NOT EXISTS idx_identidad_solicitantes_dni
    ON identidad_solicitantes (dni_declarado);

-- Permiso de la bandeja de verificación. Misma política que 012 y 014-020:
-- se registra, NO se asigna a ningún rol; ADMIN lo asigna desde
-- Configuración -> Roles y permisos.
INSERT INTO permissions (codigo, nombre, descripcion, modulo, estado) VALUES
    ('solicitantes.identidad.verify', 'Verificar identidad de solicitantes',
     'Consultar, verificar o rechazar la identidad (DNI declarado) de los solicitantes registrados por el registro público',
     'solicitantes', TRUE)
ON CONFLICT (codigo) DO NOTHING;

-- Privilegios mínimos del rol de aplicación (mismo criterio que 019-024 y
-- backend/db/roles/02_propiedad_y_permisos.sql): nunca DELETE.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'auth_app') THEN
        GRANT SELECT, INSERT, UPDATE ON identidad_solicitantes TO auth_app;
    END IF;
END
$$;

COMMIT;

-- Down migration:
-- REVOKE ALL ON identidad_solicitantes FROM auth_app;
-- DROP TABLE IF EXISTS identidad_solicitantes;
-- DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE codigo = 'solicitantes.identidad.verify');
-- DELETE FROM permissions WHERE codigo = 'solicitantes.identidad.verify';
