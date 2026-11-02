# API de Estudiantes – Colegio Tungasuca

- **Tipo de documento:** Documentación técnica de integración
- **Versión del documento:** 1.0
- **Fecha de elaboración:** 30/10/2026
- **Sistema propietario:** SIS-MESA-PARTES

> **Estado de la versión documentada.** Este documento describe el código de la rama
> `feat/psicologia-client-credentials` del 30/10/2026. Esa rama todavía **no está desplegada**.
> Hasta que se despliegue, los servicios en ejecución no tienen la ruta
> `POST /api/estudiantes/seleccion/buscar`, la respuesta mínima ni los límites de tasa.
> Confirme con el equipo de SIS-MESA-PARTES que el entorno asignado ya tiene esta versión.

## 1. Introducción

La API de Estudiantes permite que otros sistemas del Colegio Tungasuca **identifiquen y
seleccionen estudiantes** sin acceder a la base de datos de Estudiantes ni copiarla.

El microservicio de Estudiantes es la fuente oficial (maestra) de los estudiantes. Un
sistema externo que necesita asociar información a un estudiante puede:

- buscarlo por nombres, apellidos o DNI completo;
- obtener su identificador único (`id`), nombre completo, grado y sección;
- volver a consultar esos datos mínimos a partir del `id`.

La API es de **solo lectura**: ningún sistema consumidor puede crear, modificar ni
matricular estudiantes, ni leer sus datos personales.

## 2. Descripción general de la API

### 2.1 Componentes

| Componente | Función |
|---|---|
| Sistema consumidor | Aplicación externa (su backend) que consulta estudiantes. |
| Servicio de autenticación (Auth) | Valida el `client_id` y el `client_secret` del sistema y emite un token de servicio de corta duración. |
| API Gateway | Único punto de entrada HTTP/JSON. Aplica límites de tasa y traduce las solicitudes REST a llamadas internas gRPC. |
| Microservicio de Estudiantes | Valida el token (firma, emisor, audiencia, vigencia, tipo y scope), aplica las reglas de búsqueda y devuelve solo los datos mínimos. |
| Base de datos de Estudiantes | PostgreSQL. No es accesible para sistemas externos. |

### 2.2 Flujo de comunicación

```text
+--------------------+  1. POST /api/auth/token        +-------------+  gRPC  +------------+
| Sistema consumidor | ------------------------------> |             | -----> |    Auth    |
|     (backend)      | <------------------------------ |             | <----- |            |
|                    |     token de servicio (5 min)   |             |        +------------+
|                    |                                 |     API     |
|                    |  2. POST /api/estudiantes/...   |   Gateway   |  gRPC  +------------+
|                    |     Authorization: Bearer <tok> |             | -----> | Estudiantes|
|                    | ------------------------------> |             |        | (valida    |
|                    | <------------------------------ |             | <----- | token y    |
|                    |     JSON con datos mínimos      |             |        | scope)     |
+--------------------+                                 +-------------+        +------------+
                                                                                    |
                                                                                    v
                                                                          +------------------+
                                                                          | BD Estudiantes   |
                                                                          | (PostgreSQL)     |
                                                                          +------------------+
```

1. El sistema consumidor envía sus credenciales al Gateway y recibe un token de servicio.
2. El sistema consulta los endpoints de Estudiantes con ese token en el encabezado
   `Authorization`.
3. El Gateway reenvía la solicitud. Estudiantes valida el token y el scope, y responde.

El Gateway no guarda los tokens. La validación del token la hace el microservicio de
Estudiantes en cada solicitud.

## 3. Requisitos para la integración

### 3.1 Requisitos técnicos

| Requisito | Detalle |
|---|---|
| Acceso de red al Gateway | El sistema debe alcanzar la URL base del Gateway del entorno asignado. |
| URL base del entorno | Ver la sección 3.2. |
| Cliente de integración autorizado | Registrado por un administrador de SIS-MESA-PARTES. |
| Client ID | Identificador público del cliente, con el formato `cli_` seguido de 32 caracteres hexadecimales. |
| Client Secret | Secreto del cliente. Se entrega **una sola vez** al registrarlo. |
| Scopes autorizados | `estudiantes.basic.view` y/o `estudiantes.search.basic`. |
| HTTP y JSON | Solicitudes HTTP/1.1 con cuerpo `application/json` y codificación UTF-8. |
| HTTPS | Obligatorio en producción, cuando el entorno esté publicado (ver la sección 3.2). |
| Backend propio | Las credenciales y los tokens solo pueden usarse desde un servidor, nunca desde un navegador o una aplicación móvil. |

### 3.2 Entornos

