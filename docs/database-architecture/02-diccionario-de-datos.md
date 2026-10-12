# 02 — Diccionario de datos

**Cobertura:** 31 tablas y 266 columnas verificadas en 7 bases.

> Generado a partir del **catálogo verificado** de PostgreSQL 17.11 (consultas de solo lectura del 2026-09-30) y de las migraciones de cada servicio. Todas las columnas verificadas están declaradas en las migraciones (comprobación automática por servicio); por eso la columna **Estado** es "Implementado" salvo indicación.
>
> - **Tipo**: nombre interno de PostgreSQL (`varchar`, `int4`, `timestamptz`, `_text` = arreglo de texto…).
> - **UNIQUE**: restricción o índice único (los parciales se indican con *parcial*). **CHECK**: nombres de las restricciones que involucran la columna (la definición completa está en [03-relaciones-actuales.md](03-relaciones-actuales.md)).
> - **Descripción**: tomada de comentarios de migraciones y del código. "Pendiente de definición" = sin evidencia suficiente.
> - No contiene valores reales: solo metadatos y conteos de filas.


## Auth — `auth_db` (esquema `public`)

### `permissions`

Catálogo de permisos (40 códigos).

- Filas (conteo verificado): **40** · Propietario: `auth_owner` · Creada en: `003_create_permissions_tables.sql`
- Consumidores: Todos los servicios (HasPermission/JWT), Gateway

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `id` | uuid |  | No | Sí |  |  |  | gen_random_uuid() | permissions_pkey | Identificador UUID del registro. | Técnico / catálogo | Generado por la base (DEFAULT/trigger) | Implementado |
| `codigo` | varchar | 100 | No |  |  | permissions_codigo_key | chk_permissions_codigo |  | permissions_codigo_key | Código del permiso (único), p. ej. estudiantes.view. | Autenticación | Semilla de migración / administración | Implementado |
| `nombre` | varchar | 100 | No |  |  |  | chk_permissions_nombre |  |  | Nombre visible del permiso. | Técnico / catálogo | Semilla de migración / administración | Implementado |
| `descripcion` | text |  | Sí |  |  |  |  |  |  | Descripción del permiso. | Técnico / catálogo | Semilla de migración / administración | Implementado |
| `modulo` | varchar | 50 | No |  |  |  | chk_permissions_modulo |  | idx_permissions_modulo | Módulo al que pertenece. | Técnico / catálogo | Semilla de migración / administración | Implementado |
| `estado` | bool |  | No |  |  |  |  | true |  | Permiso activo. | Autenticación | Semilla de migración / administración | Implementado |
| `created_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de creación. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |
| `updated_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de la última actualización. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |

### `refresh_tokens`

Refresh tokens (solo su hash) por cuenta; rotación y revocación.

- Filas (conteo verificado): **384** · Propietario: `auth_owner` · Creada en: `001_create_refresh_tokens.sql`
- Consumidores: Todos los servicios (HasPermission/JWT), Gateway

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `id` | uuid |  | No | Sí |  |  |  | gen_random_uuid() | refresh_tokens_pkey | Identificador UUID del registro. | Técnico / catálogo | Generado por la base (DEFAULT/trigger) | Implementado |
| `usuario_id` | uuid |  | No |  | usuarios(id) |  |  |  | idx_refresh_tokens_usuario_id | Cuenta dueña del token (FK usuarios). | Autenticación | API Auth (Register/CreateUser/login) | Implementado |
| `token_hash` | bytea |  | No |  |  | refresh_tokens_token_hash_key | chk_refresh_tokens_hash_length |  | idx_refresh_tokens_active_lookup, refresh_tokens_token_hash_key | Hash del refresh token (nunca el token). | Autenticación | API Auth (Register/CreateUser/login) | Implementado |
| `expires_at` | timestamptz |  | No |  |  |  | chk_refresh_tokens_expiry |  | idx_refresh_tokens_active_lookup | Expiración del token. | Autenticación | API Auth (Register/CreateUser/login) | Implementado |
| `revoked_at` | timestamptz |  | Sí |  |  |  |  |  |  | Revocación (logout/rotación). | Autenticación | API Auth (Register/CreateUser/login) | Implementado |
| `created_at` | timestamptz |  | No |  |  |  | chk_refresh_tokens_expiry | CURRENT_TIMESTAMP |  | Fecha y hora de creación. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |

### `role_permissions`

Permisos asignados a cada rol (editable desde Roles y permisos).

- Filas (conteo verificado): **97** · Propietario: `auth_owner` · Creada en: `003_create_permissions_tables.sql`
- Consumidores: Todos los servicios (HasPermission/JWT), Gateway

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `role_id` | uuid |  | No | Sí | roles(id) |  |  |  | role_permissions_pkey | Rol (FK roles). | Autenticación | Semilla de migración / administración | Implementado |
| `permission_id` | uuid |  | No | Sí | permissions(id) |  |  |  | idx_role_permissions_permission_id, role_permissions_pkey | Permiso (FK permissions). | Autenticación | Semilla de migración / administración | Implementado |
| `created_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de creación. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |

### `roles`

Roles del sistema (8 filas semilla: ADMIN, DIRECTOR, SUBDIRECTOR, SECRETARIA, DOCENTE, AUXILIAR, PSICOLOGO, SOLICITANTE).

- Filas (conteo verificado): **8** · Propietario: `auth_owner` · Creada en: `000_create_users_and_roles.sql`
- Consumidores: Todos los servicios (HasPermission/JWT), Gateway

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `id` | uuid |  | No | Sí |  |  |  | gen_random_uuid() | roles_pkey | Identificador UUID del registro. | Técnico / catálogo | Generado por la base (DEFAULT/trigger) | Implementado |
| `nombre` | varchar | 50 | No |  |  | uq_roles_nombre | chk_roles_nombre |  | uq_roles_nombre | Código del rol (único). | Autenticación | Semilla de migración / administración | Implementado |
| `descripcion` | varchar | 255 | Sí |  |  |  |  |  |  | Descripción del rol. | Técnico / catálogo | Semilla de migración / administración | Implementado |
| `estado` | bool |  | No |  |  |  |  | true |  | Rol activo. | Autenticación | Semilla de migración / administración | Implementado |
| `created_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de creación. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |
| `updated_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de la última actualización. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |

### `user_profiles`

Perfil de persona dentro de Auth (migración 004). Sin filas y SIN uso en el código: redundante con usuarios_db.usuarios.

- Filas (conteo verificado): **0** · Propietario: `auth_owner` · Creada en: `004_create_user_profiles.sql`
- Consumidores: Todos los servicios (HasPermission/JWT), Gateway

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `id` | uuid |  | No | Sí |  |  |  | gen_random_uuid() | user_profiles_pkey | Identificador UUID del registro. | Técnico / catálogo | Generado por la base (DEFAULT/trigger) | Implementado |
| `user_id` | uuid |  | No |  | usuarios(id) | uq_user_profiles_user_id |  |  | uq_user_profiles_user_id | Cuenta (FK usuarios, único). Tabla sin uso. | Identificación | API Auth (Register/CreateUser/login) | Implementado |
| `nombres` | varchar | 100 | No |  |  |  | chk_user_profiles_nombres |  |  | Nombres. Tabla sin uso (duplica usuarios_db). | Identificación | API Auth (Register/CreateUser/login) | Implementado |
| `apellidos` | varchar | 100 | No |  |  |  | chk_user_profiles_apellidos |  |  | Apellidos. Tabla sin uso (duplica usuarios_db). | Identificación | API Auth (Register/CreateUser/login) | Implementado |
| `tipo_documento` | varchar | 20 | No |  |  | uq_user_profiles_document | chk_user_profiles_tipo_documento |  | uq_user_profiles_document | Tipo de documento (texto libre). Tabla sin uso. | Identificación | API Auth (Register/CreateUser/login) | Implementado |
| `numero_documento` | varchar | 30 | No |  |  | uq_user_profiles_document | chk_user_profiles_numero_documento |  | uq_user_profiles_document | Número de documento. Tabla sin uso. | Identificación | API Auth (Register/CreateUser/login) | Implementado |
| `telefono` | varchar | 20 | Sí |  |  |  |  |  |  | Teléfono. Tabla sin uso. | Contacto | API Auth (Register/CreateUser/login) | Implementado |
| `direccion` | text |  | Sí |  |  |  |  |  |  | Dirección. Tabla sin uso. | Contacto | API Auth (Register/CreateUser/login) | Implementado |
| `created_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de creación. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |
| `updated_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de la última actualización. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |

### `usuario_roles`

Asignación de rol a cuenta (un solo rol por cuenta desde la migración 007).

- Filas (conteo verificado): **33** · Propietario: `auth_owner` · Creada en: `000_create_users_and_roles.sql` · Alterada en: `007_enforce_single_role_per_user.sql`
- Consumidores: Todos los servicios (HasPermission/JWT), Gateway

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `usuario_id` | uuid |  | No | Sí | usuarios(id) | uq_usuario_roles_usuario |  |  | uq_usuario_roles_usuario, usuario_roles_pkey | Cuenta (FK usuarios). | Autenticación | Semilla de migración / administración | Implementado |
| `rol_id` | uuid |  | No | Sí | roles(id) |  |  |  | idx_usuario_roles_rol_id, usuario_roles_pkey | Rol (FK roles). | Autenticación | Semilla de migración / administración | Implementado |
| `created_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de creación. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |

### `usuarios`

Cuentas de acceso (credenciales, estado de la cuenta, bloqueo). Login por DNI + clave (migración 009).

- Filas (conteo verificado): **35** · Propietario: `auth_owner` · Creada en: `000_create_users_and_roles.sql` · Alterada en: `009_add_usuarios_dni.sql`
- Consumidores: Todos los servicios (HasPermission/JWT), Gateway

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `id` | uuid |  | No | Sí |  |  |  | gen_random_uuid() | usuarios_pkey | Identificador UUID del registro. | Técnico / catálogo | Generado por la base (DEFAULT/trigger) | Implementado |
| `username` | varchar | 50 | No |  |  | uq_usuarios_username | chk_usuarios_username |  | uq_usuarios_username | Nombre de usuario (único, ≥3 caracteres). | Autenticación | API Auth (Register/CreateUser/login) | Implementado |
| `email` | citext |  | No |  |  | uq_usuarios_email |  |  | uq_usuarios_email | Correo de la cuenta (único, citext). | Contacto | API Auth (Register/CreateUser/login) | Implementado |
| `password_hash` | text |  | No |  |  |  |  |  |  | Hash de la contraseña (nunca texto plano). | Autenticación | API Auth (Register/CreateUser/login) | Implementado |
| `estado` | bool |  | No |  |  |  |  | true | idx_usuarios_estado | Cuenta activa/inactiva. | Autenticación | API Auth (Register/CreateUser/login) | Implementado |
| `email_verified` | bool |  | No |  |  |  |  | false |  | Correo verificado. | Autenticación | API Auth (Register/CreateUser/login) | Implementado |
| `email_verified_at` | timestamptz |  | Sí |  |  |  |  |  |  | Fecha de verificación del correo. | Autenticación | API Auth (Register/CreateUser/login) | Implementado |
| `last_login_at` | timestamptz |  | Sí |  |  |  |  |  | idx_usuarios_last_login_at | Último inicio de sesión. | Auditoría e historial | API Auth (Register/CreateUser/login) | Implementado |
| `failed_login_attempts` | int4 |  | No |  |  |  | chk_usuarios_failed_login_attempts | 0 |  | Intentos fallidos consecutivos (bloqueo). | Autenticación | API Auth (Register/CreateUser/login) | Implementado |
| `locked_until` | timestamptz |  | Sí |  |  |  |  |  | idx_usuarios_locked_until | Bloqueo temporal de la cuenta hasta. | Autenticación | API Auth (Register/CreateUser/login) | Implementado |
| `created_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de creación. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |
| `updated_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de la última actualización. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |
| `dni` | varchar | 8 | Sí |  |  | uq_usuarios_dni (parcial) | chk_usuarios_dni |  | uq_usuarios_dni | DNI usado como identificador de login (8 dígitos, único si existe; NULL = no puede iniciar sesión). | Identificación | API Auth (Register/CreateUser/login) | Implementado |

## Usuarios — `usuarios_db` (esquema `public`)

### `usuarios`

Perfil de negocio del usuario; id = mismo UUID de la cuenta en Auth (sin FK entre bases).

- Filas (conteo verificado): **7** · Propietario: `usuarios_owner` · Creada en: `001_create_usuarios.sql` · Alterada en: `002_add_psicologo_tipo_usuario.sql`
- Consumidores: Gateway (Configuración > Usuarios, perfil), Psicología (responsable), frontend

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `id` | uuid |  | No | Sí |  |  |  |  | usuarios_pkey | Mismo UUID de la cuenta en Auth (sin FK entre bases; sin DEFAULT). | Identificación | API Usuarios (UpsertUsuario) | Implementado |
| `nombres` | varchar | 100 | No |  |  |  | chk_usuarios_nombres |  |  | Nombres del usuario. | Identificación | API Usuarios (UpsertUsuario) | Implementado |
| `apellidos` | varchar | 100 | No |  |  |  | chk_usuarios_apellidos |  | idx_usuarios_apellidos | Apellidos del usuario. | Identificación | API Usuarios (UpsertUsuario) | Implementado |
| `dni` | varchar | 20 | No |  |  | uq_usuarios_dni (parcial) |  | ''::character varying | uq_usuarios_dni | DNI ('' = no capturado; único si no está vacío). Duplica auth.usuarios.dni. | Identificación | API Usuarios (UpsertUsuario) | Implementado |
| `telefono` | varchar | 20 | No |  |  |  |  | ''::character varying |  | Teléfono ('' = no capturado). | Contacto | API Usuarios (UpsertUsuario) | Implementado |
| `correo` | citext |  | No |  |  |  |  | ''::citext |  | Correo ('' = no capturado). Puede diferir de auth.usuarios.email. | Contacto | API Usuarios (UpsertUsuario) | Implementado |
| `direccion` | text |  | No |  |  |  |  | ''::text |  | Dirección ('' = no capturada). | Contacto | API Usuarios (UpsertUsuario) | Implementado |
| `tipo_usuario` | varchar | 20 | No |  |  |  | chk_usuarios_tipo |  | idx_usuarios_tipo_usuario | Copia DENORMALIZADA del rol de Auth, sincronizada por el Gateway (UpdateUserRole). | Información administrativa | API Usuarios (UpsertUsuario) | Implementado |
| `activo` | bool |  | No |  |  |  |  | true |  | Perfil activo. | Información administrativa | API Usuarios (UpsertUsuario) | Implementado |
| `created_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de creación. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |
| `updated_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de la última actualización. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |

## Expedientes (Mesa de Partes / FUT) — `expedientes_db` (esquema `public`)

### `expediente_codigo_counters`

Contador anual para el código EXP-<año>-<n>.

- Filas (conteo verificado): **1** · Propietario: `expedientes_owner` · Creada en: `001_create_expedientes.sql`
- Consumidores: Gateway, Documentos y Derivaciones (validación por gRPC), frontend Mesa de Partes/FUT

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `anio` | int4 |  | No | Sí |  |  |  |  | expediente_codigo_counters_pkey | Año del contador (PK). | Técnico / catálogo | API Expedientes / FUT Digital | Implementado |
| `ultimo` | int4 |  | No |  |  |  |  | 0 |  | Último consecutivo emitido. | Técnico / catálogo | API Expedientes / FUT Digital | Implementado |

### `expedientes`

Expedientes de Mesa de Partes (incluye los creados por el FUT Digital).

