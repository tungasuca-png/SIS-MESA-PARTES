# D8 — Reconciliación diaria entre la fuente de verdad y la proyección

Fuente de verdad: `expedientes_db.expediente_movimientos` (+ `expediente_outbox`).
Proyección: `derivaciones_db.movimientos_proyeccion`. Objetos: migración
`expedientes/006_reconciliacion.sql`. Prototipo validado:
`harness/reconciliador.go` (E1b).

## 1. Qué detecta

| Tipo | Condición | ¿Republicable? |
|---|---|---|
| `PUBLICADO_SIN_PROYECCION` | evento PUBLICADO y sin proyección (incluido el **último** movimiento del expediente) | Sí (reencola) |
| `MOVIMIENTO_SIN_PROYECCION` | sin proyección y sin evento (purgado o perdido) | Sí (recrea el evento) |
| `EVENTO_RETRASADO` | evento PENDIENTE más antiguo que la ventana de gracia | No: lo resuelve el publicador; alerta si persiste |
| `EVENTO_FALLIDO` | evento FALLIDO | No: runbook (persona) |
| `PROYECCION_DIVERGENTE` | proyección con cualquier campo distinto (incluye `ocurrido_en` al microsegundo) | No: revisión humana |
| `PROYECCION_HUERFANA` | proyección sin movimiento de origen | No: revisión humana |
| `SECUENCIA_ORIGEN_INCOMPLETA` | huecos 1..n en el origen o `ultima_secuencia` ≠ máxima | No: incidente de integridad |
| `SECUENCIA_PROYECCION_INCOMPLETA` | huecos en la proyección | Indirecto (republica los movimientos faltantes) |
| `EXPEDIENTE_INCOHERENTE` | área/estado del expediente ≠ último movimiento | No: incidente de integridad |
| `CADENA_DISCONTINUA` | origen de un movimiento ≠ destino del anterior (cambio no trazado, p. ej. ventana de vuelta atrás) | No: informativo/documentado |

Un evento perdido se detecta **aunque sea el último movimiento**: la comparación
es movimiento a movimiento contra la proyección, no depende de que exista uno posterior.

## 2. Algoritmo

1. `INSERT` en `reconciliacion_ejecuciones` (la base fija hora, usuario de sesión y estado `EN_CURSO`; `corte_hasta = ahora − gracia`).
2. Instantánea `REPEATABLE READ READ ONLY` del origen: movimientos + estado del evento + `id, ultima_secuencia, area_actual, estado` de expedientes.
3. Instantánea `REPEATABLE READ READ ONLY` de la proyección.
4. Comparación (tabla del punto 1). Los movimientos posteriores a `corte_hasta` no cuentan como faltantes. Una fila proyectada cuyo id no está en la instantánea de origen se vuelve a buscar en el origen antes de declararla huérfana (evita falsos positivos por movimientos nuevos).
5. `INSERT` de cada discrepancia (append-only; solo en una ejecución `EN_CURSO`).
6. Solo en modo `REPUBLICACION`: `SELECT reconciliacion_republicar(ejecucion, movimiento)` para cada movimiento sin proyección (límite por ejecución).
7. Informe JSON firmado con SHA-256; la huella se registra en la ejecución, que se finaliza **una sola vez** (`SIN_DISCREPANCIAS`, `CON_DISCREPANCIAS` o `ERROR`).

## 3. Frecuencia y ejecución

Programa: `backend/services/expedientes/cmd/reconciliador` (`go build -o reconciliador.exe ./cmd/reconciliador`).

- **Diaria, 02:00**, Programador de tareas de Windows Server 2022: `reconciliador.exe -modo deteccion -gracia 15m -informes D:\...\reconciliacion-informes`.
- Ventana de gracia: 15 min (mínimo 1 min; un movimiento más reciente no se considera faltante).
- `EVENTO_RETRASADO` usa el mismo corte: un PENDIENTE de más de 15 min ya es alerta.
- La **republicación es manual** y explícita (`-modo republicacion -max 50`, máximo 1000), por una persona, tras revisar el informe de detección. (Automatizarla es una decisión posterior; el mecanismo ya es idempotente y acotado.)
- Límite de tiempo por ejecución: `-limite 10m`. `-alertar-siempre` envía correo también sin discrepancias.
- Códigos de salida: **0** sin discrepancias, **2** con discrepancias, **1** error. Log de una línea JSON (`ALERTA ...` si el código no es 0).

Variables de entorno (en la tarea programada o en un archivo protegido por ACL; **nunca** en el repositorio):