| Entorno | URL base | Estado |
|---|---|---|
| Local (desarrollo) | `http://127.0.0.1:8888` | Disponible solo en el equipo donde corre el sistema. El Gateway escucha únicamente en la interfaz local (loopback), por lo que no es accesible desde otros equipos. |
| Pruebas | Por confirmar | No existe un entorno de pruebas publicado. Debe solicitarse al equipo de SIS-MESA-PARTES. |
| Producción | Por confirmar (`https://<dominio-por-definir>`) | **No publicado.** Faltan el dominio, el certificado TLS y el proxy inverso (ver `docs/infra/publicacion-gateway.md`). |

En este documento, `{BASE_URL}` representa la URL base del entorno asignado.

**CORS:** el Gateway solo admite solicitudes de navegador desde el frontend propio de
SIS-MESA-PARTES. Esto no afecta a un backend, que es la única forma de integración
permitida.

## 4. Autenticación

### 4.1 Mecanismo

La API usa el flujo **OAuth 2.0 Client Credentials**. El sistema cambia su `client_id` y su
`client_secret` por un **token de acceso de servicio**. Ese token se envía después como
**Bearer Token** en cada consulta.

Características del token:

| Propiedad | Valor |
|---|---|
| Formato | JWT firmado con HS256. Debe tratarse como un valor opaco: el sistema consumidor no debe decodificarlo ni depender de su contenido. |
| Duración | 300 segundos (5 minutos), según la configuración actual (`ServiceTokenDuration: 5m`). Use siempre el valor de `expires_in` de la respuesta. |
| Emisor / audiencia | `tungasuca-auth` / `estudiantes` (los valida Estudiantes). |
| Tipo | Token de servicio. Es distinto de un token de usuario: no son intercambiables. |
| Scope | Permisos asignados al cliente, separados por espacios. |
| Renovación | No hay token de renovación (refresh token). Antes de que venza, se solicita un token nuevo con las mismas credenciales. |

### 4.2 Endpoint de token

| Campo | Valor |
|---|---|
| Método | `POST` |
| Ruta | `{BASE_URL}/api/auth/token` |
| Content-Type | `application/json` |
| Autenticación | Ninguna (las credenciales van en el cuerpo) |
| Límite de tasa | 10 solicitudes por minuto por dirección IP (ráfaga de 5) |

**Parámetros del cuerpo**

| Campo | Tipo | Obligatorio | Descripción |
|---|---|---|---|
| `grant_type` | string | Sí | Debe ser exactamente `client_credentials`. |
| `client_id` | string | Sí | Client ID del sistema. |
| `client_secret` | string | Sí | Client Secret del sistema. |

**Ejemplo de solicitud (cURL)**

```bash
curl -X POST "{BASE_URL}/api/auth/token" \
  -H "Content-Type: application/json" \
  -d '{
        "grant_type": "client_credentials",
        "client_id": "cli_0123456789abcdef0123456789abcdef",
        "client_secret": "SECRETO_DE_EJEMPLO_NO_REAL"
      }'
```