- Filas (conteo verificado): **37** · Propietario: `expedientes_owner` · Creada en: `001_create_expedientes.sql` · Alterada en: `002_add_area_actual.sql`, `003_add_tipos_tramite.sql`
- Consumidores: Gateway, Documentos y Derivaciones (validación por gRPC), frontend Mesa de Partes/FUT

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `id` | uuid |  | No | Sí |  |  |  | gen_random_uuid() | expedientes_pkey | Identificador UUID del registro. | Técnico / catálogo | Generado por la base (DEFAULT/trigger) | Implementado |
| `codigo` | varchar | 20 | No |  |  | uq_expedientes_codigo |  |  | uq_expedientes_codigo | Código EXP-<año>-<n> (único). | Información administrativa | API Expedientes / FUT Digital | Implementado |
| `tipo` | varchar | 30 | No |  |  |  | chk_expedientes_tipo |  |  | Tipo de trámite (catálogo por CHECK, 9 valores). | Información administrativa | API Expedientes / FUT Digital | Implementado |
| `asunto` | varchar | 255 | No |  |  |  | chk_expedientes_asunto |  |  | Asunto del trámite. | Información administrativa | API Expedientes / FUT Digital | Implementado |
| `descripcion` | text |  | No |  |  |  |  | ''::text |  | Texto libre. En los expedientes del FUT contiene DATOS DEL SOLICITANTE (nombres, DNI, teléfono, domicilio, distrito, correo), fundamentación y folios, recuperados por expresiones regulares. | Instantánea documental | API Expedientes / FUT Digital | Implementado |
| `solicitante_id` | uuid |  | No |  |  |  |  |  | idx_expedientes_solicitante_id | UUID de la cuenta (Auth) que presentó el trámite. Sin FK entre bases. | Identificación | API Expedientes / FUT Digital | Implementado |
| `estado` | varchar | 20 | No |  |  |  | chk_expedientes_estado | 'PENDIENTE'::character varying | idx_expedientes_estado | Estado del trámite (PENDIENTE, EN_PROCESO, ATENDIDO, OBSERVADO). | Información administrativa | API Expedientes / FUT Digital | Implementado |
| `prioridad` | varchar | 20 | No |  |  |  | chk_expedientes_prioridad | 'NORMAL'::character varying |  | Prioridad (NORMAL, URGENTE). | Información administrativa | API Expedientes / FUT Digital | Implementado |
| `fecha_registro` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP | idx_expedientes_fecha_registro | Fecha de registro. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |
| `fecha_actualizacion` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Última actualización. | Auditoría e historial | API Expedientes / FUT Digital | Implementado |
| `activo` | bool |  | No |  |  |  |  | true |  | Baja lógica del expediente. | Información administrativa | API Expedientes / FUT Digital | Implementado |
| `area_actual` | varchar | 20 | No |  |  |  | chk_expedientes_area_actual | 'SECRETARIA'::character varying | idx_expedientes_area_actual | Área responsable actual (códigos de rol de Auth por CHECK). | Información administrativa | API Expedientes / FUT Digital | Implementado |

## Documentos — `documentos_db` (esquema `public`)

### `documentos`

Archivos adjuntos a expedientes; el contenido binario se guarda en la base (bytea).

- Filas (conteo verificado): **33** · Propietario: `documentos_owner` · Creada en: `001_create_documentos.sql`
- Consumidores: Gateway, frontend (detalle de expediente)

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `id` | uuid |  | No | Sí |  |  |  | gen_random_uuid() | documentos_pkey | Identificador UUID del registro. | Técnico / catálogo | Generado por la base (DEFAULT/trigger) | Implementado |
| `expediente_id` | uuid |  | No |  |  |  |  |  | idx_documentos_expediente_id | UUID del expediente (sin FK entre bases). | Información administrativa | API Documentos (carga de archivos) | Implementado |
| `nombre` | varchar | 255 | No |  |  |  | chk_documentos_nombre |  |  | Nombre del archivo. | Información administrativa | API Documentos (carga de archivos) | Implementado |
| `tipo_documento` | varchar | 20 | No |  |  |  | chk_documentos_tipo |  |  | Clase de documento (ADJUNTO, PROVEIDO, ACTA, INFORME, FORMATO, OTRO). | Información administrativa | API Documentos (carga de archivos) | Implementado |
| `extension` | varchar | 10 | No |  |  |  | chk_documentos_extension |  |  | Extensión permitida (pdf, jpg, jpeg, png, doc, docx). | Técnico / catálogo | API Documentos (carga de archivos) | Implementado |
| `tamano_bytes` | int8 |  | No |  |  |  | chk_documentos_tamano |  |  | Tamaño del archivo. | Técnico / catálogo | API Documentos (carga de archivos) | Implementado |
| `contenido` | bytea |  | No |  |  |  |  |  |  | Contenido binario del archivo (puede contener documentos de identidad escaneados). | Instantánea documental | API Documentos (carga de archivos) | Implementado |
| `subido_por` | uuid |  | No |  |  |  |  |  |  | UUID de la cuenta que subió el archivo (sin FK). | Auditoría e historial | API Documentos (carga de archivos) | Implementado |
| `estado` | varchar | 20 | No |  |  |  | chk_documentos_estado | 'ACTIVO'::character varying | idx_documentos_estado | ACTIVO / ELIMINADO (baja lógica). | Información administrativa | API Documentos (carga de archivos) | Implementado |
| `fecha_registro` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha de carga. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |

## Derivaciones — `derivaciones_db` (esquema `public`)

### `derivaciones`

Movimientos de un expediente entre áreas (derivación, notificación, etc.).

- Filas (conteo verificado): **35** · Propietario: `derivaciones_owner` · Creada en: `001_create_derivaciones.sql`
- Consumidores: Gateway, frontend (seguimiento/derivados)

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `id` | uuid |  | No | Sí |  |  |  | gen_random_uuid() | derivaciones_pkey | Identificador UUID del registro. | Técnico / catálogo | Generado por la base (DEFAULT/trigger) | Implementado |
| `expediente_id` | uuid |  | No |  |  |  |  |  | idx_derivaciones_expediente_id | UUID del expediente (sin FK entre bases). | Información administrativa | API Derivaciones/Expedientes | Implementado |
| `tipo` | varchar | 20 | No |  |  |  | chk_derivaciones_tipo |  | idx_derivaciones_tipo | DERIVACION, NOTIFICACION, ASIGNACION, RECEPCION, APROBACION. | Información administrativa | API Derivaciones/Expedientes | Implementado |
| `origen` | varchar | 100 | No |  |  |  | chk_derivaciones_origen |  |  | Origen en TEXTO LIBRE (área/rol o 'Sistema'). | Información administrativa | API Derivaciones/Expedientes | Implementado |
| `destino` | varchar | 100 | No |  |  |  | chk_derivaciones_destino |  |  | Destino en TEXTO LIBRE. | Información administrativa | API Derivaciones/Expedientes | Implementado |
| `motivo` | varchar | 500 | No |  |  |  | chk_derivaciones_motivo |  |  | Motivo de la derivación. | Información administrativa | API Derivaciones/Expedientes | Implementado |
| `condicion` | varchar | 255 | No |  |  |  |  | ''::character varying |  | Pendiente de definición (texto libre, DEFAULT ''). | Información administrativa | API Derivaciones/Expedientes | Implementado |
| `registrado_por` | uuid |  | No |  |  |  |  |  |  | UUID de la cuenta que registró el movimiento (sin FK). | Auditoría e historial | API Derivaciones/Expedientes | Implementado |
| `fecha_registro` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha del movimiento. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |

## Psicología — `psicologia_db` (esquema `public`)

### `atencion_numero_counters`

Contador de número de atención por expediente psicológico.

- Filas (conteo verificado): **1** · Propietario: `psicologia_owner` · Creada en: `002_create_atenciones.sql`
- Consumidores: Gateway, frontend Psicología

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `expediente_id` | uuid |  | No | Sí | expedientes_psicologicos(id) |  |  |  | atencion_numero_counters_pkey | Expediente psicológico (PK/FK). | Técnico / catálogo | API Psicología | Implementado |
| `ultimo` | int4 |  | No |  |  |  | chk_atencion_numero_counters_ultimo | 0 |  | Último número emitido. | Técnico / catálogo | API Psicología | Implementado |

### `atencion_versiones`

Versiones inmutables de cada atención; contenido clínico CIFRADO.

