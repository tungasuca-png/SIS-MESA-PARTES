# Módulo de Psicología — Arquitectura y estado

**Este documento es la fuente vigente.** Reemplaza al diseño anterior de la fase de
inspección (que trataba "sesión" y "atención" como entidades separadas, incluía una
entidad de intervenciones y ponía `view_own` en cada recurso hijo).

Fecha: 2026-09-21. Estado: **FASE 1 implementada** (base del servicio + expediente
psicológico). Lo que no está en la sección 12 ("Implementado") es diseño para fases
posteriores.

Leyenda: **[HECHO]** = implementado y probado · **[DISEÑO]** = decidido, aún no
implementado · **[PENDIENTE]** = depende de algo que no existe o de una decisión.

---

## 1. Objetivo

Gestionar la atención psicológica individual mediante un **expediente psicológico
permanente por estudiante** (1 estudiante = 1 expediente), del que cuelgan las
atenciones, citas, evaluaciones, seguimientos y documentos de fases futuras.

El expediente psicológico **no** es el expediente administrativo de Mesa de Partes
(`expedientes`); son conceptos distintos con servicios y bases distintos.

## 2. Arquitectura [HECHO]

```text
React → Gateway :8888 → Psicología :8086 → psicologia_db (PostgreSQL :5433)
                          ├─ Auth :8080      Auth.HasPermission (fail-closed)
                          └─ Usuarios :8083  GetUsuariosBasic (validar responsable_id)
```

- **Servicio propio** `backend/services/psicologia` (Go + go-zero + gRPC), con la
  estructura real de los demás servicios: `psicologia.go`, `psicologia_service.proto`,
  `psicologia/` (pb), `psicologiaclient/`, `etc/`, `migrations/`,
  `internal/{config,svc,server,interceptor,security,authorization,validation,repository,logic,integration}`.
  No hay `cmd/`, `handler/` ni `domain/` (no existen en ningún servicio).
- **Base propia** `psicologia_db`, usuario `psicologia_user`, mismo servidor PostgreSQL.
  Sin FK entre bases: `estudiante_id`, `responsable_id`, `abierto_por` son UUID sueltos.
- **Solo loopback** (`127.0.0.1:8086`): el único llamador legítimo es el Gateway.
- **Gateway:** `PsicologiaRpc` (`Target: 127.0.0.1:8086`) en `gateway-api.yaml`; los
  handlers solo reenvían `Authorization` y no validan roles ni permisos.
- **Auth:** el servicio usa el mismo `JWT_SECRET` y el mismo helper `RequirePermission`
  → `Auth.HasPermission` que Usuarios/Derivaciones. No consulta `role_permissions`.

### Responsabilidades

| Es dueño de | No es dueño de |
|---|---|
| `expedientes_psicologicos`, `expediente_codigo_counters`, `auditoria_psicologica` (y, en fases futuras, atenciones, citas, evaluaciones, seguimientos y documentos psicológicos) | usuarios (Auth/Usuarios), roles y permisos (Auth), estudiantes (servicio pendiente), documentos de Mesa de Partes |

## 3. Integración con estudiantes [PENDIENTE]

**No existe ninguna entidad "estudiante" en el sistema.** Psicología solo guarda
`estudiante_id` (UUID) y **no** duplica DNI, nombres, grado, sección ni fecha de
nacimiento. Recomendación: un servicio **Estudiantes** propietario de los datos
oficiales (importables desde SIAGIE u otra fuente).

Punto de extensión implementado: `svc.EstudianteVerifier` (`Exists(ctx, id)`). Es
`nil` mientras el servicio no exista; entonces `OpenExpediente` solo valida el
formato UUID. **No hay API falsa ni datos de estudiantes inventados.**

Consecuencias mientras no exista Estudiantes: no se puede verificar que el estudiante
exista, ni mostrar su ficha, ni buscar por DNI/nombre (queda para fases posteriores).

## 4. Autorización [HECHO]

Permiso ≠ alcance. El **permiso** (Auth) decide *si*; el **alcance** decide *cuáles*.
Nunca se decide por el nombre del rol.

