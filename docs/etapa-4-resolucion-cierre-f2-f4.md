# ETAPA 4 — Resolución y cierre real de F2/F4

## 1. Objetivo

Implementar el cierre real de F2 (CERTIFICADO/CONSTANCIA) y F4 (PERMISO/
JUSTIFICACION_FALTA/JUSTIFICACION_TARDANZA), usando las reglas
institucionales entregadas explícitamente en esta etapa (ya no son
`REGLA PENDIENTE` — F2-02 y F4-01 quedan resueltas por la propia
instrucción de esta etapa). No se implementó F1/F3/F5, ni ninguna regla no
descrita explícitamente.

## 2. Reglas institucionales implementadas

**F2**: Solicitante → Secretaría → Dirección → Secretaría → Solicitante.
Dirección devuelve el expediente con el documento final; Secretaría lo
deja disponible y cierra como `ATENDIDO`. F2 no tiene rechazo definido —
no se inventó uno.

**F4**: Solicitante → Secretaría → Dirección → (aprueba) Subdirección →
(aprueba, formaliza proveído) Docente. Dirección o Subdirección pueden
rechazar en su etapa → el expediente vuelve a Secretaría con `OBSERVADO`
→ el solicitante corrige y vuelve a entrar (mismo expediente, mismo
código).

**Inferencia razonable documentada, no una cita textual**: la transición
`PENDIENTE → EN_PROCESO` al derivar de Secretaría a Dirección (para F2 y
F4) y el cierre a `ATENDIDO` al llegar a Docente en F4 no están escritos
con esas palabras exactas en la regla institucional de esta etapa, pero
son la única forma consistente de que el resto de la regla (que exige
"en proceso" para poder rechazar/resolver, y que llama a Docente el
"destino final") tenga sentido con la máquina de estados ya existente.
Ya estaban anticipadas en `docs/workflow-definitivo-mesa-de-partes.md`
sección 9. Si esto no es correcto, es la única parte de esta etapa que
requeriría ajuste.

## 3. Flujo F2

```text
SOLICITANTE ──crea CERTIFICADO/CONSTANCIA──► SECRETARIA (PENDIENTE)
SECRETARIA ──DerivarExpediente──► DIRECTOR (EN_PROCESO, automático)
DIRECTOR ──sube documento PROVEIDO (Documentos Service, ya existente)──►
DIRECTOR ──DerivarExpediente──► SECRETARIA
SECRETARIA ──ResolverExpediente (exige el PROVEIDO ya cargado)──► ATENDIDO
```

No existe rechazo para F2 — ninguna acción de esta etapa lo ofrece.

## 4. Flujo F4

```text
SOLICITANTE ──crea PERMISO/JUSTIFICACION_*──► SECRETARIA (PENDIENTE)
SECRETARIA ──DerivarExpediente──► DIRECTOR (EN_PROCESO, automático)

DIRECTOR:
  ├── DerivarExpediente → SUBDIRECTOR (aprueba)
  └── RechazarExpediente → SECRETARIA, estado OBSERVADO (rechaza)

SUBDIRECTOR (solo si Dirección aprobó):
  ├── sube proveído (Documentos Service) + DerivarExpediente → DOCENTE
  │   (exige el proveído ya cargado; cierra a ATENDIDO en la misma escritura)
  └── RechazarExpediente → SECRETARIA, estado OBSERVADO (rechaza)

SOLICITANTE (tras cualquier rechazo): CorregirExpediente → PENDIENTE,
mismo código, misma trazabilidad — vuelve a entrar por Secretaría.
```

## 5. Estados utilizados

Ninguno nuevo. Solo `PENDIENTE`, `EN_PROCESO`, `ATENDIDO`, `OBSERVADO` —
ya existentes. `DERIVADO` sigue sin ser un estado.

## 6. Acciones / RPC / endpoints

| Acción de negocio | RPC (Expedientes Service) | Endpoint (Gateway) |
|---|---|---|
| Derivar (ya existía, Etapa 3) | `DerivarExpediente` | `PATCH /api/expedientes/:id/derivar` |
| Rechazar (nuevo) | `RechazarExpediente` | `PATCH /api/expedientes/:id/rechazar` |
| Corregir (nuevo) | `CorregirExpediente` | `PATCH /api/expedientes/:id/corregir` |
| Resolver/cerrar F2 (nuevo) | `ResolverExpediente` | `PATCH /api/expedientes/:id/resolver` |

Ningún endpoint nuevo fuera de estos 3 RPC. No se reemplazó
`ChangeEstado`/`UpdateArea` (siguen existiendo para sus usos genéricos).

Casos especiales dentro de `DerivarExpediente` (mismo endpoint de Etapa
3, sin romper su contrato): Secretaría→Dirección para F2/F4 también pasa
`PENDIENTE→EN_PROCESO`; Subdirección→Docente para F4 también cierra a
`ATENDIDO` — ambos en la MISMA escritura SQL (repositorio
`DerivarConCambioDeEstado`), para que el actor que todavía es dueño del
área anterior sea quien complete el cambio, sin una ventana donde ya no
tenga autorización.

## 7. Autorización

- `RechazarExpediente`: exige `CanUpdateArea` + que el actor sea dueño de
  `area_actual` (Dirección o Subdirección, según corresponda) + que el
  tipo sea F4 + que el estado sea `EN_PROCESO`.
- `ResolverExpediente`: exige `CanChangeEstado` + dueño de área (debe ser
  Secretaría con el expediente de vuelta) + tipo F2 + estado `EN_PROCESO`
  + (verificado por el Gateway) que exista un documento `PROVEIDO`.
