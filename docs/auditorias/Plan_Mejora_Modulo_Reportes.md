# Plan de mejora del módulo de Reportes – SIS-MESA-PARTES

- **Fecha:** 30/10/2026
- **Tipo:** auditoría de la Fase 1 desplegada y plan de evolución. No se modificó código,
  base de datos ni servicios.
- **Fuentes verificadas:**
  - código de la rama `feat/psicologia-client-credentials`;
  - consultas **solo agregadas** (conteos, sin contenido ni datos personales) sobre
    `expedientes_db`, `derivaciones_db` y `documentos_db`.
- **Documentos previos:**
  [Auditoría funcional](Auditoria_Funcional_Modulo_Reportes.md) ·
  [Documentación de la Fase 1](../reportes/modulo-reportes.md).

> **Regla del plan.** No se propone ningún indicador de tiempos de atención, resolución,
> cumplimiento o plazos con los datos actuales. Todo lo que dependa de ellos se clasifica en
> la Fase 3 o posterior.

---

## 1. Estado real de la Fase 1 (verificado)

### 1.1 Componentes

| Capa | Archivo | Qué hace realmente |
|---|---|---|
| SQL | [reporte_repository.go](../../backend/services/expedientes/internal/repository/reporte_repository.go) | **Una** consulta `GROUP BY estado, tipo, prioridad, area_actual, mes, días_pendiente` sobre `expedientes` con `activo = TRUE`. Fechas en `America/Lima`; `hasta` inclusivo |
| Lógica | [reporteexpedienteslogic.go](../../backend/services/expedientes/internal/logic/reporteexpedienteslogic.go) | Exige `reportes.view` y rechaza roles externos. Valida los filtros. Aplica el alcance (`CanViewAll` o `AreaDelRol`). Arma las agrupaciones con catálogos completos y la serie mensual continua |
| Gateway | `reporteexpedienteslogic.go`, `exportarreporteexpedienteslogic.go`, [internal/reportes](../../backend/gateway/internal/reportes/reportes.go) | Transporta la consulta. Genera XLSX y CSV volviendo a consultar el reporte con los mismos filtros |
| Límites | [limite.go](../../backend/gateway/internal/limite/limite.go) | 60 consultas/min y 10 exportaciones/min por credencial |
| Frontend | [Reportes.jsx](../../frontend/src/pages/Reportes/Reportes.jsx), [reportesData.js](../../frontend/src/pages/Reportes/reportesData.js) | Una sola pantalla: filtros, 4 indicadores, 4 distribuciones, tendencia mensual y antigüedad. Botones Excel, CSV e Imprimir |
| Dashboard | [dashboardData.js](../../frontend/src/pages/Dashboard/dashboardData.js) | Con `reportes.view` usa el resumen agregado; si falla, vuelve al listado |

### 1.2 Indicadores y su fuente

| Indicador | Cálculo real | Fuente |
|---|---|---|
| Total | `COUNT(*)` de expedientes activos que cumplen los filtros y el alcance | `expedientes` |
| Pendientes | Estados `PENDIENTE` + `EN_PROCESO` | `expedientes.estado` |
| Atendidos / Observados | Estado `ATENDIDO` / `OBSERVADO` (actual) | `expedientes.estado` |
| Por estado, tipo, prioridad, área | Conteo por el valor **actual** de cada columna; catálogo completo con ceros | `expedientes` |
| Tendencia mensual | Conteo por mes de `fecha_registro` (hora de Lima), serie continua | `expedientes.fecha_registro` |
| Antigüedad de pendientes | `hoy − fecha_registro` en días, en 5 rangos descriptivos | `expedientes.fecha_registro` |

### 1.3 Filtros

Todos funcionan y se combinan (AND):

- desde y hasta;
- estado;
- tipo;
- prioridad;
- área actual.

Se verificaron en producción con los 5 perfiles. El filtro de área se suma al alcance, así
que DIRECTOR con área = Secretaría obtiene 0.

### 1.4 Exportación

XLSX y CSV con un **único reporte**:

- metadatos: título, institución, fecha de generación, periodo, alcance y cada filtro;
- 7 secciones con totales;
- números reales en Excel;
- CSV con BOM, CRLF y protección contra fórmulas.

