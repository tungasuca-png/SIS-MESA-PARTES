# 05 — Modelo maestro propuesto

> **Todo este documento es una PROPUESTA** salvo donde dice "existe" (verificado en [01](01-inventario-servicios-y-bases.md) y [02](02-diccionario-de-datos.md)). No se creó ni se modificó ninguna tabla.

## 0. Principios

1. **Una identidad por persona física.** Estudiante, padre, apoderado, egresado y miembro del personal son **roles** de una persona, no personas distintas.
2. **Cada dato tiene un dueño.** Los demás servicios guardan solo el UUID y, cuando hace falta, una proyección mínima documentada o una instantánea con valor probatorio.
3. **UUID estable como clave** en todas las relaciones distribuidas; nunca el DNI.
4. **Sin FK entre bases.** Dentro de una base, FK con RESTRICT; entre bases, validación por API y comprobaciones periódicas de consistencia.
5. **Sin borrado físico** de historial; bajas lógicas y cierres de vigencia.
6. **Vinculaciones verificadas** (persona ↔ cuenta, familia ↔ estudiante, egresado): nunca por coincidencia de nombres; siempre auditadas.

## 1. Dónde vive el maestro

| Alternativa | Ventajas | Costos | Evaluación |
|---|---|---|---|
| **A. Evolucionar el servicio Estudiantes a "Maestro escolar"** (personas, estudiantes, matrículas, familia y egreso en `estudiantes_db`) | Ya existen `personas`, `estudiante_familiares`, matrículas, auditoría, permisos por dominio y FK dentro de la misma base; transacciones locales entre persona y estudiante | Un servicio con más responsabilidades | **Recomendada** |
| B. Servicio nuevo "Personas" y Estudiantes como consumidor | Identidad reutilizable fuera de lo escolar | Rompe las FK actuales (`estudiante_familiares` → `personas`); transacciones distribuidas en cada alta; más infraestructura | Solo si aparecen consumidores no escolares grandes |
| C. Base compartida por varios servicios | Consultas simples | Viola los límites de los microservicios; acoplamiento de esquemas | **Descartada** |

**Recomendación:** A. El nombre técnico `estudiantes` puede mantenerse; conceptualmente pasa a ser el **Maestro escolar** (ADR-01).

## 2. Entidades del maestro (en `estudiantes_db`)

```
personas ─┬─< estudiantes ─┬─< matriculas ─< matricula_movimientos
          │                │        └── secciones ── periodos_academicos
          │                ├─< estudiante_familiares >── personas
          │                └── egresos
          ├─< persona_documentos_historial   (correcciones de identidad)
          └── persona_cuentas                 (vínculo con Auth)
```

### A. `personas` (existe con 6 campos; se amplía)

| Campo | Tipo | Regla |
|---|---|---|
| id | UUID PK | Estable de por vida |
| tipo_documento | FK `cat_tipos_documento` | `SIN_DOCUMENTO` permitido; nunca se inventa un DNI |
| numero_documento | VARCHAR(12) NULL | Único parcial `(tipo, numero) WHERE numero IS NOT NULL` (**existe**) |
| nombres, apellido_paterno, apellido_materno | texto | Normalización de espacios; sin separar apellidos compuestos |
| fecha_nacimiento | DATE NULL | Solo si hay finalidad (estudiantes: sí; padres: no hace falta) |
| sexo | CHAR(1) NULL | Solo para estudiantes (SIAGIE) |
| estado | ACTIVO / INACTIVO / FUSIONADA | FUSIONADA con `fusionada_en` (UUID de la persona que queda) |
| version, created_at, updated_at | | Concurrencia y trigger (patrón existente) |

- **Contacto:** se mueve a la persona (`correo`, `telefono`, `domicilio_*`) con permiso `personal.*`. Para un estudiante menor, el contacto primario es el de su apoderado (ADR-07).
- **Prevención de duplicados:**
  1. Unicidad por documento.
  2. Reporte de posibles duplicados (**existe** para estudiantes) extendido a personas.
  3. **Fusión controlada** (nunca automática): la persona duplicada queda como `FUSIONADA` y sus relaciones se reasignan en una transacción auditada. Los servicios externos reciben un evento `persona.fusionada` para actualizar su UUID (ver [06](06-modelo-por-microservicio.md) §3).
- **Corrección de identidad:** `persona_documentos_historial` (append-only) registra el tipo y número anterior, quién, cuándo y el motivo (con evidencia). Un documento solo cambia con el permiso `personas.identidad.corregir`.

### B. `estudiantes` (existe; se reduce a lo propio del estudiante)

