# ETAPA 12 — Especificación Funcional Oficial del Sistema de Mesa de Partes y Gestión Institucional

**I.E. Tungasuca — UGEL 04 — Carabayllo**

> Fuente: `analisis-5-flujos-completo.md` (análisis de los 5 procesos, provisto
> por el usuario el 2026-09-08). Este documento es de **análisis y definición
> únicamente** — no se programó ni modificó código como parte de esta etapa.

---

## 1. Objetivo general

Convertir el análisis de los 5 procesos administrativos de la I.E. Tungasuca en
una especificación funcional que sirva de base de diseño para los
microservicios del sistema (los ya construidos — Auth, Expedientes, Usuarios,
Documentos — y los pendientes — Derivaciones, Seguimiento, Notificaciones,
Reportes), sin convertir el sistema en un monolito.

---

## 2. Arquitectura funcional

```
React/Vite
     ↓
API Gateway
     ↓
Microservicios (Auth · Usuarios · Expedientes · Documentos ·
                 Derivaciones · Seguimiento · Notificaciones · Reportes)
     ↓
Bases de datos aisladas por servicio
```

Estado real de implementación al cierre de esta etapa: **Auth, Usuarios,
Expedientes y Documentos ya existen y están integrados** (Gateway + frontend).
Derivaciones, Seguimiento, Notificaciones y Reportes **no existen todavía**;
esta especificación es, entre otras cosas, el insumo para diseñarlos.

---

## 3–7. Flujos 1 a 5

El detalle completo de cada flujo (objetivo, actores, entradas/salidas, pasos,
decisiones, documentos, derivaciones, estados, reglas, datos) está en
`analisis-5-flujos-completo.md`. Aquí se resume solo lo que cambia el diseño:

| Flujo | Genera Expediente? | Usa el Expediente Service actual (PENDIENTE/EN_PROCESO/ATENDIDO/OBSERVADO)? |
|---|---|---|
| F1 — Cumplimiento de Horas | PENDIENTE DE CONFIRMACIÓN — ver sección 8 | Parcialmente: sus estados no calzan 1:1 |
| F2 — Certificado/Constancia | Sí, claramente | **Sí, encaja bien** (es el caso ya construido) |
| F3 — Citación a Padres | PENDIENTE DE CONFIRMACIÓN — ver sección 8 | No — sus estados son de compromiso/seguimiento, no de trámite documentario |
| F4 — Permisos y Justificaciones | Sí | Parcialmente — ver Conflicto F4-01 (sección 20) |
| F5 — Monitoreo Docente | PENDIENTE DE CONFIRMACIÓN — ver sección 8 | No — es un registro de gestión pedagógica, no un trámite de un externo |

---

## 8. Reglas de negocio (explícita / inferida / pendiente)

Reutilizando la clasificación ya hecha en la fuente, consolidada:

### REGLA CONFIRMADA (explícita en el análisis)
- Todo trámite de Mesa de Partes (F2, F4) genera un código único de
  seguimiento (`#TR-YYYY-XXXX` en el análisis; el sistema real ya usa
  `EXP-YYYY-NNNNNN` — ver sección 12).
- La validación de completitud del sistema es obligatoria antes de que
  cualquier solicitud (F2, F4) llegue a un área administrativa.
- Secretaría debe comprobar requisitos antes de derivar a Dirección (F2, F4).
- La aprobación de permisos/justificaciones (F4) requiere el visto bueno de
  Dirección y la ratificación/proveído de Subdirección.
- Una reunión de citación (F3) concluye con firma de acta y aceptación del
  compromiso; el compromiso pasa a una etapa explícita de seguimiento.
- Si hay incumplimiento de un compromiso (F3), el responsable puede reabrir
  el ciclo generando una nueva citación directamente.
- El resultado de un monitoreo (F5) impacta el historial acumulativo del
  profesor.
- El Auxiliar habilita el formato de horas (F1); la Subdirectora debe validar
  antes de elaborar el informe final para Dirección.

### REGLA INFERIDA (deducida razonablemente, no explícita)
- El solicitante puede subsanar y reenviar tanto por errores de validación
  del sistema como por observaciones humanas (F2, F4).
- Los compromisos de citación (F3) deben quedar asociados al historial del
  estudiante/padre.
- El docente no aprueba nada en F4 ni en F1; solo recibe notificaciones.
- La Subdirectora es la única facultada para abrir y calificar monitoreos
  (F5).
- El sistema no genera el informe final de F1 hasta que la Subdirectora
  declare la información completa y correcta.

### PENDIENTE DE CONFIRMACIÓN
- **F1**: ¿El "Formato de cumplimiento de horas" y el "Informe" generan un
  Expediente de Mesa de Partes formal, o viven en un módulo de control interno
  de RRHH/asistencia separado? El análisis no lo dice explícitamente.
