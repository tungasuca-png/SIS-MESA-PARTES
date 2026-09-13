# ETAPA 3 — Workflow real de expedientes

> Implementación incremental sobre lo ya construido (Etapas 1, 1.1, 2). No
> se reescribió el sistema; se agregó UNA operación de negocio nueva
> (`DerivarExpediente`) que reemplaza el patrón de dos llamadas separadas
> solo para la derivación real (`tipo=DERIVACION`). El resto de tipos de
> derivación (NOTIFICACION/ASIGNACION/RECEPCION/APROBACION) sigue exactamente
> igual que antes.

## 1. Flujo anterior

```text
Frontend (DerivacionesPanel.jsx):
    1. POST /api/expedientes/:id/derivaciones   (Derivaciones Service, solo bitácora)
    2. PATCH /api/expedientes/:id/area          (Expedientes Service, con CAS desde Etapa 1.1)
```

Dos llamadas HTTP independientes, sin ninguna validación de "¿esta
transición tiene sentido para este tipo de trámite?" — cualquier destino
válido (área existente) se aceptaba sin importar el tipo de expediente.

## 2. Problema encontrado

- Si la llamada 2 fallaba después de que la 1 tuvo éxito, quedaba una
  derivación registrada que "decía" que el expediente se movió sin que
  realmente se hubiera movido (ya documentado en
  `docs/etapa-1.1-cierre-seguridad.md`).
- No existía ninguna regla de negocio que impidiera, por ejemplo, derivar
  un `CERTIFICADO` directamente a `SUBDIRECTOR` sin pasar por `DIRECTOR`.

## 3. Arquitectura elegida

**`DerivarExpediente` (nuevo rpc en Expedientes Service)** — autoritativo:
valida propiedad de área (Etapa 1), valida la transición contra
`estados.CanDerivar(tipo, areaActual, areaDestino)` (regla nueva de esta
etapa) y mueve `area_actual` con la misma guarda de concurrencia optimista
que ya usa `UpdateArea` (Etapa 1.1) — de hecho reutiliza el mismo método
del repositorio, no lo duplica.

**Gateway compone** (mismo patrón ya establecido en Etapa 1.1 para
`CrearDerivacion`, y la misma razón por la que Documentos Service no llama
a Expedientes Service directamente): primero llama
`ExpedientesClient.DerivarExpediente` (mueve el área, autoritativo) y solo
si eso tuvo éxito llama `DerivacionesClient.CrearDerivacion` (registra el
historial). **Orden elegido a propósito**: un área movida sin su log
puntual es un problema menor que un log que afirma un movimiento que nunca
ocurrió — ese es exactamente el bug ya documentado en Etapa 1.1. No se
implementó una transacción distribuida (no tiene sentido con bases de
datos separadas por servicio, y no se pidió); si el segundo paso falla, se
devuelve un error explícito que dice que el área SÍ se movió pero el
historial no se pudo registrar — visible, no silencioso.

`UpdateArea` (genérico) **sigue existiendo sin cambios de comportamiento**
para otros usos administrativos que no pasan por esta regla.

## 4. Acciones implementadas

- `PATCH /api/expedientes/:id/derivar` — `{area_destino, motivo, condicion}` → `{expediente, derivacion}`.
- `DerivarExpediente` (Expedientes Service, gRPC interno).
- `estados.CanDerivar(tipoTramite, areaActual, areaDestino) bool`.
- Frontend: `derivarExpediente()` en `expedientesService.js`;
  `DerivacionesPanel.jsx` la usa cuando `tipo === "DERIVACION"` (con un
  selector de área real, `AREAS_DESTINO`, en vez del selector de texto
  libre que se sigue usando para el resto de tipos).

## 5. Transiciones permitidas (REGLA CONFIRMADA)

| Tipo de trámite | Desde | Hacia | Fuente |
|---|---|---|---|
| CERTIFICADO, CONSTANCIA (F2) | SECRETARIA | DIRECTOR | `analisis-5-flujos-completo.md`, FLUJO 2 |
| PERMISO, JUSTIFICACION_FALTA, JUSTIFICACION_TARDANZA (F4) | SECRETARIA | DIRECTOR | ídem, FLUJO 4 |
| PERMISO, JUSTIFICACION_FALTA, JUSTIFICACION_TARDANZA (F4) | DIRECTOR | SUBDIRECTOR | Etapa 12 sección 15 ("derivación aprobatoria") |
| SOLICITUD, TRAMITE, OFICIO, OTRO (genéricos) | cualquier área válida | cualquier área válida | REGLA ACTUAL DEL CÓDIGO (sin regla institucional — se mantiene el comportamiento previo a esta etapa, no se restringe algo que nunca estuvo definido) |

## 6. Transiciones rechazadas (verificado en vivo)

```text
F2 (CERTIFICADO): DIRECTOR -> SUBDIRECTOR           → 400 FailedPrecondition
F4 (JUSTIFICACION_TARDANZA): SECRETARIA -> SUBDIRECTOR (salto)  → 400 FailedPrecondition
Cualquier tipo, usuario fuera del área actual       → 403 PermissionDenied
```

