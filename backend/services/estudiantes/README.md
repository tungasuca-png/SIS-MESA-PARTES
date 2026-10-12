# Estudiantes Service

Microservicio propietario de la información **básica** de los estudiantes de la
I.E. Tungasuca: identificador, código, nombres/apellidos, nivel/grado/sección/turno
y estado de matrícula.

Nace de la auditoría de dependencia de estudiantes (`docs/` — fase de diseño
previa): ningún otro servicio del sistema tenía una fuente real de estudiantes;
Psicología (y en el futuro Mesa de Partes, flujo F4) solo guardaban un
`estudiante_id` sin verificar su existencia.

## Qué NO hace (esta fase — FASE 5A)

- **No tiene ningún RPC de alta.** La tabla `estudiantes` se crea vacía; no se
  insertan estudiantes reales ni ficticios. Un futuro RPC de creación (o una
  importación institucional) es una fase posterior.
- No guarda apoderados, historial académico, notas, asistencia, cursos, aulas,
  información clínica ni documentos.
- No crea cuentas en Auth Service: los estudiantes no tienen login. No existe
  (ni se agregó) un rol `ESTUDIANTE`.
- No está integrado todavía con el Gateway, con Psicología (`EstudianteVerifier`
  sigue en `nil`) ni con el frontend.
- No exige todavía el permiso `estudiantes.view` (no está registrado en Auth).
  Cada RPC solo exige un JWT válido (`AuthenticationInterceptor`), igual que el
  resto de servicios sin autorización todavía activa. El `AuthClient` ya está
  conectado en `ServiceContext` para que una fase posterior solo agregue
  `authorization.RequirePermission(...)` en cada `logic/`, sin tocar el resto.

## Puerto y base de datos

| Recurso | Valor |
|---|---|
| gRPC | `127.0.0.1:8087` (solo loopback: datos de menores de edad, mismo criterio que Derivaciones/Psicología) |
| Base de datos | `estudiantes_db` (PostgreSQL en `127.0.0.1:5433`) |
| Usuario PostgreSQL | `estudiantes_user` (`SELECT/INSERT/UPDATE`, sin `DELETE`: no hay baja física prevista) |
| Registro etcd | `estudiantes.rpc` |

Base **independiente**, sin FK hacia ninguna otra base del proyecto. El `id` se
autogenera aquí (a diferencia de `usuarios_db`, donde lo asigna Auth): el
estudiante no tiene identidad en Auth Service.

## Variables de entorno

| Variable | Uso |
|---|---|
| `JWT_SECRET` | Obligatoria. Mismo secreto que Auth Service. |
| `ESTUDIANTES_DB_PASSWORD` | Obligatoria. Contraseña de `estudiantes_user` (en `backend/.env`, nunca en el YAML). |

## Cómo ejecutar

```bash
cd backend/services/estudiantes
JWT_SECRET="<mismo secreto que Auth>" ESTUDIANTES_DB_PASSWORD="<contraseña>" \
  go run estudiantes.go -f etc/estudiantes.yaml
```

Migración inicial, con el migrador actuando como dueño (nunca con `estudiantes_user`
ni con un superusuario; ver `backend/db/roles/README.md`):

```bash
(echo "SET ROLE estudiantes_owner;"; cat migrations/001_create_estudiantes_base.sql) | \n  docker exec -i tg-postgres psql -U estudiantes_migrator -d estudiantes_db -v ON_ERROR_STOP=1
```

Pruebas de integración: `INTEGRATION_TEST=true` más `backend/.env` y
`backend/.env.test` (`TEST_ESTUDIANTES_DB_URL`) cargados en el entorno.

## API gRPC

| Método | Uso | Autorización |
|---|---|---|
| `GetEstudiante` | Detalle básico por id | Solo autenticación (por ahora) |
| `SearchEstudiantes` | Búsqueda por código/nombres/apellidos (mín. 2 caracteres), paginada | Solo autenticación (por ahora) |
| `GetEstudiantesBasic` | Resolución por lote (máx. 100 ids), mismo patrón que `Usuarios.GetUsuariosBasic` | Solo autenticación (por ahora) |

`GetEstudiante` deliberadamente **no** expone `tipo_documento`, `numero_documento`
ni `fecha_nacimiento`, aunque la tabla los guarda: ningún consumidor actual los
necesita (ver auditoría de dependencia de estudiantes).

## Cómo probar

```bash
go build ./...
go vet ./...

# Contra PostgreSQL real (crea y elimina sus propios fixtures por SQL directo,
# no hay RPC de alta todavía):
INTEGRATION_TEST=true go test ./internal/integration/...
```

## Pendientes (fases posteriores, fuera de esta)

- **FASE 5B:** registrar `estudiantes.view` en Auth (migración, sin asignar a
  ningún rol) y agregar `authorization.RequirePermission(...)` a cada RPC;
  exponer rutas en el Gateway.
- **FASE 5C:** Psicología implementa `EstudianteVerifier` contra el cliente real.
- **FASE 5D:** selector/buscador real en el frontend, reemplazando el campo de
  UUID manual.
