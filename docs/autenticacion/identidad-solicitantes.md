# Verificación de identidad de solicitantes

Para el equipo que revisa y activa la funcionalidad, y para Secretaría.

## Problema que resuelve

Verificar un correo (o una cuenta de Google) solo prueba que la persona
controla ese buzón, **no que el DNI que escribe sea suyo**. Antes, el DNI del
formulario público quedaba como DNI único de la cuenta: cualquiera podía
reservar el DNI de otra persona, impedirle registrarse y presentarse ante la
institución con esa identidad.

## Cómo funciona

1. **Registro público** (correo verificado o Google), solo rol `SOLICITANTE`,
   asignado por Auth:
   - el DNI se guarda como **DNI declarado** (`identidad_solicitantes` en
     Auth, `usuarios.dni_declarado` en Usuarios), sin unicidad;
   - `usuarios.dni` (Auth) queda en `NULL` y el `dni` del perfil (Usuarios)
     vacío: **nadie reserva un DNI**;
   - la respuesta es la misma tenga o no cuenta ese DNI (sin enumeración).
2. **Mientras la identidad está pendiente**, el solicitante:
   - inicia sesión con **correo y contraseña** (o con Google);
   - presenta trámites con normalidad; el personal ve la marca
     «Identidad no verificada» junto a su nombre;
   - ve en «Mi perfil»: «DNI declarado … Identidad pendiente de verificación».
3. **Secretaría** (permiso `solicitantes.identidad.verify`) abre
   *Administración → Identidad de solicitantes*, coteja el DNI (documento a la
   vista o copia adjunta a un trámite) y lo **verifica** o lo **rechaza** con
   un motivo.
4. **Al verificar**, el DNI pasa a ser el DNI definitivo en Auth y en
   Usuarios; desde entonces el solicitante también puede ingresar con su DNI.
5. **Si el DNI ya pertenece a otra cuenta**, la bandeja lo marca como
   conflicto, la verificación se rechaza y **no cambia ninguna cuenta**. El
   conflicto solo lo ve el personal con el permiso.

Las cuentas institucionales (ADMIN, AUXILIAR, DIRECTOR, DOCENTE, SECRETARIA,
SUBDIRECTOR, PSICOLOGO) no cambian: siguen ingresando solo por DNI; el login
por correo rechaza cualquier cuenta que no sea `SOLICITANTE` sin contar
intentos.

## Garantías técnicas

- **Permiso en el backend**: Auth (interceptor de autorización) y Usuarios
  (`RequirePermission` contra Auth) exigen `solicitantes.identidad.verify`;
  ocultar el menú no es la protección. El permiso se crea **sin asignar**.
- **Consistencia Auth ↔ Usuarios** (bases separadas; no hay transacción
  distribuida, así que se usan estados explícitos, compensación y
  reconciliación). Auth es la fuente de verdad y guarda el estado en
  `identidad_solicitantes.sincronizacion`:
  1. Auth verifica en una transacción: DNI definitivo, `VERIFICADA` y
     `PERFIL_PENDIENTE`. Si el DNI ya es de otra cuenta: conflicto y **nada
     cambia** en ningún servicio.
  2. Usuarios pasa el DNI al perfil (idempotente; solo perfiles
     `SOLICITANTE` que lo declararon).
     - Bien → Auth registra `COMPLETA`.
     - El DNI está en **otro perfil** de Usuarios → Auth **revierte** su
       verificación (compensación): quita solo ese DNI de la cuenta (nunca la
       elimina ni la recrea) y la identidad vuelve a `PENDIENTE` con el
       conflicto visible (no se reintenta a ciegas).
     - Usuarios no responde u otro fallo → sigue `PERFIL_PENDIENTE` con
       `sincronizacion_intentos`, `sincronizacion_error` (código, nunca el
       DNI) y `sincronizacion_at`.
  3. Si la compensación también falla, sigue `PERFIL_PENDIENTE` con el
     código `conflicto_perfil_reversion_pendiente`.
  4. **Reconciliación**: *Verificadas → Reintentar sincronización* (o
     `POST /api/solicitantes/identidades/reconciliar`) reintenta todas las
     pendientes con la sesión del personal: completa o compensa. Repetir una
     verificación también la reanuda; repetir una completa no hace nada.
  Los índices únicos `uq_usuarios_dni` de ambas bases son la garantía final:
  de varias verificaciones simultáneas del mismo DNI gana exactamente una.
- **Límites de solicitudes** (Gateway, en memoria por instancia; ver
  `internal/limite`): bandeja 60/min (ráfaga 20) por credencial; verificar y
  rechazar 10/min (ráfaga 5) por credencial **y** 20/min (ráfaga 10) por IP;
  reconciliar 10/min (ráfaga 5) por credencial. El contador es por patrón de
  ruta: cambiar el `:id` no lo reinicia. Valores tomados de los niveles
  existentes (reportes, exportaciones, inicio de Google). Con varias réplicas
  cada una cuenta por separado; al llenarse la tabla de contadores, las claves
  nuevas reciben 429 (falla cerrada).
- **Nadie resuelve su propia identidad**; verificar dos veces es idempotente;
  una identidad verificada no se rechaza.
- **Auditoría** en `auth_eventos` (`proveedor = 'identidad'`): evento,
  resultado, método y quién verificó (`identidad_solicitantes.verificado_por`).
  Nunca DNI ni nombres en logs ni eventos.
