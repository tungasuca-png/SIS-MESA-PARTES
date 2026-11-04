# Registro manual con verificación del correo

Segunda modalidad de registro para **solicitantes**, complementaria a «Continuar
con Google». El rol nunca se elige: Auth registra siempre `SOLICITANTE`.

## Flujo

1. **Formulario** (`/registro`): nombres, apellidos, DNI, correo, contraseña,
   confirmación y aceptación de los documentos legales.
2. Auth valida, guarda un **registro pendiente** (`registros_correo`, migración
   024) y envía un código de 6 dígitos. **La cuenta todavía no existe.**
3. **Verificación** (`/registro/verificar`): con el código correcto, Auth crea
   la cuenta (`usuarios.email_verified = TRUE`, rol SOLICITANTE), el Gateway
   crea el perfil en Usuarios y Auth marca el registro como completado.
4. **Cuenta activada**: la persona inicia sesión con su **correo** y su
   contraseña. Verificar el correo **no** verifica el DNI: queda como DNI
   declarado hasta que Secretaría coteje la identidad; desde entonces también
   puede ingresar con su DNI. Ver `identidad-solicitantes.md`.

Mientras no termine el paso 3, el login responde 403 (`correo_no_verificado`
o `registro_incompleto`), y solo si la contraseña es correcta.

## Endpoints (Gateway, públicos)

| Método y ruta | Solicitud | Respuesta 200 | Errores |
|---|---|---|---|
| `POST /api/auth/registro` | `nombres`, `apellido_paterno`, `apellido_materno`?, `dni`, `correo`, `clave`, `acepta_terminos`, `version_terminos` | `registro_id`, `expira_en`, `reenviar_en` (segundos), `mensaje` | 400 `datos_invalidos`, `clave_invalida`, `terminos`; 429 `demasiadas_solicitudes` o límite por IP; 503 envío fallido / no disponible |
| `POST /api/auth/registro/verificar` | `registro_id`, `codigo` | `mensaje`, `correo` | 400 `codigo_incorrecto` (con intentos restantes), `codigo_expirado`, `codigo_formato`, `invalido`, `datos_invalidos`; 409 `dni_registrado`, `correo_registrado`; 429 `intentos_agotados`; 503 `perfil_pendiente` (reintentar con el mismo código) |
| `POST /api/auth/registro/reenviar` | `registro_id` | `expira_en`, `reenviar_en`, `mensaje` | 400 `invalido`; 429 `espera_reenvio`, `reenvios_agotados`; 503 envío fallido |
| `POST /api/auth/login` | `dni` **o** `correo` (solo SOLICITANTE), `password` | sesión | además: 403 `correo_no_verificado`, `registro_incompleto` |

Los errores 4xx llevan `registro_<clave>: <mensaje>`; los 5xx, el mensaje
genérico del Gateway. `POST /api/auth/register` (contrato antiguo con usuario y
rol) sigue respondiendo 410.

RPC de Auth: `EmailRegister`, `EmailResendCode`, `EmailVerify`,
`EmailCompleteRegistration`, `EmailCancelRegistration`.

## Seguridad

- **Código**: 6 dígitos de `crypto/rand`. En la base solo queda
  HMAC-SHA256 (clave derivada de `JWT_SECRET`, ligada al id del registro).
- **Límites** (`auth.yaml`, `RegistroCorreo`): vigencia 10 min, 5 intentos por
  código, 60 s entre envíos, 5 envíos por registro, 5 registros por correo y
  hora. El Gateway limita además por IP.
- **Un código nuevo invalida el anterior**; un registro nuevo del mismo correo
  anula los pendientes. Tras completar, el código deja de servir y se borran
  del registro la contraseña (bcrypt) y los nombres.
- **Sin enumeración**: si el correo ya tiene cuenta, la respuesta es idéntica y
  el correo recibido es un aviso, sin código. El DNI nunca se comprueba ni se
  reserva en el registro público (solo se declara).
- **Concurrencia**: cada paso bloquea la fila del registro; varias
  verificaciones simultáneas crean una sola cuenta.
- **Compensación**: si Usuarios rechaza el perfil (DNI de otra persona), la
  función `SECURITY DEFINER` `cancelar_registro_correo_pendiente(uuid)` elimina
  la cuenta recién creada. `auth_app` sigue sin DELETE sobre `usuarios`.
- **Logs y auditoría**: los RPC están excluidos del registro de contenido
  (Auth y Gateway). `auth_eventos` guarda solo evento y resultado
  (`proveedor = 'correo'`), sin correo, DNI, código ni contraseña.

## Proveedor de correo (SMTP)

Variables de `backend/.env` (ver `backend/.env.example`). Sin `SMTP_SERVIDOR` y
`SMTP_REMITENTE` el registro manual responde «no disponible» y nada más cambia.

| Variable | Uso |
|---|---|
| `SMTP_SERVIDOR` | `host:puerto` del proveedor institucional o transaccional |
| `SMTP_USUARIO`, `SMTP_CLAVE` | credenciales del proveedor (nunca en el repositorio) |
| `SMTP_REMITENTE` | dirección remitente autorizada (SPF/DKIM del dominio) |
| `SMTP_NOMBRE_REMITENTE` | p. ej. `SIS-MESA-PARTES - I.E. Tungasuca` |
| `SMTP_SEGURIDAD` | `starttls` (587, por defecto), `tls` (465) o `ninguna` (solo 127.0.0.1/localhost, p. ej. Mailpit) |

Sin TLS el cliente se niega a enviar (salvo un servidor local de pruebas).

Documentos legales (frontend/.env): `VITE_TERMINOS_URL`,
`VITE_PRIVACIDAD_URL` y `VITE_TERMINOS_VERSION`. Los publica la institución;
la versión aceptada y la fecha quedan en `registros_correo`.

## Probar en desarrollo (sin proveedor real)

1. Aplicar la migración 024 (después de 022 y 023), con el mismo procedimiento
   de `docs/autenticacion/google-oauth-activacion.md`, y reejecutar
   `backend/db/roles/02_propiedad_y_permisos.sql` si se desea comprobar la
   matriz de permisos.
2. Levantar Mailpit (captura los correos; no los entrega a internet):
   `docker run -d --name tg-mailpit -p 127.0.0.1:1025:1025 -p 127.0.0.1:8025:8025 axllent/mailpit:v1.21.8`
3. En `backend/.env`: `SMTP_SERVIDOR=127.0.0.1:1025`,
   `SMTP_REMITENTE=no-responder@<dominio>`, `SMTP_SEGURIDAD=ninguna`.
4. Reiniciar Auth y Gateway (`backend/stop-all.ps1` y `backend/run-all.ps1`).
5. Abrir `http://localhost:5173/registro`, completar el formulario, leer el
   código en `http://127.0.0.1:8025`, verificarlo e iniciar sesión.
6. Comprobar: el login antes de verificar responde «correo no verificado»; un
   código equivocado descuenta intentos; «Reenviar código» respeta la espera.
