# Módulo de Reportes – Fase 1

**Estado:** implementado en la rama `feat/psicologia-client-credentials`.
**Pendiente de despliegue:** sin commit, sin reiniciar servicios y sin aplicar la migración 021.

Base: [Auditoría funcional del módulo de Reportes](../auditorias/Auditoria_Funcional_Modulo_Reportes.md).
La Fase 1 usa solo los datos actuales y **no cambia el esquema** de Expedientes.

## 1. Arquitectura

```text
Navegador /reportes ──GET /api/reportes/expedientes──────────► Gateway ──gRPC ReporteExpedientes──► Expedientes Service
                    ──GET /api/reportes/expedientes/exportar─► Gateway (genera XLSX/CSV)                │
                                                                                                         ▼
                                                              1 consulta SQL GROUP BY sobre expedientes_db.expedientes
```

**Expedientes Service.** Es el dueño de los datos y calcula las estadísticas.

- **Consulta:** una sola consulta agregada (`GROUP BY` estado, tipo, prioridad, área, mes y
  días de antigüedad) en `internal/repository/reporte_repository.go`.
- **Agrupaciones:** se arman en Go en `internal/logic/reporteexpedienteslogic.go`.
- **Datos que no consulta:** las bases de otros microservicios y los registros individuales.

**Gateway.** Transporta la consulta y da formato a los archivos (`internal/reportes`). No
calcula cifras. Para la exportación vuelve a pedir el reporte con los mismos filtros, así
que el archivo siempre coincide con la pantalla.

**Frontend.** Pantalla `src/pages/Reportes/` (se carga bajo demanda en su propio chunk),
servicio `src/services/reportesService.js`.

**Dashboard.** Con `reportes.view`, los indicadores y el gráfico "por tipo" usan este
endpoint:

- Antes: 7 + 9 solicitudes por ciclo.
- Ahora: 3 + 1.

Si el endpoint falla, el Dashboard vuelve al cálculo anterior con el listado. Sin
`reportes.view`, el comportamiento no cambia. La serie diaria sigue usando el listado.

## 2. Endpoints

### `GET /api/reportes/expedientes`

| Parámetro | Formato | Descripción |
|---|---|---|
| `desde` | `AAAA-MM-DD` | Fecha de registro inicial, inclusiva (hora de Lima). Entre 2000-01-01 y hoy. |
| `hasta` | `AAAA-MM-DD` | Fecha de registro final, inclusiva. No anterior a 2000-01-01 ni a `desde`. |
| `estado` | `PENDIENTE` · `EN_PROCESO` · `OBSERVADO` · `ATENDIDO` | Opcional |
| `tipo` | Los 9 tipos de `estados.go` | Opcional |
| `prioridad` | `NORMAL` · `URGENTE` | Opcional |
| `area` | `SECRETARIA` · `DIRECTOR` · `SUBDIRECTOR` · `DOCENTE` · `AUXILIAR` · `ADMIN` | Área **actual** del expediente. Opcional |

- Todos los filtros son opcionales y se combinan (AND). Afectan a todas las agrupaciones.
- El periodo puede durar como máximo 10 años.
- Un valor inválido responde `400 validation_error: …`, sin repetir el valor recibido.

Respuesta (200):

```json
{
  "total": 128,
  "por_estado":    [{"clave": "PENDIENTE", "cantidad": 21}, "…"],
  "por_tipo":      ["…catálogo completo, con ceros…"],
  "por_prioridad": ["…"],
  "por_area":      ["…"],
  "por_mes":       [{"clave": "2026-01", "cantidad": 4}, "…"],
  "pendientes": 35,
  "pendientes_por_antiguedad": [{"clave": "0-3", "cantidad": 12}, {"clave": "4-7", "cantidad": 8},
                                {"clave": "8-15", "cantidad": 7}, {"clave": "16-30", "cantidad": 5},
                                {"clave": "31+", "cantidad": 3}],
  "alcance": "INSTITUCION",
  "area_alcance": "",
  "generado": "2026-10-30T09:15:00-05:00"
}
```