**Ejemplo de respuesta (200)**

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.EJEMPLO.FIRMA_DE_EJEMPLO",
  "token_type": "Bearer",
  "expires_in": 300,
  "scope": "estudiantes.basic.view estudiantes.search.basic"
}
```

| Campo | Descripción |
|---|---|
| `access_token` | Token de servicio que se envía en el encabezado `Authorization`. |
| `token_type` | Siempre `Bearer`. |
| `expires_in` | Segundos de validez desde la emisión. |
| `scope` | Permisos efectivos del token, separados por espacios y en orden alfabético. |

**Ejemplo de solicitud (JavaScript, fetch)**

```javascript
const respuesta = await fetch(`${process.env.TUNGASUCA_BASE_URL}/api/auth/token`, {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({
    grant_type: "client_credentials",
    client_id: process.env.TUNGASUCA_CLIENT_ID,
    client_secret: process.env.TUNGASUCA_CLIENT_SECRET,
  }),
});
if (!respuesta.ok) {
  throw new Error(`No se obtuvo el token: HTTP ${respuesta.status}`);
}
const { access_token, expires_in } = await respuesta.json();
```

**Errores del endpoint de token**

| HTTP | Mensaje (campo `desc`) | Causa |
|---|---|---|
| 400 | `grant_type debe ser client_credentials` | Valor distinto de `client_credentials`. |
| 400 | `field "client_id" is not set` (o el campo que falte) | Falta un campo obligatorio. |
| 400 | `client_id y client_secret son obligatorios` | Campo presente pero vacío. |
| 401 | `credenciales de cliente inválidas` | El Client ID no existe, el secreto es incorrecto o el cliente está revocado. El mensaje es el mismo en los tres casos. |
| 403 | `el cliente no tiene permisos asignados` | El cliente no tiene ningún scope válido. |
| 429 | `rate_limited: demasiadas solicitudes; …` | Se superó el límite por IP. |
| 500 / 503 | `servicio no disponible temporalmente` | Error interno, o la emisión de tokens está deshabilitada en el entorno. |

### 4.3 Uso del token

En cada consulta, envíe el token en el encabezado `Authorization`:

```http
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.EJEMPLO.FIRMA_DE_EJEMPLO
```

- Reutilice el mismo token mientras siga vigente.
- Renuévelo unos 60 segundos antes de que venza.
- Si una consulta responde `401`, solicite un token nuevo y reintente **una sola vez**.

## 5. Permisos y scopes

| Scope | Permite | Operaciones |
|---|---|---|
| `estudiantes.search.basic` | Buscar estudiantes activos por nombres, apellidos o DNI completo. | `POST /api/estudiantes/seleccion/buscar` |
| `estudiantes.basic.view` | Consultar datos mínimos por `id` y resolver un estudiante por DNI completo. | `POST /api/estudiantes/basic-batch`, `POST /api/estudiantes/resolver` |

- Los scopes se asignan al **cliente de servicio** (el sistema), no a una persona. **No
  equivalen** a los permisos de un usuario de SIS-MESA-PARTES ni otorgan acceso a sus
  pantallas.
- Un sistema nuevo **solo** puede recibir estos dos scopes. Auth rechaza al registrar
  cualquier otro permiso, por ejemplo `estudiantes.view`, `estudiantes.personal.view`,
  `estudiantes.create`, `estudiantes.update` o `estudiantes.matricula.update`.
- Aunque un token tuviera otro permiso, Estudiantes **no permite** a los sistemas crear,
  actualizar ni matricular estudiantes, ni leer sus datos personales. La respuesta es
  `403`.
- Un token de usuario no sirve para la búsqueda de sistemas: la respuesta es `403`.
- Los scopes los asigna o retira un administrador de SIS-MESA-PARTES con el permiso
  `integraciones.manage`.

## 6. Endpoints disponibles

Hay tres endpoints de consulta disponibles para sistemas consumidores, más el endpoint de
token descrito en la sección 4. Todos usan `POST`, para que los criterios de consulta (que
pueden incluir un DNI) viajen en el cuerpo y **nunca en la URL**.

Encabezados comunes:

| Encabezado | Valor | Obligatorio |
|---|---|---|
| `Authorization` | `Bearer <access_token>` | Sí |
| `Content-Type` | `application/json` | Sí |

### 6.1 Buscar estudiantes para selección

| Campo | Valor |
|---|---|
| Nombre | Búsqueda de selección (`BuscarEstudiantesSeleccion`) |
| Descripción | Busca estudiantes activos por nombres, apellidos o DNI completo, y devuelve una página de resultados con datos mínimos. |
| Método | `POST` |
| Ruta | `{BASE_URL}/api/estudiantes/seleccion/buscar` |
| Autenticación | Token de servicio (Bearer). Un token de usuario se rechaza. |
| Scope | `estudiantes.search.basic` |
| Límite de tasa | 60/min por credencial (ráfaga de 20) y 120/min por IP (ráfaga de 30) |

**Parámetros de entrada**

| Campo | Tipo | Obligatorio | Descripción |
|---|---|---|---|
| `q` | string | Sí | Texto de búsqueda: de 2 a 100 caracteres y como máximo 5 palabras. |
| `page` | entero | No | Página solicitada, desde 1 (1 por defecto, máximo 500). |
| `page_size` | entero | No | Resultados por página (10 por defecto, máximo 20). |

**Ejemplo de solicitud (cURL)**

```bash
curl -X POST "{BASE_URL}/api/estudiantes/seleccion/buscar" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"q":"quispe huaman","page":1,"page_size":10}'
```

**Ejemplo de respuesta (200)**

```json
{
  "estudiantes": [
    {
      "id": "11111111-1111-4111-8111-111111111111",
      "nombre_completo": "Ana María Quispe Huamán",
      "grado": "3",
      "seccion": "B"
    }
  ],
  "total": 1
}
```

| Campo | Tipo | Descripción |
|---|---|---|
| `estudiantes` | arreglo | Resultados de la página. Si no hay coincidencias, es un arreglo vacío. |
| `estudiantes[].id` | string (UUID) | Identificador único y estable del estudiante. Es el dato que el sistema consumidor debe guardar. |
| `estudiantes[].nombre_completo` | string | Nombres y apellidos. |
| `estudiantes[].grado` | string | Grado actual registrado. |
| `estudiantes[].seccion` | string | Sección actual registrada. |
| `total` | entero | Total de coincidencias de la búsqueda, no solo de esta página. |

**Reglas y validaciones**

- Cada palabra de `q` debe aparecer en los nombres o apellidos del estudiante. No se
  distinguen mayúsculas ni tildes, y el orden de las palabras no importa.
- Una palabra de **exactamente 8 dígitos** se busca como **DNI exacto**, solo entre
  estudiantes con documento de tipo DNI. Se puede combinar con nombres, por ejemplo
  `quispe 45127823`.
- Un número de otra longitud (DNI parcial, por ejemplo) **no devuelve resultados**.
- El código SIAGIE y el código interno **no** son criterios de búsqueda.
- Solo se devuelven estudiantes con estado `ACTIVO`.
- La respuesta **nunca** incluye el DNI, el código SIAGIE ni otros datos personales.

**Errores posibles**

| HTTP | Mensaje (campo `desc`) | Causa |
|---|---|---|
| 400 | `validation_error: la búsqueda debe tener entre 2 y 100 caracteres` | `q` demasiado corto o demasiado largo. |
| 400 | `validation_error: use como máximo 5 palabras` | Demasiadas palabras. |
| 400 | `field "q" is not set` | Falta `q`. |
| 401 | `falta el token de acceso` / `token de acceso inválido` | Token ausente, vencido o inválido. |
| 403 | `permission_denied: …` | Falta el scope, o se usó un token de usuario. |
| 429 | `rate_limited: …` | Se superó el límite de tasa. |
| 500 / 503 | `servicio no disponible temporalmente` | Error interno o servicio no disponible. |

### 6.2 Consultar datos mínimos por identificador

| Campo | Valor |
|---|---|
| Nombre | Consulta básica por lote (`GetEstudiantesBasic`) |
| Descripción | Devuelve los datos mínimos de hasta 100 estudiantes a partir de sus `id`. Sirve para mostrar o actualizar el nombre, el grado y la sección de estudiantes que el sistema ya tiene asociados. |
| Método | `POST` |
| Ruta | `{BASE_URL}/api/estudiantes/basic-batch` |
| Autenticación | Token de servicio (Bearer) |
| Scope | `estudiantes.basic.view` |
| Límite de tasa | 120/min por credencial (ráfaga de 40) |

**Parámetros de entrada**

| Campo | Tipo | Obligatorio | Descripción |
|---|---|---|---|
| `ids` | arreglo de string (UUID) | Sí | De 1 a 100 identificadores. Los repetidos se consultan una sola vez. |

**Ejemplo de solicitud (cURL)**

```bash
curl -X POST "{BASE_URL}/api/estudiantes/basic-batch" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"ids": ["11111111-1111-4111-8111-111111111111",
               "22222222-2222-4222-8222-222222222222"]}'
