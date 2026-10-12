BEGIN;

-- Permite el tipo de usuario PSICOLOGO (nuevo rol de sistema en Auth). Cuando
-- se crea un usuario desde Configuración, Usuarios Service guarda una copia de
-- perfil con tipo_usuario = nombre del rol; sin esto el CHECK la rechazaría.
-- Solo amplía la lista permitida: no toca filas existentes. Idempotente.
ALTER TABLE usuarios DROP CONSTRAINT IF EXISTS chk_usuarios_tipo;
ALTER TABLE usuarios ADD CONSTRAINT chk_usuarios_tipo CHECK (
    tipo_usuario IN ('ADMIN', 'DIRECTOR', 'SUBDIRECTOR', 'SECRETARIA', 'DOCENTE', 'AUXILIAR', 'PSICOLOGO', 'SOLICITANTE')
);

COMMIT;

-- Down migration (solo si no existen usuarios PSICOLOGO):
-- ALTER TABLE usuarios DROP CONSTRAINT IF EXISTS chk_usuarios_tipo;
-- ALTER TABLE usuarios ADD CONSTRAINT chk_usuarios_tipo CHECK (
--     tipo_usuario IN ('ADMIN', 'DIRECTOR', 'SUBDIRECTOR', 'SECRETARIA', 'DOCENTE', 'AUXILIAR', 'SOLICITANTE')
-- );
