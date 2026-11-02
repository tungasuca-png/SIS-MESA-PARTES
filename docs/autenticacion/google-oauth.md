# Acceso con Google (solo SOLICITANTES)

Segundo método de registro e inicio de sesión del SIS-MESA-PARTES, exclusivo para solicitantes. Google
solo **verifica la identidad**: Auth Service sigue siendo dueño de las cuentas, los roles, los permisos y
las sesiones (mismo JWT y mismos refresh tokens que el login con DNI).

- Protocolo: OpenID Connect, Authorization Code Flow + PKCE (S256), `state` y `nonce`.
- Biblioteca: `github.com/coreos/go-oidc/v3` (firma, issuer, audience y expiración del ID token) y
  `golang.org/x/oauth2` (canje del código).
- Scopes: `openid email profile`.

## 1. Reglas de negocio

| Regla | Cómo se cumple |
|---|---|
| Google solo crea SOLICITANTES | El rol se fija en Auth (`GoogleRegisterUser`). El frontend nunca envía rol. Login y vinculación rechazan cualquier otro rol |
| Identidad = `sub` de Google | Tabla `user_auth_providers` con `UNIQUE(provider, provider_user_id)`. El correo nunca vincula |
| DNI ya registrado → no hay segunda cuenta | Auth (índice único de `usuarios.dni`) y Usuarios (índice único de `usuarios.dni`). Si el DNI está solo en Usuarios, se compensa eliminando la cuenta pendiente recién creada |
| Correo ya registrado → no se vincula ni se fusiona | El callback lo rechaza y orienta al login institucional y a la vinculación |
| Cuenta institucional → vinculación explícita | `POST /api/auth/google/vincular` con sesión **y** la clave actual. Mismo UUID; no cambian rol, correo, DNI, clave ni estado |
| Inactivas o bloqueadas | Se aplican `estado` y `locked_until`. Google no reactiva ni desbloquea |
| Registro público antiguo | `POST /api/auth/register` responde **410 Gone** |

Mensaje para un DNI ya registrado (idéntico para cualquier cuenta: no revela rol ni estado):
> Este DNI ya tiene una cuenta registrada. Inicia sesión con tus credenciales institucionales. Si deseas
> utilizar Google, podrás vincularlo desde tu perfil después de iniciar sesión.

## 2. Flujos

```text
Login / registro
  Navegador ── GET /api/auth/google/iniciar ──► Gateway ── GoogleStart ──► Auth
            ◄─ 302 a Google + cookie tg_google_flujo (state, nonce, verifier; HttpOnly, SameSite=Lax, 10 min)
  Google ──── GET /api/auth/google/callback?code&state ──► Gateway (compara state con la cookie y la borra)
                                               └─ GoogleCallback ──► Auth: canje con PKCE + validación del ID token
            ◄─ 302 /login/google#codigo=<un solo uso>   (o #error=<clave>)
  Frontend ── POST /api/auth/google/canjear ──► SESION (identidad vinculada) | REGISTRO_PENDIENTE (formulario)

Formulario obligatorio (primer acceso)
  POST /api/auth/google/registro ─► Gateway:
     1. Auth.GoogleRegister: crea la cuenta SOLICITANTE pendiente (o retoma la existente); DNI/correo ya
        registrados → 409 sin crear nada.
     2. Usuarios.CreateMyProfile (token del nuevo usuario, solo interno): crea el perfil; DNI ya existente
        → Auth.GoogleCancelRegistration (compensación) y 409.
     3. Auth.GoogleCompleteRegistration: emite la sesión (solo ahora).

Vinculación (cuenta institucional SOLICITANTE)
  Mi perfil ── POST /api/auth/google/vincular {clave} (Bearer) ─► cookie tg_google_vinculo (HttpOnly, Strict)
           ── GET /api/auth/google/iniciar?modo=vincular ─► Google ─► callback ─► Auth.LinkIdentity
           ◄─ 302 /mesa-de-partes/perfil?google=vinculado
```

Los códigos temporales (`oauth_login_codes`): solo el hash SHA-256 en la base, un solo uso y vencimiento
corto (login 2 min, registro 30 min, vinculación 10 min). Nunca son JWT. Llegan al frontend en el
**fragmento** de la URL (no se envía al servidor ni en el Referer) y el frontend lo borra de inmediato.

Consistencia Auth ↔ Usuarios (bases separadas): no se emite sesión hasta que el perfil existe. Si
Usuarios falla de forma transitoria, la cuenta queda pendiente (`registro_completo = FALSE`, sin sesión
posible) y el mismo formulario se puede reenviar (idempotente: el código se bloquea con `FOR UPDATE` y el
perfil se crea con `ON CONFLICT DO NOTHING`). Volver a entrar con Google retoma el formulario.

## 3. API

| Método y ruta | Autenticación | Respuesta |
|---|---|---|
| `GET /api/auth/google/iniciar[?modo=vincular]` | Pública (vincular: cookie) | 302 a Google |
| `GET /api/auth/google/callback` | Cookie del flujo | 302 al frontend |
| `POST /api/auth/google/canjear` `{codigo}` | Pública (código) | `{resultado, sesion?, correo?, nombres?, apellidos?}` |
| `POST /api/auth/google/registro` `{codigo, dni, nombres, apellido_paterno, apellido_materno?, telefono?, direccion?}` | Pública (código) | Igual que `/api/auth/login` |
| `POST /api/auth/google/vincular` `{clave}` | Bearer | `{url, expira_en}` + cookie |
| `GET /api/auth/google/estado` | Bearer | `{vinculado, correo, disponible}` |
| `POST /api/auth/register` | — | **410 Gone** |