| Permiso | Uso en FASE 1 |
|---|---|
| `psicologia.expedientes.view` | alcance **general**: cualquier expediente |
| `psicologia.expedientes.view_own` | alcance **propio**: solo `responsable_id == userID` |
| `psicologia.expedientes.create` | `OpenExpediente` |
| `psicologia.expedientes.update` | `UpdateExpediente`, `CloseExpediente`, `ReopenExpediente` |
| `psicologia.view` | registrado; reservado para el acceso al módulo en el frontend |
| `psicologia.estudiantes.view` | registrado; se usará con el servicio Estudiantes |

- El alcance se resuelve **una vez, en el expediente**; los recursos hijos futuros lo
  heredan (no hay decenas de permisos `_own`).
- Las mutaciones exigen el permiso de escritura **y** estar dentro del alcance
  (view o view_own). Un usuario con `update` pero sin ningún permiso de lectura queda
  denegado. El alcance se vuelve a aplicar dentro de la transacción (`SELECT … FOR UPDATE`).
- Con `view_own`, un expediente ajeno responde **404** igual que uno inexistente (no
  revela su existencia) y queda como `DENIED`. Sin ningún permiso → **403**.
- `Auth.HasPermission` responde solo por el usuario autenticado: **no** se puede
  verificar el permiso de otro usuario (p. ej. del `responsable_id`). Solo se valida
  que exista en Usuarios (`GetUsuariosBasic`, que tampoco expone si está activo).
- **Registro sin asignación:** la migración de Auth registra el rol y los 6 permisos y
  **no inserta nada en `role_permissions`** (ni ADMIN ni PSICOLOGO). ADMIN asigna desde
  Administración → Roles y Permisos. Comprobado: retirar el permiso corta el acceso sin
  tocar código.

### Rol PSICOLOGO [HECHO]

Existe solo como rol del sistema (agrupa usuarios). Los nombres de rol están cableados
en varios sitios; en la FASE 1 se tocaron: Auth (migración 010, lista de roles no
registrables en `registerlogic.go`), Usuarios (migración 002 del `CHECK` de
`tipo_usuario` y `validation.go`) y el frontend (`ROLE_LABELS`). Los `IsInternal` de
Expedientes/Documentos **no** se modificaron a propósito: un psicólogo no obtiene acceso
a Mesa de Partes por su rol. `COORDINADOR_PSICOLOGIA` no se creó: la diferencia entre
"ve todo" y "ve lo suyo" se resuelve con `view` frente a `view_own` en distintos roles,
y solo hace falta un segundo rol si el negocio lo pide.

## 5. Modelo de datos [HECHO]

`migrations/001_create_psicologia_base.sql` (aplicada por un rol administrador; ver README).

**`expedientes_psicologicos`:** `id UUID PK`; `estudiante_id UUID NOT NULL`
**`UNIQUE`**; `codigo VARCHAR(20)` único, formato `PSI-AAAA-NNNNNN`; `estado`
(`VARCHAR + CHECK`: ABIERTO / EN_SEGUIMIENTO / CERRADO); `responsable_id`, `abierto_por`
(UUID); `fecha_apertura`; `fecha_cierre`, `motivo_cierre`; `resumen_caso`; `created_at`,
`updated_at`. `CHECK`: un expediente CERRADO exige fecha y motivo de cierre. Índices por
responsable, estado y fecha de apertura.

**`expediente_codigo_counters`:** contador por año. El código se genera con
`INSERT … ON CONFLICT DO UPDATE … RETURNING` dentro de la misma transacción que crea el
expediente: seguro ante concurrencia y sin huecos si el INSERT falla.

**`auditoria_psicologica`:** `id`, `usuario_id`, `accion`, `recurso`, `recurso_id`,
`estudiante_id`, `fecha_hora`, `resultado`, `motivo`.

- Acciones: CREATE, VIEW, UPDATE, CLOSE, REOPEN, DENIED. `motivo` es un código corto
  del sistema (p. ej. `PERMISO_INSUFICIENTE`, `FUERA_DE_ALCANCE`, `LISTADO`) o los
  nombres de los campos modificados, **nunca** texto del usuario ni contenido clínico.
- **Append-only en dos capas:** (1) `psicologia_user` solo tiene `SELECT/INSERT` (y
  ninguna tabla admite `DELETE`); (2) un trigger rechaza `UPDATE/DELETE/TRUNCATE`
  incluso al dueño. Probado.

Diseño futuro (atenciones, versiones inmutables, motivos y derivaciones como tablas,
compromisos, firmas, citas, evaluaciones, seguimientos, documentos): ver sección 11.

## 6. RPC y endpoints [HECHO]

