BEGIN;

-- APLICAR después de la 022 (la función de compensación usa
-- user_auth_providers), con auth_migrator actuando como dueño:
--   docker exec -i tg-postgres psql -U auth_migrator -d auth_db -v ON_ERROR_STOP=1 \
--     -c "SET ROLE auth_owner" -f - < 023_create_oauth_login_codes_and_auth_eventos.sql
--
-- Códigos temporales de un solo uso del flujo con Google.
--
-- Solo se guarda el HASH (SHA-256) del código; el valor viaja una única vez
-- al navegador (o queda en una cookie HttpOnly, en la vinculación). Nunca
-- contienen ni reemplazan un JWT.
--
-- purpose:
--   LOGIN       -> identidad ya vinculada: se canjea por la sesión normal
--                  (expira en minutos, un solo uso).
--   REGISTRO    -> identidad nueva verificada por Google: habilita el
--                  formulario obligatorio. Guarda los datos que Google
--                  confirmó (sub, correo verificado y nombres sugeridos) para
--                  no confiar en lo que reenvíe el navegador. usuario_id se
--                  completa cuando la cuenta ya se creó y falta el perfil.
--   VINCULACION -> un usuario autenticado (y reautenticado con su clave)
--                  autoriza vincular Google a SU cuenta.
CREATE TABLE IF NOT EXISTS oauth_login_codes (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code_hash           BYTEA        NOT NULL,
    purpose             VARCHAR(20)  NOT NULL,
    usuario_id          UUID,
    provider_user_id    VARCHAR(255),
    provider_email      CITEXT,
    nombres_sugeridos   VARCHAR(100),
    apellidos_sugeridos VARCHAR(100),
    expires_at          TIMESTAMPTZ  NOT NULL,
    used_at             TIMESTAMPTZ,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_oauth_login_codes_hash UNIQUE (code_hash),
    CONSTRAINT fk_oauth_login_codes_usuario
        FOREIGN KEY (usuario_id) REFERENCES usuarios(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    CONSTRAINT chk_oauth_login_codes_hash CHECK (octet_length(code_hash) = 32),
    CONSTRAINT chk_oauth_login_codes_purpose CHECK (purpose IN ('LOGIN', 'REGISTRO', 'VINCULACION')),
    CONSTRAINT chk_oauth_login_codes_expiry CHECK (expires_at > created_at),
    CONSTRAINT chk_oauth_login_codes_login CHECK (purpose <> 'LOGIN' OR usuario_id IS NOT NULL),
    CONSTRAINT chk_oauth_login_codes_vinculacion CHECK (purpose <> 'VINCULACION' OR usuario_id IS NOT NULL),
    CONSTRAINT chk_oauth_login_codes_registro CHECK (
        purpose <> 'REGISTRO' OR (provider_user_id IS NOT NULL AND provider_email IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_oauth_login_codes_expires_at
    ON oauth_login_codes (expires_at);

CREATE INDEX IF NOT EXISTS idx_oauth_login_codes_usuario_pendiente
    ON oauth_login_codes (usuario_id)
    WHERE used_at IS NULL;

-- Auditoría de eventos de autenticación con Google. Sin tokens, códigos,
-- secretos, contraseñas ni datos personales: solo el evento, el resultado y
-- un motivo breve. usuario_id sin FK a propósito: el evento se conserva
-- aunque una cuenta pendiente se elimine por compensación.
CREATE TABLE IF NOT EXISTS auth_eventos (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    usuario_id UUID,
    evento     VARCHAR(40) NOT NULL,
    proveedor  VARCHAR(20) NOT NULL DEFAULT 'google',
    resultado  VARCHAR(20) NOT NULL,
    motivo     VARCHAR(60) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_auth_eventos_resultado CHECK (resultado IN ('OK', 'RECHAZADO', 'ERROR'))
);

CREATE INDEX IF NOT EXISTS idx_auth_eventos_created_at
    ON auth_eventos (created_at);

CREATE INDEX IF NOT EXISTS idx_auth_eventos_usuario
    ON auth_eventos (usuario_id, created_at);

-- Compensación del autorregistro con Google, sin DELETE general sobre
-- usuarios para auth_app (02_propiedad_y_permisos.sql no se lo concede a
-- propósito). Elimina SOLO una cuenta que cumpla TODO lo siguiente:
--   1. existe;
--   2. tiene una identidad Google con registro_completo = FALSE (solo el
--      autorregistro la crea así; la vinculación de una cuenta institucional
--      la crea en TRUE, y una cuenta institucional sin Google no tiene fila);
--   3. no tiene refresh tokens (nunca tuvo sesión: no pudo tener actividad
--      en otros servicios);
--   4. tiene exactamente un rol y es SOLICITANTE.
-- Resultado: 'ELIMINADA', 'NO_EXISTE', 'NO_PENDIENTE' o 'CON_SESION'. Las
-- filas dependientes (rol, identidad, códigos) se eliminan por ON DELETE
-- CASCADE. SECURITY DEFINER con search_path fijo (pg_catalog, pg_temp al
-- final) y todos los objetos calificados con su esquema; sin SQL dinámico.
CREATE OR REPLACE FUNCTION public.cancelar_registro_google_pendiente(p_usuario_id UUID)
RETURNS TEXT
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, pg_temp
AS $$
DECLARE
    v_completo BOOLEAN;
    v_roles    INTEGER;
    v_rol      VARCHAR;
BEGIN
    IF p_usuario_id IS NULL THEN
        RETURN 'NO_EXISTE';
    END IF;

    -- Bloquea la cuenta durante la verificación (sin carreras con un login
    -- o una renovación simultáneos).
    PERFORM 1 FROM public.usuarios u WHERE u.id = p_usuario_id FOR UPDATE;
    IF NOT FOUND THEN
        RETURN 'NO_EXISTE';
    END IF;

    SELECT p.registro_completo INTO v_completo
    FROM public.user_auth_providers p
    WHERE p.usuario_id = p_usuario_id AND p.provider = 'google'
    FOR UPDATE;
    IF NOT FOUND OR v_completo THEN
        RETURN 'NO_PENDIENTE';
    END IF;

    IF EXISTS (SELECT 1 FROM public.refresh_tokens t WHERE t.usuario_id = p_usuario_id) THEN
        RETURN 'CON_SESION';
    END IF;

    SELECT count(*), min(r.nombre) INTO v_roles, v_rol
    FROM public.usuario_roles ur
    JOIN public.roles r ON r.id = ur.rol_id
    WHERE ur.usuario_id = p_usuario_id;
    IF v_roles <> 1 OR v_rol <> 'SOLICITANTE' THEN
        RETURN 'NO_PENDIENTE';
    END IF;

    DELETE FROM public.usuarios u WHERE u.id = p_usuario_id;
    RETURN 'ELIMINADA';
END
$$;

-- Nadie más que auth_app puede invocarla; queda con dueño auth_owner (si la
-- migración no se aplicó ya con SET ROLE auth_owner).
REVOKE ALL ON FUNCTION public.cancelar_registro_google_pendiente(UUID) FROM PUBLIC;

-- Privilegios mínimos del rol de aplicación (mismo criterio que la 019):
--   oauth_login_codes: consulta, creación y marca de uso (UPDATE, que también
--                      exige el SELECT ... FOR UPDATE); nunca los borra.
--   auth_eventos:      solo INSERT (auditoría de solo escritura).
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'auth_owner') THEN
        ALTER FUNCTION public.cancelar_registro_google_pendiente(UUID) OWNER TO auth_owner;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'auth_app') THEN
        GRANT SELECT, INSERT, UPDATE ON oauth_login_codes TO auth_app;
        GRANT INSERT ON auth_eventos TO auth_app;
        GRANT EXECUTE ON FUNCTION public.cancelar_registro_google_pendiente(UUID) TO auth_app;
    END IF;
END
$$;

COMMIT;

-- Down migration:
-- REVOKE ALL ON FUNCTION public.cancelar_registro_google_pendiente(UUID) FROM auth_app;
-- DROP FUNCTION IF EXISTS public.cancelar_registro_google_pendiente(UUID);
-- REVOKE ALL ON auth_eventos, oauth_login_codes FROM auth_app;
-- DROP TABLE IF EXISTS auth_eventos;
-- DROP TABLE IF EXISTS oauth_login_codes;