- **F3**: ¿La citación a un padre genera un Expediente de Mesa de Partes, o es
  un registro propio de un futuro módulo de Tutoría/Convivencia, con
  Seguimiento Service como único punto de contacto? El "solicitante" aquí no
  es quien inicia el trámite (lo inicia el colegio), lo cual no calza con el
  rol `SOLICITANTE` actual del sistema (pensado para quien *pide* algo).
- **F5**: ¿El monitoreo docente genera un Expediente, o es un registro
  exclusivamente pedagógico sin relación con Mesa de Partes?
- Ver también los Conflictos y Preguntas Pendientes de las secciones 20–21.

---

## 9. Estados

### Estado general del expediente (ya implementado, Expedientes Service)
`PENDIENTE`, `EN_PROCESO`, `ATENDIDO`, `OBSERVADO` — con transiciones
`PENDIENTE→EN_PROCESO→ATENDIDO` y `PENDIENTE→OBSERVADO` (ambos terminales).

### Estados específicos por flujo (del análisis, NO son estados globales)

| Flujo | Estados específicos |
|---|---|
| F1 | Registrado/Habilitado, Enviado, Con Observaciones, Consolidado, Informe Elaborado, Informe Entregado |
| F2 | Recibido, Observado, Derivado a Dirección, Finalizado/Atendido |
| F3 | Citación Registrada/Notificada, Reunión Realizada, Acta Guardada/Compromiso Registrado, En Seguimiento, Compromiso Cumplido, Compromiso Incumplido, Finalizado |
| F4 | Recibido, Observado, Derivado a Dirección, Aprobado, Desaprobado |
| F5 | En Proceso/Monitoreo Iniciado, Evaluado, Finalizado/Guardado, Notificado al Profesor |

### Mapeo estado específico → estado general (donde aplica)

| Flujo | Específico | → General |
|---|---|---|
| F2 | Recibido | PENDIENTE |
| F2 | Observado | OBSERVADO |
| F2 | Derivado a Dirección | EN_PROCESO |
| F2 | Finalizado/Atendido | ATENDIDO |
| F4 | Recibido | PENDIENTE |
| F4 | Observado | OBSERVADO |
| F4 | Derivado a Dirección / En Subdirección | EN_PROCESO |
| F4 | Aprobado | ATENDIDO |
| F4 | Desaprobado | **No calza** — ver sección 8 (Expedientes Service no tiene un estado "rechazado/desaprobado"; hoy solo tiene OBSERVADO como salida negativa, que implica corrección, no cierre definitivo) |

---

## 10. Transiciones — ver sección 9 (integradas en la misma tabla para no
duplicar información entre estado y transición, dado que cada estado
específico del análisis ya implica su transición de origen).

---

## 11. Actores y permisos

### Matriz de actores (de la fuente, sin reinterpretar)

| Actor (fuente) | Tipo | Procesos | Acciones |
|---|---|---|---|
| ADMIN/Secretaría | Administrativa | F2, F4 | Revisa requisitos, registra observaciones, deriva a Dirección |
| Director | Directiva | F1, F2, F4 | Revisa informes, aprueba/desaprueba permisos, recibe derivados |
| Subdirector | Directiva | F1, F4, F5 | Verifica horas, elabora informes, emite proveídos, ejecuta monitoreos |
| Docente/Profesor | Interno | F1, F4, F5 | Completa horas, recibe notificaciones |
| Auxiliar | Interno | F1 | Habilita/asigna formato de horas |
| Solicitante/Padre | Externo | F2, F3, F4 | Inicia trámites, adjunta sustentos, subsana, firma actas |
| Trabajador del Colegio | Interno | F3 | Emite citaciones, realiza reuniones, elabora actas |
| Responsable de Seguimiento | Interno/Admin | F3 | Fiscaliza compromisos, re-cita |

**PENDIENTE DE CONFIRMACIÓN**: "Trabajador del Colegio" y "Responsable de
Seguimiento" (F3) no se mapean explícitamente a ninguno de los 7 roles reales
del sistema (`ADMIN, DIRECTOR, SUBDIRECTOR, SECRETARIA, DOCENTE, AUXILIAR,
SOLICITANTE`). Podrían ser DOCENTE (tutor de aula), AUXILIAR, o requerir un rol
nuevo — no se decide aquí (ver pregunta F3-02).

### Matriz de permisos por rol (inferida de las acciones descritas; NINGUNO de
estos permisos existe todavía dado de alta en Auth Service — son una
propuesta a validar antes de implementar, igual que se hizo con Expedientes/
Documentos)