| Campo | Situación |
|---|---|
| id UUID PK | Existe. **Se conserva** (lo usan Psicología y el frontend) |
| persona_id UUID FK `personas` UNIQUE | **Nuevo.** Un estudiante es una persona |
| codigo, codigo_siagie, estado, version | Existen; se quedan en `estudiantes` |
| nombres, apellidos, documento, fecha_nacimiento, sexo, lenguas, domicilio, contacto | Pasan a `personas`. En `estudiantes` se mantienen **temporalmente** como columnas de compatibilidad (o vista) hasta que ningún consumidor las lea |
| nivel, grado, seccion, turno, estado_matricula | Heredados. Pasan a `matriculas` (R-05) |
| colegio_procedencia, condicion | Pendientes de decisión (ADR-09) |

El contrato gRPC (`EstudianteBasico`) **no cambia**: se resuelve con un JOIN a `personas`. Los consumidores siguen usando `estudiante_id`.

### C. Matrículas (estructura existe desde la migración 007; faltan las escrituras)

- **Ya existen:** `periodos_academicos`, `secciones` (con aula y FK compuesta al periodo), `matriculas` (una por estudiante y periodo, estados D-M4) y `matricula_movimientos` (append-only: CAMBIO_SECCION, CAMBIO_TURNO, RETIRO, REINGRESO, TRASLADO_SALIDA).
- **Faltan:**
  - RPC de escritura (matricular, cambiar sección, retirar, trasladar, concluir), con el permiso `estudiantes.matricula.update` (**nuevo**, por aprobar).
  - Carga de las columnas heredadas (requiere decidir el periodo, ADR-08).
  - Movimiento `EGRESO` o estado `CONCLUIDO` en el último grado, que origina el egreso (§F).
- La **trayectoria** es la secuencia de matrículas por periodo; ninguna matrícula "actual" la reemplaza.

### D. Familiares y apoderados (existe desde la migración 008; faltan autorizaciones)

- **Ya existen:**
  - `personas`, `estudiante_familiares` (parentesco, `es_apoderado`, vigencia desde/hasta) y `cat_parentescos`.
  - Reglas: un PADRE y una MADRE vigentes, **un apoderado vigente** y una relación vigente por persona.
  - La misma persona puede relacionarse con varios estudiantes: una fila de persona, N relaciones.
- **Faltan** (propuesta):

| Tabla | Propósito |
|---|---|
| `familiar_autorizaciones` (relacion_id, tipo, vigente, desde, hasta, otorgada_por) | Autorizaciones **explícitas** separadas del parentesco. Tipos: `VER_EXPEDIENTE_ESCOLAR`, `RECOGER_ESTUDIANTE`, `RECIBIR_NOTIFICACIONES`, `TRAMITAR` |
| `familiar_restricciones` (relacion_id, tipo, motivo_codigo, vigente, evidencia_ref) | Restricciones legales (por ejemplo, una orden judicial que impide el acceso), con prioridad sobre las autorizaciones |
| `estudiante_familiares` (historial) | Los cambios de apoderado cierran la relación o el rol (vigencia) y quedan en `auditoria_estudiantes` (FAMILIA_UPDATE, **existe**) |

- **Regla del apoderado vigente:** existe como índice parcial. Debe confirmarla la institución (ADR-06).
- **Acceso de padres:** se decide por **relación verificada + autorización vigente + ausencia de restricción**, nunca solo por el parentesco.

### E. Cuentas de Auth vinculadas a personas

| Opción | Evaluación |
|---|---|
| `auth.usuarios.persona_id` | Auth conocería el dominio escolar; aceptable como referencia opaca, pero la **verificación** del vínculo es escolar |
| **`persona_cuentas` en el maestro** (persona_id FK, usuario_id UUID sin FK, estado PENDIENTE/VERIFICADA/REVOCADA, metodo, verificado_por, fechas) con **proyección `persona_id` en Auth** | **Recomendada**. El maestro decide quién es la persona; Auth guarda el UUID para emitirlo en el JWT (`pid`) sin consultar al maestro en cada petición |

- **Auth mantiene:** credenciales, correo e identificador de login, estado de la cuenta, roles, permisos, sesiones, tokens e **identidades externas** (Google, §G).
- **Auth deja de guardar** nombres y documentos de negocio: se retira `user_profiles` (R-01). El DNI de login se valida contra la persona vinculada (R-03).
- **Cuentas existentes:**
  - Se vinculan **una a una** mediante un proceso verificado: staff con permiso, o autoservicio con validación (§H).
  - No se pierde ninguna cuenta, rol ni permiso: la vinculación es **aditiva**.
  - Las cuentas sin vincular siguen funcionando con sus permisos actuales.
