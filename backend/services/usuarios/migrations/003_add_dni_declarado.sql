BEGIN;

-- DNI DECLARADO de los solicitantes del registro público (correo verificado
-- o Google). Verificar el correo no prueba la titularidad del DNI: el perfil
-- se crea con dni = '' (sin reservar el DNI en uq_usuarios_dni) y el valor
-- escrito en el formulario queda aquí, SIN índice único (varias personas
-- pueden declarar el mismo DNI). Cuando Secretaría verifica la identidad
-- (permiso solicitantes.identidad.verify), el DNI pasa a la columna dni.
--
-- Solo agrega una columna con valor por defecto: no modifica filas
-- existentes (perfiles institucionales y solicitantes anteriores conservan
-- su dni). Idempotente.
ALTER TABLE usuarios ADD COLUMN IF NOT EXISTS dni_declarado VARCHAR(8) NOT NULL DEFAULT '';

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_usuarios_dni_declarado') THEN
        ALTER TABLE usuarios
            ADD CONSTRAINT chk_usuarios_dni_declarado CHECK (dni_declarado = '' OR dni_declarado ~ '^[0-9]{8}$');
    END IF;
END
$$;

COMMIT;

-- Down migration:
-- ALTER TABLE usuarios DROP CONSTRAINT IF EXISTS chk_usuarios_dni_declarado;
-- ALTER TABLE usuarios DROP COLUMN IF EXISTS dni_declarado;