| Rol | Flujo | Puede crear | Puede revisar | Puede aprobar | Puede derivar |
|---|---|---|---|---|---|
| SOLICITANTE | F2, F4 | Sí | No | No | No |
| SECRETARIA | F2, F4 | No | Sí | No | Sí (a Dirección) |
| DIRECTOR | F2 | No | Sí | — (solo recibe) | No |
| DIRECTOR | F4 | No | Sí | Sí | Sí (a Subdirección) |
| SUBDIRECTOR | F4 | No | Sí | Sí (formalización — ver Conflicto F4-01) | No |
| SUBDIRECTOR | F1 | No | Sí | — | Sí (a Dirección) |
| SUBDIRECTOR | F5 | Sí (inicia monitoreo) | Sí (evalúa) | — | No |
| AUXILIAR | F1 | Sí (habilita formato) | No | No | No |
| DOCENTE | F1 | Sí (completa horas) | No | No | Sí (envía a Subdirectora) |
| DOCENTE | F4, F5 | No | No | No | No (solo recibe notificación) |
| (rol pendiente, F3) | F3 | Sí (cita) | Sí (revisa cumplimiento) | — | No |

---

## 12. Documentos — ver Matriz de Documentos en `analisis-5-flujos-completo.md`
(reproducida sin cambios; ya cubre generador/receptor/proceso/momento).
Clasificación adicional pedida por esta etapa:

| Documento | Clasificación |
|---|---|
| Formato de Cumplimiento de Horas | Documento principal (F1) |
| Informe de Cumplimiento de Horas | Documento generado |
| Solicitud Certificado/Constancia | Documento principal (F2) — es el propio Expediente |
| Formulario de Citación | Documento principal (F3) |
| Acta de Reunión/Compromiso | Documento firmado (F3) |
| Solicitud Permiso/Justificación | Documento principal (F4) — es el propio Expediente |
| Proveído Digital PDF | Documento firmado/generado (F4) |
| Ficha de Monitoreo/Evaluación | Informe (F5) |

No se asumen formatos no definidos (ej. plantilla exacta del PDF del
proveído) — **PENDIENTE DE CONFIRMACIÓN**.

---

## 13. Expediente (relación con Expedientes Service ya construido)

**Importante, tal como exige esta etapa**: Expedientes Service ya existe y ya
genera códigos `EXP-<año>-<consecutivo>` — **no se reemplaza ni se propone
otro mecanismo de codificación**. El análisis usa `#TR-YYYY-XXXX` como
notación conceptual; se interpreta como el mismo concepto que el `codigo` real
del sistema, no como un mecanismo paralelo.

| Flujo | Genera Expediente (real) | Cuándo | Quién lo genera | Estado inicial | Estado final |
|---|---|---|---|---|---|
| F2 | Sí | Al enviar la solicitud validada | El propio solicitante (rol SOLICITANTE, ya implementado) | PENDIENTE | ATENDIDO |
| F4 | Sí (encaja con matices — ver Conflicto F4-01) | Al enviar la solicitud validada | El propio solicitante | PENDIENTE | ATENDIDO u "otro" no soportado hoy (Desaprobado) |
| F1, F3, F5 | PENDIENTE DE CONFIRMACIÓN | — | — | — | — |

---

## 14. FUT General

**No se asumió que los 5 procesos usan el FUT** (regla explícita de esta
etapa). Del análisis:

- **Usan un formulario equivalente al FUT**: F2 (Certificado/Constancia) y F4
  (Permiso/Justificación) — ambos son "trámites" iniciados por un
  externo/padre con datos + documentos sustentatorios + envío formal.
- **No usan FUT**: F1 (es un formato interno de control docente-administrativo,
  no un trámite de un externo), F3 (lo inicia el colegio, no un solicitante),
  F5 (es un instrumento pedagógico interno).

### Campos comunes (F2 y F4) vs. específicos

| Dato | Pertenece a | Común a F2/F4 |
|---|---|---|
| Nombre, DNI, teléfono/correo del solicitante | Usuario | Sí |
| Motivo/asunto | Expediente | Sí |
| Tipo de trámite (Certificado/Constancia vs. Permiso/Justificación) | Expediente | Sí (mismo campo, distinto dominio de valores) |
| Estudiante referido | Expediente (o dato de negocio aún no modelado) | Solo F4 (F2 puede ser sobre el propio solicitante o un hijo) |
| Documentos sustentatorios | Documento | Sí |
| Derivación Secretaría→Dirección(→Subdirección en F4) | Derivación | Sí, con un paso extra en F4 |

---

## 15. Derivaciones

| Flujo | Origen | Destino | Motivo | Responsable | Condición |
|---|---|---|---|---|---|
| F1 | Profesor | Subdirectora | Envío del formato completo | Profesor | — |
| F1 | Subdirectora | Profesor | Observaciones | Subdirectora | Info incompleta/incorrecta |
| F1 | Subdirectora | Dirección | Informe consolidado | Subdirectora | Info validada |
| F2 | Sistema | Secretaría | Trámite recibido | Sistema | Validación de completitud OK |
| F2 | Secretaría | Dirección | Trámite formalizado | Secretaría | Requisitos cumplidos |
| F3 | Trabajador | Padre | Notificación de citación | Sistema | — |
| F3 | Trabajador | Responsable de Seguimiento | Acta y compromisos | Trabajador | Reunión concluida |
| F4 | Sistema | Secretaría | Alerta de expediente recibido | Sistema | Validación OK |
| F4 | Secretaría | Dirección | Trámite formalizado | Secretaría | Requisitos cumplidos |
| F4 | Dirección | Subdirección | Derivación aprobatoria | Dirección | Aprobado |
| F4 | Subdirección/Sistema | Profesor | Notificación e inclusión en módulo docente | Sistema | Proveído generado |
| F5 | Subdirectora | Sistema | Carga de evaluación | Subdirectora | Monitoreo finalizado |
| F5 | Sistema | Profesor | Notificación condicional | Sistema | Regla de notificación = sí |

