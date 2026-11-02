BEGIN;

-- APLICAR con auth_migrator actuando como dueño (las tablas quedan con
-- propietario auth_owner, igual que el resto de auth_db):
--   docker exec -i tg-postgres psql -U auth_migrator -d auth_db -v ON_ERROR_STOP=1 \
--     -c "SET ROLE auth_owner" -f - < 022_create_user_auth_providers.sql
--
-- Identidades externas (Google) vinculadas a un usuario de Auth.
--
-- La identidad se determina por (provider, provider_user_id) — el claim "sub"
-- del ID token —, NUNCA por el correo: provider_email es solo informativo.
--
-- registro_completo:
--   TRUE  -> identidad lista para iniciar sesión (cuentas creadas por Google
--            que ya tienen perfil en Usuarios Service, o cuentas
--            institucionales que vincularon Google).
--   FALSE -> cuenta creada por el autorregistro con Google cuyo perfil en
--            Usuarios Service todavía no se confirmó. No puede iniciar sesión:
--            el siguiente ingreso con Google retoma el formulario.
--
-- Columnas en convención del esquema real (usuario_id, como usuario_roles y
-- refresh_tokens). No modifica la tabla usuarios ni datos existentes.
CREATE TABLE IF NOT EXISTS user_auth_providers (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    usuario_id        UUID         NOT NULL,
    provider          VARCHAR(20)  NOT NULL,
    provider_user_id  VARCHAR(255) NOT NULL,
    provider_email    CITEXT       NOT NULL,
    registro_completo BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at      TIMESTAMPTZ,
    CONSTRAINT fk_user_auth_providers_usuario
        FOREIGN KEY (usuario_id) REFERENCES usuarios(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    CONSTRAINT chk_user_auth_providers_provider CHECK (provider IN ('google')),
    CONSTRAINT chk_user_auth_providers_subject CHECK (length(TRIM(BOTH FROM provider_user_id)) > 0),
    -- Una identidad de Google pertenece a una sola cuenta...
    CONSTRAINT uq_user_auth_providers_identidad UNIQUE (provider, provider_user_id),
    -- ...y una cuenta tiene como máximo una identidad por proveedor.
    CONSTRAINT uq_user_auth_providers_usuario UNIQUE (usuario_id, provider)
);

-- Privilegios mínimos del rol de aplicación (mismo criterio que la 019 y que
-- backend/db/roles/02_propiedad_y_permisos.sql). Auth consulta, crea y
-- actualiza identidades (last_used_at, registro_completo); nunca las borra
-- directamente: solo desaparecen por ON DELETE CASCADE al eliminar el
-- usuario (función cancelar_registro_google_pendiente, migración 023).
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'auth_app') THEN
        GRANT SELECT, INSERT, UPDATE ON user_auth_providers TO auth_app;
    END IF;
END
$$;

COMMIT;

-- Down migration:
-- REVOKE ALL ON user_auth_providers FROM auth_app;
-- DROP TABLE IF EXISTS user_auth_providers;
