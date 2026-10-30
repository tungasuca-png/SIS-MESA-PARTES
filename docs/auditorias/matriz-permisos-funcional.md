# Matriz funcional de permisos — Estudiantes y Psicología

| Dato | Valor |
|---|---|
| Fecha de la auditoría | 2026-10-21 |
| Rama auditada | `feat/integracion-estudiantes` (último commit `c6d9160`) |
| Estado de Git | Árbol limpio; sin cambios sin confirmar al iniciar |
| Tipo de trabajo | Auditoría y documentación. **No se cambió** ninguna asignación, dato, migración ni código |

Convenciones:

- **Verificado:** comprobado en el código o en datos reales (se indica la fecha si no es de hoy).
- **Acordado:** decisión funcional confirmada.
- **Propuesto:** recomendación todavía no aprobada.
- **Pendiente:** requiere información o decisión adicional.

---

## 1. Alcance y restricciones

- **Módulos:** Estudiantes (maestro de estudiantes, datos personales, familia, matrícula) y Psicología (expedientes y atenciones), con sus dependencias en Auth, Gateway y frontend.
- **Solo lectura:** no se ejecutó SQL de escritura, no se asignaron ni retiraron permisos, no se crearon migraciones, no se tocó código ni pruebas, y no hubo commits ni cambios de rama.
- **Sin datos clínicos ni personales reales** en este documento.

## 2. Estado técnico verificado

| Elemento | Estado |
|---|---|
| Rama y commits | **Verificado.** La rama contiene las etapas 5 y 6 de Estudiantes (matrícula, personas, historial), la ficha integral, las integraciones con Psicología, el saneamiento de errores del Gateway, el bloqueo de login informado y Mesa de Partes en vivo |
| PostgreSQL | **No disponible hoy:** Docker Desktop y el puerto 5433 no responden. No se inició Docker para no alterar el sistema |
| Servicios (Auth 8080, Estudiantes 8087, Gateway 8888, frontend 5173) | **No disponibles hoy** (ningún puerto escucha) |
| Última verificación real de asignaciones | **Verificado el 2026-10-20** con `TestPermisosEstudiantesPsicologiaPorRol` (`backend/services/auth/internal/integration/permisos_estudiantes_psicologia_integration_test.go`). Llama a `HasPermission` por RPC real y la contrasta con `role_permissions` vivos, con un usuario temporal por rol. **Esos datos no se pudieron re-verificar hoy** |
| Mecanismo de asignación | **Verificado en código.** Las migraciones de Auth (`010`, `012`, `014`–`018`) **solo registran** permisos y nunca los asignan. Se asignan y revocan desde la pantalla **Configuración → Roles y permisos** (`UpdateRolePermissions`, protegido por `roles.view`) |
| Autorización | **Verificado.** El Gateway no autoriza: reenvía el JWT. Cada servicio valida el JWT y llama a `Auth.HasPermission`. Si Auth falla, ambos servicios deniegan el acceso (fail-closed) |

## 3. Matriz funcional acordada

### 3.1 Psicología (Acordado)

- `PSICOLOGO` debe **consultar todos** los expedientes psicológicos y sus atenciones.
- Consultar no implica escribir: crear, editar, corregir o anular siguen protegidos por sus propios permisos.
- Se mantienen el alcance, la auditoría y la protección de datos clínicos.
- `ADMIN` **conserva** sus permisos clínicos actuales.

### 3.2 Estudiantes (Acordado, salvo DOCENTE)

| Rol | Consulta | Registro y edición |
|---|---|---|
| ADMIN | Sí | Sí |
| DIRECTOR | Sí | No |
| SUBDIRECTOR | Sí | No |
| SECRETARIA | Sí | Sí |
| AUXILIAR | Sí | No |
| DOCENTE | **Pendiente** | No |
| SOLICITANTE | No | No |

### 3.3 Datos personales y familiares (Acordado como punto de partida)

| Rol | Consultar | Modificar |
|---|---|---|
| ADMIN | Sí | Sí |
| SECRETARIA | Sí | Sí |
| DIRECTOR | Sí | No |
| SUBDIRECTOR | Sí | No |
| AUXILIAR | No | No |
| DOCENTE | No por defecto (**Pendiente**) | No |
| SOLICITANTE | No | No |