Se distingue explícitamente (regla de esta etapa):
- **Derivación** real (cambia de responsable/área): F1 (Prof→Subdir, Subdir→Dir),
  F2 (Secretaría→Dirección), F4 (Secretaría→Dirección→Subdirección).
- **Notificación** (informa, no transfiere responsabilidad de decisión):
  Sistema→Solicitante/Padre/Profesor en todos los flujos.
- **Asignación**: Auxiliar→Profesor (F1, habilitar el formato).
- **Recepción**: Dirección recibiendo el informe (F1) o la solicitud (F2, F4).
- **Aprobación**: Dirección/Subdirección en F4 (evento de decisión, no de
  transporte del documento).

---

## 16. Seguimiento

| Flujo | Tarea | Responsable | Fecha límite | Resultado | Cierre |
|---|---|---|---|---|---|
| F3 | Cumplimiento del compromiso del padre | Responsable de Seguimiento (rol sin mapear — ver sección 11) | PENDIENTE DE CONFIRMACIÓN (no se especifica plazo) | Cumplido / Incumplido | Cumplido → fin; Incumplido → nueva citación o cierre con registro |
| F5 | Ninguna tarea de seguimiento post-monitoreo explícita más allá de notificar | Subdirectora | N/A | N/A | El propio monitoreo se cierra al guardar la evaluación |

Nota: el análisis original de la skill anticipaba que F5 también tendría
"historial de monitoreo docente" como objeto de seguimiento continuo (serie de
monitoreos en el tiempo), pero el flujo paso a paso solo describe **un**
monitoreo puntual que alimenta un historial — no describe un mecanismo de
seguimiento con fecha límite propio dentro de F5. Se marca como **PENDIENTE DE
CONFIRMACIÓN** si se requiere seguimiento explícito de planes de mejora
derivados de un monitoreo con observaciones.

---

## 17. Notificaciones

| Evento | Destinatario | Medio | Motivo |
|---|---|---|---|
| Formato enviado (F1) | Subdirectora | Medio por confirmar | Informar que hay un formato para revisar |
| Observaciones de horas (F1) | Profesor | Medio por confirmar | Corrección requerida |
| Trámite recibido (F2, F4) | Secretaría | Medio por confirmar | Nuevo trámite a revisar |
| Observación de requisitos (F2, F4) | Solicitante/Padre | Medio por confirmar (F4 menciona "notifica al móvil del Padre" — posible SMS/push, no confirmado como canal oficial) | Corrección requerida |
| Derivación a Dirección (F2, F4) | — (interno, no se notifica a un externo en el análisis) | — | — |
| Citación (F3) | Padre de Familia | Medio por confirmar | Convocar a reunión |
| Aprobación/desaprobación (F4) | Solicitante (implícito) y Profesor (explícito) | Medio por confirmar | Informar resultado |
| Resultado de monitoreo (F5) | Profesor | Medio por confirmar | Retroalimentación — condicional, no siempre se envía |

No se asume que existe SMS, WhatsApp, correo o push como canal ya definido:
todos quedan como **"Medio por confirmar"**, tal como exige esta etapa.

---

## 18. Datos

| Dato | F1 | F2 | F3 | F4 | F5 |
|---|---|---|---|---|---|
| Identidad del usuario (nombre, DNI, contacto) | ✓ (profesor) | ✓ (solicitante) | ✓ (padre) | ✓ (solicitante) | ✓ (profesor) |
| Código de expediente/trámite | ? (pendiente, sec. 8) | ✓ | ? (pendiente) | ✓ | ? (pendiente) |
| Tipo de trámite/proceso | — | ✓ | — | ✓ | — |
| Asunto/motivo | ✓ (implícito: horas del periodo) | ✓ | ✓ (motivo de citación) | ✓ | ✓ (criterios evaluados) |
| Estado del trámite | ✓ | ✓ | ✓ | ✓ | ✓ |
| Documento adjunto/generado | ✓ | ✓ | ✓ | ✓ | ✓ |
| Derivación (origen/destino/fecha) | ✓ | ✓ | ✓ (parcial) | ✓ | ✓ (parcial) |
| Seguimiento (tarea/fecha límite/resultado) | — | — | ✓ | — | ? (pendiente, sec. 16) |
| Notificación (evento/destinatario/medio) | ✓ | ✓ | ✓ | ✓ | ✓ |