La impresión usa `@media print` con un encabezado propio.

### 1.5 Limitaciones verificadas de la Fase 1

| # | Limitación | Impacto |
|---|---|---|
| L1 | Solo agrupación **mensual**: no hay día ni semana | Con pocos meses de datos, la tendencia tiene apenas 2 puntos |
| L2 | Distribuciones **independientes**: no hay tablas cruzadas (área × estado, tipo × estado) | No responde "¿qué está pendiente en Dirección?" sin cambiar filtros |
| L3 | La antigüedad se calcula **hasta hoy**, aunque `hasta` esté en el pasado | Correcto para el estado actual, pero conviene aclararlo en pantalla |
| L4 | El alcance de DIRECTOR y SUBDIRECTOR es el área **actual** | No ven lo que ya pasó por su área. Requiere historial (Fase 3) |
| L5 | Estado y área son los **actuales**, no los vigentes al cierre del periodo | Un reporte de "septiembre" muestra el estado de hoy de lo registrado en septiembre |
| L6 | Sin comparación entre periodos | Hay que consultar dos veces a mano |
| L7 | Sin reportes de Derivaciones ni de Documentos | Ver la sección 3 |
| L8 | Una sola pantalla larga | Crecerá mal si se añaden secciones sin organizarlas |

Nada de esto es un error. Son límites del alcance de la Fase 1 y deben conservarse
explícitos en la interfaz.

---

## 2. Realidad de los datos (condiciona todo el plan)

Consultas agregadas del 30/10/2026:

| Fuente | Volumen | Rango temporal | Observaciones |
|---|---|---|---|
| `expedientes` activos | **5** | 11/09/2026 – 23/10/2026 (2 meses) | 4 pendientes, 1 atendido |
| `expedientes` dados de baja | 35 | 05/09 – 11/09/2026 | Casi seguro pruebas (excluidas correctamente por la Fase 1) |
| `derivaciones` | 35 | 09/09 – 14/09/2026 | **Solo 2** corresponden a expedientes activos |
| `documentos` | 20 activos (16 adjuntos y 4 proveídos), 16 eliminados | 07/09 – 23/10/2026 | 5 pertenecen a expedientes activos |

### 2.1 Problemas de calidad en `derivaciones` (verificados)

1. **Vocabulario mezclado** en `origen` y `destino`:
   - códigos: `SECRETARIA → DIRECTOR`;
   - etiquetas: `Secretaría → Dirección`, `Direccion`, `Secretaria`;
   - texto con codificación dañada: `Direcci�n`.
