# Release D8 — de `85dcee7` (desplegado) a `main` + `fix/auditoria-release-d8`

Estado verificado por SSH el 2026-11-10 (solo lectura). **Nada de este documento
se ha ejecutado en el servidor.** Cada fase requiere autorización expresa.

## 0. Punto de partida real (SRV-TUNGASUCA)

| Elemento | Estado |
|---|---|
| Repo del servidor | `C:\SIS-MESA-PARTES`, `main` = `b07e60a`, limpio salvo el archivo no versionado `bject -Skip 35 -First 120` (no tocar) |
| Binarios activos | `C:\SIS-MESA-PARTES-DEPLOY\bin`, `VERSION.txt` = `85dcee7` (anterior a `f04b9b3` y a D8) |
| BD | PostgreSQL 18.6, puerto **5433**; `pg_hba` solo admite 127.0.0.1/::1 (scram) aunque `listen_addresses='*'` |
| Migraciones aplicadas (log 2026-11-09) | auth 000–026 **excepto 011 y 021**; usuarios 001–003; expedientes 001–003; documentos 001; derivaciones 001; estudiantes 001–010; psicologia 001–004 |
| D8 en BD | **ninguna** de auth 027, derivaciones 002, expedientes 004/005/006 |
| `backend.env` | sin ninguna variable D8 |
| YAML desplegados | `Mode: pro`; sin bloques `Publicador` / `Proyector` / `ServiceJWTSecretDerivaciones` |
| HTTP | IIS en `<IP_LAN>:80` y `<IP_TAILSCALE>:80`; **sin HTTPS** (no hay certificados en `LocalMachine\My`); sin CSP ni X-Frame-Options en la SPA |
| Último respaldo de BD | `backups\bd-20261109-1115` (7 dumps + globals + SHA256SUMS) |

## 1. Bloqueadores antes de activar el publicador

1. **Cliente de servicio del publicador**: `clientes-servicio crear` exige un usuario con
   `integraciones.manage`, y ninguna migración asigna ese permiso ni crea los roles
   ADMIN/SECRETARIA/DIRECTOR/SUBDIRECTOR/DOCENTE/AUXILIAR (solo SOLICITANTE y PSICOLOGO).
   Auth 011 y 021 no se aplicaron por la misma causa. **Decisión institucional pendiente**:
   qué roles existen y quién administra integraciones.
2. **Frontend y Gateway van juntos**: el frontend nuevo envía `Idempotency-Key`, que exigen
   los RPC nuevos. Publicar binarios sin el frontend (o al revés) rompe derivar, rechazar,
   cambiar estado, resolver y corregir.
3. **Orden de 002/005**: derivaciones 002 bloquea `INSERT` de tipo `DERIVACION` y expedientes 005
   bloquea cambios de área/estado sin movimiento: ambas rompen los binarios `85dcee7`. Se aplican
   solo con los binarios nuevos detenidos/listos (ver §5).

## 2. Compilación (equipo de build, desde un checkout limpio del commit del release)

```powershell
$R = git rev-parse --short HEAD          # commit del release
$O = "C:\SIS-MESA-PARTES-DEPLOY\releases\$R\bin"
cd backend\gateway;               go build -trimpath -ldflags "-s -w" -o "$O\gateway.exe" .
foreach ($s in 'auth','derivaciones','documentos','estudiantes','expedientes','psicologia','usuarios') {
  Push-Location "backend\services\$s"; go build -trimpath -ldflags "-s -w" -o "$O\$s.exe" .; Pop-Location }
cd backend\services\expedientes;  go build -trimpath -ldflags "-s -w" -o "$O\reconciliador.exe" .\cmd\reconciliador
cd backend\services\auth;         go build -trimpath -ldflags "-s -w" -o "$O\clientes-servicio.exe" .\cmd\clientes-servicio
cd frontend; npm ci; npm run build      # dist\ -> releases\$R\web
"$R $(Get-Date -Format s)" | Set-Content "$O\VERSION.txt"
Get-ChildItem $O -File | Get-FileHash -Algorithm SHA256 | ForEach-Object { "$($_.Hash)  $(Split-Path $_.Path -Leaf)" } | Set-Content "$O\SHA256SUMS.txt"
```