Clasificación por dominio (para mapeo a microservicios, sección 19):
- **Datos de usuario**: identidad, contacto, rol — Usuarios Service.
- **Datos de expediente**: código, tipo, estado, fechas — Expedientes Service.
- **Datos de documento**: archivo, tipo, generador — Documentos Service.
- **Datos de derivación**: origen/destino/motivo/condición — futuro
  Derivaciones Service.
- **Datos de seguimiento**: tarea/responsable/fecha límite/resultado — futuro
  Seguimiento Service.
- **Datos de notificación**: evento/destinatario/medio — futuro Notificaciones
  Service.

---

## 19. Mapeo a microservicios

Reproducido de la fuente (sin modificar), con la columna "Estado real" añadida
por esta etapa:

| Funcionalidad | Microservicio responsable | Justificación | Estado real |
|---|---|---|---|
| Autenticación e identificación de todos los roles | Auth Service | Ya es su responsabilidad exclusiva | ✓ Implementado |
| Gestión de perfiles, roles e historial del docente | Usuarios Service | Ya existe; "historial del docente" (F5) es una extensión futura de su modelo actual | ✓ Implementado (perfil básico); historial docente pendiente |
| Creación y ciclo de vida del expediente | Expedientes Service | Ya existe; cubre F2 y (parcialmente) F4 | ✓ Implementado |
| Carga de adjuntos, generación de proveídos/actas/informes/formatos | Documentos Service | Ya existe como almacén de archivos; **no genera PDFs automáticamente todavía** (ver limitación ya documentada en su README) | 🟡 Parcial (guarda archivos, no los genera) |
| Reglas de pase entre áreas y asignación de tareas | Derivaciones Service | No existe | 🔴 Pendiente |
| Tareas post-atención (compromisos F3, monitoreo F5) | Seguimiento Service | No existe | 🔴 Pendiente |
| Alertas vía app/móvil/correo | Notificaciones Service | No existe; hoy nada del sistema notifica a nadie fuera de la propia UI | 🔴 Pendiente |
| Consolidación de informes y métricas | Reportes Service | No existe | 🔴 Pendiente |

**Regla verificada**: ningún microservicio existente concentra toda la lógica;
Expedientes no genera documentos, Documentos no decide flujo, ninguno de los
dos decide derivaciones. Se mantiene la separación de responsabilidades ya
establecida.

---

## 20. Dependencias entre microservicios

| Servicio origen | Servicio destino | Motivo |
|---|---|---|
| Gateway | Auth | Login/registro/validación de token |
| Gateway | Usuarios | Perfil, búsqueda, resolución de nombres |
| Gateway | Expedientes | CRUD de expedientes |
| Gateway | Documentos | Subir/descargar/listar documentos |
| Frontend (vía Gateway) | Usuarios | Resolver `solicitante_id` a nombre en pantallas de Expedientes |
| (futuro) Derivaciones | Expedientes | Necesita saber a qué expediente pertenece cada derivación (por id, sin FK — mismo patrón ya usado) |
| (futuro) Seguimiento | Derivaciones / Expedientes | Un compromiso o plan de mejora nace de una derivación o de un expediente |
| (futuro) Notificaciones | Derivaciones, Seguimiento, Expedientes | Necesita saber **qué pasó** para decidir a quién avisar |
| (futuro) Reportes | Expedientes, Seguimiento, Documentos | Necesita leer datos consolidados de los demás, probablemente por consulta directa a cada servicio, no por replicación |

```
Auth
 ↓
Usuarios
 ↓
Expedientes
 ↓
Documentos
 ↓
Derivaciones
 ↓
Seguimiento
 ↓
Notificaciones
```

Esto es un **mapa conceptual de orden de construcción**, no un mandato de que
cada servicio llame directamente al anterior. Como ya se decidió al construir
Documentos Service (que NO llama a Expedientes por gRPC, sino que el
frontend/Gateway componen la información), se recomienda **evaluar eventos en
vez de llamadas directas** para:
- Derivaciones → Notificaciones (cuando algo se deriva, notificar) — encaja
  mejor como evento (`ExpedienteDerivado`) que como llamada síncrona.
- Seguimiento → Notificaciones (compromiso por vencer) — igual, mejor como
  evento o como un job programado que consulta Seguimiento periódicamente.

No se decide una implementación concreta de eventos en esta etapa (eso sería
programar); solo se identifica dónde convendría más que una llamada síncrona.

---

## 21. Eventos del sistema

