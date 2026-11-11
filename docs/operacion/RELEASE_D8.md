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

1. **Cliente de servicio del publicador (C-01, BLOQUEANTE para activar el publicador)**: `clientes-servicio crear` exige un usuario con
   `integraciones.manage`, y ninguna migración asigna ese permiso ni crea los roles
   ADMIN/SECRETARIA/DIRECTOR/SUBDIRECTOR/DOCENTE/AUXILIAR (solo SOLICITANTE y PSICOLOGO).
   Auth 011 y 021 no se aplicaron por la misma causa. **Decisión institucional pendiente**:
   qué roles existen y quién administra integraciones.
2. **Frontend y Gateway van juntos**: el frontend nuevo envía `Idempotency-Key`, que exigen
   los RPC nuevos. Publicar binarios sin el frontend (o al revés) rompe derivar, rechazar,
   cambiar estado, resolver y corregir.
3. **H-03: seguimiento sin movimientos nuevos mientras el publicador esté deshabilitado.** El
   seguimiento (`historial_expediente`) toma los movimientos de `movimientos_proyeccion`, que solo
   llena el publicador. Con `Publicador.Habilitado: false`, derivar, rechazar, resolver, etc.
   funcionan y quedan registrados (movimiento + evento `PENDIENTE` en Expedientes), pero **no se
   ven en el seguimiento** hasta activar el publicador; al activarlo se proyectan en orden. Decidir
   antes de la ventana: resolver C-01 primero (recomendado) o aceptar ese intervalo y avisar a los usuarios.
4. **Orden de 002/005**: derivaciones 002 bloquea `INSERT` de tipo `DERIVACION` y expedientes 005
   bloquea cambios de área/estado sin movimiento: ambas rompen los binarios `85dcee7`. Se aplican
   solo con los binarios nuevos detenidos/listos (ver §5).

## 2. Compilación (equipo de build, desde un checkout limpio del commit del release)

Ejecutar en PowerShell **desde cualquier directorio dentro del checkout**. Todas las rutas se
derivan de la raíz del repositorio (`git rev-parse --show-toplevel`) y cada compilación entra y
sale de su módulo con `Push-Location`/`Pop-Location`, de modo que ningún comando depende del
directorio en que dejó el anterior. `$Salida` lo fija el administrador: una carpeta **nueva y
vacía, fuera del repositorio**; el script se detiene si ya existe.

```powershell
$ErrorActionPreference = 'Stop'
$Repo = (git rev-parse --show-toplevel) -replace '/', '\'
# Cambios rastreados o archivos sin seguimiento: ambos alteran el build (Go marca vcs.modified=true).
if (git -C $Repo status --porcelain --untracked-files=all) { throw "checkout con cambios o archivos sin seguimiento: compilar desde un clon limpio del commit" }
$R = git -C $Repo rev-parse --short HEAD                 # commit del release
$Salida = 'C:\RUTA\ELEGIDA\POR\EL\ADMINISTRADOR'          # <- fijar antes de ejecutar
$O = Join-Path $Salida "$R\bin"; $W = Join-Path $Salida "$R\web"
if (Test-Path (Join-Path $Salida $R)) { throw "ya existe ${Salida}\${R}: no se sobrescribe" }
New-Item -ItemType Directory -Path $O, $W | Out-Null

function Compilar($modulo, $salidaExe, $paquete = '.') {
  Push-Location (Join-Path $Repo $modulo)
  try { go build -trimpath -ldflags "-s -w" -o $salidaExe $paquete; if ($LASTEXITCODE) { throw "falló: $modulo $paquete" } }
  finally { Pop-Location }
}
Compilar 'backend\gateway' "$O\gateway.exe"
foreach ($s in 'auth','derivaciones','documentos','estudiantes','expedientes','psicologia','usuarios') {
  Compilar "backend\services\$s" "$O\$s.exe" }
Compilar 'backend\services\expedientes' "$O\reconciliador.exe"     '.\cmd\reconciliador'
Compilar 'backend\services\auth'        "$O\clientes-servicio.exe" '.\cmd\clientes-servicio'

Push-Location (Join-Path $Repo 'frontend')
try { npm ci; if ($LASTEXITCODE) { throw 'npm ci' }; npm run build; if ($LASTEXITCODE) { throw 'npm run build' } }
finally { Pop-Location }
Copy-Item -Path (Join-Path $Repo 'frontend\dist\*') -Destination $W -Recurse

if (git -C $Repo status --porcelain --untracked-files=all -- . ':!frontend/node_modules' ':!frontend/dist') { throw 'el build modificó archivos rastreados' }
# Cada binario debe llevar el commit del release y no estar marcado como modificado.
foreach ($exe in Get-ChildItem "$O\*.exe") {
  $info = go version -m $exe.FullName
  if (-not ($info -match "vcs.revision=$(git -C $Repo rev-parse HEAD)") -or ($info -match 'vcs.modified=true')) { throw "binario no identificable: $($exe.Name)" } }
"$(git -C $Repo rev-parse HEAD) $(Get-Date -Format s)" | Set-Content "$O\VERSION.txt"
Get-ChildItem $O -File | Where-Object Name -ne 'SHA256SUMS.txt' | Get-FileHash -Algorithm SHA256 |
  ForEach-Object { "$($_.Hash)  $(Split-Path $_.Path -Leaf)" } | Set-Content "$O\SHA256SUMS.txt"
Get-ChildItem $W -Recurse -File | Get-FileHash -Algorithm SHA256 |
  ForEach-Object { "$($_.Hash)  $($_.Path.Substring($W.Length + 1))" } | Set-Content (Join-Path $Salida "$R\web-SHA256SUMS.txt")
```

