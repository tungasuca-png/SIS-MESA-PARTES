# Psicología → Estudiantes con client credentials

Psicología consulta los datos básicos de estudiantes como **sistema consumidor**, con
una identidad propia emitida por Auth, en lugar de reenviar el JWT del usuario.

## Arquitectura

```
Usuario ──JWT humano──► Gateway (REST) ──► Psicología :8086
                                             │  1. valida el JWT humano y los permisos psicologia.*
                                             │
                                             ├─ 2. Auth.IssueServiceToken(client_id, client_secret) ──► Auth :8080
                                             │        ◄── JWT de servicio (5 min, scope estudiantes.basic.view)
                                             │
                                             └─ 3. Estudiantes.GetEstudiantesBasic + "authorization: Bearer <JWT de servicio>"
                                                      ──► Estudiantes :8087 (valida firma, iss, aud, token_type, scope)
```

- Toda la comunicación entre servicios es **gRPC interno** por `127.0.0.1`. El Gateway
  sigue siendo solo la entrada REST del frontend.
- Auth firma los tokens de servicio con `SERVICE_JWT_SECRET`, distinta de `JWT_SECRET`:
  ningún otro servicio los acepta como token de usuario.

## API gRPC utilizada

| Operación | Uso en Psicología | Permiso |
|---|---|---|
| `Auth.IssueServiceToken` | Obtener el token de servicio | Público (credenciales del cliente) |
| `Estudiantes.GetEstudiantesBasic` | `Exists` (abrir expediente) y `NombreCompleto` (Hoja de Atención) | `estudiantes.view` o `estudiantes.basic.view` |

- `GetEstudiantesBasic` recibe hasta 100 UUID. Los inexistentes **no aparecen** en la
  respuesta: así se distingue "no existe" de un error. Un UUID inválido responde
  `InvalidArgument`.
- Psicología lee `nombre_completo` (con respaldo en `nombres` + `apellidos`). Para un
  sistema sin `estudiantes.view`, Estudiantes responde solo `id`, `nombre_completo`,
  `grado` y `seccion` (ver [api-estudiantes-solo-lectura.md](api-estudiantes-solo-lectura.md)).
- `ResolveEstudiante` también acepta `basic.view`, pero Psicología no lo usa.
- `GetEstudiante` (registro completo) exige `estudiantes.view`: con el token de
  Psicología responde `PermissionDenied`.

## Cliente y permisos

- Sistema consumidor **"Servicio Psicologia"**, registrado con
  `backend/services/auth/cmd/clientes-servicio` (requiere `integraciones.manage`).
- Permiso único: `estudiantes.basic.view`. Auth guarda solo el hash SHA-256 del secreto.
- Los usuarios de Psicología **ya no necesitan** `estudiantes.view`. Quién puede abrir un
  expediente o descargar una Hoja de Atención lo deciden los permisos `psicologia.*`,
  que se comprueban **antes** de consultar Estudiantes.

## Configuración

Variables en `backend/.env` (ignorado por git; `run-all.ps1` las carga). Plantilla en
`backend/.env.example`.

| Variable | Valor | Uso |
|---|---|---|
| `PSICOLOGIA_CLIENT_ID` | `cli_…` | Credencial del cliente (obligatoria) |
| `PSICOLOGIA_CLIENT_SECRET` | `cs_…` | Credencial del cliente (obligatoria, nunca en git ni en logs) |
| `PSICOLOGIA_ALLOW_LEGACY_AUTH` | `false` (por defecto) o `true` | Modo anterior, solo en desarrollo |

`etc/psicologia.yaml` las lee como `ServiceClientId`, `ServiceClientSecret` y
`AllowLegacyAuth`. Esta última va entre comillas en el YAML para leerse como texto.

## Política de arranque

La decide `svc.ResolverModoEstudiantes` según el `Mode` de go-zero del YAML:

| Situación | Resultado |
|---|---|
| Ambas credenciales | **Sistema consumidor** (en cualquier `Mode`) |
| Sin credenciales | **No inicia** |
| Solo una de las dos | **No inicia** |
| Sin credenciales + `PSICOLOGIA_ALLOW_LEGACY_AUTH=true` con `Mode: dev` o `test` | Modo anterior: reenvía el JWT del usuario |
| Sin credenciales + `PSICOLOGIA_ALLOW_LEGACY_AUTH=true` con otro `Mode` (p. ej. `pro`) | **No inicia** |
| `PSICOLOGIA_ALLOW_LEGACY_AUTH` con un valor distinto de `true`/`false` | **No inicia** |