```

**Ejemplo de respuesta (200)**

Este endpoint comparte su estructura con el frontend interno, por eso la respuesta tiene
más claves. Para un sistema consumidor, **solo** se llenan `id`, `nombre_completo`, `grado`
y `seccion`. Las demás claves llegan vacías (`""`) o en `0`, y deben ignorarse.

```json
{
  "estudiantes": [
    {
      "id": "11111111-1111-4111-8111-111111111111",
      "codigo": "",
      "nombre_completo": "Ana María Quispe Huamán",
      "nivel": "",
      "grado": "3",
      "seccion": "B",
      "estado_matricula": "",
      "numero_documento": "",
      "nombres": "",
      "apellidos": "",
      "apellido_paterno": "",
      "apellido_materno": "",
      "codigo_siagie": "",
      "tipo_documento": "",
      "estado": "",
      "turno": "",
      "version": 0
    }
  ]
}
```

| Campo | Descripción |
|---|---|
| `estudiantes[].id` | Identificador del estudiante. |
| `estudiantes[].nombre_completo` | Nombres y apellidos. |
| `estudiantes[].grado` / `seccion` | Grado y sección actuales. |
| Resto de claves | Vacías para sistemas consumidores. No deben usarse. |

**Reglas y validaciones**

- Un `id` que no existe **no aparece** en la respuesta. Así se distingue "no existe" de un
  error: compare los `id` enviados con los recibidos.
- Con `ids` vacío, la respuesta es `{"estudiantes": []}`.

**Errores posibles**

| HTTP | Mensaje (campo `desc`) | Causa |
|---|---|---|
| 400 | `demasiados identificadores en una sola consulta` | Más de 100 `id`. |
| 400 | `uno de los identificadores no es un UUID válido` | Algún `id` con formato inválido. |
| 401 | `falta el token de acceso` / `token de acceso inválido` | Token ausente, vencido o inválido. |
| 403 | `permission_denied: …` | Falta el scope `estudiantes.basic.view`. |
| 429 | `rate_limited: …` | Se superó el límite de tasa. |
| 500 / 503 | `servicio no disponible temporalmente` | Error interno o servicio no disponible. |

### 6.3 Resolver un estudiante por DNI

| Campo | Valor |
|---|---|
| Nombre | Resolución exacta (`ResolveEstudiante`) |
| Descripción | Obtiene el estudiante que corresponde a un DNI completo. Sirve para vincular un registro del sistema externo que ya tiene el DNI. |
| Método | `POST` |
| Ruta | `{BASE_URL}/api/estudiantes/resolver` |
| Autenticación | Token de servicio (Bearer) |
| Scope | `estudiantes.basic.view` |
| Límite de tasa | 60/min por credencial (ráfaga de 20) |

**Parámetros de entrada**

| Campo | Tipo | Obligatorio | Descripción |
|---|---|---|---|
| `tipo_documento` | string | Sí | Debe ser `DNI`. |
| `numero_documento` | string | Sí | DNI completo de 8 dígitos. |

Los campos `codigo` y `codigo_siagie` existen en el endpoint, pero solo los pueden usar
los usuarios internos. Si un sistema consumidor los envía, la respuesta es `400`.

**Ejemplo de solicitud (cURL)**

```bash
curl -X POST "{BASE_URL}/api/estudiantes/resolver" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"tipo_documento":"DNI","numero_documento":"45127823"}'
```

**Ejemplo de respuesta (200)**

La respuesta usa la misma estructura que la sección 6.2: solo se llenan `id`,
`nombre_completo`, `grado` y `seccion`, y la respuesta **no devuelve el DNI** consultado.

```json
{
  "estudiante": {
    "id": "11111111-1111-4111-8111-111111111111",
    "codigo": "",
    "nombre_completo": "Ana María Quispe Huamán",
    "nivel": "",
    "grado": "3",
    "seccion": "B",
    "estado_matricula": "",
    "numero_documento": "",
    "nombres": "",
    "apellidos": "",
    "apellido_paterno": "",
    "apellido_materno": "",
    "codigo_siagie": "",
    "tipo_documento": "",
    "estado": "",
    "turno": "",
    "version": 0
  }
}
```

**Errores posibles**

| HTTP | Mensaje (campo `desc`) | Causa |
|---|---|---|
| 400 | `validation_error: los sistemas consumidores solo pueden resolver por tipo_documento DNI y numero_documento de 8 dígitos` | Se envió `codigo`, `codigo_siagie` u otro tipo de documento. |
| 400 | `invalid_document: …` | DNI incompleto o con formato inválido. |
| 401 | `falta el token de acceso` / `token de acceso inválido` | Token ausente, vencido o inválido. |
| 403 | `permission_denied: …` | Falta el scope `estudiantes.basic.view`. |
| 404 | `student_not_found: …` | No hay un estudiante con ese DNI. |
| 429 | `rate_limited: …` | Se superó el límite de tasa. |
| 500 / 503 | `servicio no disponible temporalmente` | Error interno o servicio no disponible. |

### 6.4 Operaciones no disponibles para sistemas

Los sistemas consumidores **no** pueden usar estas operaciones. La respuesta es `403`,
aunque el token tuviera el permiso.

- Registro completo del estudiante (`GET /api/estudiantes/:id`).
- Datos personales del estudiante.
- Alta, modificación y cambio de estado de estudiantes.
- Matrícula.
- Familiares, importación y exportación, plantillas, historial y posibles duplicados.

## 7. Ejemplos de integración

### 7.1 Ejemplo completo en JavaScript (Node.js 18 o superior, fetch)

Este cliente mínimo:

- obtiene el token y lo guarda **solo en memoria**;
- lo renueva 60 segundos antes de que venza;
- reintenta una vez ante `401`;
- respeta `Retry-After` ante `429`;
- procesa la respuesta JSON.

Las credenciales se leen de variables de entorno del backend.

```javascript
// cliente-estudiantes.js — Node.js 18+ (fetch nativo). Solo en el backend.
const BASE_URL = process.env.TUNGASUCA_BASE_URL; // p. ej. https://<dominio-asignado>
const CLIENT_ID = process.env.TUNGASUCA_CLIENT_ID;
const CLIENT_SECRET = process.env.TUNGASUCA_CLIENT_SECRET;