Compilar desde un checkout git (no desde una copia sin `.git`): así `go version -m bin\x.exe`
muestra `vcs.revision` y el binario es identificable. Los binarios actuales no lo tienen.

## 3. Configuración de producción (añadir; no sustituye lo existente)

go-zero expande `${VAR}` porque todos los `main` usan `conf.UseEnv()`; los valores viven
solo en `secrets\backend.env` (ACL SYSTEM + Administradores), que carga `ejecutar-servicio.ps1`.

**`secrets\backend.env`** (generar valores aleatorios de ≥ 32 caracteres; nunca en chat ni repo):

| Variable | Uso |
|---|---|
| `SERVICE_JWT_SECRET_DERIVACIONES` | firma tokens de audiencia `derivaciones` (Auth) y los valida (Derivaciones); distinto de `JWT_SECRET` y `SERVICE_JWT_SECRET` |
| `EXPEDIENTES_PUBLICADOR_DB_PASSWORD` | rol `expedientes_publicador` |
| `DERIVACIONES_PROYECTOR_DB_PASSWORD` | rol `derivaciones_proyector` |
| `EXPEDIENTES_PUBLICADOR_CLIENT_ID` / `_CLIENT_SECRET` | salida de `clientes-servicio crear -audiencia derivaciones -permisos derivaciones.proyectar` (bloqueador 1) |

**`config\auth.yaml`**: `ServiceJWTSecretDerivaciones: "${SERVICE_JWT_SECRET_DERIVACIONES}"`

**`config\derivaciones.yaml`**:
```yaml
ServiceJWTSecret: "${SERVICE_JWT_SECRET_DERIVACIONES}"
ServiceTokenIssuer: tungasuca-auth
ServiceTokenAudience: derivaciones
Proyector:
  Host: 127.0.0.1
  Port: 5433
  User: derivaciones_proyector
  Password: "${DERIVACIONES_PROYECTOR_DB_PASSWORD}"
  Database: derivaciones_db
  SSLMode: disable
# y añadir - /derivaciones.Derivaciones/AplicarMovimiento a IgnoreContentMethods
```
Si `ServiceJWTSecret` tiene valor, Derivaciones **no arranca** sin `Proyector`.

**`config\expedientes.yaml`**: `Timeout: 3000` y el bloque `Publicador` del repo con
`Habilitado: false` hasta resolver el bloqueador 1 (con `true` y credenciales vacías, Expedientes
no arranca). Desde este release el servicio también se niega a arrancar si `LeaseSegundos > 600`
o `LeaseSegundos ≤ TimeoutRPCMs + 2 s`.

**`config\gateway.yaml`**: en `ExpedientesRpc`, `Timeout: 2500`.

**Reconciliador**: no lo lanza `ejecutar-servicio.ps1`, así que no ve `backend.env`. Sus variables
(`RECONCILIADOR_EXPEDIENTES_DSN=postgres://reconciliador_movimientos@127.0.0.1:5433/expedientes_db?sslmode=disable`,
`RECONCILIADOR_DERIVACIONES_DSN=…/derivaciones_db…`, `RECONCILIADOR_DB_PASSWORD`) deben cargarse
en un envoltorio propio. Pasar siempre `-informes C:\SIS-MESA-PARTES-DEPLOY\logs\reconciliacion`:
el valor por defecto es relativo al directorio de trabajo (en una tarea programada, `System32`).

## 4. Respaldo previo (obligatorio, en carpeta NUEVA)