- Filas (conteo verificado): **5** · Propietario: `psicologia_owner` · Creada en: `002_create_atenciones.sql` · Alterada en: `003_cifrado_contenido.sql`, `004_eliminar_texto_plano.sql`
- Consumidores: Gateway, frontend Psicología

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `id` | uuid |  | No | Sí |  |  |  | gen_random_uuid() | atencion_versiones_pkey | Identificador UUID del registro. | Técnico / catálogo | Generado por la base (DEFAULT/trigger) | Implementado |
| `atencion_id` | uuid |  | No |  | atenciones_psicologicas(id) | uq_atencion_versiones_numero |  |  | idx_atencion_versiones_atencion, uq_atencion_versiones_numero | Atención (FK). | Datos sensibles de salud (clínicos) | API Psicología | Implementado |
| `numero_version` | int4 |  | No |  |  | uq_atencion_versiones_numero | chk_atencion_versiones_correccion, chk_atencion_versiones_fecha_hora, chk_atencion_versiones_numero |  | idx_atencion_versiones_atencion, uq_atencion_versiones_numero | Número de versión. | Auditoría e historial | API Psicología | Implementado |
| `version_anterior_id` | uuid |  | Sí |  | atencion_versiones(id) |  | chk_atencion_versiones_correccion |  |  | Versión anterior (FK a sí misma). | Auditoría e historial | API Psicología | Implementado |
| `fecha_atencion` | date |  | Sí |  |  |  | chk_atencion_versiones_fecha_hora |  |  | Fecha de la atención. | Datos sensibles de salud (clínicos) | API Psicología | Implementado |
| `hora_atencion` | time |  | Sí |  |  |  | chk_atencion_versiones_fecha_hora |  |  | Hora de la atención. | Datos sensibles de salud (clínicos) | API Psicología | Implementado |
| `realizada_por` | _text |  | No |  |  |  | chk_atencion_versiones_realizada_por | '{}'::text[] |  | Quién realizó la atención (códigos). | Datos sensibles de salud (clínicos) | API Psicología | Implementado |
| `motivos` | jsonb |  | No |  |  |  | chk_atencion_versiones_motivos | '[]'::jsonb |  | Motivos codificados (jsonb, catálogo). | Datos sensibles de salud (clínicos) | API Psicología | Implementado |
| `derivaciones` | jsonb |  | No |  |  |  | chk_atencion_versiones_derivaciones | '[]'::jsonb |  | Derivaciones codificadas (jsonb, catálogo). | Datos sensibles de salud (clínicos) | API Psicología | Implementado |
| `creada_por` | uuid |  | No |  |  |  |  |  |  | UUID del autor de la versión. | Auditoría e historial | API Psicología | Implementado |
| `creada_en` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha de la versión. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |
| `contenido_cifrado` | bytea |  | No |  |  |  | chk_atencion_versiones_cifrado |  |  | Contenido clínico de texto libre CIFRADO (AES-256-GCM). | Datos sensibles de salud (clínicos) | API Psicología | Implementado |
| `cifrado_key_id` | varchar | 32 | No |  |  |  | chk_atencion_versiones_cifrado |  |  | Identificador de la clave con la que se cifró el contenido. | Técnico / catálogo | API Psicología | Implementado |

### `atenciones_psicologicas`

Hoja de Atención (cabecera, estado y versión vigente).

- Filas (conteo verificado): **4** · Propietario: `psicologia_owner` · Creada en: `002_create_atenciones.sql` · Alterada en: `003_cifrado_contenido.sql`, `004_eliminar_texto_plano.sql`
- Consumidores: Gateway, frontend Psicología

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `id` | uuid |  | No | Sí |  |  |  | gen_random_uuid() | atenciones_psicologicas_pkey | Identificador UUID del registro. | Técnico / catálogo | Generado por la base (DEFAULT/trigger) | Implementado |
| `expediente_id` | uuid |  | No |  | expedientes_psicologicos(id) | uq_atenciones_psicologicas_numero |  |  | idx_atenciones_psicologicas_expediente_estado, uq_atenciones_psicologicas_numero | Expediente psicológico (FK). | Datos sensibles de salud (clínicos) | API Psicología | Implementado |
| `numero` | int4 |  | Sí |  |  | uq_atenciones_psicologicas_numero | chk_atenciones_psicologicas_numero, chk_atenciones_psicologicas_numero_positivo |  | uq_atenciones_psicologicas_numero | Número correlativo de la atención. | Información administrativa | API Psicología | Implementado |
| `estado` | varchar | 20 | No |  |  |  | chk_atenciones_psicologicas_anulacion, chk_atenciones_psicologicas_estado, chk_atenciones_psicologicas_numero, chk_atenciones_psicologicas_registro | 'BORRADOR'::character varying | idx_atenciones_psicologicas_autor_estado, idx_atenciones_psicologicas_expediente_estado | Estado (borrador, registrada, anulada…). | Información administrativa | API Psicología | Implementado |
| `autor_id` | uuid |  | No |  |  |  |  |  | idx_atenciones_psicologicas_autor_estado | UUID del autor (psicólogo). | Auditoría e historial | API Psicología | Implementado |
| `registrada_por` | uuid |  | Sí |  |  |  | chk_atenciones_psicologicas_registro |  |  | UUID de quien registró. | Auditoría e historial | API Psicología | Implementado |
| `registrada_en` | timestamptz |  | Sí |  |  |  | chk_atenciones_psicologicas_registro |  |  | Fecha de registro. | Auditoría e historial | API Psicología | Implementado |
| `version_vigente` | int4 |  | No |  |  |  | chk_atenciones_psicologicas_registro, chk_atenciones_psicologicas_version | 0 |  | Número de la versión vigente. | Auditoría e historial | API Psicología | Implementado |
| `revision` | int4 |  | No |  |  |  | chk_atenciones_psicologicas_revision | 0 |  | Contador de revisión (concurrencia). | Técnico / catálogo | API Psicología | Implementado |
| `anulada_por` | uuid |  | Sí |  |  |  | chk_atenciones_psicologicas_anulacion |  |  | UUID de quien anuló. | Auditoría e historial | API Psicología | Implementado |
| `anulada_en` | timestamptz |  | Sí |  |  |  | chk_atenciones_psicologicas_anulacion |  |  | Fecha de anulación. | Auditoría e historial | API Psicología | Implementado |
| `created_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de creación. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |
| `updated_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de la última actualización. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |
| `motivo_anulacion_cifrado` | bytea |  | Sí |  |  |  | chk_atenciones_psicologicas_anulacion, chk_atenciones_psicologicas_cifrado |  |  | Motivo de anulación CIFRADO. | Datos sensibles de salud (clínicos) | API Psicología | Implementado |
| `cifrado_key_id` | varchar | 32 | Sí |  |  |  | chk_atenciones_psicologicas_cifrado |  |  | Identificador de la clave con la que se cifró el contenido. | Técnico / catálogo | API Psicología | Implementado |

### `auditoria_psicologica`

Bitácora append-only de accesos y cambios clínicos (sin contenido).

- Filas (conteo verificado): **545** · Propietario: `psicologia_owner` · Creada en: `001_create_psicologia_base.sql` · Alterada en: `002_create_atenciones.sql`
- Consumidores: Gateway, frontend Psicología

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `id` | uuid |  | No | Sí |  |  |  | gen_random_uuid() | auditoria_psicologica_pkey | Identificador UUID del registro. | Técnico / catálogo | Generado por la base (DEFAULT/trigger) | Implementado |
| `usuario_id` | uuid |  | No |  |  |  |  |  | idx_auditoria_psicologica_usuario_fecha | UUID del actor. | Auditoría e historial | API Psicología | Implementado |
| `accion` | varchar | 20 | No |  |  |  | chk_auditoria_psicologica_accion |  |  | Acción (CREATE, VIEW, UPDATE, CLOSE, REOPEN, DENIED…). | Auditoría e historial | API Psicología | Implementado |
| `recurso` | varchar | 50 | No |  |  |  | chk_auditoria_psicologica_recurso, chk_auditoria_psicologica_recurso_valido |  | idx_auditoria_psicologica_recurso | Tipo de recurso. | Auditoría e historial | API Psicología | Implementado |
| `recurso_id` | uuid |  | Sí |  |  |  |  |  | idx_auditoria_psicologica_recurso | UUID del recurso. | Auditoría e historial | API Psicología | Implementado |
| `estudiante_id` | uuid |  | Sí |  |  |  |  |  |  | UUID del estudiante afectado. | Auditoría e historial | API Psicología | Implementado |
| `fecha_hora` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP | idx_auditoria_psicologica_fecha, idx_auditoria_psicologica_usuario_fecha | Fecha y hora del evento. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |
| `resultado` | varchar | 20 | No |  |  |  | chk_auditoria_psicologica_resultado |  |  | PERMITIDO / DENEGADO. | Auditoría e historial | API Psicología | Implementado |
| `motivo` | varchar | 100 | Sí |  |  |  |  |  |  | Código corto del sistema (no texto del usuario). | Auditoría e historial | API Psicología | Implementado |

