# Activación de Google OAuth — procedimiento

Para el responsable que activa el acceso con Google en el servidor del
proyecto (PowerShell + Docker, contenedor `tg-postgres`). Ejecutar desde la raíz
del repositorio, en orden. **Nada de esto se ha ejecutado todavía.**

Requisitos previos (no los cubre este documento):

- Cliente OAuth "Aplicación web" en Google Cloud Console, con la Authorized
  redirect URI **exacta** (desarrollo:
  `http://localhost:5173/api/auth/google/callback`).
- `backend/.env` con `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`,
  `GOOGLE_REDIRECT_URL` (y, en producción HTTPS, `GOOGLE_COOKIE_SECURE=true`).
  Ver `backend/.env.example`. Nunca versionar ni pegar estos valores.
- Ventana sin usuarios: el paso 8 reinicia los servicios.

## 1. Respaldo de auth_db

El respaldo contiene datos personales y hashes de contraseñas: guardarlo
**fuera del repositorio** y con acceso restringido.

```powershell
$fecha  = Get-Date -Format "yyyyMMdd-HHmm"
$dump   = "auth_db_pre_google_$fecha.dump"
$destino = "D:\respaldos\tungasuca"          # fuera del repo
New-Item -ItemType Directory -Force $destino | Out-Null
docker exec tg-postgres pg_dump -U tg_admin -d auth_db -Fc -f "/tmp/$dump"
docker cp "tg-postgres:/tmp/$dump" "$destino\$dump"
docker exec tg-postgres rm "/tmp/$dump"
Get-FileHash "$destino\$dump" -Algorithm SHA256 | Format-List
```

## 2. Verificar el respaldo

Índice legible y restauración de prueba en un contenedor **desechable** (nunca
sobre el servidor real):

```powershell
docker cp "$destino\$dump" "tg-postgres:/tmp/$dump"
docker exec tg-postgres pg_restore -l "/tmp/$dump" | Select-String "TABLE DATA public (usuarios|usuario_roles|refresh_tokens|permissions)"
docker exec tg-postgres rm "/tmp/$dump"

$env:PGVERIF = [guid]::NewGuid().ToString("N")
docker run -d --name tg-verif-respaldo -e POSTGRES_PASSWORD=$env:PGVERIF postgres:17 | Out-Null
Start-Sleep -Seconds 5
docker cp "$destino\$dump" "tg-verif-respaldo:/tmp/r.dump"
docker exec tg-verif-respaldo createdb -U postgres auth_db
docker exec tg-verif-respaldo pg_restore -U postgres -d auth_db --no-owner --no-acl /tmp/r.dump
docker exec tg-verif-respaldo psql -U postgres -d auth_db -At -c "SELECT 'usuarios=' || count(*) FROM usuarios"
docker exec tg-postgres psql -U tg_admin -d auth_db -At -c "SELECT 'usuarios=' || count(*) FROM usuarios"
docker rm -f tg-verif-respaldo | Out-Null
Remove-Item Env:PGVERIF
```

Los dos conteos deben coincidir. Si `pg_restore` falla, **no continuar**.

## 3. Aplicar 022

Se copia el archivo al contenedor (evita que la tubería de PowerShell 5.1
cambie la codificación) y se aplica con el migrador actuando como dueño:

```powershell
docker cp backend\services\auth\migrations\022_create_user_auth_providers.sql tg-postgres:/tmp/022.sql
docker exec tg-postgres psql -U auth_migrator -d auth_db -v ON_ERROR_STOP=1 -c "SET ROLE auth_owner" -f /tmp/022.sql
```

Debe terminar con `COMMIT`. Si da error, la transacción se revierte entera:
detenerse y no aplicar la 023.

## 4. Aplicar 023

```powershell
docker cp backend\services\auth\migrations\023_create_oauth_login_codes_and_auth_eventos.sql tg-postgres:/tmp/023.sql
docker exec tg-postgres psql -U auth_migrator -d auth_db -v ON_ERROR_STOP=1 -c "SET ROLE auth_owner" -f /tmp/023.sql
docker exec tg-postgres rm /tmp/022.sql /tmp/023.sql
```

## 5. Verificar tablas

```powershell
docker exec tg-postgres psql -U tg_admin -d auth_db -c "SELECT tablename, tableowner FROM pg_tables WHERE tablename IN ('user_auth_providers','oauth_login_codes','auth_eventos') ORDER BY 1"
docker exec tg-postgres psql -U tg_admin -d auth_db -c "SELECT conrelid::regclass AS tabla, conname FROM pg_constraint WHERE conrelid::regclass::text IN ('user_auth_providers','oauth_login_codes','auth_eventos') ORDER BY 1, 2"
```

Esperado: 3 tablas, todas de `auth_owner`.

## 6. Propietarios y permisos de auth_app

```powershell
docker exec tg-postgres psql -U tg_admin -d auth_db -At -c "SELECT t || ': ' || coalesce(string_agg(p, ',' ORDER BY o), '(ninguno)') FROM unnest(ARRAY['usuarios','user_auth_providers','oauth_login_codes','auth_eventos','service_clients','service_client_permissions']) t LEFT JOIN LATERAL (SELECT p, o FROM unnest(ARRAY['SELECT','INSERT','UPDATE','DELETE','TRUNCATE']) WITH ORDINALITY x(p, o) WHERE has_table_privilege('auth_app', 'public.' || t, p)) q ON TRUE GROUP BY t ORDER BY t"
```

Esperado (igual que en `backend/db/roles/README.md`):