`REGLA PENDIENTE` (no implementada, no inventada): qué transición sigue
para un F2 una vez que Dirección lo recibe (F2-02); qué ocurre exactamente
al "aprobar" un F4 más allá del movimiento de área ya confirmado
(Conflicto F4-01) — el movimiento DIRECTOR→SUBDIRECTOR ya está implementado
porque el dato "quién deriva a quién" SÍ está confirmado en la matriz de
Etapa 12; el *significado* de esa aprobación (si es definitiva o si
Subdirección puede todavía rechazar) NO se implementó.

## 7. Comportamiento de derivación

`DerivarExpediente` NO es una operación 100% atómica en el sentido de
transacción distribuida (los datos viven en dos bases distintas, en dos
microservicios distintos) — es una secuencia ordenada con el paso más
importante (mover el área) primero. Verificado en vivo: el `PATCH
.../derivar` devuelve en una sola respuesta tanto el expediente actualizado
como la derivación creada.

## 8. Comportamiento de bandejas

Sin cambios de código — ya funcionaban correctamente filtrando por
`estado` + `area_actual` (ver etapas anteriores). Verificado en vivo: tras
derivar un expediente de SECRETARIA a DIRECTOR, deja de aparecer en la
bandeja de Secretaría y aparece en la de Dirección, sin ninguna lógica
nueva de bandejas.

## 9. Concurrencia

`DerivarExpediente` reutiliza exactamente el mismo mecanismo CAS de
`UpdateArea` (Etapa 1.1): la escritura exige que `area_actual` siga siendo
la que se leyó al validar la transición; si cambió mientras tanto, se
rechaza (`Aborted`, o `PermissionDenied` si el área cambió lo suficiente
para que el actor ya no sea dueño — ambos casos probados en la prueba de
integración).

## 10. Pruebas

**Unitarias** (`internal/estados/estados_test.go`): `TestCanDerivar` — F2
(solo SECRETARIA→DIRECTOR), F4 (las dos transiciones confirmadas, más el
salto directo rechazado), genéricos (permisivos), área inválida siempre
rechazada.

**Integración** (`expedientes_integration_test.go`, PostgreSQL real):
- F2: DOCENTE ajeno → `PermissionDenied`; SECRETARIA→DIRECTOR → éxito;
  DIRECTOR→SUBDIRECTOR (no confirmado para F2) → `FailedPrecondition`.
- F4: SECRETARIA→SUBDIRECTOR directo → `FailedPrecondition`;
  SECRETARIA→DIRECTOR → éxito; DIRECTOR→SUBDIRECTOR → éxito.
- Genérico (SOLICITUD): cualquier área → éxito (sin romper el registro
  interno existente).
- Expediente inexistente → `NotFound`.
- Concurrencia: área cambiada por otra operación entre la lectura y el
  intento de derivar → `PermissionDenied` (el actor ya no es dueño del
  área nueva).

```text
go test ./...  (expedientes, derivaciones, gateway)                    → PASS
INTEGRATION_TEST=true go test ./internal/integration/... -v            → PASS
```

**API real** (Gateway, usuarios de prueba desechables, eliminados al
terminar): F2 completo (crear→derivar→rechazo de transición no confirmada)
y F4 completo (crear→rechazo de salto→derivar→derivar de nuevo→historial
con las 2 derivaciones en orden) — todo verificado con `curl` contra los
servicios reales.

**Prueba end-to-end** (navegador real, `browser-automation`, usuarios de
prueba desechables):
```text
FUT (SOLICITANTE) → selecciona CERTIFICADO → Presentar solicitud
    → EXP-2026-000084 creado
Login SECRETARIA → abre el expediente → panel de Derivaciones
    → tipo=DERIVACION, destino=Dirección, motivo → Registrar derivación
    → confirmado por API: area_actual=DIRECTOR, derivación en el historial
```
0 errores de consola, 0 requests fallidos.

## 11. Reglas pendientes (consolidado, ninguna inventada)

- F2-02: qué transición/acción cierra formalmente un F2 una vez que
  Dirección lo recibe.
- F4-01: si Dirección decide en firme o Subdirección tiene veto real sobre
  la aprobación (el movimiento de área SÍ está implementado; su
  significado de negocio, no).
- F1, F3, F5: fuera de alcance, sin cambios.
- Observación (`OBSERVADO`): sin cambios en esta etapa — sigue sin un
  destino de retorno definido (ya documentado en `docs/etapa-1.1-cierre-seguridad.md`).

## 12. Limitaciones

- `DerivarExpediente` no es una transacción distribuida real — si el paso
  de registrar el historial falla después de mover el área, el usuario
  recibe un mensaje explícito de estado parcial, pero no hay reintento
  automático ni cola de compensación (fuera de alcance, explícitamente).
- Los tipos genéricos (`SOLICITUD/TRAMITE/OFICIO/OTRO`) permanecen sin
  ninguna restricción de transición — correcto mientras no exista una
  regla institucional para ellos, pero significa que esta etapa no agrega
  ninguna protección nueva para el uso administrativo interno existente,
  solo para los 5 trámites reales de Mesa de Partes.
- `APROBACION` como tipo de derivación sigue sin ningún efecto sobre
  `estado`/`area_actual` — no se tocó (correcto: su significado depende de
  F4-01, que sigue pendiente).