2. **Datos personales en `origen`.** Las derivaciones que no son de tipo `DERIVACION`
   (notificación, asignación, recepción, aprobación) guardan `origen = "<usuario> (<Rol>)"`
   ([DerivacionesPanel.jsx:27](../../frontend/src/components/dashboard/DerivacionesPanel.jsx#L27)).
   **No deben mostrarse ni agruparse tal cual en reportes.**
3. **Solo `tipo = DERIVACION` usa códigos fiables.** Viene del flujo `DerivarExpediente`, que
   usa `area_actual` como origen y el código validado como destino
   ([derivarexpedientelogic.go:78-85](../../backend/gateway/internal/logic/derivarexpedientelogic.go#L78-L85)).
   Algunas filas antiguas de ese tipo tienen etiquetas.
4. **El rechazo no se distingue.** `RechazarExpediente` registra `tipo = DERIVACION`, con
   destino `SECRETARIA` ([rechazarexpedientelogic.go:61-66](../../backend/gateway/internal/logic/rechazarexpedientelogic.go#L61-L66)).
   No hay forma fiable de contar "derivaciones rechazadas".
5. **Sin columna de expediente activo.** Derivaciones y Documentos guardan `expediente_id` sin
   saber si ese expediente está activo: hay que cruzar con Expedientes.

### 2.2 Consecuencias

- **Las comparaciones entre meses o años no son válidas todavía:** hay 2 meses parciales y
  5 expedientes. Deben implementarse con una **regla de suficiencia**: mostrar la variación
  solo si ambos periodos están completos y tienen al menos N registros. Si no, mostrar los
  valores absolutos y el aviso "datos insuficientes para comparar".
- **Los reportes de derivaciones** deben limitarse a `tipo = DERIVACION`, con normalización
  de las etiquetas antiguas y excluyendo las de expedientes dados de baja.

---

## 3. Oportunidades de mejora y nuevos reportes

Leyenda de "¿Datos?": ✅ existen y son fiables · ⚠️ con salvedades · ❌ no existen.

### A. Derivaciones

| Reporte | ¿Datos? | Decisión | Fase |
|---|---|---|---|
| Derivaciones por periodo | ⚠️ `fecha_registro` existe; hay que excluir expedientes dados de baja | **Sí**, solo `tipo = DERIVACION` | 2 |
| Por área de destino | ⚠️ Códigos fiables en el flujo actual; las etiquetas antiguas se normalizan (Dirección → DIRECTOR…) y las irreconocibles van a "Sin clasificar" | **Sí** | 2 |
| Por área de origen | ⚠️ Igual que el anterior | **Sí** | 2 |
| Flujo origen → destino (matriz) | ⚠️ Igual que el anterior | **Sí**, como tabla | 2 |
| Por tipo de trámite | ⚠️ El tipo está en Expedientes: requiere cruzar `expediente_id` | **Sí** | 2 |
| Evolución mensual | ⚠️ | **Sí** (misma serie que expedientes) | 2 |
| "Derivaciones pendientes" | ✅ Es "expedientes abiertos por área actual", que **ya existe** como área × estado (ver C) | No duplicar: se cubre con la tabla área × estado | 2 (vía C) |
| Derivaciones rechazadas | ❌ El rechazo no se distingue | Requiere registrar `tipo = RECHAZO` o un evento propio | 3 |
| Notificaciones, asignaciones, recepciones, aprobaciones | ❌ Texto libre con nombres de usuario | Solo conteo **por tipo**, sin origen ni destino | 2 (conteo) / 3 (normalizar) |

### B. Atención documentaria

| Reporte | ¿Datos? | Decisión | Fase |
|---|---|---|---|
| Expedientes recibidos por día y semana | ✅ `fecha_registro` | **Sí**: añadir granularidad día, semana o mes al reporte existente (no un endpoint nuevo) | 2 |
| Documentos (adjuntos y proveídos) por periodo | ✅ `documentos.fecha_registro`, `estado = ACTIVO` | **Sí**, excluyendo los de expedientes dados de baja | 2 |
| Por tipo documental | ✅ `tipo_documento` (6 valores) | **Sí** | 2 |
| Observados / atendidos | ✅ Ya existen (estado actual) | No duplicar | — |
| Observados o atendidos **en el periodo** (por fecha del evento) | ❌ No se registra la fecha de cambio de estado | Requiere historial | 3 |
| Comparación entre periodos | ⚠️ Faltan datos históricos | Con regla de suficiencia (ver E) | 4 |

### C. Áreas

| Reporte | ¿Datos? | Decisión | Fase |
|---|---|---|---|
| Volumen por área actual | ✅ Ya existe | Conservar | — |
| **Área × estado** (pendientes y atendidos por área) | ✅ La consulta de la Fase 1 ya agrupa por estado y área | **Sí**: solo exponer la combinación que ya se calcula | 2 |
| Tipo de trámite × estado | ✅ Ídem | **Sí** | 2 |
| Comparación de carga entre áreas | ✅ Área actual × estado pendiente | **Sí**, como tabla (no ranking) | 2 |
| Carga **histórica** por área (lo que pasó por cada área) | ⚠️ Solo con derivaciones `DERIVACION`, incompletas antes de la Etapa 3 | Mostrar como "derivaciones recibidas" con aviso | 2 (aprox.) / 3 (exacto) |

Visibilidad: se conserva el alcance de la Fase 1. DIRECTOR y SUBDIRECTOR solo ven su área
actual. Las tablas cruzadas no amplían lo que cada rol puede ver.

### D. Actividad

| Reporte | ¿Datos? | Decisión | Fase |
|---|---|---|---|
| Actividad de registro por periodo | ✅ `fecha_registro` | Ya cubierto por la tendencia (más día y semana) | 2 |
| Actividad **por rol o área** (registros, derivaciones, documentos subidos) | ⚠️ `derivaciones.registrado_por` y `documentos.subido_por` son IDs de usuario: se agrupa por su **rol** consultando Auth/Usuarios | **Sí**, agregado por rol, sin nombres | 2 |
| Expedientes registrados por usuario | ⚠️ `solicitante_id` mezcla al solicitante externo con el registrador interno | No por persona. Requiere `registrado_por` | 3 |
| Volumen por responsable individual | ❌ No hay responsable por persona (solo área) | **No implementar** | 3+ |

Política recomendada: **sin rankings de personas**. Los volúmenes no reflejan la complejidad
ni el tiempo invertido, y publicarlos por nombre es engañoso y expone información personal.
Si la institución lo requiere, debe ser una vista exclusiva de ADMIN, ordenada
alfabéticamente y con aviso. Queda como decisión pendiente.

### E. Comparativos

| Reporte | ¿Datos? | Decisión | Fase |
|---|---|---|---|
| Periodo actual frente a periodo anterior equivalente | ⚠️ Técnicamente calculable (dos consultas del reporte existente) | Implementar el **mecanismo** con regla de suficiencia; hoy mostrará "datos insuficientes" | 2 (mecanismo) |
| Mes contra mes, año contra año | ❌ Hay 2 meses parciales | Activar cuando haya ≥ 12 meses | 4 |
| Periodos personalizados (A frente a B) | ⚠️ | Junto con el mecanismo anterior | 4 |
| Variación porcentual | ⚠️ Fórmula en la sección 5 | Solo con datos suficientes | 2 (mecanismo) / 4 (uso real) |

### F. Visualización

| Gráfico | Uso recomendado | No usar para |
|---|---|---|
| Barras horizontales (BarList existente) | Distribuciones con nombres largos (tipo, área, tipo documental) | — |
| Barras agrupadas o apiladas (nuevo, SVG propio, sin librería) | Área × estado, tipo × estado, periodo A frente a B | Más de 6 series |
| Líneas (LineChart existente) | Tendencias por día, semana o mes | Categorías sin orden |
| Circular o dona | **Solo** parte de un todo con 2 a 4 categorías: prioridad, pendientes/atendidos/observados | Tipos (9) o áreas (6): ilegible |
| Tabla con totales | Siempre como complemento accesible (y para la impresión) | — |
| Indicadores resumidos | Máximo 4 a 5 por sección | Repetir cifras ya visibles |

---

## 4. Calidad de los indicadores propuestos

| Indicador | Qué mide | Datos y servicio | ¿Existen? | ¿Fiables? | Limitación |
|---|---|---|---|---|---|
| Expedientes por día o semana | Volumen de recepción | `expedientes.fecha_registro` (Expedientes) | ✅ | ✅ | Ninguna |
| Área × estado | Situación actual por área | `estado`, `area_actual` | ✅ | ✅ | Foto actual, no histórica (L5) |
| Tipo × estado | Situación por trámite | `tipo`, `estado` | ✅ | ✅ | Ídem |
| Derivaciones por periodo, origen y destino | Movimiento entre áreas | `derivaciones` (Derivaciones) + `activo` (Expedientes) | ✅ | ⚠️ | Solo `DERIVACION`; requiere normalizar las etiquetas antiguas; incluye rechazos sin distinguirlos |
| Derivaciones por tipo de trámite | Qué trámites se mueven más | `derivaciones.expediente_id` → `expedientes.tipo` | ✅ | ⚠️ | Cruce entre servicios |
| Documentos por tipo y periodo | Producción documental | `documentos` (Documentos) + `activo` | ✅ | ✅ | Excluir eliminados y los de expedientes dados de baja |
| Actividad por rol | Distribución del trabajo de registro | `registrado_por` y `subido_por` → rol (Auth) | ✅ | ⚠️ | El rol es el **actual** del usuario, no el que tenía al operar |
| Variación entre periodos | Crecimiento o descenso | Dos ejecuciones del reporte | ✅ | ⚠️ | Con regla de suficiencia |
| Tiempos de atención o resolución | — | Historial de estados | ❌ | — | **Fase 3** |
| Cumplimiento de plazos o fuera de plazo | — | Plazos institucionales | ❌ | — | **Fase 3+** |
| Rechazos | — | Evento de rechazo propio | ❌ | — | **Fase 3** |

---

## 5. Fórmulas

Notación: `E` = expedientes activos visibles para el usuario que cumplen los filtros;
`D` = derivaciones `tipo = DERIVACION` cuyo expediente está en `E` (o en los expedientes
activos visibles, si el filtro no se aplica al expediente); `Doc` = documentos `ACTIVO` cuyo
expediente está en E.

| Indicador | Fórmula |
|---|---|
| Recibidos en el periodo P | `|{e ∈ E : fecha_registro(e) ∈ P}|` |
| Serie por granularidad g (día, semana ISO lunes–domingo, mes) | `count(E) GROUP BY date_trunc(g, fecha_registro AT TIME ZONE 'America/Lima')`, continua con ceros |
| Área × estado | `count(E) GROUP BY area_actual, estado`. Total de fila = volumen del área; total de columna = Fase 1 |
| % pendiente del área a | `pendientes(a) / total(a) × 100` (solo si `total(a) > 0`) |
| Derivaciones por destino | `count(D) GROUP BY normalizar(destino)` |
| Matriz de flujo | `count(D) GROUP BY normalizar(origen), normalizar(destino)` |
| Documentos por tipo | `count(Doc) GROUP BY tipo_documento` |
| Actividad por rol | `count(acciones) GROUP BY rol_actual(usuario)`, donde acciones = documentos subidos y derivaciones registradas |
| Variación % | `(V_actual − V_anterior) / V_anterior × 100`. Se muestra solo si los dos periodos están completos y `V_anterior ≥ N_min` (sugerido 10). Si `V_anterior = 0`, "sin base de comparación" |
| Periodo anterior equivalente | Mismo número de días inmediatamente antes de `desde`, o el mismo mes del año anterior (año contra año) |

Función `normalizar(x)`: convierte a mayúsculas y quita tildes; las etiquetas conocidas
(`DIRECCION` → `DIRECTOR`, `SECRETARIA`, `SUBDIRECCION` → `SUBDIRECTOR`, `ADMINISTRACION` →
`ADMIN`, `DOCENTE`, `AUXILIAR`) se traducen a su código; cualquier otro valor va a "Sin
clasificar". **Nunca** se agrupa por el texto original, porque puede contener nombres de
usuario.

---

## 6. Dependencias de datos y arquitectura propuesta

```text
/reportes (pestañas)
   │  GET /api/reportes/expedientes   (EXISTENTE; + parámetros opcionales: granularidad, cruces)
   │  GET /api/reportes/derivaciones  (NUEVO)
   │  GET /api/reportes/documentos    (NUEVO)
   ▼
Gateway ── Expedientes.ReporteExpedientes (ampliado, compatible)
        ── Derivaciones.ReporteDerivaciones (NUEVO) ──► Expedientes (ids activos visibles + tipo)
        ── Documentos.ReporteDocumentos   (NUEVO)   ──► Expedientes (ids activos visibles)
```

- **Agregación en el dueño del dato.** Derivaciones y Documentos calculan sus propios
  conteos. No se consulta la base de otro servicio. El alcance y la exclusión de bajas se
  obtienen de Expedientes:
  - patrón ya usado en `expedientesVisibles` de Derivaciones
    ([listderivacioneslogic.go:78-87](../../backend/services/derivaciones/internal/logic/listderivacioneslogic.go#L78-L87));
  - preferiblemente con un RPC ligero nuevo en Expedientes, `ExpedientesVisiblesIDs`, que
    devuelve id y tipo con los mismos filtros, para evitar paginar el listado completo.
- **Compatibilidad.** Los campos nuevos de `ReporteExpedientes` son **opcionales**
  (`granularidad`, `cruces`). Sin ellos, la respuesta es idéntica a la actual, así que el
  Dashboard y la Fase 1 no se tocan.
- **Permisos.** Mismo `reportes.view` y mismo alcance. No se crean permisos nuevos.
- **Sin migraciones en la Fase 2.** La normalización de derivaciones se hace al leer.

| Mejora | Servicio | Cambios | Migración |
|---|---|---|---|
| Granularidad día/semana, área × estado, tipo × estado | Expedientes | Ampliar el RPC existente (campos opcionales) | No |
| Reporte de derivaciones | Derivaciones + Expedientes | RPC nuevo + RPC ligero de ids | No |
| Reporte de documentos | Documentos + Expedientes | RPC nuevo | No |
| Actividad por rol | Derivaciones, Documentos + Auth/Usuarios (rol) | Agregado por `registrado_por` / `subido_por` → rol | No |
| Comparación de periodos | Frontend + Gateway (dos llamadas al mismo RPC) | — | No |
| Rechazos, tiempos, historial por área | Expedientes | Tabla de eventos de estado; `tipo = RECHAZO` | **Sí (Fase 3)** |
| Plazos | Expedientes + decisión institucional | Catálogo de plazos | **Sí (Fase 3)** |

---

## 7. Organización visual propuesta

Pestañas dentro de `/reportes`. **Un solo panel de filtros compartido** arriba; cada pestaña
usa los filtros que le aplican y desactiva los demás con una explicación. La exportación y
la impresión corresponden a la **pestaña activa**.

| Pestaña | Contenido | Estado |
|---|---|---|
| 1. Resumen | Lo actual de la Fase 1, **sin cambios** | Existente |
| 2. Expedientes y áreas | Selector de granularidad (día/semana/mes); tablas área × estado y tipo × estado; barras apiladas | Fase 2 |
| 3. Derivaciones | Total; por destino y origen; matriz de flujo; por tipo de trámite; evolución; aviso de calidad de datos | Fase 2 |
| 4. Documentos | Total; por tipo documental; evolución | Fase 2 |
| 5. Comparaciones | Periodo frente al anterior equivalente, con regla de suficiencia | Mecanismo en Fase 2; uso pleno en Fase 4 |
| (Actividad por rol) | Sección dentro de la pestaña 2, no pestaña propia | Fase 2 |

Criterios de interfaz:

- máximo unos 6 bloques por pestaña;
- cada gráfico con su tabla accesible;
- textos que digan qué fecha se usa y qué es "foto actual";
- la pestaña recordada en la URL (`/reportes?vista=derivaciones`) para compartir e imprimir.

---

## 8. Exportaciones

- **No duplicar.** El exportador actual (`internal/reportes`) se generaliza a "secciones":
  cada pestaña declara sus secciones y reutiliza los mismos metadatos (título, periodo,
  alcance, filtros, fecha de generación) y el mismo formato (XLSX numérico, CSV con BOM y
  protección).
- **Un endpoint por fuente** (`/api/reportes/<fuente>/exportar`), con los mismos parámetros
  que su consulta. Así el archivo siempre coincide con la pantalla.
- **Excel con varias hojas** para "Exportar todo": una hoja por pestaña, en la Fase 2 tardía.
- **PDF institucional:** recomendable **solo** cuando haya un destinatario formal (informe a
  UGEL, memoria anual) o cuando se requiera firma o membrete. Hasta entonces, la impresión del
  navegador con `@media print` cubre la necesidad. La generación de PDF en el servidor ya
  tiene precedente en `psicologia/internal/pdf` (Fase 4 o cuando la institución lo pida).

---

## 9. Plan por fases

### Fase 2 – Con los datos actuales (sin migraciones)

| # | Mejora | Beneficio | Complejidad | Dependencias | Riesgos | Prioridad |
|---|---|---|---|---|---|---|
| 2.1 | Pestañas en `/reportes` (Resumen intacto) | Ordena el crecimiento sin saturar | Baja | — | Ninguno si Resumen no cambia | **Alta** |
| 2.2 | Granularidad día/semana/mes | Tendencias útiles con poco historial | Baja | Ampliar RPC (opcional) | Series diarias largas: limitar a 366 puntos | **Alta** |
| 2.3 | Tablas área × estado y tipo × estado | Responde "qué está pendiente y dónde" | Baja (la consulta ya agrupa) | Ampliar RPC | Ninguno | **Alta** |
| 2.4 | Reporte de derivaciones (`DERIVACION`, normalizado, sin bajas) | Movimiento real entre áreas | Media | RPC en Derivaciones + ids visibles de Expedientes | Etiquetas antiguas: "Sin clasificar" visible y explicado | **Alta** |
| 2.5 | Reporte de documentos por tipo y periodo | Producción documental | Media | RPC en Documentos + ids visibles | Ninguno relevante | Media |
| 2.6 | Exportador por secciones (Excel/CSV por pestaña) | Mismo estándar para todo | Media | 2.3–2.5 | Mantener intacto el formato actual del Resumen | Media |
| 2.7 | Mecanismo de comparación con regla de suficiencia | Base para la Fase 4 | Baja-media | 2.1 | Mostrar variaciones con pocos datos: lo mitiga la regla | Media |
| 2.8 | Actividad por rol (sin nombres) | Distribución del trabajo | Media | Auth/Usuarios para el rol | Rol actual frente al rol histórico: aviso | Baja |
| 2.9 | Gráfico de dona para prioridad y para la composición pendientes/atendidos/observados | Lectura rápida | Baja | — | Uso indebido con muchas categorías: restringido | Baja |

### Fase 3 – Requiere nuevos registros o cambios de base de datos

| Mejora | Datos nuevos | Beneficio |
|---|---|---|
| Historial de estados (`expediente_eventos`) en **todas** las transiciones | Tabla de eventos (estado y área anterior y nueva, usuario, fecha) | Tiempos reales, estados al cierre del periodo, carga histórica por área |
| Tiempo promedio y mediana de atención por tipo y área | Fase 3.1 | Indicador clave de eficiencia (solo con datos posteriores a la migración) |
| Rechazos (`tipo = RECHAZO` o evento propio) | Cambio en Rechazar | Medir calidad de expedientes presentados |
| Normalización de derivaciones no `DERIVACION` | Códigos de área y actor por ID, no texto con nombre | Reportes completos de notificaciones y aprobaciones |
| `registrado_por` en expedientes | Columna nueva | Actividad de registro interno por usuario |
| Plazos por tipo de trámite y "fuera de plazo" | Catálogo institucional y fecha de atención | Cumplimiento normativo |
| Alcance histórico para DIRECTOR y SUBDIRECTOR | Fase 3.1 | Ver lo que pasó por su área |

### Fase 4 – Analítica avanzada

- Comparaciones mes contra mes y año contra año, y periodos personalizados (cuando haya
  ≥ 12 meses de datos).
- Estacionalidad (matrícula, inicio y fin de año).
- PDF institucional con membrete.
- Exportación consolidada multihoja.

---

## 10. Recomendación: qué implementar primero

**Primer bloque de la Fase 2 (2.1 + 2.2 + 2.3).** Pestañas, granularidad día/semana/mes y
tablas área × estado y tipo × estado.

- Solo amplía, con campos **opcionales**, el RPC y la consulta que ya existen: la consulta
  actual ya agrupa por estado, tipo y área.
- No toca la vista Resumen, el Dashboard, los permisos ni el esquema.
- Es lo que más valor aporta con el volumen de datos actual: con 2 meses de historia, una
  serie diaria o semanal y la situación actual por área informan más que una comparación
  mensual.

**Segundo bloque (2.4 + 2.6).** Derivaciones con normalización y exportación por secciones.
Es la mayor ganancia analítica nueva, pero requiere tratar la calidad de los datos y un RPC
nuevo.

**Antes de la Fase 3:** decidir institucionalmente los plazos por tipo de trámite y si se
quieren reportes por persona.

**Decisiones que necesito de la institución:**
1. ¿Se permiten reportes de actividad por persona (solo ADMIN) o solo por rol?
2. ¿Las derivaciones antiguas con etiquetas irreconocibles se muestran como "Sin
   clasificar" o se excluyen?
3. ¿Qué umbral mínimo de registros habilita las comparaciones (sugerido 10 por periodo)?
4. ¿Existe un destinatario formal que justifique el PDF institucional?

> No se implementó ninguna mejora. Queda a la espera de autorización.