Para compilar sin tocar el checkout de trabajo (que puede tener archivos propios sin seguimiento),
usar un clon limpio del commit en una carpeta nueva:
`git clone --no-local <ruta-del-repo> <carpeta-nueva>; git -C <carpeta-nueva> checkout --detach <commit>`.

Módulos y paquetes usados (existen en el repositorio): `backend\gateway`, `backend\services\<servicio>`,
`backend\services\expedientes\cmd\reconciliador`, `backend\services\auth\cmd\clientes-servicio`;
Vite genera `frontend\dist`. La carpeta `$Salida\<R>` se copia después al servidor como
`C:\SIS-MESA-PARTES-DEPLOY\releases\<R>`.

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

**Sin respaldo completo y verificado no se ejecuta ninguna migración ni el paso 2 de §5.**
La prueba de que existe es el archivo `RESPALDO_VERIFICADO.txt`, que solo escribe el paso 4.3.

### 4.1 Credencial del superusuario (no se presupone que funcione)

La credencial guardada en `secrets\pg_superuser.dpapi` **no autenticó** el 2026-11-10. Este
procedimiento no depende de ella. Reglas:

- La contraseña la introduce el administrador en una sesión interactiva (`ssh -t`), con
  `Read-Host -AsSecureString`; vive solo en la variable de entorno **del proceso** y se borra al terminar.
- Nunca se imprime, no se pasa como argumento, no se escribe en archivos ni se comparte por chat o correo.
- No se modifica `pg_hba.conf`, no se usa `trust` y no se desactiva SCRAM para respaldar.
- Si el administrador quiere volver a guardar una credencial válida, el script existente
  `C:\SIS-MESA-PARTES-DEPLOY\scripts\guardar-clave-postgres.ps1` la prueba **antes** de guardarla
  (DPAPI de máquina). Reemplaza el archivo DPAPI actual: requiere autorización. No cambia la
  contraseña en PostgreSQL.
- Si nadie conoce la contraseña del superusuario: **detener aquí**. Recuperarla es un
  procedimiento aparte, fuera de este release.

### 4.2 Comprobación previa (sesión interactiva en el servidor)

```powershell
$ErrorActionPreference = 'Stop'
$pg = 'C:\Program Files\PostgreSQL\18\bin'
$U  = 'postgres'
# Nombres propios: PowerShell no distingue mayúsculas, y $B (carpeta de 4.3) pisaría un $b aquí.
$claveSegura = Read-Host "Contraseña de $U (no se muestra)" -AsSecureString
$punteroClave = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($claveSegura)
try {
  $env:PGPASSWORD = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($punteroClave)
  $env:PGOPTIONS  = '-c default_transaction_read_only=on'      # la comprobación no puede escribir
  $r = & "$pg\psql.exe" -h 127.0.0.1 -p 5433 -U $U -d postgres -w -X -tAc "select rolsuper from pg_roles where rolname = current_user"
  if ($LASTEXITCODE -or "$r".Trim() -ne 't') { throw 'AUTENTICACION FALLIDA o el usuario no es superusuario: detener, no respaldar ni migrar' }
  'Autenticación correcta'
  # ... continuar con 4.3 en ESTA MISMA sesión ...
} finally {
  [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($punteroClave)
  Remove-Item Env:PGPASSWORD, Env:PGOPTIONS -ErrorAction SilentlyContinue
}
```