- **Usuarios Service:** el perfil laboral del personal pasa a referenciar `persona_id`. Nombres, DNI y contacto dejan de duplicarse (R-01, R-02, R-03).

### F. Egresados

**Qué es:** un egresado es una **persona** cuyo **estudiante** tiene una matrícula `CONCLUIDO` en el último grado del último nivel y un **egreso validado**. No es una segunda identidad.

| Tabla (propuesta) | Campos |
|---|---|
| `egresos` | id, estudiante_id FK UNIQUE, persona_id FK, periodo_egreso_id FK, matricula_id FK (la que concluyó), fuente (`TRAYECTORIA` \| `ACTA_HISTORICA`), evidencia_ref, estado (`PENDIENTE` \| `VALIDADO` \| `ANULADO`), validado_por, validado_en, version |
| `egresado_contacto` | persona_id, correo, telefono, consentimiento_id, vigente (contacto **autorizado por el egresado** para seguimiento, distinto del contacto escolar) |
| `consentimientos` | persona_id, finalidad (`SEGUIMIENTO_EGRESADOS`, `COMUNICACIONES`), version_texto, otorgado_en, revocado_en, origen |
| `egresado_actualizaciones` | Historial append-only de cambios hechos por el propio egresado (qué campo, cuándo, no valores sensibles) |

- **Egresados anteriores al sistema** (sin matrículas registradas): egreso con `fuente = ACTA_HISTORICA` y evidencia (acta o nómina), validado por personal autorizado. Nunca se crea desde un formulario público.
- **Solicitudes del portal** (certificados, constancias): son **trámites** de Mesa de Partes (Expedientes) con `persona_id`/`estudiante_id` del egresado. No se crea un sistema de solicitudes paralelo.
- **Actividad del portal:** Auth (inicios de sesión) más la auditoría del maestro (lecturas y actualizaciones).

**¿Módulo o microservicio?**

| | Módulo del maestro (recomendado) | Microservicio "Egresados" |
|---|---|---|
| Ventajas | Egreso = FK a estudiante y matrícula en la misma base; sin duplicar la identidad; reutiliza auditoría y permisos | Despliegue y escalado independientes para un portal público |
| Costos | El maestro atiende tráfico del portal | Necesitaría una proyección de persona y estudiante (duplicación controlada), eventos de egreso y consistencia eventual |

**Recomendación (ADR-05):** módulo del maestro. Si más adelante el portal tiene carga o requisitos propios, se separa un "BFF de egresados" **sin base de identidad propia**.

### G. Registro de egresados con Google (OAuth 2.0 / OpenID Connect)

**Auth** implementa el flujo y guarda la identidad externa. El **maestro** decide si la persona es egresada. Ver el diagrama 5 en [10](10-diagramas.md).

1. **Inicio:** el frontend redirige a Google con `response_type=code`, `scope=openid email profile`, `state` (anti-CSRF), `nonce` y PKCE (`code_challenge`).
2. **Intercambio en Auth:** Auth canjea el `code` en el servidor (el secreto del cliente vive solo en Auth). Luego valida el **ID token**: firma con JWKS de Google, `iss`, `aud` = client_id propio, `exp` e `iat`, `nonce`.
3. **Identificador estable:** la clave de la identidad externa es **(`proveedor`, `sub`)**, **no el correo** (el correo puede cambiar o reutilizarse). Tabla en Auth: `identidades_externas`, con `UNIQUE(proveedor, sub)`. Campos:
   - id, usuario_id FK, proveedor, sub;
   - correo e `email_verified` en el momento del vínculo;
   - vinculada_en, ultimo_uso_en, revocada_en.
4. **Correo verificado:** se exige `email_verified = true`. Aun así, **el correo solo identifica la cuenta de Google; no prueba la condición de egresado.**
5. **Cuenta:**
   - Si (`proveedor`, `sub`) ya está vinculado, se inicia sesión.
   - Si no, se crea una cuenta **sin rol privilegiado** (rol `EGRESADO_PENDIENTE`, sin permisos sobre datos escolares) y **sin contraseña**.
   - Si ya existe una cuenta local con ese correo, **no se fusiona automáticamente**: se pide iniciar sesión con la cuenta existente y vincular desde dentro (prueba de posesión de ambas).