```powershell
$B = "C:\SIS-MESA-PARTES-DEPLOY\backups\pre-d8-$(Get-Date -Format yyyyMMdd-HHmm)"
if (Test-Path $B) { throw "ya existe: no se sobrescribe" }; New-Item -ItemType Directory $B | Out-Null
$pg = 'C:\Program Files\PostgreSQL\18\bin'
foreach ($db in 'auth_db','expedientes_db','usuarios_db','documentos_db','derivaciones_db','psicologia_db','estudiantes_db') {
  & "$pg\pg_dump.exe" -h 127.0.0.1 -p 5433 -U postgres -Fc -f "$B\$db.dump" $db; if ($LASTEXITCODE) { throw $db } }
& "$pg\pg_dumpall.exe" -h 127.0.0.1 -p 5433 -U postgres --globals-only -f "$B\globals.sql"   # contiene hashes de contraseñas
Get-ChildItem $B -File | Get-FileHash -Algorithm SHA256 | ForEach-Object { "$($_.Hash)  $(Split-Path $_.Path -Leaf)" } | Set-Content "$B\SHA256SUMS.txt"
# Validación (no basta con que el comando termine): tamaño > 0 y catálogo legible
Get-ChildItem $B\*.dump | ForEach-Object { if ($_.Length -eq 0) { throw "vacío: $_" }; & "$pg\pg_restore.exe" --list $_.FullName > $null; if ($LASTEXITCODE) { throw "ilegible: $_" } }
```
Restauración (solo en incidente, con autorización): `pg_restore -h 127.0.0.1 -p 5433 -U postgres --clean --if-exists -d <db> <db>.dump`
por base, tras `psql -f globals.sql` si faltan roles. Copiar también `bin`, `config` y `web` actuales.

## 5. Despliegue (ventana de mantenimiento)

1. `scripts\sismp.ps1` → detener los 8 servicios. Respaldo §4.
2. `db/roles/01_roles.sql` (crea `expedientes_publicador`, `derivaciones_proyector`, `reconciliador_movimientos`, LOGIN y sin contraseña). Asignar sus contraseñas con `\password` (nunca en un argumento).
3. Migraciones, cada una como `<s>_migrator` con `SET ROLE <s>_owner` y `-v ON_ERROR_STOP=1`:
   auth **027** → derivaciones **002** → expedientes **004**.
4. `db/roles/02_propiedad_y_permisos.sql` (fija `idle_in_transaction_session_timeout` de publicador y reconciliador y reafirma los grants mínimos).
5. Copiar `releases\<R>\bin` a `bin` (conservar el anterior como `bin-85dcee7`), YAML de §3, `web` nuevo (conservar `web-85dcee7`).
6. Arrancar; comprobar logs (`Publicador del outbox DESHABILITADO`, `AplicarMovimiento habilitado`). Probar derivar/rechazar/cambiar estado/resolver/corregir con usuarios de prueba.
7. Con los binarios nuevos funcionando: expedientes **005**. Repetir las pruebas del paso 6.
8. expedientes **006** + `02` otra vez. **No** programar el reconciliador todavía.
9. Tras el bloqueador 1: crear el cliente, completar `backend.env`, `Publicador.Habilitado: true`, reiniciar Expedientes y verificar que el outbox llega a 0 pendientes.
10. Primera reconciliación manual en `-modo deteccion`. Solo después, decidir la tarea programada.

## 6. Reversión

- Binarios/frontend/YAML: restaurar `bin-85dcee7`, `web-85dcee7`, `config` respaldado.
- Antes de arrancar los binarios `85dcee7`: `docs/operacion/rollback/rollback_expedientes.sql` (quita las guardias 005) y `rollback_derivaciones.sql` (vuelve a permitir `INSERT` de `DERIVACION`). 027, 004 y 006 son compatibles con los binarios anteriores y no se revierten: no se borran movimientos ni outbox.
- Restaurar los dumps solo si hubo corrupción de datos (pérdida de lo registrado después del respaldo).

## 7. IIS: cabeceras propuestas para la SPA

Plantilla en `docs/operacion/iis/web.config` (rewrite igual al desplegado + CSP). Probar en un
navegador (consola sin violaciones de CSP) antes de reemplazar el archivo del servidor.
`Strict-Transport-Security` solo cuando exista HTTPS.