| RPC | Endpoint HTTP (Gateway) | Permiso |
|---|---|---|
| `OpenExpediente` | `POST /api/psicologia/estudiantes/:id/expediente` | `expedientes.create` |
| `GetExpedienteByEstudiante` | `GET /api/psicologia/estudiantes/:id/expediente` | view / view_own |
| `GetExpediente` | `GET /api/psicologia/expedientes/:id` | view / view_own |
| `ListExpedientes` | `GET /api/psicologia/expedientes?estado=&page=&page_size=` | view / view_own |
| `UpdateExpediente` | `PUT /api/psicologia/expedientes/:id` | `expedientes.update` + alcance |
| `CloseExpediente` | `PUT /api/psicologia/expedientes/:id/cerrar` | `expedientes.update` + alcance |
| `ReopenExpediente` | `PUT /api/psicologia/expedientes/:id/reabrir` | `expedientes.update` + alcance |

Códigos: 401 (sin/invalid token), 403 (sin permiso), 404 (inexistente o fuera de
alcance), 409 (el estudiante ya tiene expediente), 400 (datos o transición inválidos),
500 (interno, sin texto de PostgreSQL).

**Reglas:**
- `OpenExpediente`: `abierto_por` = usuario autenticado; `responsable_id` = el mismo si
  no se indica, o el indicado si es un UUID de un usuario que existe en Usuarios. No
  crea nada al consultar. La carrera concurrente la resuelve `UNIQUE(estudiante_id)`.
- `GetExpediente*` incluyen `resumen_caso`; `ListExpedientes` **no** (dato clínico).
- `UpdateExpediente`: campos opcionales `resumen_caso`, `responsable_id`, `estado`
  (solo ABIERTO ↔ EN_SEGUIMIENTO). No modifica un expediente CERRADO.
- `CloseExpediente`: ABIERTO/EN_SEGUIMIENTO → CERRADO con motivo obligatorio.
- `ReopenExpediente`: **única transición desde CERRADO**: a ABIERTO (por defecto) o a
  EN_SEGUIMIENTO. `fecha_cierre`/`motivo_cierre` conservan el **último** cierre (cada
  cierre queda como evento CLOSE; un historial de estados completo es de FASE 2).

## 7. Auditoría [HECHO]

- **Mutaciones** (CREATE/UPDATE/CLOSE/REOPEN): el evento se escribe **en la misma
  transacción** que el cambio.
- **Lecturas** (`Get*`, `List`): se registra VIEW **antes** de devolver el dato y es
  fail-closed: si la bitácora no se puede escribir, no se entrega el expediente.
- **Denegaciones:** DENIED (mejor esfuerzo) tanto por falta de permiso como por
  alcance. Un error de Auth **no** se audita como denegación: se propaga.
- **Logs:** el servicio configura `IgnoreContentMethods` para todos sus RPC (go-zero
  registra el contenido de solicitudes lentas) y el Gateway desactiva el interceptor de
  duración para `PsicologiaRpc` (registraba el **contenido** de cada llamada fallida).
  Verificado que ni texto clínico ni identificadores de estudiantes aparecen en los logs.

## 8. Seguridad — límites conocidos

- **Cifrado en reposo (FASE 6N-3):** el texto libre clínico (hoja de atención, motivos
  de corrección/anulación, resumen del caso y motivo de cierre) se guarda cifrado con
  AES-256-GCM en la aplicación (ver `services/psicologia/README.md` y
  `docs/seguridad-infraestructura.md`). No hay ninguna clave en el código. Pendiente
  la decisión institucional de custodia de la clave y el procedimiento de rotación.
- **ADMIN** podría asignarse permisos `psicologia.*` desde Roles y Permisos. No los
  recibe por defecto; falta decidir si se restringe quién puede asignarlos y auditar esos
  cambios.
- La aplicación no puede borrar la bitácora, pero un administrador de PostgreSQL sí
  (endurecimiento futuro: encadenado por hash, replicación externa).
- Datos de salud de menores: revisión legal (Ley N.º 29733) pendiente antes de producción.

## 9. Frontend

No se desarrolló pantalla de Psicología en la FASE 1. Único cambio: `ROLE_LABELS`
(`PSICOLOGO: "Psicología"`). Contrato listo para consumir (sección 6).

## 10. Operación

