# WORKFLOW DEFINITIVO DE MESA DE PARTES — I.E. TUNGASUCA

> Documento de **especificación de negocio**, no de implementación. No se
> escribió, modificó ni ejecutó código, migraciones o commits como parte de
> este documento. Es la fuente de verdad para diseñar posteriormente backend,
> frontend, base de datos, bandejas, derivaciones, estados y permisos.

**Fuentes usadas (en orden de autoridad):**
1. `docs/analisis-5-flujos-completo.md` — análisis institucional original (2026-09-08), máxima autoridad sobre el negocio.
2. `docs/etapa-12-especificacion-funcional.md` — primer contraste esperado-vs-código (2026-09-08).
3. Auditoría de flujo de Mesa de Partes (esta conversación) — evidencia de código verificada en vivo contra el backend real.
4. Auditoría de reglas de negocio y flujo real (esta conversación) — clasificación de los 5 procesos contra el código actual.

**Regla de lectura de este documento:** cada afirmación de negocio se marca
como `REGLA INSTITUCIONAL` (viene de la fuente 1/2), `IMPLEMENTACIÓN ACTUAL`
(viene del código, fuentes 3/4) o `DIFERENCIA` (la brecha entre ambas). Donde
no hay fuente institucional suficiente, se marca explícitamente:

```text
REGLA PENDIENTE DE DEFINICIÓN — REQUIERE CONFIRMACIÓN DE LA INSTITUCIÓN
```

No se inventa ninguna regla para rellenar estos vacíos.

---

## 1. Estado de este documento

Primera versión. No existía un `docs/workflow-definitivo-mesa-de-partes.md`
previo que analizar/preservar.

---

## 2. Clasificación de los 5 procesos

| Proceso | ¿Es Mesa de Partes? | ¿Es Expediente? | ¿Es Gestión Interna? | ¿Necesita workflow propio? | ¿Necesita bandeja? | ¿Necesita seguimiento? |
|---|---|---|---|---|---|---|
| F1 — Cumplimiento de Horas | REGLA PENDIENTE DE DEFINICIÓN (F1-01) | REGLA PENDIENTE DE DEFINICIÓN | Probable (control docente-administrativo interno, sin solicitante externo) | Sí, si se confirma que existe | REGLA PENDIENTE | No (según análisis, no describe tarea de seguimiento) |
| F2 — Certificado / Constancia | **Sí, confirmado** | **Sí, confirmado** | No (solicitante externo) | Ya definido (ver sección 6) | Sí, ya implementable | No |
| F3 — Citación a Padres | REGLA PENDIENTE DE DEFINICIÓN (Conflicto F1/F3/F5-01) — el análisis dice que lo inicia el colegio, no un solicitante externo, lo cual no calza con el concepto actual de "Mesa de Partes" (trámite que alguien *pide*) | REGLA PENDIENTE DE DEFINICIÓN | Probable (más cercano a Tutoría/Convivencia) | Sí, con estados propios de compromiso/seguimiento, distintos a PENDIENTE/EN_PROCESO/ATENDIDO/OBSERVADO | Sí (para el Responsable de Seguimiento) | **Sí, confirmado** (verificación de cumplimiento de compromiso) |
| F4 — Permisos / Justificaciones | **Sí, confirmado** (usa Expediente) | **Sí, confirmado** (con matices — Conflicto F4-01) | No | Sí, con matices sobre la aprobación (ver sección 10) | Sí, ya usa las mismas bandejas de F2 | No, según análisis |
| F5 — Monitoreo Docente | REGLA PENDIENTE DE DEFINICIÓN (Conflicto F1/F3/F5-01) — el análisis lo describe como instrumento pedagógico interno, sin solicitante | REGLA PENDIENTE DE DEFINICIÓN | Probable (evaluación pedagógica, no trámite documentario) | Sí, si se confirma que necesita workflow | REGLA PENDIENTE | REGLA PENDIENTE (F5-03: ¿seguimiento de planes de mejora post-monitoreo?) |

`IMPLEMENTACIÓN ACTUAL`: solo F2 existe en código, a través del Expediente
genérico ya construido. F4 usa el mismo Expediente pero **sin distinguirse**
de F2 (ambos crean `tipo="SOLICITUD"` fijo). F1, F3 y F5 no tienen ningún
código — ni frontend ni backend.

`DIFERENCIA`: crítica para F4 (falta distinguir el tipo de trámite y su
cadena de aprobación); total para F1/F3/F5 (no existen).

---

## 3. Tipos de trámite