El modo en uso se registra al arrancar (`Estudiantes: consultas como … (Mode …)`), sin
valores. Los mensajes de error nunca incluyen credenciales.

## Ciclo de vida del token (`internal/svc/servicetoken.go`)

- **Emisión:** `Auth.IssueServiceToken` con un contexto saliente limpio (nunca lleva el
  JWT del usuario).
- **Almacenamiento:** solo en memoria, protegido por un mutex. Las emisiones
  concurrentes se serializan: 20 llamadas simultáneas producen una sola emisión.
- **Reutilización:** se usa el mismo token mientras esté vigente.
- **Renovación anticipada:** 60 s antes de vencer (o a la mitad de su vida si dura
  2 min o menos). Si la renovación falla y el token aún no venció, se sigue usando
  hasta su vencimiento real.
- **Rechazo de Estudiantes:** ante `Unauthenticated`, se descarta el token, se pide otro
  y se reintenta **una sola vez**.
- **Errores:** un fallo de Auth, o un rechazo de Estudiantes a la credencial del
  servicio, llega al usuario como `Unavailable`, no como 401/403, para que el frontend
  no cierre su sesión. Ningún error se interpreta como "el estudiante no existe".
- **Logs:** nunca se registran el secreto ni los tokens. El contenido de
  `IssueServiceToken` y `GetEstudiantesBasic` está excluido de los logs del cliente
  gRPC.

## Auditoría

| Dónde | Identidad registrada |
|---|---|
| `auditoria_psicologica` (Psicología) | El **usuario humano** (p. ej. `EXPORT` al descargar una Hoja de Atención) |
| Log de Estudiantes, "acceso de sistema consumidor" | El **sistema**: `client_id`, operación, resultado, fecha y duración |
| Log de Auth | Emisiones y rechazos de token, solo con el `client_id` |

## Pruebas

```bash
cd backend/services/psicologia
go vet ./...
go test ./...      # sin INTEGRATION_TEST: sin base de datos
PSICOLOGIA_E2E_REAL=true go test -run TestE2E_ -v ./internal/svc/   # real, solo lectura
```

- **Unitarias**, con dobles de Auth y Estudiantes:
  - `internal/svc/servicetoken_test.go`: emisión, reutilización, renovación,
    concurrencia, fallo de Auth, errores de Estudiantes, metadata y logs.
  - `internal/svc/modoestudiantes_test.go`: política de arranque y modo anterior.
  - `internal/logic/permisos_servicio_test.go`: los permisos humanos se comprueban
    antes de consultar Estudiantes.
  - `internal/config/allowlegacy_test.go`: carga del YAML real.
- **E2E real** (`internal/svc/servicetoken_e2e_test.go`): solo lectura contra Auth y
  Estudiantes en ejecución. Se salta sin `PSICOLOGIA_E2E_REAL=true`.

## Validación realizada (29–30/10/2026)

- **Prueba E2E real:** pasó. Token emitido y reutilizado; estudiante existente e
  inexistente; `GetEstudiante` rechazado con `PermissionDenied`.
- **Dos descargas reales** de una Hoja de Atención desde la interfaz:
  - Estudiantes registró `GetEstudiantesBasic OK` del cliente.
  - Psicología registró `EXPORT` con el usuario.
  - Se observó la renovación anticipada del token.
- **Sin cambios de datos:** comprobado con consultas de solo lectura. No hubo
  modificaciones en expedientes, atenciones ni estudiantes.

## Limitaciones

1. **Abrir un expediente no se probó en vivo** (`Exists` dentro de `OpenExpediente`): es
   una escritura y no existe una base de pruebas aislada (`.env.test` apunta a las
   bases reales). Está cubierto por pruebas unitarias.
2. **Las pruebas de integración con base de datos están pendientes** por el mismo
   motivo. Estrategia para hacerlas en un entorno aislado:
   - una base `psicologia_db` y `estudiantes_db` desechable;
   - un cliente de servicio de prueba;
   - `INTEGRATION_TEST=true` con `TEST_*_DB_URL` apuntando a esas bases;
   - abrir un expediente con un estudiante sembrado y verificar ambas auditorías.
3. **Minimización de datos (resuelta el 30/10/2026):** a un sistema sin
   `estudiantes.view`, Estudiantes ya no le entrega el documento ni otros datos que no
   usa; los usuarios conservan la respuesta completa. Requiere desplegar Psicología
   (lector de `nombre_completo`) **antes** que Estudiantes.
4. **Revocación:** la de un cliente surte efecto cuando vence el token vigente
   (máximo 5 min con la configuración actual).