- **Servicios:** `backend/run-all.ps1` levanta 7 procesos (agrega Psicología :8086) y
  carga también `PSICOLOGIA_DB_PASSWORD` desde `backend/.env`; `stop-all.ps1` lo baja.
- **Base de datos:** creación de rol y base + migración (README del servicio). La
  migración debe aplicarla un administrador, no `psicologia_user`.
- **Pruebas:** `INTEGRATION_TEST=true go test ./...` en `services/psicologia`.

## 11. Diseño para fases posteriores [DISEÑO]

Decisiones ya tomadas (sin implementar):

- **Atención = sesión.** Una sola entidad `atenciones_psicologicas` (la ficha de
  atención personalizada al estudiante y la familia). "Sesión" solo justificaría una
  entidad propia con planes de varias sesiones (futuro). Una atención puede existir sin
  cita y una cita sin atención (`atencion.cita_id` opcional).
- **Atenciones inmutables:** estados BORRADOR → REGISTRADA → FIRMADA / ANULADA. Una
  corrección crea una **versión nueva** (`atencion_versiones`, append-only) con autor,
  fecha y motivo; los datos vigentes viven en tablas hijas normalizadas para consultar y
  reportar.
- **Motivos:** catálogo (`motivos_atencion`) + tabla puente N:M — no ENUM: es negocio
  configurable, como `roles`/`permissions`. **Derivaciones de la ficha:** tabla hija
  `atencion_derivaciones` con catálogo de destinos (nombre distinto del servicio
  Derivaciones de Mesa de Partes; sin relación con él). **Compromisos:** filas por
  `parte` (ESTUDIANTE / FAMILIA).
- **Firmas:** imagen (PNG, como el FUT) en documentos + hash del contenido firmado.
  Es constancia de conformidad, **no** firma digital calificada.
- **Intervenciones:** sin tabla propia (la ficha las recoge como "estrategias de
  atención y apoyo"). **Seguimiento:** entidad independiente y ligera (agenda de
  próxima acción/fecha), distinta del campo "seguimiento de resultados" de la atención.
- **Evaluaciones:** entidad independiente con catálogo de tipos y versiones.
- **Citas:** `profesional_id` = UUID de Auth/Usuarios; sin entidad de profesional.
- **Documentos:** tabla propia en `psicologia_db` detrás de una interfaz `DocumentStore`
  (implementación PostgreSQL BYTEA cifrado en el MVP); metadatos separados de bytes.
- **Reportes:** sin servicio nuevo; RPC de solo agregados para el módulo general de
  Reportes (`Reportes → Mesa de Partes · Psicología · Generales`). No hay "Psicología →
  Reportes".

## 12. Implementado en la FASE 1

- Servicio `psicologia` completo (7 RPC), `psicologiaclient`, configuración, README.
- Migración `001` (esquema, restricciones, triggers, privilegios mínimos).
- Auth `010` (rol PSICOLOGO + 6 permisos, sin asignar); Usuarios `002` (`CHECK`).
- Gateway: 7 rutas `/api/psicologia/...`, `PsicologiaRpc`, DTO y logs sin contenido.
- `run-all.ps1` / `stop-all.ps1` / `.env.example`.
- Pruebas de integración (BD, autorización, alcance, JWT, concurrencia, auditoría).

## 13. Dependencias pendientes

1. **Servicio Estudiantes** (o fuente institucional importable) — bloquea la ficha, la
   búsqueda y la verificación de existencia.
2. Decisión sobre quién puede asignar permisos `psicologia.*` (privacidad).
3. Cifrado en reposo y gestión de claves (antes de guardar contenido clínico de
   atenciones).
4. Reenvío de IP / User-Agent desde el Gateway (auditoría de origen).
5. Revisión legal (Ley 29733), retención y consentimiento del apoderado.
6. Hallazgo ajeno a Psicología (ver reporte de la fase): el Gateway y Auth registran en
   logs la contraseña en ciertos errores/llamadas lentas.

## 14. Fase 2 (siguiente)

Atenciones (ficha) con versiones inmutables, catálogos de motivos y destinos,
derivaciones y compromisos de la ficha; historial de estados del expediente; y, en
cuanto exista Estudiantes, la verificación real de `estudiante_id`.

## 15. Atenciones psicológicas (Hoja de Atención) [HECHO]

Implementa la Fase 2 con algunas diferencias respecto del diseño de la §11 (se indican).

