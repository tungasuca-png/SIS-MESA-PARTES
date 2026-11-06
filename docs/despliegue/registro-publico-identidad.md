# Despliegue: registro público y verificación de identidad

Procedimiento para activar en un entorno real el registro manual con correo,
el registro con Google y la verificación de identidad de solicitantes.
**Requiere autorización expresa; no se ha ejecutado en ninguna base real.**
Todo lo descrito se probó únicamente en PostgreSQL desechables.

## Orden obligatorio

| Paso | Base | Migración | Qué hace |
|---|---|---|---|
| 1 | `usuarios_db` | **Usuarios 003** `003_add_dni_declarado.sql` | Agrega `usuarios.dni_declarado` (`VARCHAR(8) NOT NULL DEFAULT ''`) y su `CHECK` |
| 2 | `auth_db` | **Auth 024** `024_create_registros_correo.sql` | Crea `registros_correo` y la función `cancelar_registro_correo_pendiente(uuid)` |
| 3 | `auth_db` | **Auth 025** `025_create_identidad_solicitantes.sql` | Crea `identidad_solicitantes` y registra el permiso `solicitantes.identidad.verify` sin asignarlo a ningún rol |
| 4 | `auth_db` | **Auth 026** `026_restringir_auth_tester_registro_publico.sql` | Retira `DELETE` de `auth_tester` sobre `registros_correo` e `identidad_solicitantes` |

Requisito previo: Auth 022 y 023 aplicadas (ya lo están en el entorno actual).

### Por qué este orden

- **Usuarios primero.** El Gateway consulta a Usuarios (`PublicRegistrationReady`)
  antes de que Auth guarde un registro o cree una cuenta. La consulta lee la
  columna `dni_declarado`. Mientras la 003 no esté aplicada, el registro
  manual y el de Google responden **503 sin crear nada**: ni cuenta, ni
  registro pendiente, ni correo enviado, ni DNI reservado. Si Usuarios va
  primero, Auth nunca crea una cuenta que no pueda tener perfil.
- **024 antes que 025.** La 024 habilita el inicio del registro manual. Si se
  detiene el despliegue entre la 024 y la 025, el registro arranca y envía el
  código, pero la verificación responde 503 sin crear la cuenta. Al aplicar
  la 025, el mismo código, si no ha vencido, completa el registro sin
  duplicados (probado). La 025 no depende técnicamente de la 024: sus claves
  foráneas solo apuntan a `usuarios`. El orden es operativo: deja el estado
  intermedio más corto y más fácil de diagnosticar.
- **Aplicar 024 y 025 en la misma ventana**, una detrás de otra.
- **026 inmediatamente después de la 025.** Los privilegios por defecto de
  `auth_db` (`02_propiedad_y_permisos.sql`) conceden a `auth_tester`, la cuenta
  de pruebas de integración, `SELECT, INSERT, UPDATE, DELETE` sobre toda tabla
  nueva de `auth_owner`. La 026 retira solo el `DELETE` de las dos tablas del
  registro público, porque las pruebas solo siembran y leen. **Siempre que se
  reaplique la 024 o la 025** (por ejemplo, tras su `Down`), hay que volver a
  aplicar la 026: al recrearse la tabla, los privilegios por defecto devuelven
  el `DELETE`. `auth_tester` sigue pudiendo eliminar sus usuarios de prueba,
  porque las acciones `ON DELETE` de las claves foráneas se ejecutan como el
  dueño de la tabla. La 026 no cambia `auth_app`, ni otras tablas, ni los
  privilegios por defecto.

### Estados intermedios probados

Todos se probaron en un entorno desechable con el código actual. Ninguno
produce un 500 ni una cuenta parcial.

| Estado | Registro manual | Google | Login existente |
|---|---|---|---|
| Sin 003, 024 ni 025 (estado actual) | 503, nada creado | 503, nada creado | Sin cambios (DNI y correo) |
| 024 y 025 sin la 003 | 503 antes de llamar a Auth | 503 antes de llamar a Auth | Sin cambios |
| 003 y 024 sin la 025 | Envía el código; la verificación da 503 sin cuenta | Sin cuenta | Sin cambios |
| 003, 024 y 025 | Completo | Completo | Sin cambios |

## Revisión de las migraciones