- Las agrupaciones por estado, tipo, prioridad y área devuelven el **catálogo completo en
  orden estable, con ceros**. Cada una suma `total`.
- `por_mes` es **continua**, con ceros:
  - empieza en el mes de `desde` o, si no se indica, en el primer mes con datos;
  - termina en el mes de `hasta` o, si no se indica, en el mes actual;
  - si no hay datos ni `desde`, la lista llega vacía.
- `alcance` indica qué expedientes entraron en el cálculo:
  - `INSTITUCION`: todos;
  - `AREA`: solo los que están actualmente en `area_alcance` (ver sección 3).

### `GET /api/reportes/expedientes/exportar`

Usa los mismos parámetros, más `formato=xlsx|csv`. Responde con el archivo codificado:

```json
{"nombre": "reporte-expedientes-20261030-0915.xlsx", "mime": "…", "contenido_base64": "…"}
```

Un formato no admitido responde `400`, y en ese caso el reporte ni siquiera se consulta.

### Errores

| HTTP | Caso |
|---|---|
| 400 | Filtro o fecha inválidos; formato de exportación no admitido |
| 401 | Sin sesión o token inválido |
| 403 | Sin `reportes.view`, o rol externo |
| 429 | Límite de tasa: 60 consultas por minuto y 10 exportaciones por minuto, por credencial |
| 5xx | Mensaje genérico, sin detalles internos |

## 3. Permisos y alcance

| Rol | `reportes.view` | Alcance de las cifras |
|---|---|---|
| ADMIN | Sí (migración 005) | Toda la institución |
| SECRETARIA | **Sí (migración 021, pendiente de aplicar)** | Toda la institución |
| DIRECTOR | Sí (005) | Solo lo que está **actualmente** en Dirección |
| SUBDIRECTOR | Sí (005) | Solo lo que está actualmente en Subdirección |
| DOCENTE, AUXILIAR, PSICOLOGO, SOLICITANTE | No | — |

- **Doble control:** el frontend usa `PermissionRoute` y el ítem de menú; el backend usa
  `RequirePermission("reportes.view")` en Expedientes. Además, un rol externo se rechaza
  aunque tuviera el permiso.
- **Alcance:** es idéntico al de `ListExpedientes`. El filtro de área se combina con ese
  alcance, así que un DIRECTOR que filtra "Secretaría" obtiene 0. Las cifras de DIRECTOR y
  SUBDIRECTOR **no son históricas por área**: es una limitación conocida (auditoría, B10).
- **Datos que se exponen:** solo conteos. Ni expedientes individuales, ni solicitantes, ni
  DNI, ni asuntos.

## 4. Indicadores

| Indicador | Definición |
|---|---|
| Total de expedientes | Expedientes activos (sin bajas lógicas) que cumplen los filtros |
| Pendientes | `PENDIENTE` + `EN_PROCESO`: abiertos dentro de la institución |
| Atendidos | `ATENDIDO` |
| Observados | `OBSERVADO`: esperan la corrección del solicitante. Se muestran aparte y no cuentan como pendientes |
| Por estado, tipo, prioridad, área | Estado y área **actuales** del expediente |
| Tendencia mensual | Expedientes registrados por mes (fecha de registro) |
| Pendientes por antigüedad | Días entre la fecha de registro y hoy (hora de Lima), en rangos 0-3, 4-7, 8-15, 16-30 y más de 30 días |

- **Fecha usada:** siempre `fecha_registro` (la de recepción). Nunca se mezcla con fechas de
  atención: el sistema no las guarda.
- **Antigüedad:** los rangos son **descriptivos**; no hay plazos institucionales definidos y
  no indican incumplimiento.
- **Lo que no se muestra:** tiempos de atención ni cumplimiento de plazos. Corresponden a las
  Fases 2 y 3.