| Evento | Quién lo produce | Qué significa | Quién podría consumirlo |
|---|---|---|---|
| `ExpedienteCreado` | Expedientes Service | Un solicitante o proceso interno registró un expediente | Notificaciones (avisar a Secretaría), Reportes |
| `ExpedienteObservado` | Expedientes Service | El expediente pasó a estado OBSERVADO | Notificaciones (avisar al solicitante) |
| `ExpedienteAtendido` | Expedientes Service | El expediente se cerró como ATENDIDO | Notificaciones, Reportes |
| `ExpedienteDerivado` | Derivaciones Service (futuro) | Un expediente cambió de área responsable | Notificaciones, Seguimiento |
| `DocumentoSubido` | Documentos Service | Se cargó un adjunto o proveído | Notificaciones (opcional), Reportes |
| `CompromisoRegistrado` | Seguimiento Service (futuro) | Un padre asumió un compromiso (F3) | Notificaciones (recordatorio), Reportes |
| `CompromisoIncumplido` | Seguimiento Service (futuro) | Venció el plazo sin cumplimiento (F3) | Notificaciones, Derivaciones (re-citación) |
| `MonitoreoFinalizado` | Seguimiento o Expedientes (según se resuelva la pregunta F5-02) | Se cerró un monitoreo docente (F5) | Notificaciones (condicional), Reportes |
| `PermisoAprobado` / `PermisoDesaprobado` | Expedientes o Derivaciones (según se resuelva F4-01) | Resultado de la aprobación de F4 | Notificaciones (Profesor, Solicitante) |

No se implementa mensajería en esta etapa — es un catálogo conceptual para
cuando se construyan Derivaciones/Seguimiento/Notificaciones.

---

## 22. Inconsistencias detectadas

### CONFLICTO F4-01 — ¿Quién aprueba el permiso/justificación?

**Descripción**: El propio texto del análisis contiene una ambigüedad interna.
La sección de decisiones dice explícitamente:

> "¿Se aprueba la solicitud? (Evaluado por Dirección) → Sí: Deriva
> digitalmente a Subdirección para formalización."

Pero el flujo paso a paso, dos pasos después, dice:

> "17. Subdirección → Recibe y revisa la solicitud. 18. Subdirección → **Aprueba** la solicitud."

Es decir, Dirección "aprueba" (decisión) y luego Subdirección también
"aprueba" (acción 18) — el mismo verbo se usa para dos roles distintos en dos
momentos distintos.

**Alternativa A**: Dirección toma la decisión sustantiva de aprobar/desaprobar;
Subdirección solo **formaliza** (genera el proveído) sin poder de veto real —
el uso de "aprueba" en el paso 18 sería impreciso, debería decir "formaliza"
o "ratifica".

**Alternativa B**: Existen dos aprobaciones reales y secuenciales — Dirección
aprueba en sustancia, y Subdirección puede **todavía** rechazar en la
formalización (una segunda instancia real de control).

**Decisión requerida**: confirmar con la institución cuál de las dos aplica,
porque cambia el modelo de autorización: si es A, `authorization.CanApprove`
sería solo `DIRECTOR`; si es B, se necesita un estado intermedio
"Aprobado por Dirección, pendiente de Subdirección" que hoy no existe en
Expedientes Service.

### CONFLICTO F1/F3/F5-01 — ¿Generan Expediente de Mesa de Partes?

Ya descrito en la sección 8 como "pendiente" para cada flujo, pero se eleva
aquí a conflicto porque el "Mapeo a microservicios" de la fuente asigna F1,
F3 y F5 conceptualmente a Expedientes/Documentos/Seguimiento sin decir
explícitamente si usan el mismo objeto "Expediente" ya construido (con su
máquina de estados PENDIENTE/EN_PROCESO/ATENDIDO/OBSERVADO) o si necesitan un
concepto propio más simple (un "registro" o "caso" sin esa máquina de
estados).

**Alternativa A**: Todo se modela como Expediente (reutiliza lo ya construido,
pero fuerza estados que no calzan bien — ej. F3 no tiene un "ATENDIDO" claro,
tiene "Compromiso Cumplido/Incumplido").

**Alternativa B**: F2 y F4 usan Expediente (ya lo hacen); F1, F3 y F5 usan un
concepto de "caso" o "registro" propio de sus futuros microservicios
(Seguimiento, y un eventual módulo de gestión docente), sin pasar por
Expedientes Service.

**Decisión requerida**: definir esto antes de diseñar Seguimiento Service,
porque determina si Seguimiento tiene su propia tabla de "casos" o si solo
adjunta tareas a expedientes existentes.

---

## 23. Preguntas pendientes

