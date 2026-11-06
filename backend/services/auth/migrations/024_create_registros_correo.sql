BEGIN;

-- APLICAR después de la 023 y de Usuarios 003 (orden: Usuarios 003 -> Auth
-- 024 -> Auth 025 -> Auth 026; ver docs/despliegue/registro-publico-identidad.md),
-- con auth_migrator actuando como dueño:
--   docker cp 024_create_registros_correo.sql tg-postgres:/tmp/024.sql
--   docker exec tg-postgres psql -U auth_migrator -d auth_db -v ON_ERROR_STOP=1 \
--     -c "SET ROLE auth_owner" -f /tmp/024.sql
--
-- Registro manual de SOLICITANTES con verificación del correo.
--
-- La cuenta (usuarios) NO existe hasta que la persona demuestra que controla
-- el correo: el formulario crea solo una fila aquí, con la clave ya
-- convertida a bcrypt y el HMAC-SHA256 del código de 6 dígitos (la clave del
-- HMAC no está en la base: una copia de esta tabla no permite probar los
-- códigos fuera de línea). Al verificar el código, en una transacción, se
-- crea el usuario (correo verificado, rol SOLICITANTE) y se anota aquí su
-- usuario_id; cuando Usuarios Service crea el perfil, el registro se marca
-- completado y se borran los datos personales y el hash de la clave.
--
-- Estados:
--   pendiente   -> usuario_id NULL, anulado_at NULL, completado_at NULL
--   verificado  -> usuario_id con valor, completado_at NULL (falta el perfil;
--                  la cuenta no puede iniciar sesión todavía)
--   completado  -> completado_at con valor (el código ya no sirve)
--   anulado     -> anulado_at con valor (reemplazado por un registro nuevo
--                  del mismo correo, intentos agotados, DNI/correo ocupados o
--                  compensación)
-- cuenta_existente: el correo ya tenía cuenta al registrarse. La respuesta
-- al navegador es idéntica (no revela que el correo existe), pero el correo
-- enviado es un aviso, no un código: ningún código de esta fila es válido.
-- Nunca se borran filas (auth_app no tiene DELETE).
CREATE TABLE IF NOT EXISTS registros_correo (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email                 CITEXT       NOT NULL,
    dni                   VARCHAR(8)   NOT NULL,
    password_hash         TEXT,
    nombres               VARCHAR(100),
    apellido_paterno      VARCHAR(100),
    apellido_materno      VARCHAR(100),
    code_hash             BYTEA        NOT NULL,
    cuenta_existente      BOOLEAN      NOT NULL DEFAULT FALSE,
    expires_at            TIMESTAMPTZ  NOT NULL,
    intentos              INTEGER      NOT NULL DEFAULT 0,
    envios                INTEGER      NOT NULL DEFAULT 1,
    ultimo_envio_at       TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    terminos_version      VARCHAR(40)  NOT NULL,
    terminos_aceptados_at TIMESTAMPTZ  NOT NULL,
    usuario_id            UUID,
    verificado_at         TIMESTAMPTZ,
    completado_at         TIMESTAMPTZ,
    anulado_at            TIMESTAMPTZ,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- ON DELETE SET NULL: si la compensación elimina la cuenta, el registro
    -- (anulado) se conserva como rastro.
    CONSTRAINT fk_registros_correo_usuario
        FOREIGN KEY (usuario_id) REFERENCES usuarios(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,
    CONSTRAINT chk_registros_correo_dni CHECK (dni ~ '^[0-9]{8}$'),
    CONSTRAINT chk_registros_correo_hash CHECK (octet_length(code_hash) = 32),
    CONSTRAINT chk_registros_correo_intentos CHECK (intentos >= 0),
    CONSTRAINT chk_registros_correo_envios CHECK (envios >= 1),
    CONSTRAINT chk_registros_correo_expiry CHECK (expires_at > created_at),
    CONSTRAINT chk_registros_correo_terminos CHECK (length(trim(terminos_version)) > 0),
    -- Pendiente o verificado: conserva la clave (bcrypt) hasta crear la cuenta.
    CONSTRAINT chk_registros_correo_clave CHECK (
        completado_at IS NOT NULL OR anulado_at IS NOT NULL OR password_hash IS NOT NULL
    ),
    CONSTRAINT chk_registros_correo_completado CHECK (
        completado_at IS NULL OR (usuario_id IS NOT NULL AND verificado_at IS NOT NULL)
    )
);

-- Límite de registros por correo (ventana de una hora), anulación de los
-- registros anteriores del mismo correo y aviso en el login por correo de una
-- cuenta todavía no verificada.
CREATE INDEX IF NOT EXISTS idx_registros_correo_email_created
    ON registros_correo (email, created_at);

-- Una cuenta proviene, como máximo, de un registro.
CREATE UNIQUE INDEX IF NOT EXISTS uq_registros_correo_usuario
    ON registros_correo (usuario_id)
    WHERE usuario_id IS NOT NULL;

-- Compensación del registro manual (mismo criterio que la 023 para Google):
-- auth_app no tiene DELETE sobre usuarios. Elimina SOLO la cuenta creada por
-- ESTE registro si cumple TODO lo siguiente:
--   1. el registro está verificado y no completado ni anulado;
--   2. la cuenta no tiene refresh tokens (nunca tuvo sesión: el login se
--      niega mientras el registro no esté completado);
--   3. tiene exactamente un rol y es SOLICITANTE;
--   4. no tiene una identidad Google.
-- Marca el registro como anulado. Resultado: 'ELIMINADA', 'NO_EXISTE',
-- 'NO_PENDIENTE' o 'CON_SESION'. SECURITY DEFINER con search_path fijo y
-- objetos calificados con su esquema; sin SQL dinámico.
CREATE OR REPLACE FUNCTION public.cancelar_registro_correo_pendiente(p_registro_id UUID)
RETURNS TEXT
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, pg_temp
AS $$
DECLARE
    v_usuario    UUID;
    v_completado TIMESTAMPTZ;
    v_anulado    TIMESTAMPTZ;
    v_roles      INTEGER;
    v_rol        VARCHAR;
BEGIN
    IF p_registro_id IS NULL THEN
        RETURN 'NO_EXISTE';
    END IF;

    SELECT r.usuario_id, r.completado_at, r.anulado_at
      INTO v_usuario, v_completado, v_anulado
      FROM public.registros_correo r
     WHERE r.id = p_registro_id
       FOR UPDATE;
    IF NOT FOUND THEN
        RETURN 'NO_EXISTE';
    END IF;
    IF v_usuario IS NULL OR v_completado IS NOT NULL OR v_anulado IS NOT NULL THEN
        RETURN 'NO_PENDIENTE';
    END IF;

    PERFORM 1 FROM public.usuarios u WHERE u.id = v_usuario FOR UPDATE;
    IF NOT FOUND THEN
        RETURN 'NO_EXISTE';
    END IF;

    IF EXISTS (SELECT 1 FROM public.refresh_tokens t WHERE t.usuario_id = v_usuario) THEN
        RETURN 'CON_SESION';
    END IF;

    IF EXISTS (SELECT 1 FROM public.user_auth_providers p WHERE p.usuario_id = v_usuario) THEN
        RETURN 'NO_PENDIENTE';
    END IF;

    SELECT count(*), min(r.nombre) INTO v_roles, v_rol
      FROM public.usuario_roles ur
      JOIN public.roles r ON r.id = ur.rol_id
     WHERE ur.usuario_id = v_usuario;
    IF v_roles <> 1 OR v_rol <> 'SOLICITANTE' THEN
        RETURN 'NO_PENDIENTE';
    END IF;

    UPDATE public.registros_correo
       SET anulado_at = CURRENT_TIMESTAMP,
           password_hash = NULL, nombres = NULL, apellido_paterno = NULL, apellido_materno = NULL
     WHERE id = p_registro_id;
    DELETE FROM public.usuarios u WHERE u.id = v_usuario;
    RETURN 'ELIMINADA';
END
$$;

REVOKE ALL ON FUNCTION public.cancelar_registro_correo_pendiente(UUID) FROM PUBLIC;

-- Privilegios mínimos del rol de aplicación (mismo criterio que 019/022/023
-- y que backend/db/roles/02_propiedad_y_permisos.sql):
--   registros_correo: consulta, alta y cambios de estado; nunca DELETE.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'auth_owner') THEN
        ALTER FUNCTION public.cancelar_registro_correo_pendiente(UUID) OWNER TO auth_owner;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'auth_app') THEN
        GRANT SELECT, INSERT, UPDATE ON registros_correo TO auth_app;
        GRANT EXECUTE ON FUNCTION public.cancelar_registro_correo_pendiente(UUID) TO auth_app;
    END IF;
END
$$;

COMMIT;

-- Down migration:
-- REVOKE ALL ON FUNCTION public.cancelar_registro_correo_pendiente(UUID) FROM auth_app;
-- DROP FUNCTION IF EXISTS public.cancelar_registro_correo_pendiente(UUID);
-- REVOKE ALL ON registros_correo FROM auth_app;
-- DROP TABLE IF EXISTS registros_correo;