## 5. Exportaciones

| Formato | Contenido |
|---|---|
| Excel (`.xlsx`) | Hoja "Reporte" con título, institución, fecha de generación, periodo, alcance y cada filtro. Luego una sección por agrupación: encabezado con estilo, cantidades **numéricas**, porcentajes con formato % y una fila de total |
| CSV | Mismo contenido; UTF-8 con BOM, CRLF y protección contra fórmulas (las mismas convenciones que Estudiantes y `pkg/tabular`) |
| Impresión | `window.print()` con hoja `@media print`: sin menú, encabezado de la app, filtros ni botones; con encabezado propio (periodo, filtros, alcance y fecha de generación) |

Los archivos usan **los filtros aplicados**, no los que se están editando sin aplicar.

Sobre la generación:

- Se usa la misma librería XLSX que `pkg/tabular` (excelize v2.11.0, ahora dependencia
  directa del Gateway).
- `tabular.Exportador` no se usa porque produce una sola rejilla de texto.

## 6. Pruebas

**Backend, Expedientes:**

- validación de filtros y fechas;
- permisos (incluido Auth caído);
- filtros inválidos que no llegan a SQL;
- consulta con filtros individuales y combinados;
- agrupación mensual continua;
- rangos de antigüedad y resultado vacío;
- catálogos.

`TestReporte_SQLReal` ejecuta el SQL en un PostgreSQL **aislado y desechable**
(`REPORTES_DB_AISLADA_URL`) y verifica el límite de día y mes en hora de Lima, el `hasta`
inclusivo, la exclusión de bajas lógicas, el alcance y los filtros.

**Backend, Gateway:**

- CSV: BOM, CRLF, metadatos, filtros, totales y porcentajes;
- XLSX: se abre y contiene números reales;
- protección contra fórmulas;
- transmisión de filtros y token, y propagación de errores;
- exportación con los mismos filtros;
- límites de tasa.

**Frontend:**

- filtros (combinados, validación, limpiar, atajos);
- indicadores y gráficos;
- estados de carga, error (incluido 429) y sin resultados;
- exportación Excel y CSV con los filtros aplicados, y error de exportación;
- impresión y estilos de impresión y responsive;
- control de acceso y menú;
- Dashboard con el endpoint agregado y su respaldo con el listado.

## 7. Despliegue

1. Aplicar `backend/services/auth/migrations/021_grant_secretaria_reportes_view.sql`. Es
   idempotente. Requiere autorización porque modifica `auth_db`. También puede asignarse
   desde *Configuración → Roles y permisos*.
2. Reiniciar **Expedientes**, luego el **Gateway**, luego publicar el frontend. Si el
   frontend se publica antes, el Dashboard usa su respaldo; `/reportes` mostraría un error
   hasta que el backend esté desplegado.
3. Comprobar con un usuario de cada rol: acceso, alcance, exportaciones e impresión.

## 8. Limitaciones (fuera de la Fase 1)

- Sin historial de estados: no hay tiempos de atención reales (Fase 2).
- Sin plazos institucionales: no hay reporte "fuera de plazo" (Fase 3).
- El alcance de DIRECTOR y SUBDIRECTOR es su área **actual**, no el histórico de su área.
- Reportes de Derivaciones y Documentos por periodo: Fase 2.
- Los límites de tasa son en memoria y por instancia del Gateway.

---

# Fase 2 – Ampliación del módulo (implementada, pendiente de despliegue)

Plan de referencia: [Plan de mejora](../auditorias/Plan_Mejora_Modulo_Reportes.md).

Decisiones aprobadas:

1. La actividad se muestra **solo por rol**, sin nombres.
2. Las derivaciones históricas irreconocibles se muestran como **«Sin clasificar»**.
3. Las comparaciones requieren un **mínimo de 10 registros por periodo**.
4. **Sin PDF institucional**: se mantienen Excel, CSV e impresión.