### 3.4 Matrícula (Acordado: fuera del alcance actual)

`estudiantes.matricula.update` no se asigna, y en esta fase no se diseña ni implementa nada de matrícula. Ver §6.4 para el estado real.

## 4. Asignaciones reales en PostgreSQL

**Verificado el 2026-10-20; no re-verificado hoy** (base no disponible).

- **Roles activos (8):** ADMIN, AUXILIAR, DIRECTOR, DOCENTE, PSICOLOGO, SECRETARIA, SOLICITANTE, SUBDIRECTOR.
- **Catálogo:** 41 permisos.
- **Hallazgo principal:** solo `ADMIN` tiene permisos de estudiantes o de psicología. Los otros 7 roles, incluido `PSICOLOGO`, tienen **0**.

| Permiso | Activo | Roles asignados (2026-10-20) |
|---|---|---|
| `estudiantes.view` | Sí | ADMIN |
| `estudiantes.create` | Sí | ADMIN |
| `estudiantes.update` | Sí | ADMIN |
| `estudiantes.change_estado` | Sí | ADMIN |
| `estudiantes.personal.view` | Sí | ADMIN |
| `estudiantes.personal.update` | Sí | ADMIN |
| `estudiantes.familia.view` | Sí | ADMIN |
| `estudiantes.familia.update` | Sí | ADMIN |
| `estudiantes.matricula.update` | Sí | **ninguno** |
| `psicologia.expedientes.view` | Sí | ADMIN |
| `psicologia.expedientes.view_own` | Sí | ADMIN |
| `psicologia.expedientes.create` | Sí | ADMIN |
| `psicologia.expedientes.update` | Sí | ADMIN |
| `psicologia.atenciones.view` | Sí | ADMIN |
| `psicologia.atenciones.create` | Sí | ADMIN |
| `psicologia.atenciones.correct` | Sí | ADMIN |
| `psicologia.atenciones.annul` | Sí | ADMIN |
| `psicologia.view` | Sí | ADMIN (**ningún código lo usa**) |
| `psicologia.estudiantes.view` | Sí | ADMIN (**ningún código lo usa**) |

## 5. Operaciones protegidas (verificado en código)

### 5.1 Estudiantes Service

Lo valida `autorizar()` → `Auth.HasPermission` en `backend/services/estudiantes/internal/logic`. Las rutas REST cuelgan de `/api/estudiantes`.

| Operación (RPC → REST) | Permiso exigido | Notas |
|---|---|---|
| SearchEstudiantes → `GET /` | `estudiantes.view` | Solo identidad básica |
| GetEstudiante → `GET /:id`, GetEstudiantesBasic → `POST /basic-batch`, ResolveEstudiante → `POST /resolver` | `estudiantes.view` | Psicología también los usa (§6.1) |
| ListHistorialEstudiante → `GET /:id/historial` | `estudiantes.view` | Devuelve nombres de campos e `usuario_id` de quien editó, nunca valores (ver riesgo R4) |
| ListMatriculasEstudiante, ListPeriodos, ListSecciones, ListMatriculasPeriodo | `estudiantes.view` | Lectura de la trayectoria |
| CreateEstudiante → `POST /` | `estudiantes.create` **+ `estudiantes.personal.update`** si se envían datos personales | Dependencia agrupada |
| UpdateEstudiante → `PATCH /:id` | `estudiantes.update` | No admite datos personales |
| ListPosiblesDuplicados → `GET /duplicados` | `estudiantes.update` | **Dependencia:** revisar duplicados está agrupado con «editar» |
| ChangeEstadoEstudiante → `PATCH /:id/estado` | `estudiantes.change_estado` | Activar o inactivar. Permiso propio, distinto de editar |
| GetEstudiantePersonal → `GET /:id/personal` | `estudiantes.personal.view` | Cada lectura se audita (PERSONAL_VIEW) |
| UpdateEstudiantePersonal → `PATCH /:id/personal` | `estudiantes.personal.update` | Incluye domicilio y contacto del estudiante |
| ListFamiliaresEstudiante, GetPersona, SearchPersonas | `estudiantes.familia.view` | Lecturas auditadas (FAMILIA_VIEW, PERSONA_VIEW) |
| VincularFamiliar, DesvincularFamiliar, ActualizarFamiliar, CambiarParentescoFamiliar, CreatePersona, UpdatePersona | `estudiantes.familia.update` | Auditado; nunca se borra |
| Periodos, secciones y operaciones de matrícula (9 RPC de escritura) | `estudiantes.matricula.update` | Ver §6.4 |

