# API de solo lectura de Estudiantes para sistemas consumidores

Estudiantes es el servicio maestro de estudiantes. Otros sistemas lo consultan
**solo para identificar y seleccionar** estudiantes, con *client credentials*.
Ningún sistema crea, modifica, matricula ni lee datos personales.

## Flujo

```text
Sistema ─► POST /api/auth/token        (client_id + client_secret) ─► token de servicio (5 min)
Sistema ─► POST /api/estudiantes/seleccion/buscar   (Authorization: Bearer <token>)
```

Ambas rutas pasan por el Gateway. Los servicios internos no se publican
(ver [publicacion-gateway.md](../infra/publicacion-gateway.md)).

### 1. Obtener el token

```http
POST /api/auth/token
Content-Type: application/json

{"grant_type": "client_credentials", "client_id": "cli_…", "client_secret": "…"}
```

Respuesta: `access_token`, `token_type` (`Bearer`), `expires_in` (300 s) y `scope`.

- El token es un JWT HS256 firmado con `SERVICE_JWT_SECRET`, distinto de la clave
  de los usuarios. Lleva `iss=tungasuca-auth`, `aud=estudiantes`,
  `token_type=service`, `exp` y el `scope`. Estudiantes valida todos esos campos.
- El sistema debe guardar el token en memoria y renovarlo poco antes de que
  venza. No debe pedir uno por cada consulta.
- **Nunca** se guarda en un frontend: el secreto y el token viven solo en el
  backend del sistema consumidor.

### 2. Buscar estudiantes

```http
POST /api/estudiantes/seleccion/buscar
Authorization: Bearer <token de servicio>
Content-Type: application/json

{"q": "quispe huaman", "page": 1, "page_size": 10}
```

```json
{"estudiantes": [{"id": "…", "nombre_completo": "…", "grado": "3", "seccion": "B"}], "total": 1}
```

Reglas de búsqueda:

- `q` debe tener entre 2 y 100 caracteres, con 5 términos como máximo. Cada
  término debe coincidir (sin tildes ni mayúsculas) con nombres o apellidos.
- Un término de **8 dígitos** busca un DNI exacto, solo entre los documentos de
  tipo DNI. Un número parcial o de otra longitud **no devuelve resultados**. El
  código SIAGIE y el código interno no son criterios de búsqueda.
- Solo devuelve estudiantes con estado `ACTIVO`.
- `page_size` admite hasta 20 (10 por defecto); `page` admite hasta 500.
- La consulta viaja en el cuerpo, **nunca en la URL**. No se registra en los
  logs del Gateway ni de Estudiantes.
- Requiere un token de servicio con el scope `estudiantes.search.basic`. Un
  JWT de usuario se rechaza (`403`), aunque el rol tuviera el permiso.

## Respuestas mínimas en gRPC

Para un sistema sin `estudiantes.view`, `GetEstudiantesBasic` y
`ResolveEstudiante` devuelven solo `id`, `nombre_completo`, `grado` y `seccion`.
Nunca devuelven DNI, SIAGIE, código, nombres o apellidos separados, nivel,
turno ni estados. `ResolveEstudiante` solo acepta `tipo_documento=DNI` con
`numero_documento` de 8 dígitos. Los usuarios conservan la respuesta completa.

## Permisos y scopes

| Permiso | Sistemas nuevos | Uso |
|---|---|---|
| `estudiantes.basic.view` | Sí | `GetEstudiantesBasic`, `ResolveEstudiante` (respuesta mínima) |
| `estudiantes.search.basic` | Sí | `BuscarEstudiantesSeleccion` / `POST /api/estudiantes/seleccion/buscar` |
| `estudiantes.view`, `personal.view`, `create`, `update`, `matricula.update` | **No** | Se rechazan al registrar un sistema |

- Auth solo registra sistemas con los dos permisos de lectura
  (`GrantableServicePermissions`). Otorgar o retirar permisos a un sistema
  existente usa esa misma lista.
- Estudiantes, además, **no expone** a sistemas `CreateEstudiante`,
  `UpdateEstudiante`, `MatricularEstudiante` ni `GetEstudiantePersonal`
  (`interceptor.ServiceMethods`), aunque el token tuviera el permiso. Un
  cliente antiguo que tuviera esos permisos queda en solo lectura sin perder
  sus lecturas.
- Los permisos de los usuarios no cambian.

## Límites de tasa (Gateway)

| Ruta | Límite |
|---|---|
| `POST /api/auth/token` | 10/min por IP (ráfaga 5) |
| `POST /api/estudiantes/seleccion/buscar` | 60/min por credencial (ráfaga 20) y 120/min por IP (ráfaga 30) |
| `POST /api/estudiantes/resolver` | 60/min por credencial (ráfaga 20) |
| `POST /api/estudiantes/basic-batch` | 120/min por credencial (ráfaga 40) |
| `POST /api/psicologia/estudiantes/buscar` | 60/min por credencial (ráfaga 20) |

Al superarlos, el Gateway responde `429` con `Retry-After`. La credencial se
identifica por el hash SHA-256 del encabezado `Authorization`, nunca por el
token en claro. Los límites se cuentan en memoria, por instancia del Gateway.

## Errores

Todas las respuestas de error usan el formato actual del Gateway: texto
`rpc error: code = <Código> desc = <mensaje>`. Se mantiene así porque el
frontend ya lo interpreta.

| HTTP | Caso |
|---|---|
| 400 | Consulta inválida (longitud, términos) |
| 401 | Token ausente, vencido, con firma, `iss`, `aud` o tipo inválidos; credenciales inválidas en `/api/auth/token` |
| 403 | Falta el scope, la operación no está disponible para sistemas o se usó un JWT de usuario |
| 429 | Límite de tasa |
| 5xx | Mensaje genérico, sin detalles internos |

Los mensajes nunca repiten el DNI, el código ni la consulta.