| ID | Pregunta | Flujo | Responsable de confirmar | Impacto |
|---|---|---|---|---|
| F1-01 | ¿El formato/informe de cumplimiento de horas genera un Expediente formal o es un registro interno separado? | F1 | Institución (Subdirección/Dirección) | Diseño de datos de un futuro módulo de control docente |
| F1-02 | ¿Cuál es la periodicidad real del reporte (mensual, bimestral)? | F1 | Institución | Define el "periodo" como dato del expediente/registro |
| F2-01 | ¿Cuáles son exactamente los requisitos documentarios para Certificado vs. Constancia? | F2 | Secretaría | Validaciones de Documentos/Expedientes |
| F3-01 | ¿Quién es oficialmente responsable del seguimiento del compromiso? | F3 | Institución | Autorización de Seguimiento Service |
| F3-02 | ¿"Trabajador del Colegio" corresponde a DOCENTE, AUXILIAR, o requiere un rol nuevo? | F3 | Institución | Matriz de permisos, Auth Service |
| F3-03 | ¿Existe un plazo estándar para verificar el cumplimiento de un compromiso? | F3 | Institución | Diseño de "fecha límite" en Seguimiento Service |
| F4-01 | Ver Conflicto F4-01 (¿quién aprueba realmente?) | F4 | Dirección/Subdirección | Modelo de autorización y máquina de estados |
| F4-02 | ¿Existe una plantilla oficial para el proveído PDF? | F4 | Institución | Diseño de generación de documentos (hoy manual) |
| F5-01 | ¿La notificación al docente del resultado del monitoreo es obligatoria? | F5 | Subdirección | Regla de negocio de Notificaciones Service |
| F5-02 | ¿El monitoreo genera Expediente, o vive solo en un historial propio? | F5 | Institución | Diseño de datos, ver Conflicto F1/F3/F5-01 |
| F5-03 | ¿Existe seguimiento explícito de planes de mejora post-monitoreo, con fecha límite? | F5 | Subdirección | Alcance de Seguimiento Service |
| GEN-01 | Medio de notificación real disponible hoy (¿correo institucional? ¿solo dentro de la plataforma?) | Todos | Institución/TI | Alcance de Notificaciones Service (sección 17) |

No se responde ninguna de estas preguntas inventando una respuesta.

---

## 24–28. Matrices consolidadas

### 24. Matriz general final

| Flujo | Inicio | Actor principal | Expediente | Documentos | Derivación | Seguimiento | Resultado |
|---|---|---|---|---|---|---|---|
| F1 | Auxiliar habilita formato | Auxiliar/Profesor | Pendiente confirmar | Formato, Informe | Prof→Subdir→Dir | No | Informe recibido por Dirección |
| F2 | Solicitante inicia trámite | Solicitante | Sí (ya implementado) | Formulario, adjuntos | Sistema→Secretaría→Dirección | No | Trámite derivado y en seguimiento de estado |
| F3 | Colegio cita al padre | Trabajador del Colegio | Pendiente confirmar | Citación, Acta, Compromiso | Trabajador→Padre; →Responsable | Sí (compromiso) | Compromiso cumplido/incumplido |
| F4 | Solicitante inicia trámite | Solicitante | Sí (con Conflicto F4-01) | Formulario, Proveído PDF | Secretaría→Dirección→Subdirección→Profesor | No (salvo reincidencia, no descrito) | Proveído aprobado o desaprobación registrada |
| F5 | Subdirectora inicia monitoreo | Subdirectora | Pendiente confirmar | Ficha de monitoreo | Subdirectora→Sistema→Profesor | Pendiente confirmar (F5-03) | Evaluación guardada en historial docente |

### 25. Matriz de estados consolidada

| Flujo | Estado inicial | Estados intermedios | Estados de excepción | Estado final |
|---|---|---|---|---|
| F1 | Registrado/Habilitado | Enviado, Consolidado | Con Observaciones | Informe Entregado/Finalizado |
| F2 | Recibido | Derivado a Dirección | Observado | Finalizado/Atendido |
| F3 | Citación Registrada | Reunión Realizada, En Seguimiento | Compromiso Incumplido | Compromiso Cumplido / Finalizado |
| F4 | Recibido | Derivado a Dirección | Observado, Desaprobado | Aprobado |
| F5 | Monitoreo Iniciado | Evaluado | (ninguno descrito) | Finalizado/Guardado o Notificado |

### 26. Matriz de documentos consolidada

| Flujo | Documento | Tipo | Generador | Responsable | Asociado a expediente |
|---|---|---|---|---|---|
| F1 | Formato de Horas | Principal | Auxiliar/Profesor | Profesor | Pendiente confirmar |
| F1 | Informe de Horas | Generado | Subdirectora | Subdirectora | Pendiente confirmar |
| F2 | Solicitud Certificado/Constancia | Principal | Solicitante | Solicitante | Sí |
| F3 | Formulario de Citación | Principal | Trabajador | Trabajador | Pendiente confirmar |
| F3 | Acta de Reunión | Firmado | Trabajador/Padre | Ambos | Pendiente confirmar |
| F4 | Solicitud Permiso/Justificación | Principal | Solicitante | Solicitante | Sí |
| F4 | Proveído Digital PDF | Generado/Firmado | Subdirección/Sistema | Subdirección | Sí |
| F5 | Ficha de Monitoreo | Informe | Subdirectora | Subdirectora | Pendiente confirmar |

### 27. Matriz de reglas de negocio

