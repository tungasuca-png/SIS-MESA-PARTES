# ETAPA 1.1 — Cierre de seguridad

> Cierra las 3 observaciones pendientes de
> `docs/etapa-1-autorizacion-expedientes.md`. No se implementó Etapa 2 ni
> ningún workflow nuevo — mismo alcance restringido que la Etapa 1.

## 1. Concurrencia en `UpdateArea`

**Antes:** `UpdateArea` sobrescribía `area_actual` sin condición (`WHERE id
= $1 AND activo = TRUE`), a diferencia de `UpdateEstado`, que ya exigía el
`estado` esperado como guarda optimista.

**Mecanismo elegido:** el mismo que ya usa `UpdateEstado` — el valor previo
del propio campo (`area_actual`) como condición del `UPDATE`, no
`fecha_actualizacion` ni una columna de versión nueva. Se prefirió por tres
razones: (1) es el patrón que el proyecto ya usa para esto exacto, (2) una
comparación de timestamp es más frágil (colisiones al mismo milisegundo,
relojes), (3) agregar una columna de versión hubiera significado una
migración nueva, fuera de lo permitido en esta etapa ("no cambiar
PostgreSQL innecesariamente").

```text
lectura válida        → UpdateAreaLogic ya leía el expediente para la
                         verificación de área (Etapa 1); se reutiliza esa
                         misma lectura como version_check
verificar versión      → se pasa current.AreaActual como "área anterior
                         esperada"
actualizar              → UPDATE ... WHERE area_actual = <área anterior
                         esperada>
si cambió entretanto   → 0 filas afectadas → ErrExpedienteNotFound →
                         codes.Aborted ("el área del expediente cambió,
                         intente nuevamente")
```

**Archivos:**
- `backend/services/expedientes/internal/repository/expediente_repository.go`
  — `UpdateArea` ahora recibe `areaAnterior` y agrega `AND area_actual = $3`
  al `UPDATE` (mismo patrón que `UpdateEstado`, líneas contiguas en el mismo
  archivo).
- `backend/services/expedientes/internal/logic/updatearealogic.go` — pasa
  `current.AreaActual` (ya leído para la verificación de la Etapa 1) al
  repositorio; mapea el conflicto a `codes.Aborted`, igual que
  `ChangeEstadoLogic` ya hacía para estado.

**Prueba:** en `expedientes_integration_test.go` se simula el conflicto
operando el repositorio directamente (mismo método que usa la lógica real):
"Request A" lee `area_actual=SECRETARIA`; "Request B" deriva primero a
`DIRECTOR`; "Request A" intenta escribir con su lectura vieja
(`SECRETARIA`) → se rechaza con `ErrExpedienteNotFound`; se verifica que el
área final quede en `DIRECTOR` (la de B), no en la escritura rechazada de A
ni en la original. Se eligió simular el conflicto así (no con una carrera
real de goroutines) para que la prueba sea determinística y no intermitente
— sigue probando el mismo código SQL que ejecuta `UpdateAreaLogic`.

Se hizo además una verificación en vivo (10 `PATCH /area` concurrentes vía
Gateway sobre el mismo expediente): las 10 se sirvieron correctamente sin
errores espurios, serializadas por el propio locking de fila de PostgreSQL,
con un estado final consistente — confirma que la guarda no rompe el uso
concurrente legítimo.

## 2. Derivación — ¿la protección en Gateway es segura?

**Se determinó que NO es completa**, y se corrigió con el cambio mínimo
posible sin mover lógica de negocio entre microservicios.

Verificado en vivo (bypass real confirmado antes de corregir): un cliente
que le habla **directamente** al puerto gRPC de Derivaciones Service
(`127.0.0.1:8085`, sin pasar por el Gateway) podía crear una derivación
sobre un expediente ajeno, porque la validación de área vive en el Gateway
(`CrearDerivacionLogic`, componiendo `GetExpediente`), no en Derivaciones
Service. Prueba: un token DOCENTE, dialeando gRPC directo con
`google.golang.org/grpc`, creó una derivación sobre un expediente en área
SECRETARIA sin ningún rechazo.

**Por qué no se movió la lógica a Derivaciones Service:** eso exige que
Derivaciones Service conozca `area_actual`, lo cual requiere un cliente
gRPC hacia Expedientes Service (una dependencia cruzada nueva) — explícita
y correctamente fuera de alcance de esta etapa ("no mover lógica entre
microservicios automáticamente", "no convertir la derivación en operación
atómica todavía").

**Cambio mínimo aplicado:** `backend/services/derivaciones/etc/derivaciones.yaml`
— `ListenOn` pasó de `0.0.0.0:8085` a `127.0.0.1:8085`. Es un cambio de
configuración de red, no de lógica: reduce la superficie real de ataque
(ya no es alcanzable desde otra máquina de la red), sin tocar ningún
archivo `.go`. Se verificó que el Gateway sigue funcionando (ya se conecta
vía `127.0.0.1:8085`, confirmado en `gateway/etc/gateway-api.yaml`,
`DerivacionesRpc.Target`) — no fue necesario cambiar nada más.

**Limitación reconocida y documentada, no oculta:** este cambio no cierra
un bypass desde la **misma máquina** (loopback sigue siendo loopback,
alcanzable con o sin el bind a `0.0.0.0`) — un proceso que corra en el
mismo host que Derivaciones Service seguiría pudiendo saltarse el Gateway.
Cerrarlo completamente requiere la validación dentro del propio servicio
(la dependencia cruzada explícitamente diferida) o un mecanismo de
autenticación de servicio a servicio (mTLS, red privada de contenedores en
producción) — fuera de alcance de esta etapa. La mitigación real y completa
en un despliegue de producción es de infraestructura (red privada /
firewall entre Gateway y los servicios RPC), no de código de aplicación.

## 3. Prueba explícita de SUBDIRECTOR

Agregada a `expedientes_integration_test.go` (contexto `ctxSubdirector`
nuevo, mismo patrón que los demás roles):

- `SUBDIRECTOR` sobre expediente con `area_actual=SECRETARIA` →
  `PermissionDenied` (ChangeEstado y UpdateArea).
- `SECRETARIA` deriva el expediente a `SUBDIRECTOR`.
- `SUBDIRECTOR` sobre ese mismo expediente (ya en su área) → éxito.

Verificado también en vivo vía API real (Gateway, usuario de prueba
desechable): mismos tres resultados confirmados con `curl`.

## 4. No modificado

Confirmado que esta etapa no tocó: tipos de trámite, aprobación,
desaprobación, observaciones nuevas, historial, notificaciones, asignación
por persona, F1/F3/F5, estados nuevos, ni frontend visual. La autorización
ya implementada en la Etapa 1 (validación de área en `ChangeEstado`/
`UpdateArea`, composición en Gateway para `CrearDerivacion`) no se rehizo,
solo se completó.

## 5. Validación final

```text
go test ./...  (expedientes, derivaciones, gateway)  → PASS / sin tests / ok
INTEGRATION_TEST=true go test ./internal/integration/... -run TestExpedientesIntegration -v
  → PASS (incluye los casos nuevos de SUBDIRECTOR y concurrencia)
```

| Verificación | Resultado |
|---|---|
| ChangeEstado continúa protegido | ✅ (403 para DOCENTE/SUBDIRECTOR ajenos, verificado en vivo) |
| UpdateArea continúa protegido | ✅ |
| CrearDerivacion continúa protegido | ✅ (403 en Gateway; bypass directo a Derivaciones Service cerrado a nivel de red) |
| SECRETARIA funciona | ✅ |
| DIRECTOR funciona en su área | ✅ (ya verificado en Etapa 1, sin regresión) |
| SUBDIRECTOR funciona en su área | ✅ (nuevo en esta etapa) |
| Usuarios fuera del área reciben 403 | ✅ |
| Conflicto de concurrencia rechazado | ✅ (prueba de integración determinística; sin regresión en 10 llamadas concurrentes legítimas en vivo) |
| Frontend existente no se rompe | ✅ (sin cambios de frontend; `apiErrors.js` ya maneja códigos no listados con un mensaje genérico, incluido el nuevo `Aborted`→409 de `UpdateArea`) |

Servicios afectados (expedientes, derivaciones) reiniciados y verificados
en vivo tras cada cambio; `gateway` reiniciado en la Etapa 1 y no requirió
otro reinicio en esta etapa salvo el ya hecho.
