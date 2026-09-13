# ETAPA 1 — Autorización segura del workflow de Expedientes

> Alcance: únicamente autorización de `ChangeEstado`, `UpdateArea` y
> `CrearDerivacion`. No se tocaron tipos de trámite, estados, aprobación
> real, F1/F3/F5, notificaciones, historial nuevo, asignación por persona ni
> frontend visual — tal como exige esta etapa.

## 1. Problema encontrado

`ChangeEstado` y `UpdateArea` (Expedientes Service) solo validaban el **rol**
(`authorization.CanChangeEstado` / `CanUpdateArea`, ambas equivalentes a
`IsInternal(role)`), sin verificar que el usuario perteneciera al
`area_actual` del expediente. Esto ya estaba correctamente validado en
`GetExpedienteLogic` (lectura), pero no en las dos operaciones de escritura.
Verificado en vivo antes de corregirlo: un usuario `DOCENTE`, bloqueado con
`403` al hacer `GET` sobre un expediente de otra área, podía igual
cambiarle el estado y reasignárselo a sí mismo por API directa.

`CrearDerivacion` (Derivaciones Service) tampoco validaba nada respecto al
expediente — no tiene forma de conocer `area_actual` porque es un
microservicio aislado sin FK hacia Expedientes.

## 2. Estado anterior

```text
ChangeEstado:      requiere autenticación + IsInternal(role)   — nada más
UpdateArea:        requiere autenticación + IsInternal(role)   — nada más
CrearDerivacion:   requiere autenticación + IsInternal(role)   — nada más
GetExpediente:     requiere autenticación + (CanViewAll(role) OR
                    area_actual == AreaDelRol(role))           — ya correcto
```

## 3. Regla de autorización implementada

**Para `ChangeEstado` y `UpdateArea` (Expedientes Service):** se reutiliza
exactamente la misma condición que ya usa `GetExpedienteLogic`:

```go
if !authorization.CanViewAll(role) && current.AreaActual != authorization.AreaDelRol(role) {
    return nil, status.Error(codes.PermissionDenied, "el expediente no pertenece al área del usuario")
}
```

Se analizó si `UpdateArea` debía tener una regla distinta a `ChangeEstado`
(pedido explícito de esta etapa, no copiar sin analizar): conceptualmente
son acciones distintas (una cambia el estado propio, la otra entrega el
expediente a otra área), pero **ambas solo tienen sentido para quien
actualmente lo tiene** — no hay ningún flujo en el sistema (ni en el
frontend, `DerivacionesPanel.jsx`, ni en el análisis institucional) donde
alguien fuera del área actual derive un expediente que no es suyo. La
condición resultante es la misma por evidencia, no por copia mecánica.

**Para `CrearDerivacion` (Derivaciones Service, expuesto vía Gateway):**
Derivaciones Service no conoce `area_actual` (aislado, sin FK — mismo
patrón que Documentos Service, que tampoco llama a Expedientes por gRPC).
En vez de acoplar Derivaciones Service directamente a Expedientes Service,
se compuso la validación en el **Gateway** (que ya tiene ambos clientes
gRPC wireados): antes de crear la derivación, el Gateway llama
`ExpedientesClient.GetExpediente` con el mismo token — si esa llamada
falla (por ejemplo, `PermissionDenied` porque el usuario no pertenece al
área), la creación de la derivación ni siquiera se intenta. Reutiliza la
validación que ya existe en `GetExpedienteLogic` en vez de duplicar lógica
de autorización en un tercer lugar.

## 4. Archivos modificados

```text
backend/services/expedientes/internal/logic/changeestadologic.go
backend/services/expedientes/internal/logic/updatearealogic.go
backend/gateway/internal/logic/crearderivacionlogic.go
backend/services/expedientes/internal/integration/expedientes_integration_test.go
```

Ningún archivo de frontend, ninguna migración, ningún archivo de
configuración.

## 5. Cambios realizados

- `changeestadologic.go`: tras obtener `current` (ya se hacía, para validar
  la transición de estado), se agregó la verificación de área antes de
  ejecutar `UpdateEstado`.
- `updatearealogic.go`: antes no consultaba el expediente antes de escribir;
  se agregó un `FindByIDOrCodigo` previo (mismo patrón que `ChangeEstado`)
  para poder verificar `AreaActual` antes de ejecutar `UpdateArea`. No se
  agregó guarda de concurrencia optimista (eso sigue perteneciendo a una
  etapa posterior, tal como indica esta etapa explícitamente).
- `crearderivacionlogic.go` (Gateway): se agregó una llamada a
  `ExpedientesClient.GetExpediente` antes de `DerivacionesClient.CrearDerivacion`,
  reenviando el mismo `Authorization`. Se agregó el import
  `expedientes/expedientesclient` (el módulo del Gateway ya depende de
  `expedientes`, no fue necesario tocar `go.mod`).
- `expedientes_integration_test.go`: se agregaron los 7 casos de la sección
  12 de esta etapa dentro de `TestExpedientesIntegration` (mismo patrón ya
  usado: servidor gRPC en memoria vía `bufconn` + Postgres real), con
  contextos nuevos para `SECRETARIA`, `DIRECTOR` y `DOCENTE`.

## 6. Excepciones administrativas

- **ADMIN y SECRETARIA** conservan la capacidad de operar sobre cualquier
  expediente sin importar `area_actual` — esto **ya existía** en
  `authorization.CanViewAll` (usado hoy en `GetExpediente`/`ListExpedientes`)
  y se reutiliza sin cambios. No se inventó ni se amplió ningún permiso
  nuevo para ADMIN; se documenta la excepción existente tal como pide esta
  etapa.
