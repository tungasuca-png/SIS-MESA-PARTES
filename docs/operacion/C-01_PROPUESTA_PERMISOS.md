# C-01 — Administrador de integraciones y matriz de permisos (PROPUESTA, pendiente de aprobación)

**Estado: BLOQUEANTE para activar el publicador D8. Nada de este documento está aplicado.**
Requiere una decisión firmada por la Dirección del Colegio Tungasuca. Este documento no crea
roles, no asigna permisos y no contiene ninguna migración ejecutable.

## 1. El problema, verificado

`clientes-servicio crear` (alta del cliente técnico del publicador) llama a `CreateServiceClient`,
que el interceptor de Auth protege con `integraciones.manage`
(`backend/services/auth/internal/interceptor/interceptor.go`). Instalando todas las migraciones
del repositorio en una base desechable (2026-11-10):

| Comprobación | Resultado |
|---|---|
| Roles que crean las migraciones | `PSICOLOGO`, `SOLICITANTE` (nada más) |
| Roles con `integraciones.manage` | 0 |
| Permisos asignados a algún rol | solo `SOLICITANTE`: `dashboard.view`, `documentos.create`, `documentos.view_own`, `expedientes.view_own`, `seguimiento.view_own`, `solicitudes.create` |
| `derivaciones.proyectar` en un rol humano | **rechazado por la base** (trigger de la migración 027) |

Sin un usuario humano con `integraciones.manage` no se puede registrar el cliente técnico, y sin
él el publicador no obtiene token: Expedientes registra los movimientos, pero la proyección y el
seguimiento no se actualizan (H-03 en `RELEASE_D8.md`).

## 2. Lo que ya está garantizado técnicamente (no depende de la decisión)

- Un rol humano **no puede** recibir `derivaciones.proyectar` (trigger de 027, probado).
- El token del publicador se firma con un secreto exclusivo de la audiencia `derivaciones`; los
  validadores humanos de los siete servicios rechazan cualquier token con `token_type` (B-01).
- `AplicarMovimiento` solo acepta tokens de servicio con ese scope; ningún usuario humano, ni ADMIN,
  puede invocarlo.
- La audiencia de un cliente técnico es inmutable y solo admite `estudiantes` o `derivaciones`
  (probado). Un cliente de `estudiantes` nunca recibe `derivaciones.proyectar` en su token.
- Tener `integraciones.manage` permite **registrar y revocar** clientes técnicos; no da acceso a
  expedientes ni a la proyección. Aun así es un privilegio alto: quien lo tenga puede emitir
  credenciales técnicas.

## 3. Opciones para la decisión institucional

| Opción | Descripción | Valoración técnica |
|---|---|---|
| **A (recomendada)** | Un rol propio, p. ej. `ADMIN_INTEGRACIONES`, con **solo** `integraciones.manage`, asignado a **una** persona designada. Se usa para registrar el cliente del publicador y, si se decide, se retira después | Mínimo privilegio; separa la emisión de credenciales técnicas de la gestión documental |
| B | Dar `integraciones.manage` al rol `ADMIN` | Más simple; concentra privilegios en un rol que ya ve todos los expedientes |
| C | Alta directa en la base por el DBA (como `auth_owner`), sin pasar por Auth | **No recomendada**: no deja el evento de auditoría de Auth y salta las validaciones de audiencia y permisos del código |

## 4. Matriz de permisos D8 PROPUESTA (marcar SÍ/NO y firmar)

Permisos que el código exige para cada operación de Mesa de Partes (verificado en
`backend/services/expedientes/internal/logic` y `backend/services/derivaciones/internal/logic`). El área del expediente debe coincidir con la del
rol, salvo `ADMIN`. La columna "Propuesta" es solo una base para discutir: **no está aprobada**.

| Permiso | Operaciones | SECRETARIA | DIRECTOR | SUBDIRECTOR | DOCENTE | AUXILIAR | ADMIN |
|---|---|---|---|---|---|---|---|
| `expedientes.create` | registrar (personal interno) | ? | ? | ? | ? | ? | ? |
| `expedientes.view` | ver la bandeja del área | ? | ? | ? | ? | ? | ? |
| `expedientes.update` | derivar, rechazar | ? | ? | ? | ? | ? | ? |
| `expedientes.change_estado` | cambiar estado, resolver | ? | ? | ? | ? | ? | ? |
| `derivaciones.view` | consultar el seguimiento/historial (lo exige Derivaciones) | ? | ? | ? | ? | ? | ? |
| `seguimiento.view` | solo muestra la opción de menú en el frontend (`navItems.js`); el backend no lo comprueba | ? | ? | ? | ? | ? | ? |
| `derivaciones.create` | anotaciones en el historial | ? | ? | ? | ? | ? | ? |
| `integraciones.manage` | registrar/revocar clientes técnicos | — | — | — | — | — | según opción A/B |

`SOLICITANTE` conserva lo que ya asignan las migraciones (`solicitudes.create`,
`expedientes.view_own`, `seguimiento.view_own`, …). El cliente técnico del publicador solo puede
tener `derivaciones.proyectar`, y la base impide que ese permiso llegue a una persona.

## 5. Procedimiento técnico, una vez aprobada la decisión

Lo ejecuta el administrador autorizado, en la ventana de §5 de `RELEASE_D8.md`:

1. Crear el rol y la cuenta aprobados con el procedimiento institucional habitual (no por SQL ad hoc).
2. Iniciar sesión con esa cuenta y obtener su access token (vida corta).
3. En el servidor, en una sesión interactiva:
   ```powershell
   $t = Read-Host 'Access token del administrador de integraciones' -AsSecureString
   $env:AUTH_ADMIN_TOKEN = [Runtime.InteropServices.Marshal]::PtrToStringBSTR([Runtime.InteropServices.Marshal]::SecureStringToBSTR($t))
   try { & 'C:\SIS-MESA-PARTES-DEPLOY\bin\clientes-servicio.exe' crear -nombre 'Publicador de Expedientes' -audiencia derivaciones -permisos derivaciones.proyectar }
   finally { Remove-Item Env:AUTH_ADMIN_TOKEN }
   ```
   El `client_secret` se muestra **una sola vez**: copiarlo directamente a `secrets\backend.env`
   (`EXPEDIENTES_PUBLICADOR_CLIENT_ID` / `_CLIENT_SECRET`), sin pegarlo en chat, correo ni tickets.
4. Paso 9 de §5 de `RELEASE_D8.md`: `Publicador.Habilitado: true`, reiniciar solo Expedientes y
   comprobar que el outbox llega a 0 pendientes y que el seguimiento muestra los movimientos.
5. Si la opción A prevé retirar el permiso después, hacerlo y dejar constancia.

Revocación de emergencia: `clientes-servicio revocar -id <client_id>` (mismo permiso); el token
vigente caduca en ≤ 15 min.

## 6. Autorización que falta

- Elegir la opción (A, B u otra) y designar a la persona.
- Aprobar la matriz del punto 4 (también desbloquea auth 011/021 y las cuentas de prueba de §5.1).