- No se guardan copias de documentos.

## Endpoints (Gateway)

| Método y ruta | Permiso | Solicitud | Respuesta |
|---|---|---|---|
| `GET /api/solicitantes/identidades?estado=PENDIENTE&limit=50&offset=0` | `solicitantes.identidad.verify` | — | `items[]` (`usuario_id`, `nombre_completo`, `dni_declarado`, `origen`, `estado`, `metodo`, `verificado_at`, `motivo_rechazo`, `created_at`, `conflicto`), `total` |
| `POST /api/solicitantes/identidades/:id/verificar` | idem | `metodo`: `PRESENCIAL` o `DOCUMENTO` | `estado`; 409 `identidad_conflicto`; 403 sin permiso |
| `POST /api/solicitantes/identidades/:id/rechazar` | idem | `motivo` (3-200) | `estado` |
| `POST /api/solicitantes/identidades/reconciliar` | idem | — | `completadas`, `revertidas`, `pendientes` |

La respuesta de verificar incluye `sincronizacion` (`COMPLETA` o
`PERFIL_PENDIENTE`); la bandeja acepta `sincronizacion_pendiente=true` y
devuelve `sincronizacion`, `sincronizacion_intentos` y `sincronizacion_error`.

**Aislamiento de trámites**: un solicitante que pide un expediente o documento
ajeno recibe la misma respuesta que para uno inexistente (404 «expediente no
encontrado» / «documento no encontrado»), así no puede averiguar qué ids
existen. El personal con alcance por área conserva su 403.
| `POST /api/auth/login` | público | `dni` **o** `correo` (solo SOLICITANTE) + `password` | sesión |

Cambios de contrato aditivos: `UsuarioBasicDTO.identidad_pendiente`,
`UsuarioDTO.dni_declarado` e `identidad_pendiente`. `POST
/api/auth/registro/verificar` devuelve `correo` (ya no `dni`).

## Activación (requiere autorización; nada de esto se ejecutó)

Orden obligatorio: **Usuarios 003 → Auth 024 → Auth 025**, con Auth 022 y 023
ya aplicadas. El procedimiento completo está en
[`docs/despliegue/registro-publico-identidad.md`](../despliegue/registro-publico-identidad.md):
respaldos, comprobación previa, migraciones, verificación, servicios y prueba
de humo, junto con la justificación del orden y los estados intermedios
probados.

Antes de que Auth guarde un registro o cree una cuenta (manual o Google), el
Gateway pregunta a Usuarios si puede crear el perfil (`PublicRegistrationReady`,
que lee la columna `dni_declarado`). Si Usuarios no está preparado, el
registro responde 503 **sin crear nada**. Por eso Usuarios va primero.

Resumen de los comandos:

```powershell
docker cp backend\services\usuarios\migrations\003_add_dni_declarado.sql tg-postgres:/tmp/u003.sql
docker exec tg-postgres psql -U usuarios_migrator -d usuarios_db -v ON_ERROR_STOP=1 -c "SET ROLE usuarios_owner" -f /tmp/u003.sql

docker cp backend\services\auth\migrations\024_create_registros_correo.sql tg-postgres:/tmp/024.sql
docker exec tg-postgres psql -U auth_migrator -d auth_db -v ON_ERROR_STOP=1 -c "SET ROLE auth_owner" -f /tmp/024.sql

docker cp backend\services\auth\migrations\025_create_identidad_solicitantes.sql tg-postgres:/tmp/025.sql
docker exec tg-postgres psql -U auth_migrator -d auth_db -v ON_ERROR_STOP=1 -c "SET ROLE auth_owner" -f /tmp/025.sql
docker exec tg-postgres rm /tmp/u003.sql /tmp/024.sql /tmp/025.sql
```

Verificación (solo lectura):

```powershell
docker exec tg-postgres psql -U tg_admin -d auth_db -At -c "SELECT 'identidad: ' || string_agg(p, ',') FROM unnest(ARRAY['SELECT','INSERT','UPDATE','DELETE']) p WHERE has_table_privilege('auth_app','public.identidad_solicitantes',p)"
docker exec tg-postgres psql -U tg_admin -d auth_db -At -c "SELECT codigo, (SELECT count(*) FROM role_permissions rp WHERE rp.permission_id = p.id) AS asignaciones FROM permissions p WHERE codigo = 'solicitantes.identidad.verify'"
docker exec tg-postgres psql -U tg_admin -d usuarios_db -At -c "SELECT column_name, column_default FROM information_schema.columns WHERE table_name='usuarios' AND column_name='dni_declarado'"
```

Esperado: `identidad: SELECT,INSERT,UPDATE`; el permiso con `0` asignaciones;
la columna con valor por defecto `''`.

Luego: reiniciar Auth, Usuarios y Gateway (`backend\stop-all.ps1` y
`backend\run-all.ps1`) y, desde *Configuración → Roles y permisos*, que ADMIN
asigne `solicitantes.identidad.verify` al rol que corresponda (la asignación es
una decisión institucional; ninguna migración la hace).

Reversión: Down documentado al final de 025 y 003 (primero detener los
servicios). La 025 revertida elimina las identidades declaradas: hacerlo solo
antes de habilitar el registro público.
