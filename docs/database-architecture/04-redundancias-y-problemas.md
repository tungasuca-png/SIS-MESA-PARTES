# 04 — Redundancias, conflictos y problemas

**Criterio:** solo se declara una redundancia cuando dos lugares guardan **el mismo dato con el mismo significado**, o cuando uno guarda una copia de un dato cuyo dueño es otro. Que dos tablas tengan una columna `nombre` no basta.

**Evidencia:** catálogo verificado ([02](02-diccionario-de-datos.md), [03](03-relaciones-actuales.md)), migraciones y código.

**Clasificación de cada hallazgo:**
- **Confirmado:** hay evidencia en el esquema, en el código o en los datos (solo conteos o UUID).
- **Potencial:** el diseño lo permite, pero todavía no ocurre.

| ID | Tipo | Resumen |
|---|---|---|
| R-01 | Redundancia confirmada | Identidad de las personas usuarias en 3 tablas de 2 bases |
| R-02 | Redundancia confirmada | Rol duplicado (Auth y `usuarios.tipo_usuario`) |
| R-03 | Redundancia confirmada | DNI del usuario en 2 bases, con reglas distintas |
| R-04 | Redundancia confirmada | Datos del solicitante copiados en texto libre dentro de `expedientes.descripcion` |
| R-05 | Redundancia confirmada (en transición) | Datos académicos en `estudiantes` y en `matriculas` |
| R-06 | Redundancia potencial | Padres o apoderados en `personas` (Estudiantes) y como cuentas SOLICITANTE (Auth y Usuarios) |
| R-07 | Redundancia potencial | Contacto del estudiante frente al contacto de su familia |
| R-08 | Catálogo repetido | Áreas y roles como texto (`area_actual`, `origen`, `destino`, `tipo_usuario`) |
| P-09 | Integridad confirmada | Referencias huérfanas entre bases |
| P-10 | Integridad confirmada | 28 cuentas sin perfil; 27 cuentas sin DNI (no pueden iniciar sesión) |
| P-11 | Diseño | Los expedientes no se vinculan al estudiante |
| P-12 | Diseño | Archivos binarios dentro de PostgreSQL |
| P-13 | Operación | Sin registro de migraciones aplicadas |
| P-14 | Seguridad | Privilegio DELETE innecesario en 4 servicios |
| P-15 | Operación | `refresh_tokens` crece sin limpieza |
| P-16 | Pruebas | Tests de 3 servicios escriben directo en `auth_db` |
| P-17 | Modelo | Identificadores de persona inconsistentes: no existe `persona_id` global |
| P-18 | Seguridad | `usuarios_db` usa `''` en vez de NULL y tiene DNI VARCHAR(20) sin formato |

---

### R-01 — Identidad de usuarios en tres lugares (confirmado)

| | |
|---|---|
| **Evidencia** | `auth_db.usuarios` (username, email, dni) · `auth_db.user_profiles` (nombres, apellidos, tipo_documento, numero_documento, telefono, direccion) · `usuarios_db.usuarios` (nombres, apellidos, dni, telefono, correo, direccion) |
| **Propietario actual** | Auth tiene `usuarios` y `user_profiles`; Usuarios tiene `usuarios` |
| **Problema** | `user_profiles` tiene **0 filas**, **ningún código la usa** y `auth_app` **no tiene privilegios** sobre ella. Es un diseño abandonado que duplica a `usuarios_db.usuarios`. El correo aparece en `auth.usuarios.email` y en `usuarios.usuarios.correo`, que pueden diferir |
| **Integridad** | Dos correos o dos DNI para la misma persona sin regla que los concilie |
| **Privacidad** | Superficie adicional de datos personales sin propósito |
| **Fuente de verdad propuesta** | Identidad de la persona: **maestro de personas** ([05](05-modelo-maestro-propuesto.md)). Credenciales y correo de acceso: **Auth**. Perfil laboral del personal: **Usuarios**, referenciando a la persona |
| **Acción** | Retirar `auth_db.user_profiles` con una migración y un respaldo previo. Definir si `usuarios.correo` es un dato de contacto distinto del correo de acceso o una copia |
| **Revisar antes** | Confirmar que ningún despliegue externo usa `user_profiles` (no aparece en el repositorio) |

### R-02 — Rol duplicado (confirmado)

| | |
|---|---|
| **Evidencia** | `usuarios_db.usuarios.tipo_usuario` (CHECK con 7 códigos más PSICOLOGO) frente a `auth_db.usuario_roles` + `roles`. En `gateway/internal/logic/updateuserrolelogic.go:38` el comentario dice "guarda una copia denormalizada (tipo_usuario)…", y la línea 67 **registra en el log un error de sincronización sin revertir** |
| **Problema** | Si falla la sincronización, el listado de usuarios muestra un rol distinto del que realmente autoriza. El CHECK de `tipo_usuario` se debe mantener a mano cada vez que se crea un rol |
| **Riesgo** | Integridad media (una sola escritura de dos fases sin transacción distribuida); seguridad baja (la autorización real usa Auth) |
| **Fuente de verdad** | **Auth** (`usuario_roles`) |
| **Acción** | Que el listado consulte el rol en Auth (en lote) o que la copia sea explícitamente una proyección reconstruible. Luego retirar el CHECK y la columna |
| **Revisar antes** | `ListUsuarios` filtra por `tipo_usuario`; ese filtro debe pasar a resolverse por rol en Auth |

