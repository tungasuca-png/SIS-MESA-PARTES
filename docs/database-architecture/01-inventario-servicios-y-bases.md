# 01 — Inventario de servicios y bases de datos

**Fuentes de evidencia** (2026-09-30):
- Repositorio en la rama `main`, último commit `e0198dc`. El árbol tiene 302 cambios sin versionar, previos a esta auditoría.
- Archivos `etc/*.yaml`, `internal/config`, `internal/svc` y las migraciones de cada servicio.
- Catálogo de PostgreSQL, consultado **solo con lecturas** (`default_transaction_read_only = on`) a través del contenedor `tg-postgres`.

No se muestran valores de secretos: solo los **nombres** de las variables de entorno.

## 1. Servicios detectados

| Servicio | Ruta | Tipo | Escucha | Base | Estado |
|---|---|---|---|---|---|
| Auth | `backend/services/auth` | gRPC (go-zero zrpc) | 127.0.0.1:8080 | `auth_db` | Implementado |
| Usuarios | `backend/services/usuarios` | gRPC | 127.0.0.1:8083 | `usuarios_db` | Implementado |
| Expedientes (Mesa de Partes + FUT) | `backend/services/expedientes` | gRPC | 127.0.0.1:8082 | `expedientes_db` | Implementado |
| Documentos | `backend/services/documentos` | gRPC | 127.0.0.1:8084 | `documentos_db` | Implementado |
| Derivaciones | `backend/services/derivaciones` | gRPC | 127.0.0.1:8085 | `derivaciones_db` | Implementado |
| Psicología | `backend/services/psicologia` | gRPC | 127.0.0.1:8086 | `psicologia_db` | Implementado |
| Estudiantes (maestro: identidad, matrícula, familia) | `backend/services/estudiantes` | gRPC | 127.0.0.1:8087 | `estudiantes_db` | Implementado (matrícula y familia en parte) |
| Gateway | `backend/gateway` | REST (go-zero rest) → gRPC | 127.0.0.1:8888 | **ninguna** | Implementado |
| Frontend | `frontend/` (React + Vite + JS) | SPA | ::1:5173 (desarrollo) | ninguna | Implementado |

Todos los servicios están escritos en **Go con go-zero** y se registran en **etcd** (`127.0.0.1:2379`). No hay consumidores de eventos ni colas.

**Cosas que NO existen como servicio independiente** (verificado):

| Dominio | Situación real |
|---|---|
| Matrículas | Módulo **dentro** de Estudiantes (`estudiantes_db`, migración 007): solo estructura y lectura |
| Personas / familiares | Módulo **dentro** de Estudiantes (migración 008) |
| FUT | Pantalla del frontend (`pages/FUT`) que crea expedientes en **Expedientes** |
| Mesa de Partes | Conjunto de Expedientes, Documentos y Derivaciones más las pantallas `pages/MesaDePartes` y `pages/Expedientes` |
| Egresados | **No existe**: ni código, ni tablas, ni diseño previo en el repositorio |
| Google OAuth / OIDC | **No existe**: las únicas coincidencias de "google" en el código son imports de `google.golang.org/grpc` |

## 2. Configuración y conexiones (sin valores)

| Servicio | Archivo | Usuario de BD (app) | Variables de entorno | Llama por gRPC a |
|---|---|---|---|---|
| Auth | `etc/auth.yaml` | `auth_app` | `AUTH_DB_PASSWORD`, `JWT_SECRET` | — |
| Usuarios | `etc/usuarios.yaml` | `usuarios_user` | `USUARIOS_DB_PASSWORD`, `JWT_SECRET` | Auth |
| Expedientes | `etc/expedientes.yaml` | `expedientes_user` | `EXPEDIENTES_DB_PASSWORD`, `JWT_SECRET` | Auth |
| Documentos | `etc/documentos.yaml` | `documentos_user` | `DOCUMENTOS_DB_PASSWORD`, `JWT_SECRET` | Auth, Expedientes |
| Derivaciones | `etc/derivaciones.yaml` | `derivaciones_user` | `DERIVACIONES_DB_PASSWORD`, `JWT_SECRET` | Auth, Expedientes |
| Psicología | `etc/psicologia.yaml` | `psicologia_user` | `PSICOLOGIA_DB_PASSWORD`, `JWT_SECRET`, `CLINICAL_DATA_ENCRYPTION_KEY`, `CLINICAL_DATA_KEY_ID`, `CLINICAL_DATA_PREVIOUS_KEYS` | Auth, Usuarios, Estudiantes |
| Estudiantes | `etc/estudiantes.yaml` | `estudiantes_user` | `ESTUDIANTES_DB_PASSWORD`, `JWT_SECRET` | Auth |
| Gateway | `etc/gateway-api.yaml` | — | — | Los 7 servicios |