Si estos bloques se guardan como `.ps1` para Windows PowerShell 5.1, guardarlos en **UTF-8 con BOM**:
sin BOM, las letras acentuadas se leen como ANSI y pueden romper las cadenas.

Comprobar también que hay espacio libre en el disco de `backups` (al menos el triple del
tamaño de las bases) y que `pg_dump.exe` es de la versión 18.

### 4.3 Respaldo (dentro del mismo bloque `try`, tras la comprobación)

```powershell
Remove-Item Env:PGOPTIONS                                  # pg_dump fija sus propios parámetros
$B = "C:\SIS-MESA-PARTES-DEPLOY\backups\pre-d8-$(Get-Date -Format yyyyMMdd-HHmm)"
if (Test-Path $B) { throw "ya existe ${B}: no se sobrescribe" }
New-Item -ItemType Directory $B | Out-Null
$bases = 'auth_db','expedientes_db','usuarios_db','documentos_db','derivaciones_db','psicologia_db','estudiantes_db'
foreach ($db in $bases) {
  & "$pg\pg_dump.exe" -h 127.0.0.1 -p 5433 -U $U -w -Fc -f "$B\$db.dump" $db
  if ($LASTEXITCODE) { throw "FALLO el respaldo de ${db}: detener" } }
# Roles y objetos globales. Contiene los hashes SCRAM: la carpeta hereda la ACL de backups (SYSTEM + Administradores).
& "$pg\pg_dumpall.exe" -h 127.0.0.1 -p 5433 -U $U -w --globals-only -f "$B\globals.sql"
if ($LASTEXITCODE) { throw 'FALLO el respaldo de roles: detener' }

# Validación: que el comando termine no basta.
foreach ($db in $bases) {
  $f = Get-Item "$B\$db.dump"
  if ($f.Length -eq 0) { throw "vacío: $db" }
  $toc = & "$pg\pg_restore.exe" --list $f.FullName
  if ($LASTEXITCODE) { throw "ilegible: $db" }
  if (-not ($toc | Select-String -SimpleMatch ' TABLE public ')) { throw "sin tablas en el catálogo: $db" } }
$g = Get-Content "$B\globals.sql" -Raw
if ($g -notmatch 'CREATE ROLE' -or $g -notmatch 'PostgreSQL database cluster dump complete') { throw 'globals.sql incompleto' }

$sumas = Get-ChildItem $B -File | Where-Object Name -ne 'SHA256SUMS.txt' | Get-FileHash -Algorithm SHA256 |
  ForEach-Object { "$($_.Hash)  $(Split-Path $_.Path -Leaf)" }
if (@($sumas).Count -ne 8) { throw "se esperaban 8 archivos (7 dumps + globals.sql)" }
$sumas | Set-Content "$B\SHA256SUMS.txt"
# Relectura: las huellas recalculadas deben coincidir con las guardadas.
foreach ($l in Get-Content "$B\SHA256SUMS.txt") { $h, $n = $l -split '  ', 2
  if ((Get-FileHash "$B\$n" -Algorithm SHA256).Hash -ne $h) { throw "huella distinta: $n" } }
"$(Get-Date -Format s) respaldo verificado: 7 bases + globales; ver SHA256SUMS.txt" | Set-Content "$B\RESPALDO_VERIFICADO.txt"
```

Cualquier `throw` detiene el procedimiento: se revisa la causa y se repite el respaldo en una
carpeta nueva. Después de 4.3, copiar `bin`, `config` y `web` actuales a la subcarpeta
`$B\aplicacion` (sin `secrets`); no altera el recuento de 8 archivos verificado arriba.

Restauración (solo en incidente, con autorización; no forma parte del despliegue):
`pg_restore -h 127.0.0.1 -p 5433 -U postgres --clean --if-exists -d <db> <db>.dump` por base,
tras `psql -f globals.sql` si faltan roles.

## 5. Despliegue (ventana de mantenimiento)

0. Requisitos previos, antes de abrir la ventana: cuentas institucionales de prueba disponibles
   (§5.1) y comprobación de autenticación del superusuario superada (§4.2). Si falta cualquiera,
   no se inicia la ventana.
1. `scripts\sismp.ps1` → detener los 8 servicios. Repetir §4.2 y, en esa misma sesión, el
   respaldo §4.3 completo, con los servicios ya detenidos; **sin `RESPALDO_VERIFICADO.txt` en la carpeta nueva, detener y no seguir al paso 2**.