**Separación verificada:** `estudiantes.view` **no** concede datos personales ni familiares. Esos endpoints exigen sus permisos propios, y `GetEstudiante` y `SearchEstudiantes` no devuelven esos campos.

### 5.2 Psicología Service

Lo valida `backend/services/psicologia/internal/logic`.

| Operación | Permiso de acción | Alcance |
|---|---|---|
| GetExpediente, GetExpedienteByEstudiante, ListExpedientes | — | `psicologia.expedientes.view` = todos; `view_own` = solo donde el usuario es responsable. Sin ninguno: 403. Fuera de alcance: 404 sin revelar existencia, y se registra DENIED |
| OpenExpediente | `psicologia.expedientes.create` | Además verifica al estudiante en Estudiantes con el JWT del usuario (§6.1) |
| UpdateExpediente, CloseExpediente, ReopenExpediente | `psicologia.expedientes.update` | Exige además alcance (`loadInScope`) |
| GetAtencion, ListAtenciones, ListAtencionVersions, GetAtencionVersion, ExportAtencionPdf | `psicologia.atenciones.view` | Más el alcance del expediente. Los borradores de otro autor no se ven |
| CreateAtencionDraft, UpdateAtencionDraft, RegisterAtencion, DiscardAtencionDraft | `psicologia.atenciones.create` | Más alcance |
| CorrectAtencion | `psicologia.atenciones.correct` | Más alcance |
| AnnulAtencion | `psicologia.atenciones.annul` | Más alcance |

- **Respuestas** (verificado en código y en pruebas de integración):
  - 401 sin JWT.
  - 403 sin permiso de acción o sin ningún alcance.
  - 404 si no existe o está fuera del alcance.
  - Auth caído: se deniega sin entregar datos ni registrar auditoría.
- **Auditoría:** cada lectura se registra antes de responder; si la bitácora falla, no se entrega el dato.
- **Contenido clínico:** cifrado AES-256-GCM. La auditoría y los logs no lo contienen.
- **Resumen del caso:** `resumen_caso` se entrega en `GetExpediente*` a quien tenga alcance (`view` o `view_own`), aunque no tenga `atenciones.view`. Ver riesgo R3.

### 5.3 Frontend (verificado)

- **Rutas:**
  - `/estudiantes`, `/estudiantes/:id` y `/estudiantes/periodos` exigen `estudiantes.view` (`PermissionRoute`).
  - Las rutas de Psicología exigen `psicologia.expedientes.view` o `view_own`.
  - Las atenciones exigen `psicologia.atenciones.*`.
- **Ficha del estudiante:**
  - Datos personales: solo se consultan con `personal.view`.
  - Familia: solo con `familia.view`; sin él, la pestaña muestra un mensaje y no consulta.
  - Psicología: sin permiso de expedientes no consulta nada; las atenciones solo con `atenciones.view`.
- **Controles de edición:** solo se muestran con su permiso. Es una ayuda de interfaz: **la autorización real está en el backend** y un 403 se traduce a un mensaje.
- **Gateway:** no autoriza. Los 5xx se devuelven saneados al navegador (`internal/errorsafe`).

## 6. Diferencias entre lo acordado y lo implementado

### 6.1 Psicología