- `CorregirExpediente`: exige rol `SOLICITANTE` **y** que
  `solicitante_id` coincida exactamente con el actor (no basta con ser
  cualquier solicitante) + estado `OBSERVADO`.
- **Corregido en esta etapa**: `DownloadDocumento` (Documentos Service)
  no validaba de quién era el expediente de un documento — un
  SOLICITANTE podía descargar el documento de cualquier expediente si
  conocía su id (limitación ya documentada desde antes). Se cerró en el
  Gateway reutilizando `GetExpediente` (que ya exige propiedad/área) antes
  de entregar el contenido — verificado en vivo: un segundo solicitante
  que no es dueño del expediente ahora recibe `403` en vez de `200`.

## 8. Documentos

Se reutilizó el mecanismo ya existente de Documentos Service sin
cambios: el "documento final" de F2 y el "proveído" de F4 se cargan como
`tipo_documento=PROVEIDO` (ya definido, no se inventó un tipo nuevo). No
se implementó ningún generador de PDF — sigue siendo carga manual, tal
como ya estaba documentado como pendiente desde Etapa 12.

El Gateway verifica la existencia de un `PROVEIDO` (vía
`ListDocumentos` filtrado) antes de: (a) `ResolverExpediente` (F2), y
(b) la derivación Subdirección→Docente (F4) — ambas fallan con
`FailedPrecondition` si todavía no se cargó.

## 9. Correcciones

`CorregirExpediente` **no crea un expediente nuevo**: mismo `id`/`codigo`,
mismo `solicitante_id`, misma fecha de registro. Solo actualiza
`descripcion` y el estado. Toda la trazabilidad previa (derivaciones ya
registradas, incluida la del rechazo con motivo/actor/fecha) permanece
intacta — verificado en la prueba de integración y en vivo.

## 10. Pruebas

**Unitarias** (`internal/estados/estados_test.go`): `TestCanDerivar`
extendido con las transiciones nuevas (Director→Secretaría para F2,
Subdirector→Docente para F4); `TestEsF2`/`TestEsF4` nuevos.

**Integración** (`expedientes_integration_test.go`, PostgreSQL real):
- F2: no se puede resolver antes de tiempo (ni desde Dirección, ni desde
  Secretaría sin el expediente de vuelta); Docente no puede resolver;
  Secretaría resuelve con éxito → `ATENDIDO`; resolver un F4 se rechaza.
- F4: no se puede rechazar un F2, ni antes de EN_PROCESO; Subdirección no
  puede rechazar lo que está en Dirección (área ajena); Dirección
  rechaza con éxito → `OBSERVADO`/`SECRETARIA`; doble rechazo se
  rechaza.
- Corrección: otro solicitante no puede corregir; personal interno
  tampoco; el dueño corrige con éxito → `PENDIENTE`, descripción
  actualizada; segunda corrección sin nuevo rechazo se rechaza.
- Ruta F4 completa aprobada: Secretaría→Dirección→Subdirección→Docente,
  termina en `ATENDIDO`/`DOCENTE`.

```text
go test ./...  (expedientes, derivaciones, documentos, gateway)  → PASS
INTEGRATION_TEST=true go test ./internal/integration/... -v      → PASS
```

Un error real se encontró y corrigió durante esta fase: la primera
versión de `DerivarExpediente` no pasaba nunca `PENDIENTE→EN_PROCESO`,
lo que hacía que `ResolverExpediente` (que exige `EN_PROCESO`) fallara
siempre — detectado por la propia prueba de integración, no en
producción.

## 11. Evidencia E2E

**API real** (Gateway, usuarios de prueba desechables, eliminados al
terminar): F2 completo (crear→derivar→devolver→bloqueo sin
documento→subir documento→resolver→`ATENDIDO`) y F4 completo en sus dos
rutas (rechazo→corrección, y aprobación completa hasta Docente con la
verificación del proveído) — todo confirmado con `curl` contra los
servicios reales. También se verificó en vivo el cierre de la descarga
de documentos (dueño → `200`, ajeno → `403`).

**Navegador real** (`browser-automation`, usuarios de prueba
desechables): se hizo clic real en los 3 botones nuevos del detalle del
expediente —
- Secretaría → "Marcar como atendido" (F2) → estado quedó en `ATENDIDO`.
- Dirección → "Rechazar y devolver" (F4) → estado `Observado`, área
  `Secretaría`.
- Solicitante → "Corregir y volver a presentar" → estado `Pendiente`,
  descripción actualizada.

0 errores de consola, 0 requests fallidos en los tres casos.

## 12. Limitaciones

- El cierre de F4 al llegar a Docente (`ATENDIDO`) y el paso
  `PENDIENTE→EN_PROCESO` al salir de Secretaría son inferencias
  razonables, no citas textuales de la regla institucional de esta etapa
  — documentado explícitamente en la sección 2, para que la institución
  pueda corregirlo si no es lo que esperaba.
- No se implementó generación automática de proveídos/documentos (sigue
  siendo carga manual, ya documentado como pendiente desde Etapa 12).
- No se implementó ningún historial/auditoría nuevo más allá de
  reutilizar Derivaciones Service (ya existente).
- F1, F3, F5 y cualquier regla no descrita explícitamente en esta etapa
  permanecen sin implementar.
- La responsabilidad sigue siendo por área, no por persona (sin cambios
  desde etapas anteriores).