6. **Verificación de egreso (en el maestro):**
   - El usuario declara su tipo y número de documento, fecha de nacimiento y año de egreso.
   - El maestro busca **coincidencia exacta** con una persona que tenga `egresos.estado = VALIDADO`, o una trayectoria concluida.
   - Hay límite de intentos y la respuesta no revela qué dato falló (anti-enumeración).
7. **Resultado:**
   - **Una coincidencia exacta:** vínculo `persona_cuentas` con estado `PENDIENTE_CONFIRMACION`. Se confirma con un segundo factor fuera de banda: un código enviado al contacto registrado **antes** del proceso (el de `egresado_contacto` o el del apoderado histórico) o la validación del personal. Después, la cuenta recibe el rol `EGRESADO`.
   - **Ninguna o varias coincidencias:** solicitud a **revisión manual** (cola del personal). Nunca se crea un estudiante o egresado nuevo desde el formulario.
8. **Recuperación y desvinculación:**
   - Perder el acceso a Google: recuperación con un segundo método verificado o con revisión del personal.
   - Desvincular: se marca `revocada_en` en `identidades_externas`; se conservan la persona y el egreso.
9. **Auditoría:** eventos `identidad_externa.vinculada/revocada`, `egreso.verificacion_intentada/aprobada/rechazada` y `persona_cuenta.vinculada/revocada`, con actor, correlación y resultado. **Nunca** tokens ni el documento completo.
10. **Consentimiento y privacidad:** antes de guardar contacto, consentimiento explícito por finalidad (`consentimientos`, versión del texto). Minimización: de Google solo se guardan `sub`, correo y `email_verified`; nunca la foto ni otros datos del perfil. Se puede revocar.

### H. Incorporación de padres y apoderados

Ver el diagrama 6 en [10](10-diagramas.md). Son cinco piezas **distintas**:

| Pieza | Dónde | Quién la crea |
|---|---|---|
| Persona | Maestro `personas` | Personal (matrícula), por documento |
| Relación familiar validada | Maestro `estudiante_familiares` | Personal con `estudiantes.familia.update` (**existe**) |
| Condición de apoderado | Maestro (`es_apoderado`, único vigente) | Personal |
| Cuenta | Auth | La activa el propio padre mediante una **invitación** |
| Autorizaciones por estudiante | Maestro `familiar_autorizaciones` | Personal |

**Flujo de invitación** (propuesto):
1. El personal verifica la relación con documentos, la registra y emite una **invitación**: tabla `invitaciones_cuenta` en el maestro con `persona_id`, token **hasheado**, expiración (48–72 h), un solo uso, canal (correo o teléfono **ya registrado**) y emisor.
2. El padre abre el enlace:
   - Sin cuenta: crea sus credenciales en Auth (sin contraseñas predeterminadas).
   - Con cuenta: inicia sesión y confirma el vínculo.
   - En ambos casos queda `persona_cuentas` en estado VERIFICADA.
3. **Una cuenta y varios hijos:** la misma persona tiene N relaciones. No se crean cuentas por estudiante.
4. **Acceso:** el permiso de rol (por ejemplo `familia.portal.view_own`) **más** la regla del maestro "la persona de la cuenta tiene una relación vigente, con autorización, sin restricción" con ese estudiante. Mismo patrón `view_own` que ya usa Expedientes.

**Controles:**
- No se vincula por coincidencia de nombres.
- Los tokens son de un solo uso y se guardan con hash.
- La invitación se envía solo a un contacto ya registrado.
- Hay límite de intentos.
- Cualquier cambio de apoderado o de autorizaciones se audita (FAMILIA_UPDATE).
- Revocar la relación corta el acceso de inmediato: la regla se evalúa en cada petición.
- Los documentos del expediente escolar se ven solo con autorización explícita.

## 3. Modelo lógico completo (propuesto)

| Dominio | Tablas | Estado |
|---|---|---|
| Identidad | personas*, persona_documentos_historial, persona_cuentas | *personas existe (mínima) |
| Estudiante | estudiantes (+persona_id), cat_tipos_documento | Existe; `persona_id` propuesto |
| Académico | periodos_academicos, secciones, matriculas, matricula_movimientos | Existe (sin escrituras) |
| Familia | estudiante_familiares, cat_parentescos, familiar_autorizaciones, familiar_restricciones, invitaciones_cuenta | Las dos primeras existen |
| Egreso | egresos, egresado_contacto, consentimientos, egresado_actualizaciones | Propuesto |
| Auditoría | auditoria_estudiantes (se amplía con acciones de persona, egreso y vínculos) | Existe |
| Auth | usuarios (+persona_id), identidades_externas | usuarios existe; el resto propuesto |
