# Auditoría funcional del módulo de Reportes – SIS-MESA-PARTES

- **Fecha:** 30/10/2026
- **Tipo:** auditoría de solo lectura. No se modificó código, configuración, base de datos ni servicios.
- **Alcance:**
  - frontend: `frontend/src`;
  - Gateway: `backend/gateway`;
  - servicios Expedientes, Documentos, Derivaciones, Auth, Estudiantes y Psicología;
  - documentación de `docs/`.
- **Criterio:** solo se considera implementado lo que existe en el código y se puede usar.
  Lo que aparece en diseños, comentarios o en el menú sin ruta se registra como
  **no implementado**.

---

## 1. Resumen ejecutivo

**El módulo de Reportes no existe todavía.**

- **Menú.** "Reportes" aparece en el menú como un ítem sin ruta (`path: null`), así que no
  navega a ningún lado ([navItems.js:113-117](../../frontend/src/components/dashboard/navItems.js#L113-L117)).
- **Rutas y backend.** No hay una ruta `/reportes` en [App.jsx](../../frontend/src/App.jsx), ni un
  servicio o endpoint de reportes en el backend.
- **Permiso.** `reportes.view` existe en Auth y lo tienen ADMIN, DIRECTOR y SUBDIRECTOR
  ([005_seed_permissions_and_role_permissions.sql:30-86](../../backend/services/auth/migrations/005_seed_permissions_and_role_permissions.sql#L30-L86)),
  pero ningún componente ni servicio lo usa.

La única analítica real del sistema es el **Dashboard administrativo**
([dashboard.jsx](../../frontend/src/pages/Dashboard/dashboard.jsx)). Ofrece:

- 5 indicadores (total y conteo por estado);
- un gráfico de expedientes registrados por día (7, 30 o 90 días);
- una distribución por tipo de trámite;
- la lista de expedientes recientes.

Calcula todo en el navegador a partir del listado paginado `GET /api/expedientes`. No hay
endpoint de estadísticas.

Fuera de Mesa de Partes hay **exportaciones de módulo**, no reportes institucionales:

- **Estudiantes:** Excel/CSV/TSV con filtros y plantillas.
- **Psicología:** PDF de la Hoja de Atención.
- **FUT:** impresión con `window.print()`.

**Brechas de datos que condicionan los reportes:**

1. **Sin historial de estados.** `expedientes` solo guarda `fecha_registro` y
   `fecha_actualizacion`. No se registra cuándo un expediente pasó a ATENDIDO u OBSERVADO,
   así que los tiempos de atención solo pueden **aproximarse**.
2. **Historial de derivaciones incompleto.** La tabla `derivaciones` solo se llena al
   derivar y al rechazar. Crear, resolver, corregir y cambiar de estado no dejan evento.
3. **Sin plazos.** No hay campo de plazo ni catálogo de plazos por tipo de trámite. Un
   reporte de "fuera de plazo" no es calculable hoy.
4. **Sin filtros de fecha en las API.** Ni el listado de expedientes, ni el de documentos
   ni el de derivaciones aceptan un rango de fechas.

**Recomendación principal.** Implementar primero (Fase 1) una pantalla `/reportes` con un
**endpoint de agregación en Expedientes**. Debe filtrar por rango de fechas y agrupar por
estado, tipo, prioridad, área y mes, y exportar a Excel/CSV e impresión. No requiere cambios
de esquema. Los tiempos de atención reales y los plazos requieren cambios de datos
(Fases 2 y 3).

---

## 2. Estado actual del módulo

| Elemento | Estado | Evidencia |
|---|---|---|
| Ítem "Reportes" en el menú | Visible para quien tiene `reportes.view`; **no navega** (`path: null`) | [navItems.js:116](../../frontend/src/components/dashboard/navItems.js#L116) |
| Tarjeta en "Tus módulos" del Dashboard | No se muestra: solo se listan ítems con ruta | [ModuleCards.jsx:13-23](../../frontend/src/components/dashboard/ModuleCards.jsx#L13-L23) |
| Ruta `/reportes` | **No existe** | [App.jsx](../../frontend/src/App.jsx) (rutas: dashboard, expedientes, FUT, mesa de partes, psicología, estudiantes, configuración) |
| Página o componente de reportes | **No existe** | `frontend/src/pages/` no contiene Reportes |
| Servicio de reportes en el backend | **No existe** | `docs/etapa-12-especificacion-funcional.md:352` lo registra como "🔴 Pendiente" |
| Endpoint de estadísticas | **No existe** | [dashboardData.js:4-8](../../frontend/src/pages/Dashboard/dashboardData.js#L4-L8): "No existe un endpoint de estadísticas" |
| Permiso `reportes.view` | Sembrado; ADMIN, DIRECTOR y SUBDIRECTOR. **SECRETARIA y DOCENTE no lo tienen** | [005_seed…sql:56,72,85-86,99](../../backend/services/auth/migrations/005_seed_permissions_and_role_permissions.sql#L56) |
| Uso de `reportes.view` en el backend | Ninguno | Búsqueda sin coincidencias en `backend/**/*.go` |

**Conclusión:** lo que hoy funciona como "reporte" es el Dashboard, más las exportaciones
propias de Estudiantes, Psicología y FUT.

---

## 3. Inventario de reportes existentes

### 3.1 Dashboard administrativo (Mesa de Partes)

Archivos:

- [pages/Dashboard/dashboard.jsx](../../frontend/src/pages/Dashboard/dashboard.jsx)
- [pages/Dashboard/dashboardData.js](../../frontend/src/pages/Dashboard/dashboardData.js)
- [components/dashboard/charts/](../../frontend/src/components/dashboard/charts/)

Acceso: personal interno con `expedientes.view`. Un SOLICITANTE se redirige a su portal.

| # | Reporte | Objetivo | Información / indicador | Filtros | Periodo | Agrupación | Visualización | Exportación | Fuente | Restricción |
|---|---|---|---|---|---|---|---|---|---|---|
| 1 | Total de expedientes | Volumen vigente | Total de expedientes activos; texto de "urgentes sin atender" (PENDIENTE + EN_PROCESO con prioridad URGENTE) | Ninguno | Todo el histórico | — | Tarjeta (StatCard) que enlaza a `/expedientes` | No | `GET /api/expedientes` (`total` con `page_size=1`) | `expedientes.view`, más el alcance por rol del backend |
| 2 | Expedientes por estado | Carga por etapa | Conteo de PENDIENTE ("Por revisar"), EN_PROCESO, OBSERVADO y ATENDIDO, con % del total | Ninguno | Todo el histórico | Estado | 4 tarjetas que enlazan a la bandeja filtrada | No | Ídem (una consulta por estado) | Ídem |
| 3 | Expedientes registrados por día | Evolución de la recepción | Cantidad por día de `fecha_registro` y total del periodo | Periodo | Últimos 7, 30 o 90 días (botones fijos) | Día | Gráfico de líneas SVG con tooltip y tabla accesible oculta | No | Paginación de `GET /api/expedientes` (100 por página, máximo 10 páginas = 1000 registros) | Ídem; si se supera el tope muestra "Serie parcial" |
| 4 | Por tipo de trámite | Distribución de la demanda | Conteo por cada uno de los 9 tipos, sin los tipos en cero, ordenado de mayor a menor | Ninguno | Todo el histórico | Tipo | Lista de barras horizontales (BarList) | No | `GET /api/expedientes?tipo=…` (una consulta por tipo) | Ídem |
| 5 | Expedientes recientes | Seguimiento rápido | Los 6 últimos registrados | Ninguno | — | — | Tabla (RecentExpedientes) | No | `GET /api/expedientes?page_size=6` | Ídem |

Características comunes:

- **Actualización en vivo.** Los indicadores se refrescan cada 10 s
  ([useActualizacionEnVivo.js:9](../../frontend/src/hooks/useActualizacionEnVivo.js#L9)) y
  los gráficos cada 6 ciclos, unos 60 s.
- **Alcance por rol.** Lo aplica el backend: ADMIN y SECRETARIA ven toda la institución;
  DIRECTOR, SUBDIRECTOR, DOCENTE y AUXILIAR ven solo lo que está **actualmente** en su área
  (`area_actual`) ([listexpedienteslogic.go](../../backend/services/expedientes/internal/logic/listexpedienteslogic.go)).
- **Solo expedientes activos.** Las bajas lógicas (`activo = FALSE`) no se cuentan
  ([expediente_repository.go:123](../../backend/services/expedientes/internal/repository/expediente_repository.go#L123)).
- **Sin variaciones ni tiempos.** No muestra comparaciones entre periodos ni tiempos de
  atención; el propio código lo justifica: "el backend no entrega historia de estados"
  ([dashboard.jsx](../../frontend/src/pages/Dashboard/dashboard.jsx), comentario inicial).

### 3.2 Listados operativos con filtros (no son reportes, pero se usan como tales)

| Pantalla | Filtros | Exportación | Fuente |
|---|---|---|---|
| Expedientes y bandejas ([ExpedientesList.jsx:171-194](../../frontend/src/pages/Expedientes/ExpedientesList.jsx#L171-L194)) | Estado, prioridad, tipo | No | `GET /api/expedientes` |
| Derivados ([ExpedientesDerivados.jsx](../../frontend/src/pages/Expedientes/ExpedientesDerivados.jsx)) | — (historial de derivaciones tipo DERIVACION) | No | `GET /api/derivaciones`, más un `GET /api/expedientes/:id` **por cada fila** para validar visibilidad |
| Seguimiento del solicitante ([Seguimiento.jsx](../../frontend/src/pages/MesaDePartes/Seguimiento.jsx)) | Expedientes propios | No | `GET /api/expedientes` |

### 3.3 Exportaciones de otros módulos

| Módulo | Qué exporta | Formatos | Filtros, encabezados | Evidencia |
|---|---|---|---|---|
| Estudiantes | Directorio con campos y plantillas configurables | **XLSX, CSV, TSV**; separador, codificación y protección contra fórmulas configurables | q, estado, nivel, grado, sección, turno, orden | [ExportarDatosModal.jsx](../../frontend/src/components/estudiantes/ExportarDatosModal.jsx), `POST /api/estudiantes/datos/exportar`, `GET /api/estudiantes/exportar` |
| Estudiantes (importación) | Informe de errores y avisos | CSV (UTF-8 con BOM) | — | [informeDatos.js](../../frontend/src/utils/informeDatos.js) |
| Psicología | Hoja de Atención | PDF (servidor) | Por atención | [PsicologiaAtencion.jsx](../../frontend/src/pages/Psicologia/PsicologiaAtencion.jsx) |
| FUT | Formulario | Impresión (`window.print()`) | — | [FutDigital.jsx:621](../../frontend/src/pages/FUT/FutDigital.jsx#L621) |
| Mesa de Partes | — | **Ninguno** | — | — |

---

## 4. Arquitectura y fuentes de datos

### 4.1 Flujo actual del Dashboard

```text
Navegador (dashboardData.js)
  ├─ ~7 consultas  GET /api/expedientes?estado|prioridad=…&page_size=1   (conteos)
  ├─  9 consultas  GET /api/expedientes?tipo=…&page_size=1               (por tipo)
  └─ ≤10 consultas GET /api/expedientes?page=n&page_size=100             (serie diaria)
        │
        ▼
   Gateway ──gRPC──► Expedientes Service ──► expedientes_db.expedientes
```

En total son unas 26 solicitudes por usuario cada minuto, más 7 cada 10 s por los
indicadores. La agregación ocurre en el navegador.

### 4.2 Datos disponibles por servicio

| Servicio / tabla | Campos útiles para reportes | Lo que falta | Endpoint de lectura y filtros |
|---|---|---|---|
| Expedientes / `expedientes` ([001](../../backend/services/expedientes/migrations/001_create_expedientes.sql), [002](../../backend/services/expedientes/migrations/002_add_area_actual.sql), [003](../../backend/services/expedientes/migrations/003_add_tipos_tramite.sql)) | `codigo`, `tipo` (9 valores), `estado` (4), `prioridad` (2), `area_actual` (6), `solicitante_id`, `fecha_registro`, `fecha_actualizacion`, `activo` | Fechas por transición (atendido, observado, resuelto); plazo; responsable individual; quién registró (si lo hace Secretaría por un tercero, `solicitante_id` es el usuario que registra) | `GET /api/expedientes`: estado, prioridad, tipo, página (máximo 100). **Sin rango de fechas, área ni búsqueda** |
| Derivaciones / `derivaciones` ([001](../../backend/services/derivaciones/migrations/001_create_derivaciones.sql)) | `expediente_id`, `tipo` (DERIVACION, NOTIFICACION, ASIGNACION, RECEPCION, APROBACION), `origen`, `destino`, `registrado_por`, `fecha_registro` | Solo se registran **DERIVACION** al derivar ([derivarexpedientelogic.go:78-80](../../backend/gateway/internal/logic/derivarexpedientelogic.go#L78-L80)) y al rechazar ([rechazarexpedientelogic.go:61-63](../../backend/gateway/internal/logic/rechazarexpedientelogic.go#L61-L63)). No hay evento de recepción ni de atención | `GET /api/derivaciones`: expediente_id, tipo, página. Alcance general solo con `derivaciones.view`. **Sin rango de fechas ni filtro por área** |
| Documentos / `documentos` ([001](../../backend/services/documentos/migrations/001_create_documentos.sql)) | `expediente_id`, `tipo_documento` (ADJUNTO, PROVEIDO, ACTA, INFORME, FORMATO, OTRO), `extension`, `tamano_bytes`, `subido_por`, `estado`, `fecha_registro` | — | `GET /api/documentos`: expediente_id, tipo_documento, página. **Sin rango de fechas** |
| Usuarios / Auth | Usuarios, roles y permisos | Las áreas son **roles**, no unidades organizacionales | `POST /api/usuarios/basic-batch` (nombres) |
| Estudiantes | Maestro de estudiantes y matrícula | Ningún expediente de Mesa de Partes está vinculado a `estudiante_id` | Exportaciones propias (3.3) |
| Psicología | Expedientes y atenciones clínicas | Contenido clínico: **no apto** para reportes generales | Fuera de alcance; solo conteos anónimos con autorización expresa |

### 4.3 Relaciones entre módulos

- Expedientes es la entidad central. Documentos y Derivaciones guardan `expediente_id`
  **sin FK** entre bases, porque cada servicio tiene su propia base de datos.
- Un reporte que cruce servicios (por ejemplo, documentos por tipo de expediente) necesita
  una de dos cosas:
  - consultas coordinadas desde el Gateway o un servicio de reportes; o
  - que cada servicio exponga sus propias agregaciones.

  La especificación de la etapa 12 ya prevé la primera opción
  (`docs/etapa-12-especificacion-funcional.md:373`).

---

## 5. Evaluación de filtros e indicadores

| Aspecto | Situación actual | Brecha |
|---|---|---|
| Rango de fechas | Solo 7, 30 o 90 días, y únicamente en el gráfico diario | No hay selector desde/hasta, mes, año ni periodo académico; las API no aceptan fechas |
| Filtro por estado / tipo / prioridad | En los listados; el Dashboard no los combina | Los indicadores no se pueden filtrar (por ejemplo, "por tipo en marzo") |
| Filtro por área | No existe en la API (`area_actual` no es un filtro expuesto) | Necesario para Dirección |
| Agrupación mensual o anual | No existe | El gráfico diario sobre 90 días es poco legible para tendencias |
| Indicadores de eficiencia | Ninguno: ni tiempo de atención, ni % atendido en plazo, ni backlog por antigüedad | Requieren datos (sección 9) |
| Precisión de los conteos | Exactos: usan el `COUNT` del backend | La serie diaria se trunca en 1000 registros; el aviso "Serie parcial" lo indica correctamente |
| Significado del alcance | Explicado en la cabecera ("Expedientes derivados actualmente a…") | Para DIRECTOR el total cambia cuando un expediente sale de su área: no sirve como estadística histórica |
| Bajas lógicas | Excluidas | Correcto para gestión; un reporte de auditoría podría necesitarlas |
| "Atendidos" | Estado ATENDIDO | ATENDIDO y OBSERVADO son finales; OBSERVADO puede volver a PENDIENTE por corrección. Conviene distinguir "cerrados" de "observados en espera" |

---

## 6. Evaluación de exportaciones

| Formato | Mesa de Partes / Dashboard | Otros módulos | ¿Funciona? | Observaciones |
|---|---|---|---|---|
| PDF | **No existe** | Psicología (Hoja de Atención) | Sí (validado en sesiones anteriores) | Hay generación de PDF en el servidor reutilizable como patrón (`psicologia/internal/pdf`) |
| Excel (XLSX) | **No existe** | Estudiantes | Sí | Incluye encabezados, filtros, plantillas y protección contra fórmulas: es el patrón a reutilizar (`backend/pkg/tabular`) |
| CSV | **No existe** | Estudiantes, informe de importación | Sí | UTF-8 con BOM y escape de fórmulas: buen estándar |
| Impresión | **No existe** (no hay estilos `@media print` en el Dashboard) | FUT (`window.print()`) | Sí en FUT | El Dashboard impreso incluiría el menú lateral y los botones |

Requisitos para las exportaciones futuras de reportes (no se cumplen hoy porque no existen):

- título;
- institución;
- fecha y hora de generación;
- usuario que genera;
- filtros aplicados;
- periodo;
- totales y subtotales;
- nota de alcance ("cifras según su área").

La infraestructura `backend/pkg/tabular` (rejilla, protección de fórmulas) ya resuelve
XLSX/CSV.

---

## 7. Evaluación de interfaz

| Criterio | Evaluación | Mejora concreta |
|---|---|---|
| Organización | El ítem "Reportes" del menú no lleva a nada: confunde | Crear `/reportes` o, mientras no exista, mostrarlo como "Próximamente" de forma explícita |
| Claridad de indicadores | Buena: etiquetas en español, % del total, enlaces a las bandejas, nota de alcance | Agregar el periodo al que corresponde cada cifra (hoy todas son históricas sin indicarlo) |
| Gráficos | Línea y barras propias, accesibles (tabla oculta, `aria-label`), respetan `prefers-reduced-motion` | Agregar agrupación mensual y un gráfico por estado (barras apiladas) |
| Filtros | Solo botones de periodo | Panel de filtros común (desde/hasta, tipo, estado, área) con un botón "Aplicar" |
| Selección de fechas | No hay calendario | Atajos (mes actual, mes anterior, año) más un rango libre |
| Facilidad de uso | Alta para la consulta rápida | Separar el Dashboard (operativo, en vivo) de Reportes (analítico, bajo demanda, exportable) |
| Adaptación a pantallas | Hay puntos de quiebre a 1280, 1100 y 640 px ([dashboard.css:191-204](../../frontend/src/pages/Dashboard/dashboard.css#L191-L204)) | Mantenerlos; las tablas de reportes necesitan desplazamiento horizontal propio |
| Descarga e impresión | Inexistentes en Mesa de Partes | Botones "Exportar Excel/CSV" e "Imprimir", con hoja de estilo de impresión |
| Consistencia visual | Usa `dp-panel`, `StatCard` y `db-segmented`, consistentes | Reutilizar los mismos componentes en Reportes |
| Rendimiento | Unas 26 solicitudes por minuto por usuario solo en gráficos | Con un endpoint agregado sería 1 solicitud por vista |
| Página Derivados | Una solicitud `GET /api/expedientes/:id` por fila (N+1); su comentario dice que Derivaciones no filtra por área, lo que **ya no es cierto** ([listderivacioneslogic.go:78-87](../../backend/services/derivaciones/internal/logic/listderivacioneslogic.go#L78-L87)) | Eliminar la validación fila por fila cuando se toque esa pantalla |

---

## 8. Reportes adicionales recomendados

Leyenda de las columnas:

- **¿Datos?:** ✅ existen · ⚠️ parcial o aproximado · ❌ no existen.
- **¿Calculable hoy?:** se puede calcular sin cambios de esquema.

### A. Gestión documentaria

> En SIS-MESA-PARTES el "documento recibido" es el **expediente** (trámite o FUT). Los
> registros de `documentos` son sus adjuntos y proveídos.

| Reporte | Información necesaria | Servicio / tabla | ¿Datos? | ¿Nuevos campos? | ¿Calculable hoy? | Utilidad para Dirección / Secretaría |
|---|---|---|---|---|---|---|
| Expedientes recibidos por periodo | `fecha_registro` | Expedientes / `expedientes` | ✅ | No (falta un filtro de fechas en la API) | ⚠️ Solo paginando en el cliente; requiere un endpoint agregado | Volumen mensual para memoria institucional |
| Por tipo de trámite | `tipo` | Ídem | ✅ | No | ✅ (sin periodo) | Saber qué trámites predominan |
| Por estado | `estado` | Ídem | ✅ | No | ✅ (sin periodo) | Carga pendiente |
| Pendientes vs. atendidos | `estado` | Ídem | ✅ | No | ✅ | Control diario de Secretaría |
| Por área responsable | `area_actual` | Ídem | ✅ (situación **actual**) | Filtro o agrupación en la API | ⚠️ Solo foto actual, no histórica | Distribución de la carga |
| Evolución mensual de recepción | `fecha_registro` agrupado por mes | Ídem | ✅ | No | ⚠️ Requiere agregación en el servidor | Tendencias y estacionalidad (inicio de año, matrícula) |
| Adjuntos por tipo documental | `tipo_documento`, `fecha_registro` | Documentos / `documentos` | ✅ | Filtro de fechas | ✅ por tipo; ⚠️ por periodo | Uso de proveídos, actas e informes |

### B. Derivaciones

| Reporte | Información necesaria | Servicio / tabla | ¿Datos? | ¿Nuevos campos? | ¿Calculable hoy? | Utilidad |
|---|---|---|---|---|---|---|
| Derivaciones por área (destino/origen) | `destino`, `origen`, `fecha_registro` | Derivaciones / `derivaciones` | ✅ | Agregación y filtro de fechas | ⚠️ Paginando; mejor en el servidor | Ver qué áreas reciben más trámites |
| Derivaciones pendientes | Expedientes EN_PROCESO por `area_actual` | Expedientes | ✅ (foto actual) | No | ✅ con filtro de área | Bandeja por área para Dirección |
| Derivaciones atendidas | Evento de atención por área | — | ❌ No se registra quién ni cuándo atiende | Evento de recepción/atención o historial de estados | ❌ | Cumplimiento por área |
| Tiempo promedio de atención por área | Diferencia entre derivaciones consecutivas | Derivaciones | ⚠️ El tiempo entre dos DERIVACION de un mismo expediente es una aproximación; el último tramo (hasta atender) no tiene evento | Historial de estados | ⚠️ Aproximado | Detectar cuellos de botella |
| Expedientes con retraso | Antigüedad desde `fecha_registro` o desde la última derivación | Expedientes + Derivaciones | ✅ antigüedad | Plazo (para "retraso") | ⚠️ "Antigüedad > N días" sí; "retraso" formal no | Alertas para Secretaría |
| Historial de derivaciones | Eventos por expediente | Derivaciones | ✅ (ya se ve por expediente) | No | ✅ | Trazabilidad y auditoría |

### C. Atención y cumplimiento

| Reporte | Información necesaria | ¿Datos? | ¿Nuevos campos? | ¿Calculable hoy? | Utilidad |
|---|---|---|---|---|---|
| Tiempo promedio de atención | Registro → ATENDIDO | ⚠️ `fecha_actualizacion` coincide con la fecha de atención solo si nada se editó después ([expediente_repository.go:241](../../backend/services/expedientes/internal/repository/expediente_repository.go#L241)) | `fecha_atencion` o tabla de historial de estados | ⚠️ Aproximado y marcado como tal | Indicador clave de eficiencia |
| % atendidos / % pendientes | Conteo por estado | ✅ | No | ✅ (por periodo, con endpoint) | Cumplimiento general |
| Fuera de plazo | Plazo por tipo de trámite y fecha de atención | ❌ | Catálogo de plazos (decisión institucional) y fecha de atención | ❌ | Cumplimiento normativo |
| Comparación entre periodos | Agregados por mes y año | ✅ (registro); ⚠️ (atención) | No para registro | ⚠️ Con endpoint agregado | Memoria anual, metas |

### D. Actividad institucional

| Reporte | Información necesaria | ¿Datos? | ¿Nuevos campos? | ¿Calculable hoy? | Utilidad |
|---|---|---|---|---|---|
| Expedientes registrados por usuario | Quién registró | ⚠️ `solicitante_id` mezcla al solicitante externo con el usuario interno que registra | `registrado_por` | ⚠️ Solo para registros internos, con matices | Productividad de Secretaría |
| Adjuntos subidos por usuario | `subido_por` | ✅ | No | ✅ | Actividad por usuario |
| Derivaciones por usuario | `registrado_por` | ✅ | No | ✅ | Actividad de Dirección |
| Actividad por área | `area_actual`, `destino` | ✅ | No | ⚠️ Con agregación | Comparar áreas |
| Volumen por periodo | `fecha_registro` | ✅ | No | ⚠️ Con agregación | Planificación |
| Carga por responsable (persona) | Asignación a una persona | ❌ Solo hay área (= rol) | Responsable individual | ❌ | Requiere un cambio de modelo; prioridad baja |

### E. Análisis y estadísticas

| Reporte | ¿Calculable hoy? | Comentario |
|---|---|---|
| Tendencias mensuales | ⚠️ Con endpoint agregado | La base para el resto |
| Comparación mes contra mes y año contra año | ⚠️ Ídem | Mostrar la variación solo con datos completos de ambos periodos |
| Distribución porcentual (tipo, estado, prioridad, área) | ✅ / ⚠️ | Barras o tabla con % |
| Indicadores de eficiencia | ❌ hasta la Fase 2 | Dependen del historial de estados |
| Gráficos de evolución | ✅ (componentes LineChart y BarList existentes) | Reutilizar |

**Privacidad.** Todos los reportes deben ser **agregados**: conteos, porcentajes y tiempos,
sin nombres de solicitantes ni de estudiantes. Psicología queda excluida de los reportes
generales: su información es contenido clínico.

---

## 9. Brechas técnicas y funcionales

| # | Brecha | Tipo | Impacto | Evidencia |
|---|---|---|---|---|
| B1 | No existe la pantalla `/reportes` (ítem de menú sin ruta) | Funcional | Alto | navItems.js:116, App.jsx |
| B2 | No existe un endpoint de agregación; los cálculos se hacen en el navegador con unas 26 solicitudes por minuto | Técnica | Alto (rendimiento, tope de 1000 registros) | dashboardData.js |
| B3 | Las API de listado no aceptan rango de fechas ni área | Técnica | Alto | gateway.api:135-142, 536-542, 623-629 |
| B4 | Sin historial de estados ni fecha de atención | Datos | Alto para indicadores de eficiencia | Migraciones 001-003 de Expedientes; solo `fecha_actualizacion` |
| B5 | Derivaciones solo registra derivar y rechazar | Datos | Medio | derivarexpedientelogic.go:78, rechazarexpedientelogic.go:61 |
| B6 | Sin plazos por tipo de trámite | Datos / institucional | Medio | No hay campo ni catálogo |
| B7 | Sin exportación ni impresión en Mesa de Partes | Funcional | Alto para Secretaría y Dirección | — |
| B8 | `reportes.view` no lo tiene SECRETARIA, que opera la Mesa de Partes | Permisos (decisión institucional) | Medio | 005_seed…sql:86 |
| B9 | `reportes.view` no se valida en ningún backend | Seguridad (cuando exista el módulo) | Alto al implementar | — |
| B10 | Alcance "actual" por área: DIRECTOR o SUBDIRECTOR no ven lo que ya pasó por su área | Funcional | Medio (los reportes históricos por área necesitan otra regla de alcance) | listexpedienteslogic.go |
| B11 | `solicitante_id` no distingue al registrador interno | Datos | Bajo | createexpedientelogic.go:82 |
| B12 | Página Derivados con N+1 solicitudes y comentario desactualizado | Técnica | Bajo | ExpedientesDerivados.jsx |

---

## 10. Tabla de priorización

| Prioridad | Mejora | Esfuerzo | Datos | Reutiliza |
|---|---|---|---|---|
| **ALTA** | Pantalla `/reportes` protegida por `reportes.view` (frontend y backend) | Bajo | — | DashboardLayout, StatCard, charts |
| **ALTA** | Endpoint de agregación en Expedientes (`ReporteExpedientes`): rango de fechas, agrupación por estado, tipo, prioridad, `area_actual`, mes y día | Medio | ✅ Existentes | Repositorio y reglas de alcance de Expedientes |
| **ALTA** | Filtros comunes: desde/hasta con atajos, tipo, estado, prioridad, área | Bajo | ✅ | Componentes de filtros actuales |
| **ALTA** | Exportación XLSX/CSV con encabezado (título, filtros, fecha, usuario, totales) | Medio | ✅ | `backend/pkg/tabular` (patrón de Estudiantes) |
| **ALTA** | Impresión con hoja `@media print` | Bajo | ✅ | — |
| **ALTA** | Pendientes por antigüedad (0-3, 4-7, 8-15, más de 15 días) | Bajo | ✅ `fecha_registro` | Endpoint agregado |
| **MEDIA** | Derivaciones por área y periodo (agregación en Derivaciones) | Medio | ✅ | Repositorio de Derivaciones |
| **MEDIA** | Adjuntos por tipo documental y periodo; actividad por usuario (`subido_por`, `registrado_por`) | Medio | ✅ | Documentos, Derivaciones |
| **MEDIA** | Historial de estados (tabla de eventos con fecha por transición) y tiempos de atención reales | Medio-alto (migración y cambios en todas las transiciones) | ❌ Nuevo | Patrón de auditoría de Estudiantes y Psicología |
| **MEDIA** | Tendencia mensual y comparación con el periodo anterior | Bajo (con el endpoint agregado) | ✅ | LineChart |
| **MEDIA** | Decidir si SECRETARIA recibe `reportes.view` | Bajo (migración de seed) | — | — |
| **BAJA** | Plazos por tipo de trámite y reporte "fuera de plazo" | Medio, con decisión institucional | ❌ Nuevo | — |
| **BAJA** | Reporte PDF institucional | Medio | ✅ | `psicologia/internal/pdf` como patrón |
| **BAJA** | Responsable individual y carga por persona | Alto (cambio de modelo) | ❌ Nuevo | — |
| **BAJA** | Campo `registrado_por` distinto de `solicitante_id` | Medio | ❌ Nuevo | — |
| **BAJA** | Corregir el N+1 de Derivados | Bajo | — | — |

---

## 11. Propuesta de implementación por fases

### Fase 1 – Reportes con los datos actuales (sin cambios de esquema)

1. **Backend, Expedientes.** RPC `ReporteExpedientes` con:
   - entrada: `desde`, `hasta`, `tipo`, `estado`, `prioridad`, `area`, `agrupar_por`
     (`estado` | `tipo` | `prioridad` | `area` | `mes` | `dia`);
   - salida: grupos con conteo, más los totales;
   - solo SQL `COUNT … GROUP BY` sobre `activo = TRUE`, con las **mismas reglas de alcance**
     del listado;
   - validación de `reportes.view` en el backend.
2. **Gateway.** `POST /api/reportes/expedientes` (y una variante de exportación), con
   errores sanitizados y fuera de los logs de contenido si incluye filtros sensibles.
3. **Frontend.** Página `/reportes` con:
   - filtros comunes;
   - indicadores del periodo;
   - tendencia mensual;
   - distribución por tipo, estado y área;
   - pendientes por antigüedad;
   - botones Exportar (XLSX/CSV) e Imprimir.
4. **Pruebas.** Unitarias (agregación, alcance por rol, validación de fechas) y de integración
   en una base aislada.

### Fase 2 – Trazabilidad para indicadores de eficiencia

1. Tabla `expediente_eventos`: estado anterior y nuevo, área anterior y nueva, usuario y
   fecha. Debe escribirse en **todas** las transiciones (crear, derivar, rechazar, corregir,
   resolver, cambiar estado).
2. Reportes de tiempos de atención (promedio y mediana) por tipo y por área, solo con los
   datos posteriores a la migración, indicando la fecha desde la que hay datos.
3. Agregaciones en Derivaciones (por área y periodo) y en Documentos (por tipo y periodo).

### Fase 3 – Cumplimiento y análisis avanzado

1. Catálogo de plazos por tipo de trámite (definición institucional) y reporte "fuera de
   plazo".
2. Comparativos interanuales y PDF institucional con membrete.
3. Responsable individual, si la institución lo requiere.

---

## 12. Conclusiones y recomendaciones

1. **Hoy no hay un módulo de Reportes.** Hay un Dashboard operativo bien construido (cifras
   exactas, accesible, honesto sobre sus límites) y exportaciones solo en Estudiantes,
   Psicología y FUT.
2. **Los datos para un primer módulo útil ya existen:** volumen, distribución y estado actual
   por periodo, tipo y área. Lo que falta es **agregarlos en el servidor**, filtrar por fechas
   y exportar.
3. **Los indicadores de eficiencia** (tiempo de atención, cumplimiento, fuera de plazo)
   **no deben mostrarse** hasta contar con historial de estados (Fase 2) y plazos (Fase 3).
   Mostrarlos antes sería presentar cifras inventadas o engañosas.
4. **Decisiones institucionales pendientes:**
   - ¿SECRETARIA debe ver reportes?
   - ¿Qué alcance histórico tienen DIRECTOR y SUBDIRECTOR?
   - ¿Qué plazos aplican por tipo de trámite?
   - ¿Qué periodicidad tienen los reportes? (pregunta F1-02 de la etapa 12)
5. **Implementar primero:** la Fase 1 completa (endpoint agregado, `/reportes` con filtros,
   exportación XLSX/CSV e impresión). Reutiliza `pkg/tabular`, los componentes del Dashboard
   y las reglas de alcance de Expedientes, y no requiere migraciones.

> No se implementó ninguna recomendación. Queda a la espera de autorización.