`REGLA INSTITUCIONAL`: el análisis nombra tipos de trámite en texto libre
("Certificado", "Constancia de Estudios", "Permiso", "Justificación de
Falta", "Justificación de Tardanza") pero **no define códigos formales**
(los nombres del ejemplo del skill — `CERTIFICADO`, `CONSTANCIA`, `PERMISO`,
`JUSTIFICACION_FALTA`, `JUSTIFICACION_TARDANZA` — son solo ilustrativos, no
confirmados como los valores reales a implementar).

`IMPLEMENTACIÓN ACTUAL`: `Expedientes Service` solo tiene un campo `tipo`
con 4 valores genéricos: `SOLICITUD, TRAMITE, OFICIO, OTRO`
(`backend/services/expedientes/internal/estados/estados.go:5-10`). El FUT
Digital (`frontend/src/pages/FUT/FutDigital.jsx`) no ofrece ningún selector
de tipo de trámite — siempre envía `tipo: "SOLICITUD"` fijo
(`frontend/src/services/futService.js`, función `submitFut`). El asunto
("sumilla") es texto completamente libre.

`DIFERENCIA`: **crítica**. El sistema no puede hoy distinguir un Certificado
de una Constancia, ni un Permiso de una Justificación — todo es la misma
"SOLICITUD" con un texto libre de asunto.

### F2 — Certificado / Constancia de Estudios

| Campo | Certificado de Estudios | Constancia de Estudios |
|---|---|---|
| Código | REGLA PENDIENTE DE DEFINICIÓN | REGLA PENDIENTE DE DEFINICIÓN |
| Nombre | "Certificado de Estudios" (nombre del análisis, no un código confirmado) | "Constancia de Estudios" |
| Proceso | F2 | F2 |
| Quién inicia | Solicitante (padre de familia) — confirmado | igual |
| Canal | Sistema de Mesa de Partes (FUT Digital) — confirmado | igual |
| Requisitos | **REGLA PENDIENTE DE DEFINICIÓN (F2-01)** — el análisis no detalla los documentos sustentatorios exactos | igual |
| Área inicial | SECRETARIA — confirmado (implementación actual coincide) | igual |
| Estado inicial | PENDIENTE ("Recibido" en el análisis) — confirmado | igual |
| Primera acción | Secretaría revisa requisitos — confirmado | igual |

### F4 — Permisos y Justificaciones

| Campo | Permiso | Justificación de Falta | Justificación de Tardanza |
|---|---|---|---|
| Código | REGLA PENDIENTE DE DEFINICIÓN | REGLA PENDIENTE DE DEFINICIÓN | REGLA PENDIENTE DE DEFINICIÓN |
| Proceso | F4 | F4 | F4 |
| Quién inicia | Solicitante (padre de familia) — confirmado | igual | igual |
| Canal | Sistema de Mesa de Partes — confirmado | igual | igual |
| Requisitos | Documentos sustentatorios (sin detalle — pendiente) | igual | igual |
| Área inicial | SECRETARIA — confirmado | igual | igual |
| Estado inicial | PENDIENTE ("Recibido") — confirmado | igual | igual |
| Primera acción | Secretaría revisa — confirmado | igual | igual |

**Dato adicional confirmado**: F4 requiere seleccionar un "estudiante
asociado" (el trámite no es sobre el propio solicitante necesariamente, a
diferencia de F2). Este dato **no existe** en el Expediente actual
(`expedientes` no tiene columna de estudiante referido) — `DIFERENCIA`.

### F1, F3, F5

No se definen tipos de trámite: **REGLA PENDIENTE DE DEFINICIÓN** — depende
de resolver primero si estos procesos usan Expediente o un concepto propio
(Conflicto F1/F3/F5-01).

---

## 4. Workflow por trámite

### F2 — Certificado / Constancia de Estudios (CONFIRMADO)

```text
INICIO
  ↓
REGISTRO           Solicitante completa formulario + adjunta documentos
  ↓
VALIDACIÓN         Sistema valida completitud (obligatorio, confirmado)
  ↓
SECRETARIA         Revisa requisitos documentarios
  ↓
DECISIÓN
  ├── Requisitos OK → DERIVAR a DIRECCIÓN
  └── Requisitos incompletos → OBSERVAR → vuelve al Solicitante para corregir
  ↓
DIRECCIÓN          Recibe y revisa el trámite
  ↓
ATENDIDO/FINALIZADO   (el análisis NO especifica una acción explícita de
                       "aprobar" en Dirección para F2 — solo dice que
                       "revisa" y el sistema "permite seguimiento del
                       trámite" — REGLA PENDIENTE DE DEFINICIÓN si Dirección
                       debe ejecutar una acción explícita para cerrar F2)
```