2. `db/roles/01_roles.sql` (crea `expedientes_publicador`, `derivaciones_proyector`, `reconciliador_movimientos`, LOGIN y sin contraseña). Asignar sus contraseñas con `\password` (nunca en un argumento).
3. Migraciones, cada una como `<s>_migrator` con `SET ROLE <s>_owner` y `-v ON_ERROR_STOP=1`:
   auth **027** → derivaciones **002** → expedientes **004**.
4. `db/roles/02_propiedad_y_permisos.sql` (fija `idle_in_transaction_session_timeout` de publicador y reconciliador y reafirma los grants mínimos).
5. Copiar `releases\<R>\bin` a `bin` (conservar el anterior como `bin-85dcee7`), YAML de §3, `web` nuevo (conservar `web-85dcee7`).
6. Arrancar; comprobar logs (`Publicador del outbox DESHABILITADO`, `AplicarMovimiento habilitado`).
   Ejecutar la matriz de §5.1 con las cuentas institucionales de prueba. Si alguna prueba falla: reversión (§6).
7. Con los binarios nuevos funcionando: expedientes **005**. Repetir la matriz de §5.1 completa.
8. expedientes **006** + `02` otra vez. **No** programar el reconciliador todavía.
9. Tras el bloqueador 1: crear el cliente, completar `backend.env`, `Publicador.Habilitado: true`, reiniciar Expedientes y verificar que el outbox llega a 0 pendientes.
10. Primera reconciliación manual en `-modo deteccion`. Solo después, decidir la tarea programada.

### 5.1 Pruebas funcionales con usuarios institucionales

**Roles que deben existir** (los que el código acepta como actor de un movimiento,
`expedientes/internal/logic/movimientos.go`, y las áreas de `estados.go`): `SECRETARIA`,
`DIRECTOR`, `SUBDIRECTOR`, `DOCENTE`, `AUXILIAR`, `ADMIN` y `SOLICITANTE`. Las migraciones solo
crean `SOLICITANTE` y `PSICOLOGO`: los demás, y qué permisos recibe cada uno, son una **decisión
institucional** (mismo origen que C-01 y que auth 011/021 sin aplicar). Este release **no** crea
roles ni asigna permisos.

**Permisos que exige cada operación** (comprobado en el código; además el expediente debe estar
en el área del rol, salvo `ADMIN`):

| Operación | Permiso | Regla adicional |
|---|---|---|
| Derivar | `expedientes.update` | transición confirmada para el tipo (`estados.CanDerivar`) |
| Rechazar | `expedientes.update` | solo F4 y `EN_PROCESO`; vuelve a SECRETARIA como `OBSERVADO` |
| Cambiar estado | `expedientes.change_estado` | `PENDIENTE→EN_PROCESO`, `PENDIENTE→OBSERVADO`, `EN_PROCESO→ATENDIDO` |
| Resolver | `expedientes.change_estado` | solo F2, en SECRETARIA y `EN_PROCESO` |
| Corregir | `expedientes.view_own` | solo el dueño (`solicitante_id`) y `OBSERVADO` |
| Registrar | `expedientes.create` (personal) o `solicitudes.create` (SOLICITANTE) | nace `PENDIENTE` en SECRETARIA |
| Consultar seguimiento | `derivaciones.view` o `derivaciones.view_own` | ver H-03 |

Propuesta de matriz por rol, pendiente de aprobación: `docs/operacion/C-01_PROPUESTA_PERMISOS.md`.

**Cuentas de prueba**: una por rol, creadas y autorizadas por el Colegio Tungasuca (responsable
designado por la Dirección), con nombre que las identifique como prueba. **No** sirven las cuentas
de PostgreSQL `*_tester`: son roles de base de datos, no usuarios de la aplicación, y no existen
para producción. Las credenciales no se registran en este documento.

**Datos**: solo expedientes **nuevos creados para la prueba** (asunto que empiece por `PRUEBA-D8`),
nunca expedientes reales. Qué hacer con ellos al final (baja lógica o conservarlos como evidencia)
lo decide la institución; el sistema no los borra físicamente.

**Matriz** (marcar cada fila al ejecutarla; hasta entonces todas están PENDIENTES):