| Acordado | Situación real | Clasificación |
|---|---|---|
| PSICOLOGO consulta todos los expedientes y atenciones | Tiene 0 permisos. Hacen falta `psicologia.expedientes.view` (el que reconoce el backend como alcance global) y `psicologia.atenciones.view` | Diferencia (Verificado 2026-10-20) |
| Lo anterior **funciona en el frontend** | **Dependencia oculta:** abrir un expediente (`OpenExpediente`) verifica al alumno en Estudiantes **con el JWT del psicólogo**, y el buscador de alumnos y los nombres en los listados también llaman a Estudiantes. **Sin `estudiantes.view`, el psicólogo ve los expedientes pero sin nombres y no puede abrir expedientes nuevos (403)** | **Pendiente:** decidir si PSICOLOGO recibe `estudiantes.view` |
| Escritura con permisos propios | Así ocurre en el backend (§5.2) | Conforme |
| ADMIN conserva permisos clínicos | Los tiene | Conforme |
| `view` y `view_own` | Si un rol tiene ambos, prevalece `view` (alcance global) | Verificado; asignar `view_own` al PSICOLOGO sería redundante si recibe `view` |

### 6.2 Estudiantes (consulta y edición)

| Rol | Acordado | Real (2026-10-20) | Permisos a asignar en el futuro (Propuesto) |
|---|---|---|---|
| ADMIN | Consulta y edición | Todo, salvo matrícula | Ninguno |
| DIRECTOR | Consulta | Nada | `estudiantes.view` |
| SUBDIRECTOR | Consulta | Nada | `estudiantes.view` |
| SECRETARIA | Consulta y edición | Nada | `estudiantes.view`, `create`, `update`; `change_estado` **pendiente** (§7) |
| AUXILIAR | Consulta | Nada | `estudiantes.view` |
| DOCENTE | Pendiente | Nada | Ninguno hasta decidir |
| SOLICITANTE | Ninguno | Nada | Ninguno (lo vigila una invariante de la prueba de Auth) |

### 6.3 Datos personales y familiares

Personal y familia son **permisos separados** (`personal.*` y `familia.*`). La matriz los agrupa; la implementación permitiría separarlos si se quisiera.

| Rol | Acordado | Propuesto |
|---|---|---|
| ADMIN | Consultar y modificar | Ya los tiene |
| SECRETARIA | Consultar y modificar | `personal.view`, `personal.update`, `familia.view`, `familia.update` |
| DIRECTOR | Consultar | `personal.view`, `familia.view` |
| SUBDIRECTOR | Consultar | `personal.view`, `familia.view` |
| AUXILIAR | Ninguno | Ninguno (verá solo la identidad básica con `estudiantes.view`) |
| DOCENTE | Pendiente | Ninguno |
| SOLICITANTE | Ninguno | Ninguno |

**Dependencia:** si SECRETARIA registra estudiantes con datos personales en el alta, necesita `estudiantes.create` + `estudiantes.personal.update`. Con la propuesta anterior los tendría.

### 6.4 Matrícula: estado real

- **Verificado en código:** la funcionalidad **está implementada dentro de Estudiantes Service** en esta rama. No es un servicio aparte.
  - Tablas `periodos_academicos`, `secciones`, `matriculas` y `matricula_movimientos` (migraciones 007 y 009).
  - 9 RPC de escritura y 4 de lectura.
  - Rutas en el Gateway.
  - Pestaña «Historial académico» y página «Periodos y secciones» en el frontend.
  - Pruebas de integración contra PostgreSQL (`TestEtapa5`).
- **Verificado el 2026-10-20:** las 4 tablas tienen **0 filas** y ningún rol tiene `estudiantes.matricula.update`.
- **Conclusión:** existe código probado, pero **no hay uso operativo ni validación funcional del colegio**. El permiso no prueba nada por sí solo, y el código tampoco reemplaza la confirmación funcional (reglas de periodos, vacantes, conclusión y egreso).
- **Acordado:** no asignar el permiso y dejar las decisiones de matrícula para una fase posterior. Mientras tanto, cualquier usuario con `estudiantes.view` puede **ver** la página de periodos y la trayectoria, que estarán vacías.

## 7. Riesgos de seguridad identificados