```
auth_eventos: INSERT
oauth_login_codes: SELECT,INSERT,UPDATE
service_client_permissions: SELECT,INSERT,DELETE
service_clients: SELECT,INSERT,UPDATE
user_auth_providers: SELECT,INSERT,UPDATE
usuarios: SELECT,INSERT,UPDATE
```

Cualquier `DELETE` en `usuarios`, `user_auth_providers`, `oauth_login_codes` o
`auth_eventos`, o un `SELECT` en `auth_eventos`, es un error: detenerse.

## 7. Función de compensación

```powershell
docker exec tg-postgres psql -U tg_admin -d auth_db -At -c "SELECT 'dueño=' || pg_get_userbyid(proowner) || ' secdef=' || prosecdef || ' config=' || array_to_string(proconfig, ';') || ' acl=' || proacl::text FROM pg_proc WHERE oid = 'public.cancelar_registro_google_pendiente(uuid)'::regprocedure"
docker exec tg-postgres psql -U tg_admin -d auth_db -At -c "SELECT 'app=' || has_function_privilege('auth_app','public.cancelar_registro_google_pendiente(uuid)','EXECUTE') || ' tester=' || has_function_privilege('auth_tester','public.cancelar_registro_google_pendiente(uuid)','EXECUTE')"
```

Esperado: `dueño=auth_owner secdef=true config=search_path=pg_catalog, pg_temp`,
ACL solo con `auth_owner` y `auth_app` (sin `=X/` de PUBLIC), y
`app=true tester=false`. **No invocar la función** contra la base real para
probarla: con un UUID real eliminaría la cuenta si cumple las condiciones.

## 8. Reiniciar Auth, Usuarios y Gateway

`run-all.ps1` carga `backend/.env` y arranca en orden Auth (8080) → … →
Usuarios (8083) → … → Gateway (8888), que es el orden necesario (el Gateway
depende de los otros dos):

```powershell
cd backend
.\stop-all.ps1
.\run-all.ps1
cd ..
```

Esperar a que los tres puertos respondan:

```powershell
8080, 8083, 8888 | ForEach-Object { "{0}: {1}" -f $_, (Test-NetConnection 127.0.0.1 -Port $_ -WarningAction SilentlyContinue).TcpTestSucceeded }
```

## 9. Pruebas HTTP posteriores

```powershell
# a) Inicio con Google: 302 a accounts.google.com con PKCE S256, state y nonce.
curl.exe -s -o NUL -D - "http://127.0.0.1:8888/api/auth/google/iniciar" | Select-String "^HTTP|^Location|^Set-Cookie"
#    Esperado: HTTP 302; Location con code_challenge_method=S256, state= y nonce=;
#    Set-Cookie tg_google_flujo con HttpOnly, Path=/api/auth/google y SameSite=Lax.

# b) Registro antiguo deshabilitado: 410.
curl.exe -s -o NUL -w "%{http_code}`n" -X POST -H "Content-Type: application/json" -d "{}" http://127.0.0.1:8888/api/auth/register

# c) Estado de vinculación sin sesión: 401.
curl.exe -s -o NUL -w "%{http_code}`n" http://127.0.0.1:8888/api/auth/google/estado

# d) Canje con un código inválido: 400 (codigo_invalido), sin crear nada.
curl.exe -s -w "`n%{http_code}`n" -X POST -H "Content-Type: application/json" -d '{\"codigo\":\"invalido\"}' http://127.0.0.1:8888/api/auth/google/canjear
```

e) Login tradicional: entrar por `http://localhost:5173/login` con DNI y clave
de una cuenta de prueba existente (no pegar credenciales en la terminal).

f) Flujo real con una cuenta de Google de **prueba** (no institucional):
"Continuar con Google" → formulario obligatorio → cuenta SOLICITANTE creada →
cerrar sesión → volver a entrar con Google (misma cuenta). Desde "Mi perfil" de
una cuenta SOLICITANTE con clave: vincular Google (pide la clave).

g) Auditoría (solo lectura):

```powershell
docker exec tg-postgres psql -U tg_admin -d auth_db -c "SELECT evento, resultado, motivo, created_at FROM auth_eventos ORDER BY created_at DESC LIMIT 20"
```

## 10. Recuperación ante errores

| Situación | Acción |
|---|---|
| 022 o 023 fallan al aplicarse | Cada una es una transacción: no queda nada a medias. Corregir y reintentar; no reiniciar servicios. |
| Problema funcional tras activar, sin datos que conservar | **Desactivar sin tocar la base:** dejar vacío `GOOGLE_CLIENT_ID` en `backend/.env` y repetir el paso 8. El botón responde "no habilitado" y el login con DNI sigue igual. |
| Hay que retirar las tablas | Down documentado al final de cada migración, **primero 023 y luego 022**, con `auth_migrator` + `SET ROLE auth_owner` (mismo procedimiento que los pasos 3-4, copiando un archivo con las líneas `REVOKE`/`DROP` sin el `-- `). Elimina identidades, códigos y eventos de Google. Las cuentas SOLICITANTE ya creadas con Google **se conservan** (son filas de `usuarios`). |
| Daño en datos de auth_db | Detener servicios (`stop-all.ps1`) y restaurar el respaldo del paso 1: `docker cp` del archivo al contenedor y `docker exec tg-postgres pg_restore -U tg_admin -d auth_db --clean --if-exists /tmp/<archivo>`; después reejecutar `backend/db/roles/02_propiedad_y_permisos.sql` (idempotente) y repetir el paso 6. |
| Permisos de auth_app alterados | Reejecutar `02_propiedad_y_permisos.sql` como `tg_admin` (ver `backend/db/roles/README.md`): deja exactamente el mínimo declarado, también para 019-023. |