- **Flujo:** Psicología → Buscar alumno (DNI / nombre / apellidos, `EstudianteSelector`)
  → expediente (uno por estudiante) → Atenciones (sin límite; los dos bloques del formato
  impreso son solo diseño de hoja) → Nueva atención → Guardar borrador → Registrar
  (confirmación) → Detalle → Versiones → Corregir → Anular → Descargar PDF.
- **Estados:** `BORRADOR → REGISTRADA → ANULADA`, más `DESCARTADA` (borrador descartado
  por su autor; no hay DELETE físico). **Sin `FIRMADA`**: la firma sigue pendiente de
  decisión institucional (el PDF deja el recuadro vacío para firma manuscrita).
- **Migración `002_create_atenciones.sql`:** `atenciones_psicologicas`,
  `atencion_versiones` (versión 0 = borrador editable; 1..N inmutables por trigger
  `atencion_versiones_inmutables`), `atencion_numero_counters` (correlativo por
  expediente con `INSERT … ON CONFLICT DO UPDATE … RETURNING`, nunca `MAX+1`), catálogos
  `catalogo_motivos_atencion` y `catalogo_destinos_derivacion`. Motivos y derivaciones se
  guardan en la versión como JSONB con la **etiqueta congelada** (diferencia con la §11,
  que preveía tablas puente). La app no tiene `DELETE` en ninguna tabla de atenciones.
- **Concurrencia:** borrador con `revision` (409 si otra operación lo cambió); corrección
  con `version_base` (409 si ya existe una versión más nueva); bloqueo `FOR UPDATE` del
  expediente y de la atención.
- **Permisos (Auth `014`, sin asignar a ningún rol):** `psicologia.atenciones.view`
  (listar, consultar, versiones, PDF), `.create` (borrador, registrar, descartar),
  `.correct`, `.annul`. Siempre combinados con el alcance del expediente
  (`psicologia.expedientes.view` / `view_own`). Los borradores solo los ve su autor.
- **Expediente cerrado:** bloquea crear, registrar, corregir y anular; descartar y leer
  siguen permitidos.
- **RPC (11):** `CreateAtencionDraft`, `UpdateAtencionDraft`, `RegisterAtencion`,
  `DiscardAtencionDraft`, `GetAtencion`, `ListAtenciones`, `CorrectAtencion`,
  `ListAtencionVersions`, `GetAtencionVersion`, `AnnulAtencion`, `ExportAtencionPdf`
  (todos en `IgnoreContentMethods` y en `logsafe` del Gateway).
- **Endpoints:** `GET|POST /api/psicologia/expedientes/:id/atenciones`,
  `GET /api/psicologia/atenciones/:id`, `PUT …/:id/borrador|registrar|descartar|anular`,
  `POST|GET …/:id/versiones`, `GET …/:id/versiones/:version`, `GET …/:id/pdf[?version=n]`
  (`{nombre, contenido_base64}`).
- **PDF:** generado en Psicología Service (`internal/pdf`, go-pdf/fpdf, en memoria) con
  escudo embebido; franja "Estudiante / Expediente / Atención / Versión / Estado" fuera
  de la tabla; nombre `PSI-AAAA-NNNNNN-ATnn-vN.pdf` (sin DNI). Los borradores no se
  exportan; cada descarga se audita (`EXPORT`) y, si no se puede identificar al
  estudiante, no hay PDF ni evento.
- **Auditoría:** `CREATE`, `UPDATE`, `REGISTER`, `DISCARD`, `VIEW`, `CORRECT`, `ANNUL`,
  `EXPORT`, `DENIED` con recurso `ATENCION_PSICOLOGICA`; nunca contenido de la hoja.
- **Frontend:** `BuscarAlumnoPanel`, `AtencionesSection` (en el detalle del expediente),
  página `PsicologiaAtencion` (`/psicologia/expedientes/:id/atenciones/nueva`,
  `/psicologia/atenciones/:id`), `HojaAtencion`. Nada clínico en `localStorage`.
- **Cifrado (FASE 6N-3):** el contenido de la hoja se guarda cifrado (AES-256-GCM); las
  columnas en claro se eliminaron (migraciones 003 y 004).
- **Pendiente antes de datos reales:** custodia institucional de la clave clínica,
  revisión legal (Ley N.º 29733), HTTPS en producción, firma y definición
  institucional de campos obligatorios (hoy solo fecha, hora y el texto de "Otro").