`IMPLEMENTACIÓN ACTUAL`: coincide en el registro/validación/Secretaría→
Dirección conceptualmente, pero:
- La derivación Secretaría→Dirección NO es una operación atómica (ver
  sección 6 más abajo, y Crítico #2 de la auditoría de flujo).
- No existe una acción "Dirección revisa y cierra" — cualquier interno
  puede llamar `ChangeEstado(ATENDIDO)` sin relación con el paso del
  análisis.

`DIFERENCIA`: alta — el "cierre" institucional (Dirección revisa) no tiene
equivalente unívoco en el código.

### F4 — Permisos y Justificaciones (PARCIALMENTE CONFIRMADO — ver Conflicto F4-01)

```text
INICIO
  ↓
REGISTRO           Solicitante selecciona estudiante + tipo + completa
  ↓
VALIDACIÓN         Sistema valida completitud
  ↓
SECRETARIA         Revisa que esté correctamente presentada
  ↓
DECISIÓN (Secretaría)
  ├── OK → DERIVAR a DIRECCIÓN
  └── Incompleta → OBSERVAR → vuelve al Solicitante
  ↓
DIRECCIÓN          Revisa el trámite
  ↓
DECISIÓN (Dirección)
  ├── APRUEBA  → ??? (ver Conflicto F4-01 — no se confirma si pasa a
  │              Subdirección para una segunda instancia, o si Dirección ya
  │              decide en firme)
  └── DESAPRUEBA → estado "Desaprobado", registra motivo, FIN INMEDIATO
  ↓
[SI APLICA] SUBDIRECCIÓN   Recibe, revisa, "aprueba"/formaliza (el propio
                            análisis usa el verbo "aprobar" para dos roles
                            distintos — Conflicto F4-01, sin resolver)
  ↓
Sistema genera Proveído PDF (manual, no automático — confirmado en
                              analisis-5-flujos-completo.md y
                              etapa-12-especificacion-funcional.md sección 29
                              punto 5)
  ↓
NOTIFICA a Profesor (solo recibe información, no decide — confirmado,
                      regla inferida RN-10)
  ↓
ATENDIDO/FINALIZADO
```

`IMPLEMENTACIÓN ACTUAL`: **ninguno de los pasos posteriores a "Secretaría
deriva a Dirección" existe en código**. No hay estado "Desaprobado", no hay
paso automático a Subdirección, no hay generación de proveído, no hay
notificación al profesor (Notificaciones Service no existe; el panel admin
usa datos inventados — ver auditoría de flujo, hallazgo ALTO #8).

`DIFERENCIA`: crítica — F4 hoy es indistinguible de F2 en el código; toda la
cadena de aprobación/desaprobación descrita por la institución está sin
construir.

### F1, F3, F5

**REGLA PENDIENTE DE DEFINICIÓN** para representarlos como workflow del
sistema — el análisis SÍ describe un flujo paso a paso completo para cada
uno (reproducido íntegro en `docs/analisis-5-flujos-completo.md`, secciones
"FLUJO 1", "FLUJO 3", "FLUJO 5"), pero antes de convertirlo en workflow del
sistema debe resolverse el Conflicto F1/F3/F5-01 (¿usan Expediente o un
concepto propio?). Se referencian aquí sin repetirlos para no duplicar
contenido ya documentado; cualquier implementación debe volver a esa fuente.

---

## 5. Definición de cada paso — F2 (el único trámite con suficiente confirmación)

| Paso | Actor | Rol | Área | Acción | Estado antes | Estado después | Área anterior | Área destino | Responsable siguiente | Acción siguiente | Documento generado | Notificación | Condición |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 | Solicitante | SOLICITANTE | — | Registra trámite (FUT Digital) | - | PENDIENTE | - | SECRETARIA | Secretaría | Revisar requisitos | Formulario digital | REGLA PENDIENTE (¿se notifica a Secretaría? — sí, según análisis, "Sistema notifica automáticamente a Secretaría"; medio de notificación sin confirmar) | Validación de completitud debe pasar primero |
| 2 | Secretaría | SECRETARIA | SECRETARIA | Revisa requisitos | PENDIENTE | PENDIENTE (sin cambio de estado general — el análisis usa un estado específico "Observado"/"Derivado" que no tiene mapeo 1:1) | SECRETARIA | DIRECTOR (si OK) | Dirección | Revisar trámite | — | REGLA PENDIENTE DE DEFINICIÓN | Requisitos completos |
| 2b | Secretaría | SECRETARIA | SECRETARIA | Observa (si falta algo) | PENDIENTE | OBSERVADO | SECRETARIA | **REGLA PENDIENTE DE DEFINICIÓN** (¿vuelve al Solicitante para corregir, permaneciendo el área en SECRETARIA, o se modela distinto?) | Solicitante | Corregir y reenviar | Registro de observación | Sí, al Solicitante (confirmado que se notifica; medio sin confirmar) | Requisitos incompletos |
| 3 | Dirección | DIRECTOR | DIRECTOR | Revisa el trámite | PENDIENTE/EN_PROCESO | **REGLA PENDIENTE DE DEFINICIÓN** (el análisis no dice si Dirección ejecuta una acción de cierre explícita) | DIRECTOR | **REGLA PENDIENTE DE DEFINICIÓN** | — | — | — | — | — |

---

## 6. Bandejas

**Regla formal (confirmada, ya coincide entre negocio e implementación
actual):**

> Un expediente aparece en la bandeja de un área cuando `area_actual` de ese
> expediente coincide con el área del usuario, **combinado con** el `estado`
> que define qué tipo de bandeja es (Por revisar/En proceso/Observados/
> Finalizados) — nunca el estado solo, y nunca el área sola.

`IMPLEMENTACIÓN ACTUAL`: así están construidas ya las bandejas del frontend
(`frontend/src/constants/bandejas.js` + filtro automático por
`area_actual`/`solicitante_id` en `listexpedienteslogic.go:70-76`) — este
punto **no tiene diferencia** que corregir, es una regla ya bien resuelta.

**Por transición (F2/F4, lo único implementable hoy):**

```text
BANDEJA "Por revisar" de SECRETARIA
        ↓  (Secretaría deriva con éxito, area_actual → DIRECTOR)
SALE de la bandeja de SECRETARIA
        ↓
ENTRA a la bandeja de DIRECCIÓN (estado según corresponda)
```

`DIFERENCIA CRÍTICA` (ya documentada en la auditoría de flujo): esta salida/
entrada **no está garantizada por el backend** — depende de que el frontend
ejecute con éxito una segunda llamada (`PATCH /area`) después de crear la
derivación. Si falla, el expediente sale visualmente de ningún lado (sigue
en la bandeja de Secretaría) pero ya existe un registro de derivación,
generando una apariencia de movimiento que no ocurrió realmente.

---

## 7. Responsable: ¿área o persona?

`REGLA INSTITUCIONAL`: el análisis siempre habla de responsables por
**rol/cargo** ("Secretaría", "Dirección", "la Subdirectora", "el Profesor")
— nunca menciona una asignación a una persona específica dentro de un área
con varias personas, ni un mecanismo de "tomar/liberar" tarea. No hay
evidencia institucional de que se requiera responsabilidad individual.

`IMPLEMENTACIÓN ACTUAL`: **responsabilidad por área**, confirmado —
`expedientes.area_actual` es el único campo de asignación; no existe
`responsable_id`/`asignado_a` en ningún modelo del sistema (búsqueda
exhaustiva sin resultados, ver auditoría de flujo, sección 9).

`DIFERENCIA`: ninguna — ambas fuentes coinciden en que la responsabilidad es
por área, no por persona.

**Consecuencia documentada (no un defecto, una característica confirmada):**
cualquier usuario con el rol correspondiente a `area_actual` puede actuar
sobre el expediente; no hay "tomar tarea" ni "liberar tarea". Si la
institución llegara a necesitar trazar qué persona específica atendió cada
trámite, eso es un requisito nuevo, no confirmado hoy.

---

## 8. Aprobación

### F2 — no tiene una decisión formal de aprobar/rechazar

`REGLA INSTITUCIONAL`: el análisis de F2 no incluye una bifurcación
Aprobar/Desaprobar en Dirección (a diferencia de F4) — solo "revisa" y el
sistema "permite seguimiento". **No se debe inventar** una acción de
aprobación para F2 que la fuente no describe.

### F4 — Conflicto F4-01, sin resolver

```text
ALTERNATIVA A (Dirección decide, Subdirección solo formaliza):

DIRECCIÓN
    │
    ├── APROBAR ──► SUBDIRECCIÓN (solo formaliza/genera proveído,
    │                              sin poder de veto real)
    │
    └── DESAPROBAR ──► FIN (estado "Desaprobado", motivo registrado)

ALTERNATIVA B (dos aprobaciones reales, secuenciales):

DIRECCIÓN
    │
    ├── APROBAR (sustantivo) ──► SUBDIRECCIÓN
    │                                │
    │                                ├── APRUEBA (formal, con poder de
    │                                │    veto real) ──► continúa
    │                                └── RECHAZA ──► FIN
    │
    └── DESAPROBAR ──► FIN
```

**REGLA PENDIENTE DE DEFINICIÓN — REQUIERE CONFIRMACIÓN DE LA INSTITUCIÓN**
(Conflicto F4-01, ya documentado en Etapa 12 sección 22). Esta decisión
determina si `authorization.CanApprove` debe ser solo `DIRECTOR`, o si
además `SUBDIRECTOR` necesita poder de veto real — y si Expedientes Service
necesita un estado intermedio ("Aprobado por Dirección, pendiente de
Subdirección") que **hoy no existe** (`estados.go` solo tiene
PENDIENTE/EN_PROCESO/ATENDIDO/OBSERVADO).

`IMPLEMENTACIÓN ACTUAL`: ninguna alternativa está implementada. Existe un
`tipo=APROBACION` en Derivaciones Service, pero **verificado en vivo**
(auditoría de flujo, Crítico #3): registrarlo no cambia `estado` ni
`area_actual` del expediente. Es un comentario auditable, no una decisión
de flujo.

---

## 9. Observación

`REGLA INSTITUCIONAL` (F2/F4): Secretaría observa por requisitos
incompletos → notifica al Solicitante → el Solicitante corrige y reenvía
→ Secretaría vuelve a revisar. El análisis **no dice explícitamente** que el
expediente "regrese" de área (en la implementación actual, Secretaría nunca
lo había soltado todavía en ese punto del flujo, así que no hay a dónde
"regresar" — el problema de "a quién regresa" solo aparece si Dirección
observa después de recibirlo, escenario que el análisis de F2/F4 no cubre
explícitamente).

```text
SECRETARIA
   │
   │ OBSERVAR (requisitos incompletos)
   ▼
SOLICITANTE (corrige)
   │
   ▼
SECRETARIA (vuelve a revisar) — confirmado por el análisis
```

**Caso NO cubierto por el análisis, REGLA PENDIENTE DE DEFINICIÓN:** ¿qué
pasa si **Dirección** (no Secretaría) decide que un trámite ya derivado
necesita observación? ¿Regresa a Secretaría, o al Solicitante directamente?
No hay evidencia institucional para decidirlo.

`IMPLEMENTACIÓN ACTUAL`: `ChangeEstado` permite `PENDIENTE→OBSERVADO` desde
cualquier área que tenga el expediente en ese momento (sin restricción de
cuál área lo ejecuta — Crítico #1 de la auditoría de flujo), y **no mueve
`area_actual`** — el expediente se queda con la misma área que lo observó,
sin importar si institucionalmente "debería" volver a Secretaría o al
Solicitante.

`DIFERENCIA`: crítica para el caso no cubierto por el análisis; el sistema
no tiene ninguna regla, ni siquiera una equivocada — simplemente no mueve
nada.

---

## 10. Rechazo / Desaprobación

`REGLA INSTITUCIONAL`: solo **F4** tiene un estado de rechazo real:
"Desaprobado" — decisión de Dirección, corta el proceso inmediatamente,
registra motivo (confirmado, regla explícita RN citada en Etapa 12 sección
8). F2, F1, F3, F5 no mencionan un estado de rechazo equivalente en el
análisis (F3 tiene "Compromiso Incumplido", que es conceptualmente distinto
— no es un rechazo del trámite, es un resultado de seguimiento).

| Campo | OBSERVADO (confirmado, existe hoy) | DESAPROBADO/RECHAZADO (F4, confirmado en negocio, NO existe en código) |
|---|---|---|
| Significado | Requiere corrección — el trámite sigue vivo | El trámite no continúa — cierre negativo definitivo |
| Motivo | REGLA PENDIENTE (no hay campo `motivo` en `ChangeEstadoRequest` hoy) | Confirmado como obligatorio (registrar motivo de desaprobación) |
| Responsable | Quien observa (Secretaría o Dirección, según el paso) | Dirección, solo en F4 |
| Destino | No definido (ver sección 9) | FIN — no hay "destino" |
| Notificación | Sí, al Solicitante (confirmado) | Sí, implícito (confirmado en el análisis, medio sin confirmar) |
| ¿Puede volver a presentarse? | Sí (corrige y reenvía) | REGLA PENDIENTE DE DEFINICIÓN — el análisis no dice si tras un "Desaprobado" el solicitante puede iniciar un trámite nuevo idéntico, o si queda bloqueado |

`DIFERENCIA`: crítica — `RECHAZADO`/`DESAPROBADO` no existe como estado en
`estados.go`; hoy la única salida negativa es `OBSERVADO`, que institucional
y conceptualmente significa algo distinto (corrección, no cierre).

---

## 11. Atendido / Finalizado

`REGLA INSTITUCIONAL`: ninguno de los 5 flujos define explícitamente estos
puntos con el detalle que exige esta sección. Lo único confirmado:
- F2: el trámite queda "Finalizado/Atendido" tras la revisión de Dirección
  (sin decir quién ejecuta el cierre formalmente).
- F4: el trámite queda "Aprobado" tras la formalización de Subdirección
  (sujeto al Conflicto F4-01).

| Pregunta | Respuesta |
|---|---|
| ¿Quién puede finalizar? | REGLA PENDIENTE DE DEFINICIÓN (ninguna fuente institucional lo dice explícitamente para F2; para F4 sería quien formaliza, sujeto a F4-01) |
| ¿Qué condición permite finalizar? | REGLA PENDIENTE DE DEFINICIÓN |
| ¿Se genera documento? | Solo para F4 (proveído PDF) — confirmado. F2 no menciona un documento de cierre explícito más allá del propio expediente. |
| ¿Se notifica al solicitante? | REGLA PENDIENTE DE DEFINICIÓN — la matriz de notificaciones de Etapa 12 (sección 17) no incluye "atención" como evento notificado para F2, sí lo insinúa para F4 ("Aprobación/desaprobación... informar resultado") |
| ¿El solicitante puede descargarlo? | Confirmado en `IMPLEMENTACIÓN ACTUAL` para el FUT/cargo (ya construido esta sesión); no confirmado institucionalmente para un "proveído" de F4, que no existe todavía |
| ¿El expediente queda bloqueado? | `IMPLEMENTACIÓN ACTUAL`: sí, técnicamente — `ATENDIDO` es un estado terminal, `CanTransition` no permite salir de él (`estados.go:66-71`). No hay confirmación institucional de que esto sea lo deseado. |
| ¿Puede reabrirse? | REGLA PENDIENTE DE DEFINICIÓN — ninguna fuente institucional lo menciona; el código actual no lo permite (no hay transición desde ATENDIDO) |

`DIFERENCIA`: `ATENDIDO = FINALIZADO` es una decisión **técnica** ya tomada
en el código (estado terminal), pero **no está confirmada como regla de
negocio** — se marca explícitamente como pendiente de validar con la
institución, tal como exige la sección 13 del skill.

---

## 12. Tipo de trámite vs. Estado

`REGLA INSTITUCIONAL` + `IMPLEMENTACIÓN ACTUAL` ya coinciden en el
**principio** (el estado no debe representar el tipo) — el sistema real
nunca usó el estado para eso; el problema es el inverso: **el tipo de
trámite no existe como campo real**, todo es `tipo="SOLICITUD"` (ver sección
3). Modelo propuesto, **no implementado**, solo para referencia futura:

```text
tipo_tramite      = JUSTIFICACION_TARDANZA   (REGLA PENDIENTE — nombre no confirmado)
estado            = EN_PROCESO               (ya existe, confirmado)
area_actual       = SUBDIRECCION             (ya existe, confirmado)
accion_pendiente  = FORMALIZAR               (no existe — ver sección 13)
```

---

## 13. Acción pendiente

`REGLA INSTITUCIONAL`: el análisis describe acciones concretas por paso
(revisar, aprobar, observar, derivar, formalizar, generar proveído,
notificar) pero **no las modela como un campo/catálogo formal** — son texto
narrativo del flujo, no un dato a persistir.

`IMPLEMENTACIÓN ACTUAL`: no existe ningún campo `accion_pendiente`. Se
infiere combinando `estado`+`area_actual`+rol, de forma ambigua.

**Decisión que debe tomar el negocio**: si vale la pena incorporar este
concepto como dato explícito (catálogo propuesto, no confirmado):

```text
REVISAR_REQUISITOS · APROBAR · OBSERVAR · FORMALIZAR · GENERAR_PROVEIDO ·
NOTIFICAR · EMITIR_DOCUMENTO · REGISTRAR_RESULTADO
```

**REGLA PENDIENTE DE DEFINICIÓN** — no se implementa, solo se dejan estos
nombres como insumo si la institución confirma que lo necesita.

---

## 14. Notificaciones

`REGLA INSTITUCIONAL` (reproducida de Etapa 12 sección 17, sin
reinterpretar):

| Evento | Destinatario | Medio | Motivo |
|---|---|---|---|
| Trámite recibido (F2, F4) | Secretaría | Medio por confirmar | Nuevo trámite a revisar |
| Observación de requisitos (F2, F4) | Solicitante/Padre | Medio por confirmar | Corrección requerida |
| Derivación a Dirección (F2, F4) | — (interno, sin notificación a externo) | — | — |
| Aprobación/desaprobación (F4) | Solicitante (implícito), Profesor (explícito) | Medio por confirmar | Informar resultado |

`IMPLEMENTACIÓN ACTUAL`: no existe Notificaciones Service. El portal
SOLICITANTE tiene una página de "Novedades" construida esta sesión con datos
**reales** (compuestos de expedientes+derivaciones existentes, sin canal
push/correo). El panel interno (ADMIN/DIRECTOR/etc.) tiene un botón de
notificaciones que **hoy muestra datos 100% inventados**
(`frontend/src/data/mockDashboardData.js`, consumido por
`frontend/src/components/dashboard/NotificationButton.jsx`) — hallazgo ALTO
de la auditoría de flujo.

`DIFERENCIA`: crítica de cara al usuario interno — el sistema aparenta tener
notificaciones funcionando cuando son datos de muestra.

**GEN-01 sigue sin resolver**: no hay confirmación de un medio real
(correo institucional, SMS, solo dentro de la plataforma).

---

## 15. Documentos generados

`REGLA INSTITUCIONAL` (Matriz de Documentos, reproducida sin cambios):

| Trámite | Documento | Generador | Receptor |
|---|---|---|---|
| F2 | Solicitud Certificado/Constancia | Solicitante | Solicitante (es el propio expediente) |
| F4 | Solicitud Permiso/Justificación | Solicitante | Solicitante (es el propio expediente) |
| F4 | Proveído Digital PDF | Subdirección/Sistema | Profesor |

`IMPLEMENTACIÓN ACTUAL`: Documentos Service permite **carga manual** de
archivos (adjuntos, proveídos, actas, informes, formatos — tipos ya
definidos en su migración: `ADJUNTO, PROVEIDO, ACTA, INFORME, FORMATO,
OTRO`), pero **no genera ningún PDF automáticamente** — confirmado
explícitamente como decisión ya tomada (sin plantilla definida, pregunta
F4-02 sin resolver). El "Cargo del FUT" (PDF del propio expediente F2/F4) sí
se genera automáticamente hoy, pero es un documento *de recepción*, no un
*proveído de resolución*.

`DIFERENCIA`: ninguna respecto a la decisión ya documentada de no generar
PDFs automáticamente — este punto ya está resuelto y alineado.

---

## 16. Historial

`REGLA INSTITUCIONAL`: cada flujo espera poder reconstruir su trazabilidad
completa (fechas, actor, decisión) — explícito para F4 ("todo el flujo debe
quedar registrado en una trazabilidad/historial del trámite", RN
consolidada).

**Especificación mínima requerida** (no implementada):

```text
EXPEDIENTE
    ↓
HISTORIAL
    ├── fecha
    ├── usuario
    ├── acción
    ├── estado anterior
    ├── estado nuevo
    ├── área anterior
    ├── área nueva
    ├── motivo
    └── observación
```

`IMPLEMENTACIÓN ACTUAL`: no existe tabla de historial en ningún
microservicio (confirmado por búsqueda exhaustiva en migraciones). Lo más
cercano son las filas de `derivaciones` (fecha/usuario/origen/destino/
motivo, sin estado_anterior/estado_nuevo) y el campo único
`fecha_actualizacion` del expediente (se sobrescribe, no acumula).

`DIFERENCIA`: alta — no se puede reconstruir hoy el recorrido completo de
estados de un expediente, solo su situación vigente más el registro parcial
de derivaciones.

---

## 17. Seguridad del workflow

`REGLA INSTITUCIONAL` (Matriz de permisos, Etapa 12 sección 11, reproducida)
vs. `IMPLEMENTACIÓN ACTUAL` (verificada en código y en vivo):

| Acción | ADMIN | SECRETARIA | DIRECCIÓN | SUBDIRECCIÓN | DOCENTE | AUXILIAR |
|---|---|---|---|---|---|---|
| **Ver** — Institucional | Transversal (no asumir absoluto) | Sí (F2,F4) | Sí (F2,F4) | Sí (F1,F4) | No decide, solo recibe notif. | No |
| **Ver** — Actual | Todo (`CanViewAll`) | Todo (`CanViewAll`) | Solo su área (`AreaDelRol`) | Solo su área | Solo su área | Solo su área |
| **Ver** — Diferencia | Ninguna | Ninguna | Coincide en espíritu (acotado) | Coincide | Coincide | Coincide |
| **Derivar** — Institucional | — | Sí, a Dirección (F2,F4) | Sí, a Subdirección (F4) | No (F4); sí a Dirección (F1) | Sí, a Subdirectora (F1) | No |
| **Derivar** — Actual | Cualquiera puede crear cualquier derivación hacia cualquier destino | igual | igual | igual | igual | igual |
| **Derivar** — Diferencia | **CRÍTICA** — la institución espera reglas específicas de quién deriva a quién; el código no restringe nada |
| **Aprobar** — Institucional | — | No | Sí (F4) | Sí, formalización (F4) — sujeto a F4-01 | No | No |
| **Aprobar** — Actual | No existe la acción; `tipo=APROBACION` no tiene efecto para nadie | | | | | |
| **Aprobar** — Diferencia | **CRÍTICA** — no implementado para nadie |
| **Observar** — Institucional | — | Sí (F2,F4) | Implícito en su revisión | — | No | No |
| **Observar** — Actual | Cualquier interno, sin importar si pertenece al área actual | | | | | |
| **Observar** — Diferencia | **CRÍTICA** (Crítico #1 de la auditoría de flujo) |
| **Rechazar** — Institucional | — | No | Sí (F4, "Desaprobar") | No | No | No |
| **Rechazar** — Actual | No existe el estado/acción para nadie | | | | | |
| **Rechazar** — Diferencia | **CRÍTICA** — no implementado |
| **Atender** — Institucional | — | REGLA PENDIENTE | Sí (F2, implícito) | Sí (F4, tras formalizar) | No | No |
| **Atender** — Actual | Cualquier interno, sin importar el área actual | | | | | |
| **Atender** — Diferencia | **CRÍTICA** (mismo gap que Observar) |

---

## 18. Matriz maestra — F2 (único trámite con evidencia suficiente)

| Paso | Actor | Área | Acción | Estado antes | Estado después | Destino | Acción siguiente | Documento | Notificación |
|---|---|---|---|---|---|---|---|---|---|
| 1 | Solicitante | — | Registra | - | PENDIENTE | SECRETARIA | Revisar requisitos | Formulario | A Secretaría (medio sin confirmar) |
| 2 | Secretaría | SECRETARIA | Revisa / deriva | PENDIENTE | EN_PROCESO (equivalencia aproximada — ver sección 9 mapeo Etapa 12) | DIRECTOR | Revisar trámite | — | REGLA PENDIENTE |
| 2b | Secretaría | SECRETARIA | Observa (si falta algo) | PENDIENTE | OBSERVADO | REGLA PENDIENTE | Corregir | Registro de observación | Al Solicitante |
| 3 | Dirección | DIRECTOR | Revisa / cierra | EN_PROCESO | ATENDIDO (supuesto, no confirmado explícitamente como acción de Dirección) | sin cambio | — | — | REGLA PENDIENTE |

F1, F3, F4 (más allá del paso 2), F5: **REGLA PENDIENTE DE DEFINICIÓN** —
no se rellenan filas sin evidencia, tal como exige la sección 9 del skill.

---

## 19. Diagrama definitivo (solo reglas confirmadas)

```text
┌──────────────┐
│ SOLICITANTE  │
└──────┬───────┘
       │  Registra (F2 o F4, indistinguibles hoy)
       ▼
┌──────────────┐
│  SECRETARÍA  │
│   REVISAR    │
└──────┬───────┘
       │
   ┌───┴────────────┐
   │                │
   ▼                ▼
REQUISITOS OK   REQUISITOS INCOMPLETOS
   │                │
   ▼                ▼
DERIVAR         OBSERVAR ──► SOLICITANTE (corrige) ──► vuelve a SECRETARÍA
   │
   ▼
┌──────────────┐
│  DIRECCIÓN   │
│   REVISAR    │
└──────┬───────┘
       │
       ▼
  ??? (F2: sin acción de cierre confirmada — REGLA PENDIENTE)
  ??? (F4: Aprobar/Desaprobar — Conflicto F4-01, REGLA PENDIENTE)
```

No se dibuja un diagrama único "workflow institucional de Tungasuca" más
allá de este punto porque, con la evidencia disponible, el flujo se bifurca
en preguntas sin responder — dibujar más allá sería inventar.

---

## 20. Consolidado de reglas pendientes de definición

Toda pregunta que bloquea completar este documento, en un solo lugar (une
las ya identificadas en Etapa 12 sección 23 con las nuevas de este
documento):

| ID | Pregunta | Bloquea |
|---|---|---|
| F1-01 | ¿El cumplimiento de horas genera Expediente o es un módulo interno separado? | Clasificación de F1, workflow, modelo de datos |
| F1-02 | ¿Periodicidad real del reporte? | Diseño de datos de F1 |
| F2-01 | ¿Requisitos documentarios exactos de Certificado vs. Constancia? | Validaciones de Documentos/Expedientes |
| F2-02 (nuevo) | ¿Quién y con qué acción cierra formalmente un F2 (Dirección revisa, pero no hay un "aprobar" definido)? | Sección 11 (Atendido/Finalizado) |
| F3-01 | ¿Quién es responsable del seguimiento del compromiso? | Autorización de Seguimiento Service |
| F3-02 | ¿"Trabajador del Colegio"/"Responsable de Seguimiento" mapean a un rol existente o uno nuevo? | Matriz de permisos, Auth Service |
| F3-03 | ¿Plazo estándar para verificar cumplimiento de compromiso? | Diseño de "fecha límite" en Seguimiento |
| F4-01 | ¿Quién aprueba realmente un permiso: solo Dirección, o Dirección + Subdirección con veto real? | Modelo de autorización y máquina de estados de F4 |
| F4-02 | ¿Existe plantilla oficial para el proveído PDF? | Generación de documentos |
| F4-03 (nuevo) | ¿Un expediente F4 "Desaprobado" puede volver a presentarse? | Sección 10 (Rechazo) |
| F5-01 | ¿La notificación al docente del resultado del monitoreo es obligatoria? | Regla de negocio de Notificaciones |
| F5-02 | ¿El monitoreo genera Expediente o vive en un historial propio? | Diseño de datos de F5 |
| F5-03 | ¿Existe seguimiento explícito post-monitoreo con fecha límite? | Alcance de Seguimiento Service |
| GEN-01 | ¿Medio real de notificación disponible hoy? | Alcance de Notificaciones Service |
| GEN-02 (nuevo) | ¿`ATENDIDO` debe ser realmente un estado terminal sin reapertura, para todos los trámites? | Máquina de estados general |
| GEN-03 (nuevo) | ¿La responsabilidad debe seguir siendo solo por área, o algún trámite requiere asignación a persona? | Modelo de datos, Fase 5 de implementación |

Ninguna de estas preguntas se respondió inventando una regla.

---

## 21. Criterio de aceptación por trámite (17 preguntas)

### F2 — Certificado / Constancia

| # | Pregunta | Respuesta |
|---|---|---|
| 1 | ¿Quién inicia? | Solicitante (confirmado) |
| 2 | ¿Qué tipo de trámite es? | Certificado o Constancia — **no distinguible hoy en el sistema** (sección 3) |
| 3 | ¿Dónde entra? | Bandeja de Secretaría (confirmado) |
| 4 | ¿Quién lo recibe? | Secretaría (confirmado) |
| 5 | ¿Qué debe hacer? | Revisar requisitos (confirmado); requisitos exactos: REGLA PENDIENTE (F2-01) |
| 6 | ¿Qué opciones tiene? | Derivar a Dirección / Observar (confirmado) |
| 7 | ¿Qué ocurre si aprueba? | No aplica — F2 no tiene un paso de aprobación explícito (sección 8) |
| 8 | ¿Qué ocurre si observa? | Notifica al Solicitante, quien corrige y reenvía (confirmado) |
| 9 | ¿Qué ocurre si rechaza? | No aplica — F2 no tiene rechazo definido |
| 10 | ¿A quién se deriva? | A Dirección (confirmado) |
| 11 | ¿Quién recibe la siguiente tarea? | Dirección (confirmado) |
| 12 | ¿Qué bandeja lo muestra? | Bandeja de Dirección (implementable con lo ya construido) |
| 13 | ¿Qué documento se genera? | El propio expediente digital + cargo (ya implementado) |
| 14 | ¿Quién firma? | El Solicitante firma el FUT (ya implementado); Dirección no firma nada en el análisis |
| 15 | ¿Quién es notificado? | Secretaría al recibir; Solicitante al observar (confirmado); REGLA PENDIENTE si se notifica al cerrar |
| 16 | ¿Cuándo termina? | REGLA PENDIENTE DE DEFINICIÓN (F2-02) — no hay acción de cierre explícita en el análisis |
| 17 | ¿Puede reabrirse? | REGLA PENDIENTE DE DEFINICIÓN |

### F4 — Permisos / Justificaciones

| # | Pregunta | Respuesta |
|---|---|---|
| 1 | ¿Quién inicia? | Solicitante (confirmado) |
| 2 | ¿Qué tipo de trámite es? | Permiso / Justificación de Falta / Justificación de Tardanza — **no distinguible hoy** |
| 3 | ¿Dónde entra? | Bandeja de Secretaría (confirmado) |
| 4 | ¿Quién lo recibe? | Secretaría (confirmado) |
| 5 | ¿Qué debe hacer? | Revisar presentación correcta (confirmado) |
| 6 | ¿Qué opciones tiene? | Derivar a Dirección / Observar (confirmado) |
| 7 | ¿Qué ocurre si aprueba? | Conflicto F4-01 — REGLA PENDIENTE DE DEFINICIÓN |
| 8 | ¿Qué ocurre si observa? | Notifica al padre, corrige y reenvía (confirmado, mismo patrón que F2) |
| 9 | ¿Qué ocurre si rechaza? | Estado "Desaprobado", motivo registrado, FIN (confirmado en negocio; **no implementado**) |
| 10 | ¿A quién se deriva? | Secretaría→Dirección→(Subdirección si aplica, F4-01) (confirmado con matiz) |
| 11 | ¿Quién recibe la siguiente tarea? | Dirección, luego posiblemente Subdirección (F4-01) |
| 12 | ¿Qué bandeja lo muestra? | Mismas bandejas de Expediente ya construidas |
| 13 | ¿Qué documento se genera? | Proveído Digital PDF (confirmado, generación manual) |
| 14 | ¿Quién firma? | REGLA PENDIENTE DE DEFINICIÓN (F4-02, plantilla no confirmada) |
| 15 | ¿Quién es notificado? | Profesor (confirmado, solo receptor); Solicitante (implícito) |
| 16 | ¿Cuándo termina? | Al "Aprobado" (confirmado como nombre de estado; acción exacta sujeta a F4-01) |
| 17 | ¿Puede reabrirse? | REGLA PENDIENTE DE DEFINICIÓN (F4-03) |

### F1, F3, F5

Las 17 preguntas: **REGLA PENDIENTE DE DEFINICIÓN** en bloque — no se puede
completar el criterio de aceptación sin resolver primero el Conflicto
F1/F3/F5-01 (¿generan Expediente o un concepto propio?). El análisis
institucional sí responde varias de estas preguntas a nivel narrativo (ver
`docs/analisis-5-flujos-completo.md`), pero convertir eso en una
especificación de sistema requiere esa decisión previa.

---

## 22. Fases de implementación posterior (referencia, no ejecutar aún)

Reproducidas del skill que originó este documento, como guía para cuando se
apruebe esta especificación:

```text
FASE 1 — Modelo de negocio       (tipo_tramite, estado, área, acción,
                                   responsable, transiciones)
FASE 2 — Modelo de datos          (tablas)
FASE 3 — Workflow Backend         (el backend debe ser dueño de las reglas,
                                   no el frontend — ver Crítico #2 de la
                                   auditoría de flujo)
FASE 4 — Seguridad                (rol + área + responsable + permiso +
                                   acción — ver Crítico #1)
FASE 5 — Derivaciones             (registrar movimiento + cambiar área +
                                   actualizar bandeja + historial, como UNA
                                   operación coherente, no varias sueltas)
FASE 6 — Bandejas                 (derivadas del workflow real)
FASE 7 — Frontend                 (solo representa reglas del backend; cero
                                   lógica crítica tipo "si tipo=X entonces
                                   PATCH /area")
FASE 8 — Historial
FASE 9 — Notificaciones           (solo tras confirmar GEN-01)
FASE 10 — Pruebas                 (cada transición del workflow)
```

**No se ejecuta ninguna fase en este documento.**

---

## 23. Nota final

Este documento cubre con evidencia sólida **F2** (y parcialmente F4). **F1,
F3 y F5 no tienen suficiente definición institucional confirmada** para
completarse sin antes resolver el Conflicto F1/F3/F5-01 — completarlos aquí
habría significado inventar reglas de negocio, lo cual esta tarea prohíbe
explícitamente. La sección 20 (consolidado de preguntas pendientes) es la
lista concreta de lo que debe responder la institución antes de continuar.