### `catalogo_destinos_derivacion`

Catálogo de destinos de derivación psicológica.

- Filas (conteo verificado): **6** · Propietario: `psicologia_owner` · Creada en: `002_create_atenciones.sql`
- Consumidores: Gateway, frontend Psicología

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `codigo` | varchar | 40 | No | Sí |  |  | chk_catalogo_destinos_codigo |  | catalogo_destinos_derivacion_pkey | Código del catálogo. | Técnico / catálogo | Semilla de migración / administración | Implementado |
| `etiqueta` | varchar | 100 | No |  |  |  | chk_catalogo_destinos_etiqueta |  |  | Etiqueta visible. | Técnico / catálogo | Semilla de migración / administración | Implementado |
| `activo` | bool |  | No |  |  |  |  | true |  | Registro activo. | Técnico / catálogo | Semilla de migración / administración | Implementado |
| `orden` | int4 |  | No |  |  |  |  | 0 |  | Orden de presentación. | Técnico / catálogo | Semilla de migración / administración | Implementado |

### `catalogo_motivos_atencion`

Catálogo de motivos de atención.

- Filas (conteo verificado): **5** · Propietario: `psicologia_owner` · Creada en: `002_create_atenciones.sql`
- Consumidores: Gateway, frontend Psicología

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `codigo` | varchar | 40 | No | Sí |  |  | chk_catalogo_motivos_codigo |  | catalogo_motivos_atencion_pkey | Código del catálogo. | Técnico / catálogo | Semilla de migración / administración | Implementado |
| `etiqueta` | varchar | 100 | No |  |  |  | chk_catalogo_motivos_etiqueta |  |  | Etiqueta visible. | Técnico / catálogo | Semilla de migración / administración | Implementado |
| `activo` | bool |  | No |  |  |  |  | true |  | Registro activo. | Técnico / catálogo | Semilla de migración / administración | Implementado |
| `orden` | int4 |  | No |  |  |  |  | 0 |  | Orden de presentación. | Técnico / catálogo | Semilla de migración / administración | Implementado |

### `cifrado_canario`

Canario de verificación de la clave de cifrado clínico (no contiene datos de estudiantes).

- Filas (conteo verificado): **1** · Propietario: `psicologia_owner` · Creada en: `003_cifrado_contenido.sql`
- Consumidores: Gateway, frontend Psicología

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `key_id` | varchar | 32 | No | Sí |  |  |  |  | cifrado_canario_pkey | Identificador de la clave. | Técnico / catálogo | API Psicología | Implementado |
| `canario` | bytea |  | No |  |  |  |  |  |  | Valor cifrado de verificación (sin datos personales). | Técnico / catálogo | API Psicología | Implementado |
| `creado_en` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha de creación. | Técnico / catálogo | Generado por la base (DEFAULT/trigger) | Implementado |

### `expediente_codigo_counters`

Contador anual del código de expediente psicológico.

- Filas (conteo verificado): **1** · Propietario: `psicologia_owner` · Creada en: `001_create_psicologia_base.sql`
- Consumidores: Gateway, frontend Psicología

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `anio` | int4 |  | No | Sí |  |  |  |  | expediente_codigo_counters_pkey | Año del contador. | Técnico / catálogo | API Psicología | Implementado |
| `ultimo` | int4 |  | No |  |  |  | chk_expediente_codigo_counters_ultimo | 0 |  | Último consecutivo emitido. | Técnico / catálogo | API Psicología | Implementado |

### `expedientes_psicologicos`

Expediente psicológico de un estudiante (uno por estudiante).

- Filas (conteo verificado): **1** · Propietario: `psicologia_owner` · Creada en: `001_create_psicologia_base.sql` · Alterada en: `003_cifrado_contenido.sql`, `004_eliminar_texto_plano.sql`
- Consumidores: Gateway, frontend Psicología

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `id` | uuid |  | No | Sí |  |  |  | gen_random_uuid() | expedientes_psicologicos_pkey | Identificador UUID del registro. | Técnico / catálogo | Generado por la base (DEFAULT/trigger) | Implementado |
| `estudiante_id` | uuid |  | No |  |  | uq_expedientes_psicologicos_estudiante |  |  | uq_expedientes_psicologicos_estudiante | UUID del estudiante (maestro); validado con Estudiantes.GetEstudiante al abrir; sin FK entre bases. | Identificación | API Psicología | Implementado |
| `codigo` | varchar | 20 | No |  |  | uq_expedientes_psicologicos_codigo | chk_expedientes_psicologicos_codigo |  | uq_expedientes_psicologicos_codigo | Código del expediente psicológico. | Información administrativa | API Psicología | Implementado |
| `estado` | varchar | 20 | No |  |  |  | chk_expedientes_psicologicos_cierre, chk_expedientes_psicologicos_estado | 'ABIERTO'::character varying | idx_expedientes_psicologicos_estado | Estado del expediente (abierto/cerrado). | Información administrativa | API Psicología | Implementado |
| `responsable_id` | uuid |  | No |  |  |  |  |  | idx_expedientes_psicologicos_responsable | UUID del psicólogo responsable (Usuarios). | Información administrativa | API Psicología | Implementado |
| `abierto_por` | uuid |  | No |  |  |  |  |  |  | UUID de quien abrió el expediente. | Auditoría e historial | API Psicología | Implementado |
| `fecha_apertura` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP | idx_expedientes_psicologicos_fecha_apertura | Fecha de apertura. | Auditoría e historial | API Psicología | Implementado |
| `fecha_cierre` | timestamptz |  | Sí |  |  |  | chk_expedientes_psicologicos_cierre |  |  | Fecha de cierre. | Auditoría e historial | API Psicología | Implementado |
| `created_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de creación. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |
| `updated_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de la última actualización. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |
| `resumen_caso_cifrado` | bytea |  | Sí |  |  |  | chk_expedientes_psicologicos_cifrado |  |  | Resumen del caso CIFRADO (AES-256-GCM). | Datos sensibles de salud (clínicos) | API Psicología | Implementado |
| `motivo_cierre_cifrado` | bytea |  | Sí |  |  |  | chk_expedientes_psicologicos_cierre, chk_expedientes_psicologicos_cifrado |  |  | Motivo de cierre CIFRADO. | Datos sensibles de salud (clínicos) | API Psicología | Implementado |
| `cifrado_key_id` | varchar | 32 | Sí |  |  |  | chk_expedientes_psicologicos_cifrado |  |  | Identificador de la clave con la que se cifró el contenido. | Técnico / catálogo | API Psicología | Implementado |

## Estudiantes (maestro) — `estudiantes_db` (esquema `public`)

### `auditoria_estudiantes`

Auditoría append-only de lecturas sensibles y escrituras del maestro (solo nombres de campos, nunca valores).