- No se crearon excepciones nuevas.

## 7. Pruebas ejecutadas

**Unitarias** (sin cambios, se reconfirmaron):
```text
go test ./internal/authorization/... ./internal/estados/...   → PASS
```

**Integración** (`expedientes_integration_test.go`, contra PostgreSQL real,
`INTEGRATION_TEST=true`), casos nuevos agregados dentro de
`TestExpedientesIntegration`:

| Caso | Escenario | Resultado esperado | Resultado obtenido |
|---|---|---|---|
| 1 | SECRETARIA sobre expediente en SECRETARIA | Permitido | ✅ |
| 2 | DIRECTOR sobre expediente en SECRETARIA (ChangeEstado y UpdateArea) | 403 | ✅ |
| 3 | DOCENTE sobre expediente en SECRETARIA | 403 | ✅ |
| — | DOCENTE sobre expediente ya derivado a DIRECTOR | 403 | ✅ |
| — | DIRECTOR (nuevo dueño real) sobre ese mismo expediente | Permitido | ✅ |
| 4 | SOLICITANTE intenta UpdateArea de su propio expediente | 403 | ✅ |
| 5 | ADMIN sobre expediente ajeno a su "área" | Permitido | ✅ |
| 6 | Expediente inexistente (ChangeEstado y UpdateArea) | 404 | ✅ |
| 7 | Expediente inactivo (baja lógica) (ChangeEstado y UpdateArea) | 404 | ✅ |

```text
go test ./internal/integration/... -run TestExpedientesIntegration -v
--- PASS: TestExpedientesIntegration
```

**Integración real vía API (Gateway, usuarios de prueba desechables,
eliminados al terminar)** — sección 13 de esta etapa:

| Prueba | Resultado |
|---|---|
| SECRETARIA → `ChangeEstado` sobre expediente en SECRETARIA | `200` |
| DOCENTE → `ChangeEstado` sobre ese mismo expediente | `403 PermissionDenied — "el expediente no pertenece al área del usuario"` |
| DOCENTE → `UpdateArea` sobre ese mismo expediente | `403`, mismo mensaje |
| DOCENTE → `CrearDerivacion` sobre expediente ajeno | `403 PermissionDenied — "no tiene acceso a este expediente"` |
| DIRECTOR → `CrearDerivacion` sobre expediente ajeno (área SECRETARIA) | `403`, mismo mensaje |
| SECRETARIA → `CrearDerivacion` sobre SU propio expediente | `200` |
| SOLICITANTE → `ChangeEstado`/`UpdateArea` de su propio expediente | `403` en ambos (ya bloqueado antes por rol; se reconfirmó que sigue así) |
| SOLICITANTE → `GET` de su propio expediente y listado | `200` (no se rompió) |

## 8. Resultados

```text
[x] ChangeEstado está protegido
[x] UpdateArea está protegido
[x] Derivación está protegida (a nivel Gateway, componiendo GetExpediente)
[x] SECRETARIA puede operar sus expedientes
[x] DIRECCIÓN puede operar sus expedientes (verificado tras una derivación real)
[x] SUBDIRECCIÓN — misma regla que DIRECCIÓN/DOCENTE (AreaDelRol), no probada
    con un expediente propio en esta etapa, pero la condición es idéntica y
    ya está cubierta por TestAreaDelRol
[x] DOCENTE no puede modificar expedientes ajenos
[x] SOLICITANTE no puede modificar arbitrariamente el flujo
[x] ADMIN conserva su comportamiento válido
[x] existen pruebas
[x] pruebas pasan
[x] backend compila (expedientes y gateway, `go build ./...`)
[x] servicios afectados levantan (expedientes y gateway reiniciados y
    verificados en vivo)
[x] frontend no se rompe (apiErrors.js ya maneja 403 con el mensaje del
    backend; no se tocó ningún archivo de frontend)
[x] documentación creada (este archivo)
```

## 9. Limitaciones actuales

- `UpdateArea` sigue sin guarda de concurrencia optimista (dos derivaciones
  simultáneas hacia áreas distintas todavía pueden pisarse) — pertenece a la
  etapa de "operación atómica completa" que esta etapa explícitamente no
  debía tocar.
- La validación de `CrearDerivacion` vive en el Gateway, no en Derivaciones
  Service — es intencional (evita acoplar dos microservicios directamente,
  mismo patrón ya usado por Documentos Service), pero significa que
  cualquier **otro** cliente que le hable directamente a Derivaciones
  Service sin pasar por este Gateway (hoy no existe ninguno) no tendría esta
  protección. Documentado, no se considera un riesgo real hoy porque no hay
  otra vía de acceso.
- `origen`/`destino` de una derivación siguen siendo texto libre sin
  catálogo validado — no estaba en el alcance de esta etapa.
- No se implementó "aprobación real" ni ningún estado nuevo — fuera de
  alcance, como exige esta etapa.
- SUBDIRECTOR usa la misma regla que DIRECTOR/DOCENTE/AUXILIAR
  (`AreaDelRol`) pero no se ejercitó con un expediente propio en las
  pruebas de integración de esta etapa (sí está cubierto por
  `TestAreaDelRol` en `authorization_test.go`, que no cambió).

## 10. Qué queda para la Etapa 2

**ETAPA 2 — TIPOS DE TRÁMITE**