- **Conexión:** todos usan PostgreSQL en `127.0.0.1:5433` con `SSLMode: disable`. Es aceptable en desarrollo (loopback); en producción hace falta TLS (`docs/seguridad-infraestructura.md` §2).
- **Secretos:** los YAML solo tienen referencias `${…}`. Los valores viven en `backend/.env` y `backend/.env.test`, ambos ignorados por git.
- **Pruebas:** usan las variables `TEST_<SERVICIO>_DB_URL` de `backend/.env.test`.
- **Infraestructura** (`backend/docker-compose.dev.yml`): `postgres` (volumen `pgdata`), `etcd` y `adminer`, este último solo con `--profile tools`.
  - **Observación:** Adminer estaba corriendo durante la auditoría. Su puerto 8080 no está publicado al host, pero la documentación indica levantarlo solo bajo demanda.

## 3. Bases de datos (verificadas)

Motor: **PostgreSQL 17.11** (Debian), un solo servidor, **una base por servicio**. Todas usan únicamente el esquema `public`.

| Base | Propietario | Tablas | Columnas | Restricciones | Índices | Triggers | Funciones propias | Migraciones |
|---|---|---|---|---|---|---|---|---|
| `auth_db` | `auth_owner` | 7 | 49 | 34 | 24 | 0 | 0 | 000–017 (18 archivos) |
| `usuarios_db` | `usuarios_owner` | 1 | 11 | 4 | 4 | 0 | 0 | 001–002 |
| `expedientes_db` | `expedientes_owner` | 2 | 14 | 8 | 7 | 0 | 0 | 001–003 |
| `documentos_db` | `documentos_owner` | 1 | 10 | 6 | 3 | 0 | 0 | 001 |
| `derivaciones_db` | `derivaciones_owner` | 1 | 9 | 5 | 3 | 0 | 0 | 001 |
| `psicologia_db` | `psicologia_owner` | 9 | 65 | 46 | 22 | 3 | 2 | 001–004 |
| `estudiantes_db` | `estudiantes_owner` | 10 | 108 | 69 | 30 | 10 | 6 | 001–008 |
| **Total** | | **31** | **266** | 172 | 93 | 13 | 8 | |

No hay vistas, secuencias ni tipos ENUM propios en ninguna base (verificado). La base `postgres` es la de administración (`tg_admin`) y no contiene objetos del proyecto.

### Tablas declaradas frente a verificadas

| Base | Declaradas en migraciones | Verificadas en PostgreSQL | Diferencias |
|---|---|---|---|
| auth_db | permissions, refresh_tokens, role_permissions, roles, user_profiles, usuario_roles, usuarios | las mismas 7 | ninguna |
| usuarios_db | usuarios | usuarios | ninguna |
| expedientes_db | expedientes, expediente_codigo_counters | las mismas | ninguna |
| documentos_db | documentos | documentos | ninguna |
| derivaciones_db | derivaciones | derivaciones | ninguna |
| psicologia_db | 9 tablas | las mismas 9 | ninguna |
| estudiantes_db | 10 tablas | las mismas 10 | ninguna |

- **A nivel de columna:** las 266 columnas verificadas aparecen en las migraciones de su servicio (sin contar los comentarios). Las únicas columnas declaradas que ya **no existen** son las 14 de texto clínico en claro que Psicología eliminó a propósito en `004_eliminar_texto_plano.sql`, sustituidas por contenido cifrado.
- **Estado de aplicación de las migraciones:** **no hay tabla de control** en ninguna base. La aplicación se infiere porque el esquema coincide con lo declarado, no porque exista un registro.

### Volumen (conteo de filas, sin leer datos)

| Base | Tablas con filas |
|---|---|
| auth_db | usuarios 35, roles 8, usuario_roles 33, permissions 40, role_permissions 97, refresh_tokens 384, user_profiles **0** |
| usuarios_db | usuarios 7 |
| expedientes_db | expedientes 37, expediente_codigo_counters 1 |
| documentos_db | documentos 33 |
| derivaciones_db | derivaciones 35 |
| psicologia_db | expedientes_psicologicos 1, atenciones 4, versiones 5, auditoria_psicologica 545, catálogos 5 y 6, contadores 1, canario 1 |
| estudiantes_db | estudiantes 1, cat_tipos_documento 4, cat_parentescos 7; el resto **0** |