- Filas (conteo verificado): **0** · Propietario: `estudiantes_owner` · Creada en: `004_auditoria_estudiantes.sql` · Alterada en: `005_auditoria_datos_personales.sql`, `006_domicilio_contacto_y_edad.sql`, `008_familia.sql`
- Consumidores: Gateway, Psicología (GetEstudiante), frontend Estudiantes/Psicología

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `id` | uuid |  | No | Sí |  |  |  | gen_random_uuid() | auditoria_estudiantes_pkey | Identificador UUID del registro. | Técnico / catálogo | Generado por la base (DEFAULT/trigger) | Implementado |
| `usuario_id` | uuid |  | No |  |  |  |  |  | idx_auditoria_estudiantes_usuario_fecha | Actor (del JWT). | Auditoría e historial | API Estudiantes | Implementado |
| `accion` | varchar | 20 | No |  |  |  | chk_auditoria_estudiantes_accion, chk_auditoria_estudiantes_estados |  |  | CREATE, UPDATE, CHANGE_ESTADO, PERSONAL_VIEW/UPDATE, FAMILIA_VIEW/UPDATE. | Auditoría e historial | API Estudiantes | Implementado |
| `estudiante_id` | uuid |  | No |  | estudiantes(id) |  |  |  | idx_auditoria_estudiantes_estudiante_fecha | Estudiante afectado (FK). | Auditoría e historial | API Estudiantes | Implementado |
| `fecha_hora` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP | idx_auditoria_estudiantes_estudiante_fecha, idx_auditoria_estudiantes_usuario_fecha | Fecha y hora. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |
| `version_resultante` | int4 |  | No |  |  |  | chk_auditoria_estudiantes_version |  |  | Versión resultante u observada del estudiante. | Auditoría e historial | API Estudiantes | Implementado |
| `campos` | _text |  | No |  |  |  | chk_auditoria_estudiantes_campos | '{}'::text[] |  | Nombres de campos (lista blanca por CHECK; nunca valores). | Auditoría e historial | API Estudiantes | Implementado |
| `estado_anterior` | varchar | 20 | Sí |  |  |  | chk_auditoria_estudiantes_estados |  |  | Estado anterior (solo CHANGE_ESTADO). | Auditoría e historial | API Estudiantes | Implementado |
| `estado_nuevo` | varchar | 20 | Sí |  |  |  | chk_auditoria_estudiantes_estados |  |  | Estado nuevo (solo CHANGE_ESTADO). | Auditoría e historial | API Estudiantes | Implementado |

### `cat_parentescos`

Catálogo de parentescos aprobado en E-02.

- Filas (conteo verificado): **7** · Propietario: `estudiantes_owner` · Creada en: `008_familia.sql`
- Consumidores: Gateway, Psicología (GetEstudiante), frontend Estudiantes/Psicología

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `codigo` | varchar | 20 | No | Sí |  |  | chk_cat_parentescos_codigo |  | cat_parentescos_pkey | Código del parentesco. | Técnico / catálogo | Semilla de migración / administración | Implementado |
| `nombre` | varchar | 60 | No |  |  |  |  |  |  | Nombre visible. | Técnico / catálogo | Semilla de migración / administración | Implementado |
| `activo` | bool |  | No |  |  |  |  | true |  | Registro activo. | Técnico / catálogo | Semilla de migración / administración | Implementado |
| `orden` | int2 |  | No |  |  |  |  | 0 |  | Orden de presentación. | Técnico / catálogo | Semilla de migración / administración | Implementado |

### `cat_tipos_documento`

Catálogo de tipos de documento (DNI, CE, PASAPORTE, SIN_DOCUMENTO).

- Filas (conteo verificado): **4** · Propietario: `estudiantes_owner` · Creada en: `002_identidad_y_catalogos_base.sql`
- Consumidores: Gateway, Psicología (GetEstudiante), frontend Estudiantes/Psicología

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `codigo` | varchar | 20 | No | Sí |  |  | chk_cat_tipos_documento_codigo |  | cat_tipos_documento_pkey | Código del tipo. | Técnico / catálogo | Semilla de migración / administración | Implementado |
| `nombre` | varchar | 60 | No |  |  |  |  |  |  | Nombre visible. | Técnico / catálogo | Semilla de migración / administración | Implementado |
| `activo` | bool |  | No |  |  |  |  | true |  | Registro activo. | Técnico / catálogo | Semilla de migración / administración | Implementado |
| `orden` | int2 |  | No |  |  |  |  | 0 |  | Orden de presentación. | Técnico / catálogo | Semilla de migración / administración | Implementado |

### `estudiante_familiares`

Relación estudiante-persona con parentesco, apoderado y vigencia. Sin filas.

- Filas (conteo verificado): **0** · Propietario: `estudiantes_owner` · Creada en: `008_familia.sql`
- Consumidores: Gateway, Psicología (GetEstudiante), frontend Estudiantes/Psicología

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `id` | uuid |  | No | Sí |  |  |  | gen_random_uuid() | estudiante_familiares_pkey | Identificador UUID del registro. | Técnico / catálogo | Generado por la base (DEFAULT/trigger) | Implementado |
| `estudiante_id` | uuid |  | No |  | estudiantes(id) | uq_estudiante_familiares_apoderado (parcial), uq_estudiante_familiares_progenitor (parcial), uq_estudiante_familiares_vigente (parcial) |  |  | uq_estudiante_familiares_apoderado, uq_estudiante_familiares_progenitor, uq_estudiante_familiares_vigente | Estudiante (FK). | Familiares y apoderados | API Estudiantes | Implementado |
| `persona_id` | uuid |  | No |  | personas(id) | uq_estudiante_familiares_vigente (parcial) |  |  | idx_estudiante_familiares_persona, uq_estudiante_familiares_vigente | Persona (FK). | Familiares y apoderados | API Estudiantes | Implementado |
| `parentesco` | varchar | 20 | No |  | cat_parentescos(codigo) | uq_estudiante_familiares_progenitor (parcial) |  |  | uq_estudiante_familiares_progenitor | Parentesco (FK catálogo). | Familiares y apoderados | API Estudiantes | Implementado |
| `es_apoderado` | bool |  | No |  |  |  |  | false |  | Rol de apoderado (uno vigente por estudiante). | Familiares y apoderados | API Estudiantes | Implementado |
| `vigente` | bool |  | No |  |  |  | chk_estudiante_familiares_vigencia | true |  | Relación vigente. | Familiares y apoderados | API Estudiantes | Implementado |
| `desde` | date |  | Sí |  |  |  | chk_estudiante_familiares_fechas |  |  | Inicio de la relación. | Familiares y apoderados | API Estudiantes | Implementado |
| `hasta` | date |  | Sí |  |  |  | chk_estudiante_familiares_fechas, chk_estudiante_familiares_vigencia |  |  | Fin de la relación (obligatorio si no vigente). | Familiares y apoderados | API Estudiantes | Implementado |
| `created_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de creación. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |
| `updated_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de la última actualización. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |

### `estudiantes`

Maestro de estudiantes: identidad, datos personales, domicilio/contacto y columnas académicas HEREDADAS (en transición a matrículas).