| ID | Riesgo | Evidencia | Clasificación |
|---|---|---|---|
| R1 | Todo el uso real depende de cuentas ADMIN: concentración de privilegios y operación con la cuenta más poderosa | §4 | Verificado (2026-10-20) |
| R2 | Permisos sin uso asignados a ADMIN (`psicologia.view`, `psicologia.estudiantes.view`; también `seguimiento.view_own`, fuera de este alcance): confunden la administración de roles | Ningún código los consulta | Verificado |
| R3 | `resumen_caso` (dato clínico) llega a quien tiene alcance de expediente aunque no tenga `atenciones.view`. La ficha no lo muestra, pero la API sí lo entrega | §5.2 y diseño de Psicología | Verificado; decisión **Pendiente** |
| R4 | `GET /api/estudiantes/:id/historial` devuelve a cualquier usuario con `estudiantes.view` (p. ej. AUXILIAR) quién editó qué campos y cuándo. La interfaz ya no lo muestra, pero el endpoint sigue abierto a ese permiso | §5.1 | Verificado; **Propuesto** restringirlo |
| R5 | Darle `estudiantes.view` al PSICOLOGO, necesario para su flujo, le da también acceso al directorio completo (solo identidad básica) | §6.1 | **Pendiente** |
| R6 | Ocultar botones no es autorización. Hoy se cumple porque el backend valida cada operación; debe mantenerse así | §5 | Verificado (conforme) |
| R7 | La matriz agrupa «edición», pero el backend separa `update`, `change_estado` y la revisión de duplicados (atada a `update`) | §5.1 | Verificado; decisión **Pendiente** |

## 8. Decisiones pendientes

1. **DOCENTE:** ¿consulta estudiantes (`estudiantes.view`)? ¿Necesita datos personales o familiares?
2. **PSICOLOGO y `estudiantes.view`:** necesario para abrir expedientes y ver nombres (R5).
3. **PSICOLOGO, escritura clínica:** ¿recibe `psicologia.expedientes.create` y `update` y `psicologia.atenciones.create`, `correct` y `annul`? Lo acordado solo cubre la consulta.
4. **SECRETARIA y `estudiantes.change_estado`:** ¿activar e inactivar forma parte de «edición»?
5. **Revisión de duplicados:** ¿basta con quien tenga `estudiantes.update`?
6. **`resumen_caso`** (R3): ¿debe exigir `psicologia.atenciones.view`? Afecta al módulo de Psicología.
7. **Historial de cambios** (R4): ¿se restringe el endpoint?
8. **Permisos sin uso** (R2): ¿desactivarlos? Hoy ningún código los consulta.
9. **Matrícula:** validación funcional y responsables (fase posterior).

## 9. Propuesta de implementación futura (no ejecutada)

Todo se haría desde **Configuración → Roles y permisos** (mecanismo oficial), sin SQL manual y con un respaldo previo de `auth_db`. La propuesta depende de las decisiones del §8.

| Rol | Permisos a asignar |
|---|---|
| PSICOLOGO | `psicologia.expedientes.view`, `psicologia.atenciones.view`; según §8.2 y §8.3: `estudiantes.view` y los de escritura clínica |
| SECRETARIA | `estudiantes.view`, `create`, `update`, `personal.view`, `personal.update`, `familia.view`, `familia.update`; según §8.4: `change_estado` |
| DIRECTOR, SUBDIRECTOR | `estudiantes.view`, `personal.view`, `familia.view` |
| AUXILIAR | `estudiantes.view` |
| DOCENTE | Ninguno hasta §8.1 |
| ADMIN | Sin cambios |
| Todos | **No** asignar `estudiantes.matricula.update` |

- **Cambios de código, solo si se aprueban:** R3 (afecta a Psicología) y R4 (permiso más estricto para el historial).
- **Prueba de Auth:** las asignaciones propuestas **no requieren cambiarla**. La prueba lee las asignaciones vivas, la lista `rolesClinicosPermitidos` ya incluye PSICOLOGO y ADMIN, y SOLICITANTE no recibe nada. Sí conviene **ampliarla** con las invariantes de §10.

## 10. Pruebas necesarias para validar los cambios futuros

Ampliar `TestPermisosEstudiantesPsicologiaPorRol`, que es de solo lectura y crea y borra solo sus usuarios temporales, con estas invariantes:

1. **AUXILIAR:** sin `personal.*` ni `familia.*`.
2. **DIRECTOR y SUBDIRECTOR:** sin permisos de escritura de estudiantes.
3. **Escritura de estudiantes, personal y familia:** solo en ADMIN y SECRETARIA.
4. **`estudiantes.matricula.update`:** sin ningún rol mientras siga fuera de alcance.
5. **PSICOLOGO:** con `psicologia.expedientes.view` y `psicologia.atenciones.view`.
6. **DOCENTE:** según la decisión §8.1.

Además:

- **Psicología, ya cubierta por su suite de integración:**
  - Alcance general frente al propio.
  - 404 sin revelar existencia.
  - 403 sin `atenciones.view`.
  - Auth caído.
  - Rechazo de Estudiantes cuando falta `estudiantes.view`.
- **E2E autenticado** con una cuenta de prueba por rol, tras reasignar:
  - Lecturas permitidas.
  - Escrituras rechazadas sin rastro (403/400).
  - La pestaña Psicología de la ficha con PSICOLOGO.
  - Abrir un expediente con y sin `estudiantes.view`.
- **Antes de ejecutar pruebas que escriben:** confirmar que apuntan al entorno de pruebas (`backend/.env.test`) y que limpian lo que crean, como ya se comprobó el 2026-10-20 con conteos antes y después.

## 11. Limitaciones de esta auditoría

- **PostgreSQL no disponible el 2026-10-21:** las asignaciones de §4 son las verificadas el 2026-10-20 y pueden haber cambiado desde entonces por la pantalla de Roles. Hay que re-verificarlas antes de aplicar cambios.
- **Servicios apagados:** no hubo verificación HTTP ni gRPC en vivo hoy.
- **No se ejecutaron pruebas** en esta auditoría (restricción de solo lectura). Las referencias a pruebas se basan en su código y en la última ejecución registrada (2026-10-20).
- **No se consultó** el número de usuarios por rol.

## 12. Actualización 2026-10-21: implementación autorizada (sin commit)

- **R3 corregido (Psicología).** El resumen del caso y el motivo de cierre solo se entregan con `psicologia.atenciones.view` en Get, GetByEstudiante, Open, Update, Close y Reopen. Escribir el resumen (al abrir o editar) sin ese permiso responde 403 y registra DENIED. El estado y el responsable se siguen editando con `psicologia.expedientes.update`. El frontend no muestra ni envía el resumen sin el permiso.
- **R4 corregido (Estudiantes).** `GET /api/estudiantes/:id/historial` exige ahora `estudiantes.update`; `estudiantes.view` ya no basta. La interfaz no consumía ese endpoint.
- **«Roles y permisos»:** ya permite asignar cualquier permiso del catálogo a cualquier rol, PSICOLOGO incluido, de forma atómica. **No requirió cambios.**
- **Prueba de Auth ampliada:** ahora vigila la matriz aprobada.
  - PSICOLOGO sin datos personales, familiares ni escritura de Estudiantes.
  - AUXILIAR sin datos sensibles.
  - DIRECTOR, SUBDIRECTOR y DOCENTE sin escritura.
  - Ningún rol con `estudiantes.matricula.update`.
  - ADMIN con sus 7 permisos clínicos.
- **Sin cambios en:** asignaciones, migraciones ni matrícula. Las asignaciones se aplican desde la interfaz cuando el entorno esté disponible.

## 13. Archivos consultados

- **Auth:**
  - `backend/services/auth/migrations/010`, `012`, `014`–`018`.
  - `internal/interceptor/interceptor.go`.
  - `internal/integration/permisos_estudiantes_psicologia_integration_test.go`.
- **Estudiantes:** `backend/services/estudiantes/internal/logic/*.go` (32 archivos de RPC).
- **Psicología:**
  - `internal/logic/{common,readlogic,writelogic,atencionlogic}.go`.
  - `internal/svc/estudianteverifier.go`.
  - `internal/integration/*.go`.
- **Gateway:** `gateway.api`, `internal/errorsafe`.
- **Frontend:**
  - `App.jsx`, `components/dashboard/navItems.js`, `EstudianteNombre.jsx`, `Header.jsx`.
  - `components/estudiantes/**`, `pages/Estudiantes/**`, `pages/Psicologia/**`.
- **Documentación:** `docs/modulo-psicologia-diseno.md`, `docs/etapa-12-especificacion-funcional.md`.