let token = null; // solo en memoria; nunca en disco, base de datos ni logs
let venceEn = 0;  // marca de tiempo (ms)

class ErrorApi extends Error {
  constructor(status, mensaje) {
    super(`HTTP ${status}: ${mensaje}`);
    this.status = status;
  }
}

async function solicitarToken() {
  const r = await fetch(`${BASE_URL}/api/auth/token`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      grant_type: "client_credentials",
      client_id: CLIENT_ID,
      client_secret: CLIENT_SECRET,
    }),
  });
  if (!r.ok) throw new ErrorApi(r.status, await r.text());
  const datos = await r.json();
  token = datos.access_token;
  venceEn = Date.now() + datos.expires_in * 1000;
}

async function tokenVigente() {
  // Renueva 60 s antes del vencimiento.
  if (!token || Date.now() > venceEn - 60_000) await solicitarToken();
  return token;
}

const esperar = (ms) => new Promise((ok) => setTimeout(ok, ms));

async function llamar(ruta, cuerpo, reintentado = false) {
  const r = await fetch(`${BASE_URL}${ruta}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${await tokenVigente()}`,
    },
    body: JSON.stringify(cuerpo),
  });
  if (r.ok) return r.json();

  // Token vencido o revocado: renovar y reintentar una vez.
  if (r.status === 401 && !reintentado) {
    token = null;
    return llamar(ruta, cuerpo, true);
  }
  // Límite de tasa: esperar lo indicado y reintentar una vez.
  if (r.status === 429 && !reintentado) {
    const segundos = Number(r.headers.get("Retry-After")) || 10;
    await esperar(segundos * 1000);
    return llamar(ruta, cuerpo, true);
  }
  // 400, 403, 404, 5xx: no reintentar a ciegas.
  // No registrar el cuerpo enviado (puede contener un DNI).
  throw new ErrorApi(r.status, await r.text());
}