| ID | Regla | Flujo | Tipo | Confirmada |
|---|---|---|---|---|
| RN-01 | Código único de seguimiento por trámite | F2, F4 | Explícita | Sí |
| RN-02 | Validación de completitud obligatoria antes de llegar a un área | F2, F4 | Explícita | Sí |
| RN-03 | Secretaría valida requisitos antes de derivar a Dirección | F2, F4 | Explícita | Sí |
| RN-04 | Aprobación de F4 requiere Dirección + Subdirección | F4 | Explícita (con ambigüedad, ver F4-01) | Parcial |
| RN-05 | Reunión de citación cierra con acta firmada y compromiso | F3 | Explícita | Sí |
| RN-06 | Incumplimiento habilita re-citación directa | F3 | Explícita | Sí |
| RN-07 | Monitoreo impacta historial acumulativo del docente | F5 | Explícita | Sí |
| RN-08 | Auxiliar habilita, Subdirectora valida antes del informe (F1) | F1 | Explícita | Sí |
| RN-09 | El solicitante puede subsanar y reenviar | F2, F4 | Inferida | No |
| RN-10 | Docente nunca aprueba, solo recibe notificación | F1, F4, F5 | Inferida | No |
| RN-11 | Compromisos deben asociarse al historial del estudiante/padre | F3 | Inferida | No |
| RN-12 | Notificación al docente en F5 es condicional/opcional | F5 | Explícita | Sí (mecanismo de la condición no descrito) |

### 28. Matriz de microservicios

| Flujo | Auth | Usuarios | Expedientes | Documentos | Derivaciones | Seguimiento | Notificaciones | Reportes |
|---|---|---|---|---|---|---|---|---|
| F1 | ✓ | ✓ | ? | ✓ | ✓ | — | ✓ | ✓ |
| F2 | ✓ | ✓ | ✓ | ✓ | ✓ | — | ✓ | ✓ |
| F3 | ✓ | ✓ | ? | ✓ | ✓ | ✓ | ✓ | ✓ |
| F4 | ✓ | ✓ | ✓ | ✓ | ✓ | ? | ✓ | ✓ |
| F5 | ✓ | ✓ | ? | ✓ | — | ? | ✓ | ✓ |

(`✓` = participa, `—` = no participa, `?` = pendiente de las decisiones en
sección 22)

---

## 29. Recomendaciones

1. Resolver **Conflicto F4-01** con la institución antes de construir
   Derivaciones Service, porque define si su máquina de estados necesita un
   paso intermedio "Aprobado por Dirección, pendiente de Subdirección".
2. Resolver **si F1/F3/F5 usan Expediente o un concepto propio** antes de
   diseñar Seguimiento Service — construirlo mal aquí obligaría a
   rediseñarlo después, como ya pasó una vez con `user_profiles` en Auth
   (tabla creada y nunca usada).
3. Diseñar **Derivaciones Service** primero (ya tiene datos claros: origen,
   destino, motivo, condición — sección 15), reutilizando exactamente el
   patrón de Expedientes/Usuarios/Documentos (base aislada, gRPC, JWT propio,
   autorización local documentada).
4. Diseñar **Notificaciones Service** como consumidor de eventos (sección 21)
   en vez de que cada servicio lo llame directamente — evita que Expedientes,
   Derivaciones y Seguimiento terminen todos con un cliente gRPC hacia
   Notificaciones.
5. No implementar generación automática de PDFs (proveídos, informes) todavía
   — no hay plantilla definida (pregunta F4-02); mantener la carga manual ya
   implementada en Documentos Service hasta confirmar el formato oficial.
6. Levantar las preguntas de la sección 23 con la institución antes de la
   siguiente etapa de construcción — varias (F3-02, F4-01, F5-02) cambian el
   modelo de datos, no solo el texto de una pantalla.

---

## 30. Próxima etapa

**ETAPA 12 — ESPECIFICACIÓN FUNCIONAL: COMPLETADA**

Pendientes que deben resolverse antes de comenzar la siguiente etapa de
construcción:

- [ ] Confirmar con la institución: Conflicto F4-01 (aprobación de permisos).
- [ ] Confirmar con la institución: si F1/F3/F5 generan Expediente o un
      concepto propio (Conflicto sección 22).
- [ ] Confirmar F3-02 (rol real de "Trabajador del Colegio"/"Responsable de
      Seguimiento").
- [ ] Confirmar GEN-01 (medio real de notificación disponible hoy).

Recomendación de secuencia (ver sección 29): **Derivaciones Service** es el
siguiente candidato natural para construir — tiene datos y reglas más claras
en el análisis que Seguimiento o Notificaciones, y no depende de resolver el
Conflicto F1/F3/F5 (F2 y F4, que sí usan Expediente, ya tienen sus
derivaciones bien descritas). Seguimiento y Notificaciones deberían esperar a
que se resuelvan sus preguntas pendientes respectivas.