- Filas (conteo verificado): **1** · Propietario: `estudiantes_owner` · Creada en: `001_create_estudiantes_base.sql` · Alterada en: `002_identidad_y_catalogos_base.sql`, `003_busqueda_normalizada.sql`, `006_domicilio_contacto_y_edad.sql`
- Consumidores: Gateway, Psicología (GetEstudiante), frontend Estudiantes/Psicología

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `id` | uuid |  | No | Sí |  |  |  | gen_random_uuid() | estudiantes_pkey | Identificador UUID del registro. | Técnico / catálogo | Generado por la base (DEFAULT/trigger) | Implementado |
| `codigo` | varchar | 20 | No |  |  | uq_estudiantes_codigo | chk_estudiantes_codigo |  | uq_estudiantes_codigo | Código institucional (único; lo envía el cliente). | Identificación | API Estudiantes | Implementado |
| `nombres` | varchar | 100 | No |  |  |  | chk_estudiantes_nombres |  |  | Nombres. | Identificación | API Estudiantes | Implementado |
| `apellidos` | varchar | 100 | No |  |  |  | chk_estudiantes_apellidos, chk_estudiantes_apellidos_separados |  | idx_estudiantes_apellidos | Apellidos completos; derivados de paterno+materno cuando existen. | Identificación | API Estudiantes | Implementado |
| `tipo_documento` | varchar | 20 | No |  | cat_tipos_documento(codigo) | uq_estudiantes_documento (parcial) | chk_estudiantes_documento | 'SIN_DOCUMENTO'::character varying | uq_estudiantes_documento | Tipo de documento (FK catálogo). | Identificación | API Estudiantes | Implementado |
| `numero_documento` | varchar | 30 | Sí |  |  | uq_estudiantes_documento (parcial) | chk_estudiantes_documento |  | uq_estudiantes_documento | Número de documento (NULL si SIN_DOCUMENTO; único por tipo). | Identificación | API Estudiantes | Implementado |
| `fecha_nacimiento` | date |  | Sí |  |  |  |  |  |  | Fecha de nacimiento (la edad se calcula, no se guarda). | Datos personales | API Estudiantes | Implementado |
| `nivel` | varchar | 20 | No |  |  |  |  | ''::character varying | idx_estudiantes_nivel_grado_seccion | HEREDADO: nivel ('' = sin dato). En transición a secciones/matriculas. | Datos académicos | API Estudiantes | Implementado |
| `grado` | varchar | 20 | No |  |  |  |  | ''::character varying | idx_estudiantes_nivel_grado_seccion | HEREDADO: grado ('' = sin dato). | Datos académicos | API Estudiantes | Implementado |
| `seccion` | varchar | 10 | No |  |  |  |  | ''::character varying | idx_estudiantes_nivel_grado_seccion | HEREDADO: sección ('' = sin dato). | Datos académicos | API Estudiantes | Implementado |
| `turno` | varchar | 20 | No |  |  |  |  | ''::character varying |  | HEREDADO: turno ('' = sin dato). | Datos académicos | API Estudiantes | Implementado |
| `estado_matricula` | varchar | 20 | No |  |  |  | chk_estudiantes_estado_matricula | 'ACTIVO'::character varying | idx_estudiantes_estado_matricula | HEREDADO: ACTIVO/RETIRADO. Distinto de matriculas.estado_matricula (D-M4). | Datos académicos | API Estudiantes | Implementado |
| `created_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de creación. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |
| `updated_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de la última actualización. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |
| `codigo_siagie` | varchar | 14 | Sí |  |  | uq_estudiantes_codigo_siagie (parcial) | chk_estudiantes_codigo_siagie |  | uq_estudiantes_codigo_siagie | Código SIAGIE (14 dígitos, formato provisional; único si existe). | Identificación | API Estudiantes | Implementado |
| `apellido_paterno` | varchar | 60 | Sí |  |  |  | chk_estudiantes_apellidos_separados |  |  | Apellido paterno. | Identificación | API Estudiantes | Implementado |
| `apellido_materno` | varchar | 60 | Sí |  |  |  | chk_estudiantes_apellidos_separados |  |  | Apellido materno. | Identificación | API Estudiantes | Implementado |
| `sexo` | bpchar | 1 | Sí |  |  |  | chk_estudiantes_sexo |  |  | M / F. | Datos personales | API Estudiantes | Implementado |
| `pais_nacimiento` | bpchar | 2 | Sí |  |  |  | chk_estudiantes_pais_nacimiento, chk_estudiantes_ubigeo_nacimiento |  |  | País de nacimiento (ISO alfa-2, sin catálogo). | Datos personales | API Estudiantes | Implementado |
| `ubigeo_nacimiento` | bpchar | 6 | Sí |  |  |  | chk_estudiantes_ubigeo_nacimiento |  |  | Ubigeo de nacimiento (6 dígitos, solo PE). | Datos personales | API Estudiantes | Implementado |
| `lengua_materna` | varchar | 10 | Sí |  |  |  | chk_estudiantes_lenguas |  |  | Lengua materna (código, sin catálogo). | Datos personales | API Estudiantes | Implementado |
| `segunda_lengua` | varchar | 10 | Sí |  |  |  | chk_estudiantes_lenguas |  |  | Segunda lengua (código). | Datos personales | API Estudiantes | Implementado |
| `foto_ref` | varchar | 200 | Sí |  |  |  | chk_estudiantes_foto_ref |  |  | Referencia interna a la foto (sin almacenamiento implementado). | Datos personales | API Estudiantes | Implementado |
| `estado` | varchar | 20 | No |  |  |  | chk_estudiantes_estado | 'ACTIVO'::character varying |  | Estado del REGISTRO (ACTIVO/INACTIVO). | Información administrativa | API Estudiantes | Implementado |
| `version` | int4 |  | No |  |  |  | chk_estudiantes_version | 1 |  | Versión para concurrencia optimista (la incrementa un trigger). | Técnico / catálogo | Generado por la base (DEFAULT/trigger) | Implementado |
| `texto_busqueda` | text |  | Sí |  |  |  |  | GENERATED ALWAYS (STORED) | idx_estudiantes_texto_busqueda_trgm | Columna GENERADA normalizada para búsqueda (incluye documento y SIAGIE); no se expone. | Técnico / catálogo | Calculado por la base | Implementado |
| `domicilio_direccion` | varchar | 200 | Sí |  |  |  | chk_estudiantes_domicilio |  |  | Dirección del domicilio vigente. | Contacto | API Estudiantes | Implementado |
| `domicilio_departamento` | varchar | 60 | Sí |  |  |  | chk_estudiantes_domicilio |  |  | Departamento (texto, sin catálogo). | Contacto | API Estudiantes | Implementado |
| `domicilio_provincia` | varchar | 60 | Sí |  |  |  | chk_estudiantes_domicilio |  |  | Provincia (texto). | Contacto | API Estudiantes | Implementado |
| `domicilio_distrito` | varchar | 60 | Sí |  |  |  | chk_estudiantes_domicilio |  |  | Distrito (texto). | Contacto | API Estudiantes | Implementado |
| `domicilio_ubigeo` | bpchar | 6 | Sí |  |  |  | chk_estudiantes_domicilio |  |  | Ubigeo del domicilio (opcional, nunca deducido). | Contacto | API Estudiantes | Implementado |
| `correo` | varchar | 254 | Sí |  |  |  | chk_estudiantes_contacto |  |  | Correo de contacto registrado (titularidad pendiente de decisión). | Contacto | API Estudiantes | Implementado |
| `telefono` | varchar | 20 | Sí |  |  |  | chk_estudiantes_contacto |  |  | Teléfono de contacto registrado. | Contacto | API Estudiantes | Implementado |

### `matricula_movimientos`

Movimientos dentro de una matrícula (append-only). Sin filas.