export async function buscarEstudiantes(q, page = 1, pageSize = 10) {
  const cuerpo = { q, page, page_size: pageSize };
  const datos = await llamar("/api/estudiantes/seleccion/buscar", cuerpo);
  return {
    total: datos.total,
    estudiantes: datos.estudiantes.map((e) => ({
      id: e.id, nombreCompleto: e.nombre_completo, grado: e.grado, seccion: e.seccion,
    })),
  };
}

export async function datosMinimos(ids) {
  const datos = await llamar("/api/estudiantes/basic-batch", { ids });
  // Solo se usan los cuatro campos mínimos; los demás llegan vacíos.
  return new Map(datos.estudiantes.map((e) => [
    e.id,
    { nombreCompleto: e.nombre_completo, grado: e.grado, seccion: e.seccion },
  ]));
}

export async function resolverPorDni(dni) {
  try {
    const { estudiante: e } = await llamar("/api/estudiantes/resolver", {
      tipo_documento: "DNI",
      numero_documento: dni,
    });
    return { id: e.id, nombreCompleto: e.nombre_completo, grado: e.grado, seccion: e.seccion };
  } catch (err) {
    if (err.status === 404) return null; // no existe un estudiante con ese DNI
    throw err;
  }
}

// Uso:
// const { total, estudiantes } = await buscarEstudiantes("quispe huaman");
// console.log(`Coincidencias: ${total}`); // sin nombres ni DNI en logs de producción
```

### 7.2 Ejemplo completo con cURL (bash)

```bash
#!/usr/bin/env bash
# Requiere: curl y jq. Las credenciales vienen de variables de entorno
# (nunca escritas en el script).
set -euo pipefail
BASE_URL="${TUNGASUCA_BASE_URL:?defina TUNGASUCA_BASE_URL}"

# 1) Obtener el token (se guarda solo en una variable de la sesión).
#    jq arma el JSON y escapa correctamente las credenciales.
CUERPO=$(jq -n --arg id "$TUNGASUCA_CLIENT_ID" --arg secreto "$TUNGASUCA_CLIENT_SECRET" \
  '{grant_type: "client_credentials", client_id: $id, client_secret: $secreto}')
TOKEN=$(curl -sS --fail-with-body -X POST "$BASE_URL/api/auth/token" \
  -H "Content-Type: application/json" -d "$CUERPO" | jq -r '.access_token')

# 2) Buscar estudiantes. El cuerpo de la respuesta va a un archivo temporal
#    y el código HTTP, a una variable.
RESP=$(mktemp)
CODIGO=$(curl -sS -o "$RESP" -w '%{http_code}' -X POST \
  "$BASE_URL/api/estudiantes/seleccion/buscar" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"q":"quispe huaman","page":1,"page_size":10}')

# 3) Procesar la respuesta o el error.
if [ "$CODIGO" = "200" ]; then
  jq -r '"Total: \(.total)", (.estudiantes[] | "\(.id)\t\(.grado) \(.seccion)")' "$RESP"