| Aspecto | Usuarios 003 | Auth 024 | Auth 025 |
|---|---|---|---|
| Idempotencia | `ADD COLUMN IF NOT EXISTS`; `CHECK` dentro de `IF NOT EXISTS` (probado aplicándola dos veces) | `CREATE TABLE/INDEX IF NOT EXISTS`, `CREATE OR REPLACE FUNCTION` | `CREATE TABLE/INDEX IF NOT EXISTS`, `INSERT … ON CONFLICT (codigo) DO NOTHING` |
| Transacción | `BEGIN … COMMIT` | `BEGIN … COMMIT` | `BEGIN … COMMIT` |
| Destructiva | No: no modifica filas existentes (huella md5 de la tabla idéntica antes y después) | No: solo objetos nuevos | No: solo objetos nuevos y un permiso nuevo |
| Datos existentes | Las filas existentes quedan con `dni_declarado = ''`; ningún `dni` cambia | No toca `usuarios` | No toca `usuarios` ni `role_permissions` |
| Índices | Ninguno; sin índice único a propósito (varias personas pueden declarar el mismo DNI) | `idx_registros_correo_email_created`, `uq_registros_correo_usuario` | `idx_identidad_solicitantes_estado`, `…_sincronizacion`, `…_dni` (no únicos) |
| Bloqueo | `ADD COLUMN` con valor constante: cambio de metadatos en PostgreSQL ≥ 11, sin reescribir la tabla | Tabla nueva | Tabla nueva |
| Permisos de `auth_app` | No aplica. `usuarios_user` tiene privilegios a nivel de tabla, que cubren la columna nueva | `SELECT, INSERT, UPDATE` en `registros_correo` (sin `DELETE`); `EXECUTE` de la función; `REVOKE … FROM PUBLIC` | `SELECT, INSERT, UPDATE` en `identidad_solicitantes` (sin `DELETE`) |
| Dueño | `usuarios_owner` (con `SET ROLE`) | `auth_owner` (la función también, con `ALTER FUNCTION … OWNER`) | `auth_owner` |
| Compatibilidad con código anterior | Sí: los `INSERT` antiguos toman el valor por defecto | Sí: tabla no usada por el código anterior | Sí |
| Reversión | `Down` comentado al final del archivo | `Down` comentado | `Down` comentado. Elimina las identidades declaradas: solo antes de abrir el registro público |

`backend/db/roles/02_propiedad_y_permisos.sql` ya incluye los permisos de
`registros_correo` e `identidad_solicitantes`, también la restricción de la 026.
Si se reejecuta después de la 024, la 025 y la 026, deja los mismos permisos
(probado dos veces seguidas en un entorno desechable con los roles reales).

## Procedimiento

Antes de empezar:

- Ventana sin usuarios: el paso 5 reinicia los servicios.
- El código nuevo ya debe estar desplegado. Funciona con y sin las migraciones
  (ver los estados intermedios), así que puede desplegarse antes.

### 1. Respaldos (fuera del repositorio)

Los respaldos contienen datos personales y hashes de contraseñas. Deben
guardarse **fuera del repositorio** y con acceso restringido. Nunca deben
subirse a GitHub.

```powershell
$fecha   = Get-Date -Format "yyyyMMdd-HHmm"
$destino = "D:\respaldos\tungasuca"          # fuera del repo
New-Item -ItemType Directory -Force $destino | Out-Null
foreach ($db in "usuarios_db", "auth_db") {
  $dump = "${db}_pre_registro_publico_$fecha.dump"
  docker exec tg-postgres pg_dump -U tg_admin -d $db -Fc -f "/tmp/$dump"
  docker cp "tg-postgres:/tmp/$dump" "$destino\$dump"
  docker exec tg-postgres rm "/tmp/$dump"
  Get-FileHash "$destino\$dump" -Algorithm SHA256 | Format-List
}
```

### 2. Comprobación previa (solo lectura)

```powershell
docker exec tg-postgres psql -U tg_admin -d usuarios_db -At -c "SELECT 'dni_declarado existe: ' || EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='usuarios' AND column_name='dni_declarado')"
docker exec tg-postgres psql -U tg_admin -d auth_db -At -c "SELECT t || ': ' || (to_regclass('public.' || t) IS NOT NULL) FROM unnest(ARRAY['user_auth_providers','oauth_login_codes','auth_eventos','registros_correo','identidad_solicitantes']) t"
```

Resultado esperado hoy:

- `dni_declarado existe: f`;
- las tablas de la 022 y la 023 en `t`;
- `registros_correo` e `identidad_solicitantes` en `f`.

### 3. Migraciones, en este orden

```powershell
# Paso 1: Usuarios 003
docker cp backend\services\usuarios\migrations\003_add_dni_declarado.sql tg-postgres:/tmp/u003.sql
docker exec tg-postgres psql -U usuarios_migrator -d usuarios_db -v ON_ERROR_STOP=1 -c "SET ROLE usuarios_owner" -f /tmp/u003.sql

# Paso 2: Auth 024
docker cp backend\services\auth\migrations\024_create_registros_correo.sql tg-postgres:/tmp/024.sql
docker exec tg-postgres psql -U auth_migrator -d auth_db -v ON_ERROR_STOP=1 -c "SET ROLE auth_owner" -f /tmp/024.sql

# Paso 3: Auth 025
docker cp backend\services\auth\migrations\025_create_identidad_solicitantes.sql tg-postgres:/tmp/025.sql
docker exec tg-postgres psql -U auth_migrator -d auth_db -v ON_ERROR_STOP=1 -c "SET ROLE auth_owner" -f /tmp/025.sql

# Paso 4: Auth 026
docker cp backend\services\auth\migrations\026_restringir_auth_tester_registro_publico.sql tg-postgres:/tmp/026.sql
docker exec tg-postgres psql -U auth_migrator -d auth_db -v ON_ERROR_STOP=1 -c "SET ROLE auth_owner" -f /tmp/026.sql

docker exec tg-postgres rm /tmp/u003.sql /tmp/024.sql /tmp/025.sql /tmp/026.sql
```