| # | Caso | Cuenta | Resultado esperado | Estado |
|---|---|---|---|---|
| 0a | Registrar expediente F2 y F4 (`POST /api/expedientes`) | SOLICITANTE | código `EXP-<año>-<n>`, `PENDIENTE`, área SECRETARIA | PENDIENTE |
| 0b | Recibir: tras cada derivación, la bandeja del área destino lo lista (`GET /api/expedientes`) y la del origen ya no | rol destino y rol origen | aparece solo en el área destino | PENDIENTE |
| 0c | Consultar seguimiento (`GET /api/expedientes/:id/derivaciones`) **con el publicador deshabilitado** | SECRETARIA | **no** muestra los movimientos nuevos (H-03, esperado) | PENDIENTE |
| 0d | Consultar seguimiento **tras activar el publicador** (paso 9 de §5) | SECRETARIA | movimientos en orden de secuencia, clase `MOVIMIENTO` | PENDIENTE (bloqueado por C-01) |
| 1 | F2 (CERTIFICADO): SECRETARIA deriva a DIRECTOR | SECRETARIA | `PENDIENTE→EN_PROCESO`, área DIRECTOR | PENDIENTE |
| 2 | F2: DIRECTOR deriva a SECRETARIA | DIRECTOR | área SECRETARIA, `EN_PROCESO` | PENDIENTE |
| 3 | F2: SECRETARIA resuelve | SECRETARIA | `ATENDIDO` | PENDIENTE |
| 4 | F4 (PERMISO): SECRETARIA→DIRECTOR→SUBDIRECTOR→DOCENTE | cada rol | último paso `ATENDIDO` en DOCENTE | PENDIENTE |
| 5 | F4 nuevo, registrado por la cuenta SOLICITANTE y derivado a DIRECTOR: rechazo en `EN_PROCESO` | DIRECTOR | área SECRETARIA, `OBSERVADO` | PENDIENTE |
| 6 | Corrección del expediente del caso 5 por su dueño | SOLICITANTE | `PENDIENTE`, descripción nueva | PENDIENTE |
| 7 | Cambio de estado `PENDIENTE→OBSERVADO` | SECRETARIA | `OBSERVADO` | PENDIENTE |
| 8 | Repetir la misma operación con la misma clave (reintento) | cualquiera | respuesta con `"repetido": true`, sin movimiento nuevo | PENDIENTE |
| 9 | Operar un expediente de otra área | DOCENTE | rechazado (sin permiso), sin cambios | PENDIENTE |
| 10 | Tras cada caso con cambio: el expediente tiene movimiento con secuencia contigua y evento de outbox `PENDIENTE` (publicador deshabilitado) | administrador, consulta de solo lectura | 1 movimiento + 1 evento por operación | PENDIENTE |

Registrar fecha, responsable y resultado de cada fila. Una prueba solo se marca REALIZADA con su
evidencia (captura o salida de la consulta); documentarla aquí no equivale a haberla ejecutado.

**C-01 sigue BLOQUEANTE** para el paso 9 de §5: sin una decisión institucional sobre quién es el
administrador de integraciones y con qué permisos (`integraciones.manage`), no se crea el cliente
técnico ni se activa el publicador. Las pruebas de esta matriz sí pueden hacerse con el publicador
deshabilitado.

## 6. Reversión

- Binarios/frontend/YAML: restaurar `bin-85dcee7`, `web-85dcee7`, `config` respaldado.
- Antes de arrancar los binarios `85dcee7`: `docs/operacion/rollback/rollback_expedientes.sql` (quita las guardias 005) y `rollback_derivaciones.sql` (vuelve a permitir `INSERT` de `DERIVACION`). 027, 004 y 006 son compatibles con los binarios anteriores y no se revierten: no se borran movimientos ni outbox.
- Restaurar los dumps solo si hubo corrupción de datos (pérdida de lo registrado después del respaldo).
- Probado en una base desechable (2026-11-10): tras los dos SQL vuelven a funcionar el cambio de estado
  sin movimiento y el `INSERT` de `DERIVACION` (comportamiento de `85dcee7`); movimientos y derivaciones
  siguen siendo append-only; reaplicar 005 y 002 restablece las guardias. Los cambios hechos durante la
  reversión no tienen movimiento: la reconciliación posterior los mostrará como `CADENA_DISCONTINUA`
  o `EXPEDIENTE_INCOHERENTE` (esperado; documentarlos).

Estado de cada hallazgo y evidencia de las pruebas: `docs/operacion/REGISTRO_HALLAZGOS_D8.md`.
El release oficial se compila con §2 **después** del commit que incluya estos cambios; el de prueba
(`ba18882`) solo validó el procedimiento.

## 7. IIS: cabeceras propuestas para la SPA

Plantilla en `docs/operacion/iis/web.config` (rewrite igual al desplegado + CSP). Probar en un
navegador (consola sin violaciones de CSP) antes de reemplazar el archivo del servidor.
`Strict-Transport-Security` solo cuando exista HTTPS.
