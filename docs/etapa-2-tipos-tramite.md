# ETAPA 2 — Tipos de trámite

> Solo implementa el catálogo y registro del tipo de trámite. No se
> implementó ningún workflow específico por tipo (eso es Etapa 3).

## 1. Tipos implementados

| Valor | Etiqueta (frontend) | Origen |
|---|---|---|
| `CERTIFICADO` | Certificado de Estudios | `docs/analisis-5-flujos-completo.md`, FLUJO 2 |
| `CONSTANCIA` | Constancia de Estudios | ídem, FLUJO 2 |
| `PERMISO` | Permiso | ídem, FLUJO 4 |
| `JUSTIFICACION_FALTA` | Justificación de Falta | ídem, FLUJO 4 |
| `JUSTIFICACION_TARDANZA` | Justificación de Tardanza | ídem, FLUJO 4 |

Se mantienen los 4 genéricos existentes (`SOLICITUD`, `TRAMITE`, `OFICIO`,
`OTRO`) — compatibilidad con expedientes existentes y con el registro
interno de "Nuevo expediente" (`ExpedienteFormModal.jsx`, para casos que no
corresponden a ninguno de los 5 trámites reales).

**Desviación explícita respecto al catálogo de referencia del skill**: el
skill proponía `CERTIFICADO, CONSTANCIA, PERMISO, JUSTIFICACION` (una sola
"JUSTIFICACION"), pero la documentación institucional
(`docs/analisis-5-flujos-completo.md`, FLUJO 4) distingue explícitamente
tres selecciones: "Permiso / Justificación de Falta / Justificación de
Tardanza" — no una "Justificación" genérica. Siguiendo la instrucción
"si la documentación del proyecto utiliza nombres diferentes, respetar los
nombres ya definidos", se implementaron `JUSTIFICACION_FALTA` y
`JUSTIFICACION_TARDANZA` como tipos independientes.

**CERTIFICADO vs. CONSTANCIA**: se mantienen como tipos independientes, sin
ninguna regla de validación distinta entre ambos — la diferencia de
requisitos documentarios sigue sin definir (pregunta F2-01, ya documentada
en `docs/etapa-12-especificacion-funcional.md`). No se inventaron
requisitos.

## 2. Backend

- `internal/estados/estados.go`: se agregaron las 5 constantes nuevas
  (`TipoCertificado`, `TipoConstancia`, `TipoPermiso`,
  `TipoJustificacionFalta`, `TipoJustificacionTardanza`) al mapa
  `tiposValidos`, junto a las 4 genéricas existentes. `IsValidTipo()` ya
  hacía la validación — no se duplicó lógica.
- `internal/logic/createexpedientelogic.go`: se eliminó el valor por
  defecto silencioso (`tipo == "" → tipo = TipoSolicitud`). Un tipo vacío
  ahora se rechaza igual que cualquier valor inválido (`InvalidArgument`,
  "el tipo de expediente no es válido") — antes se completaba solo, lo que
  le permitía al FUT no distinguir nunca un trámite real.
- No se tocó `ChangeEstado`/`UpdateArea`/`CrearDerivacion` — esta etapa no
  incluye reglas específicas por tipo (explícitamente fuera de alcance).

## 3. Frontend

- `constants/expedientes.js`: `TIPOS` se separó en `TIPOS_TRAMITE` (los 5
  reales, con etiquetas) y `TIPOS_GENERICOS` (los 4 existentes); se agregó
  `tipoLabel(value)` para mostrar la etiqueta legible donde se necesite.
- `pages/FUT/FutDigital.jsx`: se agregó el campo obligatorio "Tipo de
  trámite" (primer campo del formulario), con las 5 opciones de
  `TIPOS_TRAMITE` — **no** se muestran las 4 genéricas (un solicitante
  externo no debería ver "Trámite"/"Oficio"/"Otro"). Sin valor
  preseleccionado: el solicitante debe elegir explícitamente. Se valida en
  `validate()` y se muestra también en el paso de revisión (antes de
  enviar).
- `services/futService.js`: `submitFut()` ya no envía `tipo: "SOLICITUD"`
  fijo — envía `fut.tipo` (lo que el solicitante eligió).
- `pages/Expedientes/ExpedienteDetail.jsx`: el campo "Tipo" ahora muestra
  `tipoLabel(expediente.tipo)` (etiqueta legible) en vez del valor crudo.
- `pages/Expedientes/ExpedientesList.jsx`: se agregó una columna "Tipo" a
  la tabla (listado y bandejas, que reutilizan este mismo componente — ver
  Etapa de bandejas anterior). El filtro por tipo ya existía y ya
  reutilizaba la constante `TIPOS`, así que ganó las 5 opciones nuevas sin
  tocar su código.
- `pages/MesaDePartes/Seguimiento.jsx`: columna "Tipo" agregada a la tabla
  de seguimiento del solicitante.
- `pages/MesaDePartes/SeguimientoDetalle.jsx`: el tipo se muestra junto al
  código del expediente en el encabezado.