| Variable | Contenido |
|---|---|
| `RECONCILIADOR_EXPEDIENTES_DSN` | `postgres://reconciliador_movimientos@HOST:PUERTO/expedientes_db?sslmode=...` **sin contraseña** (si la incluye, el programa se niega a correr) |
| `RECONCILIADOR_DERIVACIONES_DSN` | ídem para `derivaciones_db` |
| `RECONCILIADOR_DB_PASSWORD` | contraseña del rol `reconciliador_movimientos` |
| `RECONCILIADOR_SMTP_SERVIDOR`, `_USUARIO`, `_CLAVE`, `_REMITENTE`, `_DESTINATARIOS` | alerta por correo opcional (destinatarios separados por coma). Con usuario se exige TLS (STARTTLS) salvo `localhost`. |

Tarea programada (referencia; **no se ejecutó en E2**):

```powershell
$accion = New-ScheduledTaskAction -Execute 'D:\SIS-MESA-PARTES\bin\reconciliador.exe' `
  -Argument '-modo deteccion -gracia 15m -informes D:\SIS-MESA-PARTES\reconciliacion-informes' `
  -WorkingDirectory 'D:\SIS-MESA-PARTES\bin'
$disparador = New-ScheduledTaskTrigger -Daily -At 2am
Register-ScheduledTask -TaskName 'SIS-MESA-PARTES D8' -Action $accion -Trigger $disparador `
  -User 'NT SERVICE\...' -RunLevel Limited   # cuenta de servicio sin privilegios; variables de entorno de esa cuenta
```

## 4. Alertas

| Condición | Señal |
|---|---|
| Ejecución `CON_DISCREPANCIAS` | código de salida 2, log ERROR, correo a responsables (SMTP institucional) |
| Ejecución `ERROR` | código de salida 1, log ERROR, correo |
| Sin ejecución correcta en 26 h | vista `reconciliacion_vigilancia.alerta_sin_ejecucion = true` (consultada por el monitoreo) |
| `EVENTO_FALLIDO`, `SECUENCIA_ORIGEN_INCOMPLETA`, `EXPEDIENTE_INCOHERENTE`, `PROYECCION_HUERFANA`, `PROYECCION_DIVERGENTE` | prioridad alta (posible manipulación o pérdida de datos) |

## 5. Límites de permisos (`reconciliador_movimientos`)

- `expedientes_db`: `SELECT` en `expediente_movimientos` y `expediente_outbox`; `SELECT (id, area_actual, estado, activo, ultima_secuencia)` en `expedientes` (sin asunto, descripción ni solicitante); `INSERT` en ejecuciones y discrepancias; `UPDATE` solo de las columnas de finalización; `EXECUTE reconciliacion_republicar`.
- `derivaciones_db`: solo `SELECT` en `movimientos_proyeccion` (no lee `derivaciones`).
- Sin `CONNECT` a ninguna otra base; sin `SET ROLE`; sin `session_replication_role`.
- **No puede** modificar movimientos, proyección, outbox, historial ni el propio registro de reconciliación (triggers append-only, finalización única).

## 6. Republicación controlada e idempotente

`reconciliacion_republicar(ejecucion, movimiento)` (`SECURITY DEFINER`, `search_path` fijo):
- solo en una ejecución `REPUBLICACION` **en curso**;
- solo para movimientos registrados en **esa** ejecución como `MOVIMIENTO_SIN_PROYECCION` o `PUBLICADO_SIN_PROYECCION`;
- una sola acción por (ejecución, movimiento) — la repetición devuelve `YA_PROCESADO_EN_ESTA_EJECUCION`;
- PUBLICADO → reencola; sin evento → lo recrea; PENDIENTE → sin cambio; FALLIDO → **no lo toca** (`RECHAZADO_FALLIDO`);
- registra la acción (append-only).

El movimiento llega a la proyección por la vía normal (publicador → consumidor), que
compara contenido: un reenvío de algo ya proyectado es `DUPLICADO` y un contenido
distinto es `DIVERGENTE`. Por eso la reconciliación **nunca modifica el historial**:
solo puede pedir que se reenvíe un movimiento que ya existe en el origen.

## 7. Evidencia

Por ejecución: fila en `reconciliacion_ejecuciones` (contadores, estado, usuario,
horas, huella), filas en `reconciliacion_discrepancias` y `reconciliacion_acciones`,
y el informe JSON (solo lectura) cuya SHA-256 coincide con `huella_informe`.
La recuperación se evidencia con la ejecución de detección siguiente (las
discrepancias republicadas desaparecen; las no republicables permanecen).
