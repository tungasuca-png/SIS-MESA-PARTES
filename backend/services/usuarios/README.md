# Usuarios Service

Microservicio responsable del **perfil de negocio** de los usuarios de la I.E. Tungasuca:
nombres, apellidos, DNI, teléfono, correo, dirección, tipo de usuario y estado.

Resuelve el problema de que otros módulos (Expedientes, y a futuro Derivaciones o
Seguimiento) solo guardan el UUID del usuario y necesitan mostrar un nombre real.

## Qué NO hace

- No maneja login, contraseñas, JWT ni refresh tokens: eso sigue siendo de **Auth Service**.
- No almacena `password_hash` ni ningún secreto.
- No reemplaza los roles/permisos de Auth. `tipo_usuario` describe la identidad del
  usuario; la **autorización** sigue viviendo en Auth Service.
- No genera UUIDs: el `id` es siempre el mismo que Auth Service asignó al usuario.

## Puerto y base de datos

| Recurso | Valor |
|---|---|
| gRPC | `0.0.0.0:8083` |
| Base de datos | `usuarios_db` (PostgreSQL en `127.0.0.1:5433`) |
| Usuario PostgreSQL | `usuarios_user` |
| Registro etcd | `usuarios.rpc` |

La base es **independiente**: no hay FK ni consultas SQL hacia `auth_db` ni
`expedientes_db`. La relación entre servicios se resuelve por gRPC.

## Variables de entorno

| Variable | Uso |
|---|---|
| `JWT_SECRET` | Obligatoria. Debe ser el mismo secreto que usa Auth Service para firmar los access tokens. Sin ella, todas las llamadas fallan con `Unauthenticated`. |

El resto de la configuración vive en `etc/usuarios.yaml`.

## Cómo ejecutar

```bash
cd backend/services/usuarios
JWT_SECRET="<mismo secreto que Auth>" go run usuarios.go -f etc/usuarios.yaml
```

Migración inicial:

```bash
docker exec -i postgres-auth psql -U usuarios_user -d usuarios_db < migrations/001_create_usuarios.sql
```

## API gRPC

| Método | Uso | Autorización |
|---|---|---|
| `GetUsuario` | Perfil completo | Personal interno, o el propio usuario sobre sí mismo |
| `GetUsuarioBasic` | Nombre + tipo (para resolver un UUID) | Cualquier usuario autenticado; el DNI solo viaja si el llamador está autorizado |
| `GetUsuariosBasic` | Igual que el anterior pero por lote (máx. 100 ids) | Igual que `GetUsuarioBasic` |
| `ListUsuarios` | Listado administrativo | Solo `ADMIN` |
| `SearchUsuarios` | Búsqueda por nombre/apellido/DNI (mín. 3 caracteres) | Solo `ADMIN` |
| `UpsertUsuario` | Crear/actualizar un perfil para un UUID de Auth | Solo `ADMIN` |

Todas exigen un access token válido en la metadata `authorization: Bearer <token>`.

## Endpoints expuestos por el Gateway

| Ruta | Método gRPC |
|---|---|
| `GET /api/usuarios/:id` | `GetUsuario` |
| `GET /api/usuarios/:id/basic` | `GetUsuarioBasic` |
| `POST /api/usuarios/basic-batch` | `GetUsuariosBasic` |
| `GET /api/usuarios` (con `q=` opcional) | `ListUsuarios` / `SearchUsuarios` |

`UpsertUsuario` **no** está expuesto por el Gateway: hoy solo se usa por gRPC
para administración/sembrado.

## Relación con Auth Service

Auth Service es la fuente de verdad de la identidad y la autorización. Usuarios
Service solo valida la firma del JWT que Auth emitió (mismo `JWT_SECRET`) y lee
del claim `sub` el id del usuario y del claim `role` su rol.

`auth_db` contiene una tabla `user_profiles` creada en una etapa anterior que
**ningún código usa**. Este servicio no la lee ni la modifica; queda como
pendiente de limpieza en Auth.

## Relación con Expedientes Service

Expedientes guarda `solicitante_id UUID` y **no** tiene FK hacia este servicio.
Hoy la resolución de nombre la hace el frontend llamando al Gateway
(`/api/usuarios/basic-batch`) con los ids que ya trae en la respuesta de
expedientes; Expedientes Service no fue modificado.

## Cómo probar

```bash
go build ./...
go vet ./...
go test ./...

# Pruebas contra PostgreSQL real:
INTEGRATION_TEST=true go test ./internal/integration/...
```