- `components/dashboard/ExpedienteFormModal.jsx` (registro interno): sin
  cambios de código — ya itera sobre la constante `TIPOS` compartida, así
  que el personal interno también puede elegir cualquiera de los 9 tipos
  (5 nuevos + 4 genéricos) sin que hiciera falta tocar el componente.

No se crearon pantallas nuevas ni se modificaron filtros más allá de lo
necesario para mostrar el tipo.

## 4. Base de datos

**Sí hubo migración** (`003_add_tipos_tramite.sql`) — no era evitable:

- La columna `tipo` era `VARCHAR(20)`. `JUSTIFICACION_TARDANZA` tiene 22
  caracteres y no entraba. Se amplió a `VARCHAR(30)`.
- El `CHECK` constraint (`chk_expedientes_tipo`) solo permitía los 4
  valores genéricos a nivel de base de datos — sin ampliarlo, el backend
  habría validado un tipo nuevo como válido pero PostgreSQL lo habría
  rechazado igual al insertar.

Ambos cambios son aditivos: ampliar un `VARCHAR` no trunca datos
existentes, y el nuevo `CHECK` incluye los 4 valores originales además de
los 5 nuevos (`SOLICITUD, TRAMITE, OFICIO, OTRO, CERTIFICADO, CONSTANCIA,
PERMISO, JUSTIFICACION_FALTA, JUSTIFICACION_TARDANZA`). No se tocó ninguna
fila existente. Aplicada contra la base real (`expedientes_db`) y
verificada con `\d expedientes`.

## 5. Compatibilidad con expedientes antiguos

Verificado explícitamente (código + prueba de integración + en vivo): un
expediente con `tipo = SOLICITUD` sigue pudiendo consultarse (`GetExpediente`),
listarse (`ListExpedientes`), y todo lo demás (cambiar estado, derivar) sigue
funcionando exactamente igual que antes — no se migró ni reinterpretó
ningún dato existente.

## 6. Validaciones

- Backend: `IsValidTipo()` es la única fuente de verdad; el frontend no
  decide qué es válido, solo ofrece las opciones del catálogo compartido.
- Tipo vacío, `null` (equivalente a campo ausente en el request), o
  cualquier texto que no esté en el catálogo → `InvalidArgument` (mapeado a
  HTTP 400 por el Gateway, mismo mecanismo de errores ya existente).

## 7. Pruebas ejecutadas

**Unitarias** (`internal/estados/estados_test.go`): se extendió
`TestIsValidTipo` con los 5 tipos nuevos como válidos y se agregó `"ABC"`
como caso inválido adicional. `go test ./internal/estados/...` → PASS.

**Integración** (`expedientes_integration_test.go`, PostgreSQL real,
`INTEGRATION_TEST=true`):
- Los 5 tipos nuevos se crean correctamente y el valor guardado coincide
  exactamente con lo enviado (verificado también releyendo con
  `GetExpediente`).
- Tipo vacío → `InvalidArgument` (antes tenía éxito con el valor por
  defecto).
- Tipo inválido (`"INVALIDO"`, `"ABC"`, `"JUSTIFICACION"` sin sufijo) →
  `InvalidArgument`.
- Compatibilidad: el expediente `tipo=SOLICITUD` creado al inicio de la
  prueba sigue siendo consultable y aparece en el listado del solicitante
  al final de la prueba.

```text
go test ./...  (expedientes)                                           → PASS
INTEGRATION_TEST=true go test ./internal/integration/... -run TestExpedientesIntegration -v
                                                                          → PASS
```

**API real** (Gateway, usuario de prueba desechable, eliminado al
terminar):

| Prueba | Resultado |
|---|---|
| `POST /api/expedientes` `tipo=CERTIFICADO` | `200`, `tipo` guardado correctamente |
| `POST /api/expedientes` `tipo=ABC` | `400 InvalidArgument` |
| `POST /api/expedientes` `tipo=""` | `400 InvalidArgument` |

**Frontend** (navegador real, vía `browser-automation`, usuario de prueba
desechable):

```text
FUT → login SOLICITANTE → selecciona "Certificado de Estudios" en el
nuevo campo "Tipo de trámite" → completa el resto → Presentar solicitud
→ expediente creado (EXP-2026-000065) → detalle del expediente muestra
"TIPO: Certificado de Estudios"
```

0 errores de consola, 0 requests fallidos.

## 8. Decisiones pendientes

- `REGLA PENDIENTE DE DEFINICIÓN` — diferencia exacta de requisitos
  documentarios entre `CERTIFICADO` y `CONSTANCIA` (pregunta F2-01, sin
  resolver desde Etapa 12).
- `REGLA PENDIENTE DE DEFINICIÓN` — si F1/F3/F5 alguna vez necesitan un
  tipo de trámite propio, eso depende primero de resolver si usan
  Expediente o un concepto propio (Conflicto F1/F3/F5-01, sin resolver) —
  fuera de alcance de esta etapa.
- No se implementó ninguna regla de negocio específica por tipo (qué área
  recibe cada uno, aprobación, proveído, etc.) — eso es explícitamente la
  Etapa 3.