else
  # 401: renovar el token; 429: esperar Retry-After; 400/403: corregir la solicitud.
  echo "Error HTTP $CODIGO: $(cat "$RESP")" >&2
fi
rm -f "$RESP"
```

### 7.3 Renovación del token

- Guarde el token y su hora de vencimiento (`ahora + expires_in`) **en memoria**.
- Renuévelo unos 60 segundos antes de que venza.
- Ante un `401` inesperado, por ejemplo tras una revocación o un cambio de clave en el
  servidor, solicite un token nuevo y reintente **una vez**. Si vuelve a fallar, deténgase
  y revise las credenciales.
- Si la renovación falla pero el token actual sigue vigente, puede seguir usándolo hasta
  que venza.
- No solicite un token por cada consulta: el endpoint de token tiene un límite de 10
  solicitudes por minuto por IP.

## 8. Respuestas y códigos HTTP

### 8.1 Formato de las respuestas

- **Éxito (200):** cuerpo JSON con la estructura de cada endpoint.
- **Error (4xx y 5xx):** cuerpo de **texto plano** con este formato:

```text
rpc error: code = <Código> desc = <mensaje>
```

Ejemplo:

```text
rpc error: code = Unauthenticated desc = token de acceso inválido
```

Para decidir qué hacer, use el **código HTTP**. El texto de `desc` sirve como diagnóstico
y puede cambiar. Los mensajes **nunca** repiten el DNI ni los valores consultados. En los
errores 5xx el mensaje siempre es genérico (`servicio no disponible temporalmente`).

### 8.2 Códigos y acción recomendada

| HTTP | Significado | Acción del sistema consumidor |
|---|---|---|
| 200 | Operación exitosa. | Procesar el JSON. |
| 400 | Solicitud incorrecta (validación de campos, `q`, DNI, cantidad de `ids`). | Corregir la solicitud. No reintentar sin cambios. |
| 401 | Token ausente, vencido o inválido; o credenciales inválidas en `/api/auth/token`. | Pedir un token nuevo y reintentar una vez. En `/api/auth/token`, revisar las credenciales. |
| 403 | Falta el scope, la operación no está disponible para sistemas, se usó un token de usuario o el cliente no tiene permisos. | No reintentar. Solicitar el scope al administrador si corresponde. |
| 404 | Recurso no encontrado (solo en `/api/estudiantes/resolver`). | Tratarlo como "sin coincidencia". |
| 429 | Límite de solicitudes excedido. Incluye el encabezado `Retry-After` (segundos). | Esperar lo indicado en `Retry-After` y reducir la frecuencia. |
| 500 | Error interno. | Reintentar más tarde con espera creciente. Reportarlo si persiste. |
| 503 | Servicio no disponible temporalmente. | Igual que 500. |
| 504 | Tiempo de espera agotado. | Igual que 500. |

## 9. Seguridad y buenas prácticas

- **No comparta el Client Secret.** Es único por sistema. Si se expone, solicite de
  inmediato la revocación del cliente y un cliente nuevo.
- **No coloque credenciales ni tokens en un frontend**, una aplicación móvil o un
  repositorio de código. Úselos solo en el backend.
- **Guarde las credenciales en variables de entorno** o en un gestor de secretos del
  backend, con acceso restringido.
- **Use HTTPS** en cualquier entorno que no sea el equipo local.
- **No almacene tokens:** manténgalos solo en memoria mientras sean válidos.
- **Respete los scopes autorizados.** Solicite únicamente los que su sistema necesita.
- **Implemente el manejo de la expiración**: renovación anticipada y un único reintento
  ante `401`.
- **Evite las consultas excesivas.**
  - Busque con espera entre pulsaciones (debounce) cuando la búsqueda sea interactiva.
  - Agrupe los `id` en `basic-batch` (hasta 100 por solicitud).
  - Respete `Retry-After`.
- **No registre tokens ni información personal en logs**, por ejemplo cuerpos de
  solicitud, DNI o nombres. Registre solo la ruta, el código HTTP y la hora.
- **Consuma solo los datos mínimos.** Guarde el `id` del estudiante como referencia y
  consulte el nombre, el grado y la sección cuando los necesite, en lugar de copiarlos de
  forma permanente.

## 10. Flujo completo de integración

1. **Solicitar acceso a la API.** El responsable del sistema externo envía al equipo de
   SIS-MESA-PARTES:
   - el nombre del sistema;
   - su finalidad;
   - las operaciones que necesita;
   - el entorno;
   - la dirección o red de origen.
2. **Registrar y autorizar el cliente de servicio.** Un administrador con el permiso
   `integraciones.manage` registra el cliente con la herramienta `clientes-servicio` de
   Auth y le asigna solo los scopes necesarios (`estudiantes.basic.view` y/o
   `estudiantes.search.basic`).
3. **Recibir las credenciales.** El administrador entrega el `client_id` y el
   `client_secret` por un canal seguro. El secreto se muestra **una sola vez** y Auth solo
   guarda su hash. Si se pierde, hay que registrar un cliente nuevo.
4. **Solicitar un token.** Con `POST /api/auth/token` (sección 4).
5. **Consultar los endpoints autorizados.** Use la sección 6, con el token en
   `Authorization: Bearer`.
6. **Procesar la información recibida.** Guarde el `id` como referencia del estudiante y
   use solo `nombre_completo`, `grado` y `seccion`.
7. **Gestionar errores y renovación del token.** Siga las secciones 7.3 y 8.2.

## 11. Limitaciones y consideraciones

- **Alcance de los datos:** solo `id`, `nombre_completo`, `grado` y `seccion`. La API no
  entrega DNI, código SIAGIE, código interno, nivel, turno, fecha de nacimiento, datos
  familiares, matrícula ni datos personales.
- **Restricciones de búsqueda:**
  - solo estudiantes con estado `ACTIVO`;
  - DNI solo completo (8 dígitos) y solo para documentos de tipo DNI;
  - no se busca por SIAGIE ni por código interno;
  - como máximo 20 resultados por página y 500 páginas.
- **Scopes:** solo `estudiantes.basic.view` y `estudiantes.search.basic`. No existe un
  scope de escritura para sistemas.
- **Expiración del token:** 5 minutos, sin token de renovación (refresh token). Si se revoca un cliente, el
  token que ya tenía sigue siendo válido hasta que vence (como máximo 5 minutos).
- **Límites de tasa:** se aplican por instancia del Gateway y se reinician si este se
  reinicia.
- **Disponibilidad de red:** hoy el Gateway solo escucha en la interfaz local del servidor
  (`127.0.0.1`). No es accesible desde otros equipos.
- **Estado de publicación:** producción **no está publicada**. Faltan el dominio, el
  certificado TLS y el proxy inverso. La URL de producción se informará cuando exista.
- **Versión desplegada:** la búsqueda de selección, la respuesta mínima y los límites de
  tasa están implementados, pero **pendientes de despliegue** (ver el aviso inicial).
- **Diferencias entre entornos:**
  - en local se usa HTTP sin cifrar y solo desde el mismo equipo;
  - en producción será obligatorio HTTPS a través del proxy inverso;
  - las credenciales de cada entorno son distintas y no se pueden usar en otro.
- **Formato de error:** en texto plano, no JSON (sección 8.1).

## 12. Glosario

| Término | Definición |
|---|---|
| API REST | Interfaz que expone operaciones mediante HTTP, rutas y métodos estándar (GET, POST, …), con datos generalmente en JSON. |
| Gateway | Punto de entrada único que recibe las solicitudes HTTP y las dirige a los microservicios internos. |
| Microservicio | Servicio independiente responsable de un dominio concreto (por ejemplo, Estudiantes), con su propia base de datos. |
| OAuth 2.0 | Estándar de autorización que define cómo un cliente obtiene tokens de acceso. |
| Client Credentials | Flujo de OAuth 2.0 en el que un sistema (no una persona) se autentica con su `client_id` y su `client_secret`. |
| Bearer Token | Token que se presenta en el encabezado `Authorization: Bearer <token>`. Quien lo posee obtiene el acceso, por eso debe protegerse. |
| Scope | Permiso concreto asignado a un cliente que limita las operaciones que puede realizar. |
| JWT | JSON Web Token: formato de token firmado que contiene datos verificables, como el emisor, la audiencia, el vencimiento y el scope. |
| JSON | Formato de texto para intercambiar datos estructurados (objetos, arreglos, cadenas y números). |

## 13. Soporte e integración

Para solicitar soporte, el sistema externo debe proporcionar:

| Dato | Ejemplo |
|---|---|
| Nombre del sistema | "Sistema de Biblioteca" |
| Client ID | `cli_…` (es un dato público; **nunca** enviar el Client Secret) |
| Entorno | Local, pruebas o producción |
| Endpoint utilizado | `POST /api/estudiantes/seleccion/buscar` |
| Fecha y hora (con zona horaria) | 30/10/2026 10:15:30 (UTC-5) |
| Código HTTP | 403 |
| Mensaje de error | El texto recibido (`rpc error: code = … desc = …`) |

**No incluya** en la solicitud de soporte el Client Secret, tokens, DNI, nombres de
estudiantes ni cuerpos completos de solicitud o respuesta.