Es un entorno de **desarrollo**: los volúmenes corresponden a datos de demostración y a restos de pruebas.

## 4. Roles de PostgreSQL (verificados)

- **Por servicio:** `<s>_owner` (NOLOGIN, dueño de los objetos), `<s>_migrator` (aplica migraciones con `SET ROLE`), `<s>_tester` (solo desarrollo) y el usuario de aplicación (`auth_app` o `<s>_user`).
- **Superusuario:** `tg_admin`, el único.
- **Configuración:** `password_encryption = scram-sha-256`; `ssl = off`.

Privilegios del usuario de aplicación:

| Base | Usuario app | Privilegios |
|---|---|---|
| auth_db | auth_app | usuarios, permissions, refresh_tokens: SELECT/INSERT/UPDATE · role_permissions, usuario_roles: SELECT/INSERT/DELETE · roles: SELECT · **user_profiles: ninguno** |
| usuarios_db | usuarios_user | usuarios: **SELECT/INSERT/UPDATE/DELETE** |
| expedientes_db | expedientes_user | expedientes, contadores: **SELECT/INSERT/UPDATE/DELETE** |
| documentos_db | documentos_user | documentos: **SELECT/INSERT/UPDATE/DELETE** |
| derivaciones_db | derivaciones_user | derivaciones: **SELECT/INSERT/UPDATE/DELETE** |
| psicologia_db | psicologia_user | sin DELETE; auditoría solo SELECT/INSERT; catálogos solo SELECT |
| estudiantes_db | estudiantes_user | sin DELETE; auditoría solo SELECT/INSERT; matrículas y catálogos solo SELECT |

**El código productivo no hace `DELETE` físico en Usuarios, Expedientes, Documentos ni Derivaciones.** Esas bajas son lógicas (`activo`, `estado = 'ELIMINADO'`). El privilegio DELETE sobra (ver [04](04-redundancias-y-problemas.md), P-14).

## 5. Operaciones (RPC) por servicio

| Servicio | Lectura | Escritura |
|---|---|---|
| Auth | Ping, ValidateToken, HasPermission, GetMyPermissions, ListRoles, ListPermissions, GetRolePermissions | Register (público, crea SOLICITANTE), Login, RefreshToken, Logout, UpdateRolePermissions, UpdateUserRole, CreateUser |
| Usuarios | GetUsuario, GetUsuarioBasic, GetUsuariosBasic, ListUsuarios, SearchUsuarios | UpsertUsuario, UpdateMyProfile, UpdateUsuarioRole |
| Expedientes | GetExpediente, ListExpedientes, ValidateDerivacion | CreateExpediente, UpdateExpediente, ChangeEstado, DeleteExpediente (lógica), UpdateArea, DerivarExpediente, RechazarExpediente, CorregirExpediente, ResolverExpediente |
| Documentos | GetDocumento, DownloadDocumento, ListDocumentos | UploadDocumento, DeleteDocumento (lógica) |
| Derivaciones | GetDerivacion, ListDerivaciones | CrearDerivacion |
| Psicología | GetExpediente, GetExpedienteByEstudiante, ListExpedientes, GetAtencion, ListAtenciones, ListAtencionVersions, GetAtencionVersion, ExportAtencionPdf | OpenExpediente, UpdateExpediente, CloseExpediente, ReopenExpediente, CreateAtencionDraft, UpdateAtencionDraft, RegisterAtencion, DiscardAtencionDraft, CorrectAtencion, AnnulAtencion |
| Estudiantes | GetEstudiante, SearchEstudiantes, GetEstudiantesBasic, ResolveEstudiante, ListPosiblesDuplicados, ListMatriculasEstudiante, GetEstudiantePersonal (auditada), ListFamiliaresEstudiante (auditada) | CreateEstudiante, UpdateEstudiante, ChangeEstadoEstudiante, UpdateEstudiantePersonal, VincularFamiliar, DesvincularFamiliar |

## 6. Limitaciones de esta auditoría

- Las reglas de negocio de cada RPC no se reauditaron en detalle: se verificaron el esquema, las relaciones y los puntos de acceso a datos.
- No se leyeron valores de filas: solo conteos y UUID, para comprobar huérfanos. Por eso no se afirma la **causa** de los huérfanos detectados.
- El estado de producción no existe todavía: todo lo verificado corresponde al entorno de desarrollo.