- Filas (conteo verificado): **0** · Propietario: `estudiantes_owner` · Creada en: `007_matricula.sql`
- Consumidores: Gateway, Psicología (GetEstudiante), frontend Estudiantes/Psicología

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `id` | uuid |  | No | Sí |  |  |  | gen_random_uuid() | matricula_movimientos_pkey | Identificador UUID del registro. | Técnico / catálogo | Generado por la base (DEFAULT/trigger) | Implementado |
| `matricula_id` | uuid |  | No |  | matriculas(id) |  |  |  | idx_matricula_movimientos_matricula | Matrícula (FK). | Datos académicos | API Estudiantes | Implementado |
| `tipo` | varchar | 20 | No |  |  |  | chk_movimientos_tipo |  |  | CAMBIO_SECCION, CAMBIO_TURNO, RETIRO, REINGRESO, TRASLADO_SALIDA. | Datos académicos | API Estudiantes | Implementado |
| `seccion_anterior_id` | uuid |  | Sí |  | secciones(id) |  |  |  |  | Sección anterior (FK). | Datos académicos | API Estudiantes | Implementado |
| `seccion_nueva_id` | uuid |  | Sí |  | secciones(id) |  |  |  |  | Sección nueva (FK). | Datos académicos | API Estudiantes | Implementado |
| `fecha` | date |  | No |  |  |  |  |  |  | Fecha del movimiento. | Datos académicos | API Estudiantes | Implementado |
| `motivo` | varchar | 200 | Sí |  |  |  | chk_movimientos_motivo |  |  | Motivo (opcional). | Datos académicos | API Estudiantes | Implementado |
| `registrado_por` | uuid |  | No |  |  |  |  |  |  | UUID del actor. | Auditoría e historial | API Estudiantes | Implementado |
| `created_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP | idx_matricula_movimientos_matricula | Fecha y hora de creación. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |

### `matriculas`

Matrícula de un estudiante en un periodo (historial por año). Sin filas.

- Filas (conteo verificado): **0** · Propietario: `estudiantes_owner` · Creada en: `007_matricula.sql`
- Consumidores: Gateway, Psicología (GetEstudiante), frontend Estudiantes/Psicología

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `id` | uuid |  | No | Sí |  |  |  | gen_random_uuid() | matriculas_pkey | Identificador UUID del registro. | Técnico / catálogo | Generado por la base (DEFAULT/trigger) | Implementado |
| `estudiante_id` | uuid |  | No |  | estudiantes(id) | uq_matriculas_estudiante_periodo |  |  | uq_matriculas_estudiante_periodo | Estudiante (FK). | Datos académicos | API Estudiantes | Implementado |
| `periodo_id` | uuid |  | No |  | periodos_academicos(id); secciones(id,periodo_id) | uq_matriculas_estudiante_periodo |  |  | idx_matriculas_periodo_seccion, uq_matriculas_estudiante_periodo | Periodo (FK; único con estudiante). | Datos académicos | API Estudiantes | Implementado |
| `seccion_id` | uuid |  | Sí |  | secciones(id,periodo_id) |  |  |  | idx_matriculas_periodo_seccion | Sección del mismo periodo (FK compuesta; NULL = sin asignar). | Datos académicos | API Estudiantes | Implementado |
| `estado_matricula` | varchar | 20 | No |  |  |  | chk_matriculas_estado, chk_matriculas_retiro | 'MATRICULADO'::character varying |  | MATRICULADO, RETIRADO, TRASLADADO, CONCLUIDO (D-M4). | Datos académicos | API Estudiantes | Implementado |
| `fecha_matricula` | date |  | Sí |  |  |  | chk_matriculas_fechas |  |  | Fecha de matrícula. | Datos académicos | API Estudiantes | Implementado |
| `fecha_retiro` | date |  | Sí |  |  |  | chk_matriculas_fechas, chk_matriculas_retiro |  |  | Fecha de retiro/traslado. | Datos académicos | API Estudiantes | Implementado |
| `motivo_retiro` | varchar | 200 | Sí |  |  |  | chk_matriculas_motivo, chk_matriculas_retiro |  |  | Motivo de retiro (texto; no se expone por API). | Datos académicos | API Estudiantes | Implementado |
| `origen` | varchar | 20 | No |  |  |  | chk_matriculas_origen | 'REGISTRO'::character varying |  | REGISTRO / MIGRACION_LEGADO (trazabilidad). | Auditoría e historial | API Estudiantes | Implementado |
| `version` | int4 |  | No |  |  |  | chk_matriculas_version | 1 |  | Versión para concurrencia optimista (la incrementa un trigger). | Técnico / catálogo | Generado por la base (DEFAULT/trigger) | Implementado |
| `created_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de creación. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |
| `updated_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de la última actualización. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |

### `periodos_academicos`

Periodos académicos (años). Sin filas.

- Filas (conteo verificado): **0** · Propietario: `estudiantes_owner` · Creada en: `007_matricula.sql`
- Consumidores: Gateway, Psicología (GetEstudiante), frontend Estudiantes/Psicología

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `id` | uuid |  | No | Sí |  |  |  | gen_random_uuid() | periodos_academicos_pkey | Identificador UUID del registro. | Técnico / catálogo | Generado por la base (DEFAULT/trigger) | Implementado |
| `anio` | int2 |  | No |  |  | uq_periodos_academicos_anio | chk_periodos_academicos_anio |  | uq_periodos_academicos_anio | Año del periodo (único). | Datos académicos | API Estudiantes | Implementado |
| `nombre` | varchar | 60 | No |  |  |  | chk_periodos_academicos_nombre |  |  | Nombre del periodo. | Datos académicos | API Estudiantes | Implementado |
| `fecha_inicio` | date |  | Sí |  |  |  | chk_periodos_academicos_fechas |  |  | Inicio (opcional). | Datos académicos | API Estudiantes | Implementado |
| `fecha_fin` | date |  | Sí |  |  |  | chk_periodos_academicos_fechas |  |  | Fin (opcional). | Datos académicos | API Estudiantes | Implementado |
| `estado` | varchar | 20 | No |  |  |  | chk_periodos_academicos_estado | 'PLANIFICADO'::character varying |  | PLANIFICADO, EN_CURSO, CERRADO. | Datos académicos | API Estudiantes | Implementado |
| `created_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de creación. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |
| `updated_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de la última actualización. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |

### `personas`

Personas relacionadas con estudiantes (padres, tutores). Sin filas.

- Filas (conteo verificado): **0** · Propietario: `estudiantes_owner` · Creada en: `008_familia.sql`
- Consumidores: Gateway, Psicología (GetEstudiante), frontend Estudiantes/Psicología

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `id` | uuid |  | No | Sí |  |  |  | gen_random_uuid() | personas_pkey | Identificador UUID del registro. | Técnico / catálogo | Generado por la base (DEFAULT/trigger) | Implementado |
| `tipo_documento` | varchar | 20 | No |  | cat_tipos_documento(codigo) | uq_personas_documento (parcial) | chk_personas_documento | 'SIN_DOCUMENTO'::character varying | uq_personas_documento | Tipo de documento (FK catálogo). | Familiares y apoderados | API Estudiantes | Implementado |
| `numero_documento` | varchar | 12 | Sí |  |  | uq_personas_documento (parcial) | chk_personas_documento |  | uq_personas_documento | Número de documento (NULL si no tiene; único por tipo). | Familiares y apoderados | API Estudiantes | Implementado |
| `nombres` | varchar | 100 | No |  |  |  | chk_personas_textos |  |  | Nombres. | Familiares y apoderados | API Estudiantes | Implementado |
| `apellido_paterno` | varchar | 60 | Sí |  |  |  | chk_personas_textos |  |  | Apellido paterno. | Familiares y apoderados | API Estudiantes | Implementado |
| `apellido_materno` | varchar | 60 | Sí |  |  |  | chk_personas_textos |  |  | Apellido materno. | Familiares y apoderados | API Estudiantes | Implementado |
| `version` | int4 |  | No |  |  |  | chk_personas_version | 1 |  | Versión para concurrencia optimista (la incrementa un trigger). | Técnico / catálogo | Generado por la base (DEFAULT/trigger) | Implementado |
| `created_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de creación. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |
| `updated_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de la última actualización. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |

### `secciones`

Secciones/aulas de un periodo. Sin filas.

- Filas (conteo verificado): **0** · Propietario: `estudiantes_owner` · Creada en: `007_matricula.sql`
- Consumidores: Gateway, Psicología (GetEstudiante), frontend Estudiantes/Psicología

| Columna | Tipo | Long./prec. | Null | PK | FK → | UNIQUE | CHECK | Default | Índices | Descripción | Sensibilidad | Origen | Estado |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `id` | uuid |  | No | Sí |  | uq_secciones_id_periodo |  | gen_random_uuid() | secciones_pkey, uq_secciones_id_periodo | Identificador UUID del registro. | Técnico / catálogo | Generado por la base (DEFAULT/trigger) | Implementado |
| `periodo_id` | uuid |  | No |  | periodos_academicos(id) | uq_secciones_id_periodo, uq_secciones_periodo_grupo |  |  | uq_secciones_id_periodo, uq_secciones_periodo_grupo | Periodo (FK). | Datos académicos | API Estudiantes | Implementado |
| `nivel` | varchar | 20 | No |  |  | uq_secciones_periodo_grupo | chk_secciones_nivel |  | uq_secciones_periodo_grupo | INICIAL, PRIMARIA, SECUNDARIA. | Datos académicos | API Estudiantes | Implementado |
| `grado` | varchar | 20 | No |  |  | uq_secciones_periodo_grupo | chk_secciones_textos |  | uq_secciones_periodo_grupo | Grado. | Datos académicos | API Estudiantes | Implementado |
| `seccion` | varchar | 10 | No |  |  | uq_secciones_periodo_grupo | chk_secciones_textos |  | uq_secciones_periodo_grupo | Sección. | Datos académicos | API Estudiantes | Implementado |
| `turno` | varchar | 20 | Sí |  |  | uq_secciones_periodo_grupo | chk_secciones_textos |  | uq_secciones_periodo_grupo | Turno (opcional). | Datos académicos | API Estudiantes | Implementado |
| `aula` | varchar | 40 | Sí |  |  |  | chk_secciones_textos |  |  | Aula (opcional). | Datos académicos | API Estudiantes | Implementado |
| `vacantes` | int2 |  | Sí |  |  |  | chk_secciones_vacantes |  |  | Vacantes (opcional). | Datos académicos | API Estudiantes | Implementado |
| `created_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de creación. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |
| `updated_at` | timestamptz |  | No |  |  |  |  | CURRENT_TIMESTAMP |  | Fecha y hora de la última actualización. | Auditoría e historial | Generado por la base (DEFAULT/trigger) | Implementado |