## F2.1 Arquitectura

```text
/reportes (pestañas: Resumen · Expedientes y áreas · Derivaciones · Documentos · Actividad por rol · Comparación)
   │
   ├─ GET /api/reportes/expedientes(+/exportar)   ─► Expedientes.ReporteExpedientes (ampliado con campos OPCIONALES)
   ├─ GET /api/reportes/derivaciones(+/exportar)  ─► Derivaciones.ReporteDerivaciones ─► Expedientes.ReporteAlcanceExpedientes
   ├─ GET /api/reportes/documentos(+/exportar)    ─► Documentos.ReporteDocumentos     ─► Expedientes.ReporteAlcanceExpedientes
   ├─ GET /api/reportes/actividad(+/exportar)     ─► los 3 anteriores (con registros) + Usuarios.GetUsuariosBasic (solo el rol)
   └─ GET /api/reportes/comparacion(+/exportar)   ─► Expedientes.ReporteExpedientes × 2 (mismos filtros)
```

- **Reglas comunes** (periodo, zona de Lima, granularidad, serie continua y normalización
  de áreas) en el paquete público `backend/services/expedientes/reportes`. Derivaciones,
  Documentos y el Gateway ya dependían del módulo `expedientes`, así que no se modificó
  ningún `go.mod`.
- **Alcance y bajas lógicas.** Derivaciones y Documentos no leen la base de Expedientes.
  Piden a `ReporteAlcanceExpedientes` los IDs de los expedientes **activos y visibles** para
  el usuario, con el mismo permiso y el mismo alcance que la Fase 1, y filtran su propia
  consulta con `expediente_id = ANY(...)`.
- **Cada servicio hace una sola consulta agregada** (`GROUP BY`) en su propia base.
- **Exportación** (`gateway/internal/reportes`): motor genérico de documento (metadatos y
  tablas) con el mismo formato que la Fase 1. El Resumen genera exactamente el mismo
  contenido que antes; lo verifican las pruebas de la Fase 1.

## F2.2 Endpoints nuevos o ampliados

| Endpoint | Parámetros | Notas |
|---|---|---|
| `GET /api/reportes/expedientes` | + `granularidad` (DIA, SEMANA o MES, opcional) | La respuesta añade `serie`, `granularidad`, `area_estado` y `tipo_estado` (celdas `{fila, columna, cantidad}`). Sin `granularidad`, los campos de la Fase 1 son idénticos |
| `GET /api/reportes/expedientes/exportar` | + `vista` (RESUMEN por defecto, o EXPEDIENTES), `granularidad` | RESUMEN es el archivo de la Fase 1 |
| `GET /api/reportes/derivaciones` (+ `/exportar`) | `desde`, `hasta`, `tipo`, `granularidad` (+ `formato`) | Fecha usada: la de la derivación |
| `GET /api/reportes/documentos` (+ `/exportar`) | Ídem | Fecha usada: el registro del documento |
| `GET /api/reportes/actividad` (+ `/exportar`) | `desde`, `hasta`, `tipo` (+ `formato`) | Respuesta `roles[]` con conteos por rol |
| `GET /api/reportes/comparacion` (+ `/exportar`) | `desde` y `hasta` (**obligatorios**), `modo` (ANTERIOR, ANIO_ANTERIOR o PERSONALIZADO), `desde_b`, `hasta_b`, `estado`, `tipo`, `prioridad`, `area` (+ `formato`) | `comparable`, `minimo` (10), `mensaje`; cada fila trae `tiene_variacion` y `variacion` |

RPC nuevos:

- `Expedientes.ReporteAlcanceExpedientes`;
- `Derivaciones.ReporteDerivaciones`;
- `Documentos.ReporteDocumentos`.

Todos exigen `reportes.view` y aplican el alcance por rol, sin cambios de permisos.

Límites de solicitudes por credencial:

- 60 por minuto en derivaciones y documentos;
- 30 por minuto en actividad y comparación;
- 10 por minuto en cada exportación.