Si un paso falla, se revierte solo: cada migración es una transacción.
**No continuar con el siguiente paso.** Revisar el error y repetir; las
migraciones son idempotentes.

### 4. Verificación (solo lectura)

```powershell
docker exec tg-postgres psql -U tg_admin -d usuarios_db -At -c "SELECT column_name || ' default=' || column_default FROM information_schema.columns WHERE table_name='usuarios' AND column_name='dni_declarado'"
docker exec tg-postgres psql -U tg_admin -d auth_db -At -c "SELECT t || ': ' || coalesce(string_agg(p, ','), '(ninguno)') FROM unnest(ARRAY['registros_correo','identidad_solicitantes']) t LEFT JOIN LATERAL (SELECT p FROM unnest(ARRAY['SELECT','INSERT','UPDATE','DELETE','TRUNCATE']) p WHERE has_table_privilege('auth_app', 'public.' || t, p)) q ON TRUE GROUP BY t"
docker exec tg-postgres psql -U tg_admin -d auth_db -At -c "SELECT codigo || ' asignaciones=' || (SELECT count(*) FROM role_permissions rp WHERE rp.permission_id = p.id) FROM permissions p WHERE codigo = 'solicitantes.identidad.verify'"
docker exec tg-postgres psql -U tg_admin -d auth_db -At -c "SELECT 'auth_tester ' || t || ': ' || coalesce(string_agg(p, ','), '(ninguno)') FROM unnest(ARRAY['registros_correo','identidad_solicitantes']) t LEFT JOIN LATERAL (SELECT p FROM unnest(ARRAY['SELECT','INSERT','UPDATE','DELETE','TRUNCATE']) p WHERE has_table_privilege('auth_tester', 'public.' || t, p)) q ON TRUE GROUP BY t"
```

Esperado:

- `dni_declarado default=''::character varying`;
- `registros_correo: SELECT,INSERT,UPDATE` e `identidad_solicitantes: SELECT,INSERT,UPDATE`, sin `DELETE`;
- `solicitantes.identidad.verify asignaciones=0`;
- `auth_tester registros_correo: SELECT,INSERT,UPDATE` y `auth_tester identidad_solicitantes: SELECT,INSERT,UPDATE`, sin `DELETE`.

### 5. Servicios

Reiniciar Auth, Usuarios y Gateway (`backend\stop-all.ps1` y
`backend\run-all.ps1`). Usuarios detecta la columna nueva en un máximo de
30 s aunque no se reinicie. La comprobación de preparación del registro no
usa caché.

### 6. Configuración institucional

Desde *Configuración → Roles y permisos*, el ADMIN asigna
`solicitantes.identidad.verify` al rol que corresponda, normalmente
SECRETARIA. Ninguna migración hace esta asignación, porque es una decisión
institucional. Sin ella, nadie ve la bandeja de identidades (403).

`derivaciones.view_own` **no** se asigna automáticamente a SOLICITANTE. Si
se decide asignarlo, la consulta de una derivación ajena responde igual que
la de una inexistente (404 «derivación no encontrada»).

### 7. Prueba de humo

1. Registrarse con un correo de prueba controlado por la institución.
2. Ingresar el código recibido; el login por correo funciona.
3. El login con el DNI declarado **no** funciona: el DNI aún no está verificado.
4. Secretaría verifica la identidad; a partir de ahí el DNI sirve para entrar.

El SMTP y Google OAuth reales requieren sus credenciales en `backend/.env`.
Ese archivo es local y nunca se versiona; ver `registro-correo.md` y
`google-oauth-activacion.md`.

## Por qué la comprobación de preparación vive en el Gateway

El Gateway ya orquesta el registro: llama a Auth y después a Usuarios. Por
eso es el lugar donde puede impedir que se cree algo que no se pueda
terminar.

| Alternativa | Decisión |
|---|---|
| Comprobación TCP o de salud de Usuarios | Descartada: no detecta una migración pendiente, porque Usuarios responde aunque falte `dni_declarado` |
| Auth consulta a Usuarios antes de crear la cuenta | Descartada: crea una dependencia nueva de Auth hacia Usuarios, y Auth es la fuente de verdad que debe funcionar sola |
| **RPC `Usuarios.PublicRegistrationReady` consultado por el Gateway** | **Elegida.** Hace un `SELECT dni_declarado FROM usuarios LIMIT 0` en cada llamada (prueba funcional y sin caché). No exige sesión porque el registro es anónimo. Solo devuelve un booleano, el motivo queda en el log de Usuarios, y Usuarios escucha en `127.0.0.1` |

Riesgo residual: si Usuarios cae justo entre la comprobación y la creación
del perfil, la cuenta queda pendiente de perfil. Ese caso ya lo cubren la
compensación existente (anulación del registro pendiente) y el reintento
idempotente.