### R-03 — DNI del usuario en dos bases con reglas distintas (confirmado)

| | |
|---|---|
| **Evidencia** | `auth.usuarios.dni` VARCHAR(8), CHECK `^[0-9]{8}$`, NULL permitido, único parcial · `usuarios.usuarios.dni` VARCHAR(20) sin formato, `''` = "no capturado", único donde `<> ''` |
| **Problema** | Dos fuentes para el identificador de login y el de perfil; no hay regla de consistencia |
| **Fuente de verdad** | El documento de identidad pertenece a la **persona** (maestro). Auth guarda el DNI solo como **identificador de inicio de sesión**, validado contra la persona vinculada |
| **Acción** | Vincular la cuenta a la persona (`persona_id`) y derivar o validar el DNI de login contra ella; retirar el DNI de `usuarios_db` |

### R-04 — Datos del solicitante en texto libre (confirmado)

| | |
|---|---|
| **Evidencia** | `frontend/src/services/futService.js:17` (`buildDescripcion`) escribe "Nombres y apellidos", "DNI", "Teléfono", "Domicilio", "Distrito" y "Correo" dentro de `expedientes.descripcion`; la línea 35 en adelante los **recupera con expresiones regulares** |
| **Problema** | Datos personales sin estructura, sin validación y sin control de acceso por campo, mezclados con la fundamentación. Cualquiera que lea el expediente los ve |
| **Justificación parcial** | Un expediente necesita una **instantánea** de lo declarado al presentarlo, con valor probatorio: no debe cambiar si la persona actualiza su perfil |
| **Fuente de verdad** | Identidad actual: el maestro o Auth. Lo declarado en el trámite: una **instantánea estructurada** propia de Expedientes |
| **Acción** | Tabla `expediente_solicitante_snapshot` (campos tipados, solo escritura al crear) y dejar de guardar datos personales en `descripcion`. Migrar los expedientes existentes **sin reescribir** la descripción original: se conserva como evidencia |

### R-05 — Datos académicos en transición (confirmado, controlado)

| | |
|---|---|
| **Evidencia** | `estudiantes.nivel/grado/seccion/turno/estado_matricula` (heredados, `''` = sin dato) y las tablas `secciones` y `matriculas` (migración 007, sin filas) |
| **Estado** | Doble estructura **deliberada** durante la transición. Hoy no hay datos en `matriculas`: la fuente vigente siguen siendo las columnas heredadas |
| **Riesgo** | Cuando existan escrituras de matrícula, las dos fuentes pueden divergir |
| **Acción** | Cargar las columnas heredadas en matrículas (requiere decidir el periodo), hacer que los RPC lean de la matrícula vigente y retirar las columnas en dos pasos ([08](08-plan-de-migracion.md), F4) |
| **Estado de matrícula** | `estudiantes.estado_matricula` (ACTIVO/RETIRADO) y `matriculas.estado_matricula` (MATRICULADO, RETIRADO, TRASLADADO, CONCLUIDO) comparten nombre pero **no significado**. Se debe documentar el mapeo al migrar |

### R-06 — Padres y apoderados: persona y cuenta sin vínculo (potencial)

| | |
|---|---|
| **Evidencia** | `estudiantes_db.personas` (padres o tutores, 0 filas) · Auth `Register` público crea cuentas **SOLICITANTE** con DNI (`services/auth/internal/logic/registerlogic.go:72`); en la práctica son padres o terceros que presentan trámites |
| **Problema** | El mismo padre puede existir como `persona` (Estudiantes) y como cuenta (Auth) sin ningún vínculo; su nombre y DNI quedarían en dos bases |
| **Fuente de verdad** | Persona: el **maestro**. Cuenta: **Auth**, con referencia `persona_id` |
| **Acción** | Vinculación explícita y verificada ([05](05-modelo-maestro-propuesto.md) §E y [10](10-diagramas.md), diagrama 6); nunca por coincidencia de nombres |

### R-07 — Contacto del estudiante frente al de la familia (potencial)

`estudiantes.correo` y `telefono` (migración 006) guardan un canal de contacto "del estudiante" que en la práctica suele ser el de su familia. Si la familia también se registra en `personas` con contacto propio, habría dos lugares. Es una **decisión pendiente** (ver [11](11-decisiones-arquitectonicas.md), ADR-07).

### R-08 — Áreas y roles como texto repetido (confirmado)

`expedientes.area_actual` (CHECK con códigos de rol), `derivaciones.origen/destino` (**texto libre**) y `usuarios.tipo_usuario` (CHECK) repiten los códigos de `auth.roles` sin catálogo común. Un rol nuevo exige cambiar varios CHECK; los textos libres no se pueden validar.

**Acción:** catálogo de áreas propio de Mesa de Partes, con código estable, y que `derivaciones` guarde códigos.