## F2.3 Indicadores y reglas

| Pestaña | Indicadores | Reglas de cálculo |
|---|---|---|
| Expedientes y áreas | Registros por día, semana (lunes a domingo) o mes; área actual × estado; tipo × estado | Fecha de registro. Estado y área **actuales**. Serie continua con ceros; máximo 366 días por día, 260 semanas o 240 meses |
| Derivaciones | Total entre áreas; por destino; por origen; flujo origen → destino; por tipo de trámite; por periodo; otros movimientos (solo cantidad); sin clasificar | Solo `tipo = DERIVACION` se analiza por áreas. Origen y destino se normalizan (código, etiqueta con o sin tilde, «usuario (Rol)» → rol). Lo irreconocible es **«Sin clasificar»**. **Nunca** se devuelve el texto original. Solo expedientes activos y visibles. Las devoluciones a Secretaría **no** se muestran como rechazos (no se registran diferenciados) |
| Documentos | Total; por tipo documental; por tipo de trámite; por periodo | Solo documentos `ACTIVO` de expedientes activos y visibles |
| Actividad por rol | Expedientes registrados, derivaciones registradas y documentos subidos, por rol | Agrupa por el rol **actual** de cada usuario. Los IDs solo existen dentro del Gateway y se descartan (nombres y DNI incluidos). Orden fijo de roles: no es un ranking |
| Comparación | Total, pendientes, por estado, tipo, prioridad y área; periodo actual frente al de referencia | Variación `(actual − referencia) / referencia × 100` **solo si ambos periodos tienen ≥ 10 expedientes**. Si no, «Datos insuficientes para comparar» sin ninguna variación. Con referencia 0, "Sin base". No se estiman datos |

**No se presentan** tiempos de atención o de resolución, cumplimiento de plazos ni rechazos:
requieren historial de estados y registros nuevos (Fase 3).

## F2.4 Exportaciones

Cada pestaña exporta con **su** endpoint y **sus** filtros aplicados, en XLSX o CSV. Cada
archivo incluye:

- título;
- institución;
- fecha de generación;
- periodo, indicando qué fecha se usó;
- alcance;
- filtros;
- agrupación;
- tablas con totales.

La impresión usa la misma hoja `@media print`, sin pestañas, filtros ni botones, con el
encabezado de la pestaña activa. No hay PDF institucional (decisión aprobada).

## F2.5 Pruebas

- **Paquete común:** periodo, granularidad, claves de semana (lunes), series continuas y
  topes, normalización (incluidos textos dañados y «usuario (Rol)», que nunca se
  devuelven).
- **Expedientes:** tablas cruzadas que suman el total; serie por día, semana y mes;
  registros opcionales; sin opciones = Fase 1; permisos y validación del RPC de alcance.
- **Derivaciones y Documentos:**
  - normalización y conteo;
  - exclusión de otros tipos y de nombres;
  - permisos y reenvío del token;
  - alcance vacío;
  - consulta parametrizada.
- **SQL real en bases aisladas** (Expedientes, Derivaciones y Documentos):
  - hora de Lima;
  - `hasta` inclusivo;
  - exclusión de bajas y de expedientes fuera del alcance;
  - documentos eliminados excluidos.
- **Gateway:**
  - periodo de referencia;
  - umbral de 10;
  - "Sin base";
  - actividad sin identidades;
  - exportaciones por pestaña con los filtros aplicados;
  - formato PDF rechazado;
  - el Resumen sigue siendo el archivo de la Fase 1.
- **Frontend:**
  - las 14 pruebas de la Fase 1 sin cambios;
  - pestañas y URL;
  - filtros desactivados según la pestaña;
  - granularidad;
  - tablas cruzadas con totales;
  - derivaciones normalizadas y aviso de rechazos;
  - documentos;
  - actividad sin personas;
  - comparación (falta de periodo, variaciones, datos insuficientes);
  - exportación por pestaña;
  - el Dashboard sin cambios (sus pruebas pasan).