RPC nuevos: Auth `GoogleStart`, `GoogleCallback`, `GoogleExchange`, `GoogleRegister`,
`GoogleCompleteRegistration`, `GoogleCancelRegistration`, `GoogleStartLink`, `GoogleLinkStatus`; Usuarios
`CreateMyProfile` (id desde el JWT, rol SOLICITANTE firmado, solo crea si no existe).

Límites por IP (Gateway): iniciar y callback 20/min, canjear 20/min, registro 10/min, vincular 10/min
(+5/min por credencial). La clave de la vinculación cuenta intentos y bloquea igual que el login.

## 4. Base de datos (auth_db)

- `022_create_user_auth_providers.sql`: identidades externas (`google`), `registro_completo`.
- `023_create_oauth_login_codes_and_auth_eventos.sql`: códigos de un solo uso (LOGIN, REGISTRO,
  VINCULACION) y auditoría `auth_eventos` (evento, resultado y motivo; sin tokens ni datos personales).

Ambas son idempotentes y tienen reversión (`-- Down migration`). No modifican `usuarios` ni datos existentes.
Usuarios Service no requiere migraciones.

## 5. Configuración en Google Cloud Console (pendiente: la hace el administrador)

1. Crear o elegir el proyecto de Google Cloud de la institución.
2. **Pantalla de consentimiento OAuth**: tipo *Externo*, nombre de la aplicación, correo de soporte,
   dominio autorizado (producción) y scopes `openid`, `email`, `profile`. Mientras esté en modo de
   prueba, agregar los correos de prueba.
3. **Credenciales → Crear ID de cliente OAuth → Aplicación web**:
   - *Authorized redirect URIs*:
     - desarrollo: `http://localhost:5173/api/auth/google/callback`
     - producción: `https://<dominio-real>/api/auth/google/callback`
   - *Authorized JavaScript origins*: no son necesarios (no se usa Google Identity Services en el
     navegador).
4. Copiar el **Client ID** y el **Client Secret** a `backend/.env` (nunca al repositorio ni al frontend).

## 6. Variables de entorno (backend/.env)

| Variable | Uso | Ejemplo |
|---|---|---|
| `GOOGLE_CLIENT_ID` | Auth | `123-abc.apps.googleusercontent.com` |
| `GOOGLE_CLIENT_SECRET` | Auth (único servicio que lo conoce) | (secreto) |
| `GOOGLE_REDIRECT_URL` | Auth; igual a la URI registrada | `http://localhost:5173/api/auth/google/callback` |
| `GOOGLE_OIDC_ISSUER` | Auth; vacío = `https://accounts.google.com` | |
| `FRONTEND_URL` | Gateway; vacío = mismo origen | |
| `GOOGLE_COOKIE_SECURE` | Gateway; `true` en producción (HTTPS) | `false` |

Sin `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` y `GOOGLE_REDIRECT_URL`, el botón responde «no
habilitado» y el login con DNI no cambia.

## 7. Puesta en marcha

1. Aplicar en `auth_db` las migraciones `022` y `023` (requiere autorización).
2. Completar las variables de la sección 6 en `backend/.env`.
3. Reiniciar **Auth**, **Usuarios** y **Gateway** (`run-all.ps1` carga `backend/.env`). Expedientes y
   Documentos solo cambiaron archivos de prueba.
4. Frontend: Vite sirve el código actual; en producción, publicar el build.
5. Producción: HTTPS obligatorio, `GOOGLE_COOKIE_SECURE=true`, CORS y `FRONTEND_URL` del dominio real.

## 8. Pruebas

- `auth/internal/security/google_oidc_test.go`: proveedor OIDC simulado (httptest) con ID tokens firmados
  RS256 — identidad válida; audience, issuer, expiración, firma, nonce, PKCE, código reutilizado y `sub`
  ausente rechazados; `email_verified=false`; proveedor caído o no configurado.
- `auth/internal/logic/google_flujos_test.go` (PostgreSQL desechable, `AUTH_DB_AISLADA_URL`): registro y
  login, DNI existente (sin crear nada ni revelar rol/estado), correo existente, email no verificado,
  códigos vencidos/inválidos/reutilizados, concurrencia (doble envío y mismo DNI), compensación,
  vinculación (clave, mismo UUID, nada cambia, Google ya vinculado a otra cuenta), ADMIN, inactiva y
  bloqueada; login y refresh tradicionales.
- `usuarios`: `CreateMyProfile` (solo SOLICITANTE, validaciones) y `CreateIfNotExists` contra SQL real.
- `gateway`: state y cookie, destinos de redirección, orquestación del registro y compensación,
  vinculación, 410 del registro antiguo, cookies HttpOnly/Secure/SameSite.
- `frontend/src/pages/Login/google.test.jsx`: botón, callback, formulario (validación, doble envío, sin
  rol, DNI existente) y vinculación en el perfil.

## 9. Limitaciones

- El acceso real con Google no se probó contra Google: faltan las credenciales y la configuración de la
  sección 5. Todo lo demás se probó con un proveedor OIDC simulado.
- `Auth.Register` (gRPC interno, solo `127.0.0.1`) sigue existiendo porque lo usan las pruebas de
  integración como fixture; ya no tiene ruta HTTP. Se recomienda retirarlo cuando esas pruebas migren a
  `CreateUser`.
- No hay desvinculación de Google (la política actual no la contempla).
- Los registros pendientes vencidos y los códigos usados no se depuran automáticamente.
- Las cuentas creadas con Google tienen una clave aleatoria que nadie conoce: para usar también DNI y
  clave, la institución debe asignarle una.