### P-09 — Referencias huérfanas entre bases (confirmado)

Verificación con UUID, sin leer datos:
- `expedientes.solicitante_id`: **11 de 12** valores distintos no existen en `auth.usuarios`.
- `documentos.expediente_id`: **5 de 26** no existen en `expedientes`.
- `derivaciones.expediente_id`: **6 de 23** no existen.
- Actores sin cuenta: 5 en `documentos.subido_por` y 13 en `derivaciones.registrado_por`.

**Causa no confirmada.** La hipótesis es que son restos de pruebas de integración (P-16).

**Riesgo:** en producción, documentos o derivaciones inaccesibles o atribuidos a cuentas inexistentes.

**Acción:**
- Limpiar el entorno de desarrollo, con respaldo y en un entorno aislado.
- Agregar comprobaciones periódicas de consistencia.
- Que las pruebas no dejen residuos.

### P-10 — Cuentas sin perfil y sin DNI (confirmado)

28 de 35 cuentas de Auth no tienen fila en `usuarios_db.usuarios`: el `Register` público no crea perfil. 27 no tienen DNI y, por diseño de la migración 009, **no pueden iniciar sesión**.

**Acción:** decidir si las cuentas SOLICITANTE necesitan perfil (probablemente no, si la persona vive en el maestro) y depurar las cuentas de prueba.

### P-11 — Los expedientes no se vinculan al estudiante (confirmado)

Trámites como certificados o constancias de estudios, permisos o justificaciones se refieren a un estudiante, pero `expedientes` no tiene `estudiante_id`. Hoy el estudiante solo aparecería en el texto libre.

**Acción:** `estudiante_id` UUID opcional, validado con `Estudiantes.ResolveEstudiante` o `GetEstudiante`, para los tipos que lo requieran.

### P-12 — Archivos binarios en PostgreSQL (confirmado)

`documentos.contenido` BYTEA (33 filas). Pueden contener documentos de identidad escaneados.

- **Consecuencia:** los respaldos crecen con los archivos; no hay cifrado de aplicación ni retención diferenciada.
- **Decisión pendiente (ADR-10):** mantener en BYTEA con cifrado, o pasar a almacenamiento de objetos con referencia.

### P-13 — Sin registro de migraciones (confirmado)

Ninguna base tiene tabla de control. La aplicación es manual (`SET ROLE <s>_owner` + `psql`), así que no se puede saber qué versión tiene cada entorno sin comparar el esquema.

### P-14 — Privilegio DELETE innecesario (confirmado)

`usuarios_user`, `expedientes_user`, `documentos_user` y `derivaciones_user` tienen DELETE, pero el código no borra físicamente: sus bajas son lógicas. Viola el mínimo privilegio y facilita la pérdida de historial si se compromete el servicio.

**Acción:** revocar DELETE, con verificación previa en las pruebas.

### P-15 — `refresh_tokens` sin limpieza (confirmado)

Hay 384 filas para 35 cuentas y no se encontró un proceso de purga de tokens expirados o revocados.

**Acción:** política de retención (por ejemplo, borrar los expirados más de 30 días) mediante un proceso de mantenimiento con su propio rol.

### P-16 — Tests que escriben en la base de otro servicio (confirmado)

`services/{expedientes,documentos,derivaciones}/internal/integration/haspermission_*_integration_test.go` insertan y borran cuentas y roles **directamente en `auth_db`**. Esto no ocurre en el código productivo, pero acopla las pruebas al esquema de Auth y es la causa probable de P-09.

### P-17 — Sin `persona_id` global (confirmado)

- Un estudiante se identifica con `estudiantes.id`; una cuenta, con `auth.usuarios.id`; un padre, con `personas.id`.
- Una misma persona física (por ejemplo, un docente que también es padre, o un exalumno que ahora es apoderado) tendría identidades separadas, sin forma de reconocerla como una sola.

### P-18 — Convenciones de ausencia inconsistentes (confirmado)

- `usuarios_db` usa `''` como "no capturado" (dni, teléfono, correo, dirección) y `DEFAULT ''`. Estudiantes usa NULL.
- `usuarios.dni` no valida formato.

Esto dificulta la unicidad y las integraciones.

## Resumen de fuentes de verdad propuestas

| Dato | Fuente de verdad | Otros lugares (situación futura) |
|---|---|---|
| Identidad de una persona (documento, nombres) | Maestro (`personas`) | Proyecciones mínimas y documentadas; instantáneas en trámites |
| Estudiante (código, SIAGIE, estado) | Maestro (`estudiantes` → `persona_id`) | Solo `estudiante_id` en otros servicios |
| Trayectoria académica | Maestro (`matriculas`) | — |
| Familia, apoderado y autorizaciones | Maestro (`estudiante_familiares`) | — |
| Cuenta, credenciales, roles y permisos | Auth | — |
| Perfil laboral del personal | Usuarios (referencia `persona_id`) | — |
| Egreso | Maestro (derivado de la matrícula más la validación) | — |
| Lo declarado en un trámite | Expedientes (instantánea inmutable) | — |
| Datos clínicos | Psicología (cifrado) | Nunca fuera de Psicología |