## F2.6 Despliegue

No hay migraciones ni cambios de permisos. Reiniciar **en este orden**:

1. **Expedientes**: `ReporteAlcanceExpedientes` y los campos nuevos. Es compatible con el
   Gateway actual.
2. **Derivaciones** y **Documentos**: sus RPC nuevos dependen del punto 1.
3. **Gateway**: rutas nuevas.
4. Frontend (Vite sirve el código actual).

Luego, verificar con los 5 perfiles: alcance de DIRECTOR y SUBDIRECTOR, exclusión de
bajas en derivaciones, ausencia de nombres y exportaciones por pestaña.

## F2.7 Limitaciones

- Estado y área son los **actuales** (no al cierre del periodo).
- El alcance de DIRECTOR y SUBDIRECTOR es su área **actual**: sus derivaciones, documentos y
  actividad se limitan a los expedientes que hoy están en su área.
- El rol de la actividad es el **actual** del usuario.
- Con el volumen de datos actual, la mayoría de las comparaciones mostrarán «Datos
  insuficientes para comparar».


---

# Mejora incremental de la Fase 2 (implementada, pendiente de despliegue)

Sin datos nuevos, sin migraciones y sin cambios de permisos. Todo respeta el alcance por
institución o por área actual de la Fase 2.

## M.1 Indicadores corregidos

| Indicador | Antes | Ahora |
|---|---|---|
| Actividad: columna de movimientos | «Derivaciones registradas», pero contaba todos los movimientos | **«Movimientos registrados»** (derivaciones, notificaciones, asignaciones, recepciones y aprobaciones) más **«De ellos, derivaciones»** (solo tipo DERIVACION, ya incluido en movimientos) |
| Actividad: total | Expedientes + movimientos + documentos | Igual, ahora rotulado **«Total de acciones»**, con su **participación** por rol |
| Derivaciones: tarjetas | Derivaciones, sin clasificar y otros movimientos | **Movimientos registrados**, **Derivaciones entre áreas**, **Aprobaciones** y **Sin clasificar**, con su porcentaje y su base explícita |
| Expedientes: resumen exportado | No existía | Registrados, pendientes (Pendiente + En proceso), observados y atendidos, con aclaración de **estado actual** |

Las pestañas que muestran estado o área indican que son los **actuales**, no un historial.

## M.2 Análisis de expedientes

- **Evolución por estado:** barras apiladas por día, semana o mes, por fecha de registro y
  coloreadas por estado actual. Reemplaza a la línea simple: no se repite la misma serie.
  - Si Expedientes aún no entrega `serie_estado`, se muestra la línea anterior.
  - Colores validados para daltonismo, con leyenda, tooltip y tabla accesible.
- **Áreas con mayor carga pendiente:** Pendiente + En proceso por área actual, de mayor a menor,
  con % de la carga. No mide plazos.
- **Participación:** columna «% del total» en las tablas área × estado y tipo × estado.
  Tipo × estado va de más a menos frecuente.
- **Tipos más frecuentes:** tipos documentales y tipos de trámite (en Derivaciones y Documentos)
  ordenados por cantidad.

API: `GET /api/reportes/expedientes` añade `serie_estado` (celdas `{fila: periodo, columna:
estado, cantidad}`, solo con `granularidad`). Actividad añade `movimientos_registrados`;
`derivaciones_registradas` pasa a contar solo DERIVACION.

RPC (campos nuevos, compatibles):

- `ReporteExpedientesResponse.serie_estado = 17`;
- `ReporteDerivacionesResponse.derivaciones_por_usuario = 14`.

## M.3 Comparación

Se mantienen los tres modos y el mínimo de 10 expedientes por periodo. Con periodos comparables,
cada fila muestra:

- **diferencia absoluta**, con ▲ / ▼ / = y su texto para lectores de pantalla;
- **variación porcentual**: solo si la referencia es mayor que 0; si no, «Sin base».

Se añade una tarjeta «Cambio en el total». Con datos insuficientes no se muestran diferencias,
variaciones ni tendencias. La exportación añade las columnas Diferencia y Tendencia.

## M.4 Presentación

- Estado vacío uniforme (`role="status"`), distinto del error (`role="alert"`) y de la carga.
- Encabezados de tabla en varias líneas, para que ninguna columna quede oculta en escritorio o
  tablet.
- «Comparar con» muestra el nombre completo del modo.
- Sin scroll horizontal de página en escritorio, tablet ni móvil. En móvil, las tablas anchas se
  desplazan dentro de su contenedor.

## M.5 Exportaciones

- XLSX con porcentajes a **un decimal**, igual que el CSV y la pantalla. El contenido del Resumen
  de la Fase 1 no cambia.
- Expedientes:
  - resumen del periodo;
  - evolución por estado (una sola tabla por periodo);
  - carga pendiente;
  - participación en las tablas cruzadas.
- Derivaciones:
  - movimientos, derivaciones, sin clasificar, otros movimientos y aprobaciones, con base explícita;
  - tipos de trámite de más a menos frecuente.
- Actividad: columnas iguales a las de la pantalla.
- Comparación: diferencia y tendencia, solo si es comparable.

## M.6 Despliegue

No hay migraciones ni cambios de permisos. Reiniciar en este orden:

1. Expedientes.
2. Derivaciones.
3. Gateway.

Documentos no cambia. El frontend tolera un backend anterior: sin `serie_estado` muestra la línea
simple y sin `movimientos_registrados` adapta la tabla de actividad.

## M.7 Limitaciones

- Estado y área siguen siendo los actuales. La evolución por estado agrupa por fecha de registro;
  no es un historial de estados.
- No hay tiempos de atención, cumplimiento de plazos ni rechazos diferenciados (Fase 3).
- Las aprobaciones por rol no se desglosan en Actividad: se cuentan dentro de los movimientos.

---

# Exportaciones: CSV para Excel en español y PDF (implementado, pendiente de despliegue)

Reemplaza la decisión anterior «sin PDF institucional»: ahora el PDF se pidió expresamente.

- **CSV:** separador `;`, el separador de listas de Excel en configuración regional española.
  - Con `,` Excel dejaba todo en una sola columna. Además, los porcentajes `12,3 %` obligaban a
    entrecomillar.
  - Se mantienen BOM UTF-8, CRLF, escape de comillas, `;` y saltos de línea (`encoding/csv`) y
    la protección contra fórmulas.
  - El contenido no cambia: solo el separador.
- **PDF:** las 6 pestañas, por `GET /api/reportes/{expedientes|derivaciones|documentos|actividad|comparacion}/exportar?formato=pdf`
  (Resumen y Expedientes y áreas comparten la ruta de expedientes con `vista`).
  - Mismo `Documento` que el CSV y el XLSX: título, institución, generación en hora de Lima,
    periodo, alcance, filtros, tablas con notas y totales.
  - Horizontal si alguna tabla tiene más de 5 columnas.
  - Las tablas largas continúan con su título («continuación») y su encabezado repetido.
  - Pie institucional con «Página N de M».
  - Biblioteca `github.com/go-pdf/fpdf` v0.9.0, que el Gateway ya usaba para el FUT: no se agregó
    ninguna dependencia.
  - Fuentes estándar con cp1252 (tildes, ñ, «», –, ×). La flecha «→» se imprime como «->».
- **XLSX:** sin cambios.
- **Frontend:** botón «Exportar PDF» junto a Excel y CSV, con el mismo mecanismo, los mismos
  filtros y el mismo manejo de errores.
- **Permisos:** el PDF pasa por las mismas lógicas que el CSV y el XLSX (`reportes.view` y alcance
  por área). No cambió ningún permiso.
