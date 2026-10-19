# Estudiantes Service

Maestro escolar de la I.E. Tungasuca. Es la única fuente de verdad de:

- identidad del estudiante;
- datos personales;
- familia (padres, apoderados y otros familiares);
- periodos académicos, secciones y matrículas, con su historial.

Los demás sistemas (Psicología, Mesa de Partes y los que vengan) guardan solo el
**UUID estable** del estudiante y consultan aquí lo mínimo, siempre por gRPC o
por el Gateway, nunca leyendo la base.

## Qué NO hace

- No autentica ni guarda usuarios, contraseñas, roles ni permisos: eso es de
  **Auth Service**. Cada RPC valida el JWT y pregunta el permiso a Auth
  (`HasPermission`); si Auth no responde, se deniega.
- No guarda notas, asistencia, documentos ni información clínica.
- No borra nada: no hay `DELETE` en la API ni privilegio `DELETE` para la
  aplicación. Los retiros, traslados y desvinculaciones cierran registros y
  conservan el historial.
- No decide aprobación, promoción ni egreso. Esas reglas académicas no están
  definidas por el colegio (ver «Decisiones pendientes»).

## Puerto y base de datos

| Recurso | Valor |
|---|---|
| gRPC | `127.0.0.1:8087` (solo loopback: datos de menores de edad) |
| Base de datos | `estudiantes_db` (PostgreSQL en `127.0.0.1:5433`) |
| Usuario de la aplicación | `estudiantes_user`: `SELECT`/`INSERT`/`UPDATE` según la tabla, **nunca `DELETE`** |
| Registro etcd | `estudiantes.rpc` |

## Variables de entorno

| Variable | Uso |
|---|---|
| `JWT_SECRET` | Obligatoria. El mismo secreto que Auth Service. |
| `ESTUDIANTES_DB_PASSWORD` | Obligatoria. Contraseña de `estudiantes_user` (en `backend/.env`, nunca en el YAML). |

## Cómo ejecutar

Todo el backend: `backend\run-all.ps1`; para detenerlo, `backend\stop-all.ps1`.

Solo este servicio:

```bash
cd backend/services/estudiantes
go run estudiantes.go -f etc/estudiantes.yaml   # con las variables de backend/.env cargadas
```

## Migraciones

Se aplican en orden con el migrador actuando como dueño. Nunca con
`estudiantes_user` ni con un superusuario (ver `backend/db/roles/README.md`).
Antes de aplicar una, se toma un respaldo cifrado de la base.

```bash
(echo "SET ROLE estudiantes_owner;"; cat migrations/NNN_xxx.sql) | \
  docker exec -i tg-postgres psql -h 127.0.0.1 -U estudiantes_migrator -d estudiantes_db -v ON_ERROR_STOP=1
```

| Migración | Contenido |
|---|---|
| 001 | Tabla `estudiantes` (base) |
| 002 | Identidad: documento, código SIAGIE, apellidos separados, catálogo de tipos de documento |
| 003 | Búsqueda normalizada (sin tildes) con índice de trigramas |
| 004 | `auditoria_estudiantes` (solo inserción, sin valores) y control de versión |
| 005 | Auditoría de lectura y edición de datos personales |
| 006 | Domicilio, contacto y edad calculada |
| 007 | Periodos, secciones, matrículas y movimientos (solo inserción) |
| 008 | Familia: `personas`, `estudiante_familiares` y catálogo de parentescos |
| 009 | Escritura de matrículas, movimiento `CONCLUSION`, auditoría de personas (`persona_id`) |

Los permisos se registran en el catálogo de Auth con sus propias migraciones:
`auth/migrations/015–018`. **No se asignan a ningún rol**; la asignación se
hace desde la pantalla **Roles y permisos**.

## API gRPC y REST (Gateway)

En la tabla, los RPC de escritura llevan *(W)*. Todos los PATCH de escritura
exigen `expected_version`; si el registro cambió, responden `version_conflict`.

| RPC | REST (`/api/estudiantes…`) | Permiso |
|---|---|---|
| SearchEstudiantes | `GET /` | `estudiantes.view` |
| GetEstudiante | `GET /:id` | `estudiantes.view` |
| GetEstudiantesBasic | `POST /basic-batch` | `estudiantes.view` |
| ResolveEstudiante | `POST /resolver` (código, código SIAGIE o documento) | `estudiantes.view` |
| CreateEstudiante *(W)* | `POST /` | `estudiantes.create` (+ `personal.update` si trae datos personales) |
| UpdateEstudiante *(W)* | `PATCH /:id` | `estudiantes.update` |
| ChangeEstadoEstudiante *(W)* | `PATCH /:id/estado` (ACTIVO/INACTIVO) | `estudiantes.change_estado` |
| ListPosiblesDuplicados | `GET /duplicados` | `estudiantes.update` |
| GetEstudiantePersonal | `GET /:id/personal` | `estudiantes.personal.view` (auditada) |
| UpdateEstudiantePersonal *(W)* | `PATCH /:id/personal` | `estudiantes.personal.update` |
| ListFamiliaresEstudiante | `GET /:id/familiares` | `estudiantes.familia.view` (auditada) |
| VincularFamiliar *(W)* | `POST /:id/familiares` | `estudiantes.familia.update` |
| ActualizarFamiliar *(W)* | `PATCH /:id/familiares/:relacion_id` (rol de apoderado) | `estudiantes.familia.update` |
| DesvincularFamiliar *(W)* | `PATCH /:id/familiares/:relacion_id/desvincular` | `estudiantes.familia.update` |
| SearchPersonas | `GET /personas?q=` | `estudiantes.familia.view` |
| CreatePersona *(W)* | `POST /personas` | `estudiantes.familia.update` |
| GetPersona | `GET /personas/:persona_id` (con sus estudiantes) | `estudiantes.familia.view` (auditada) |
| UpdatePersona *(W)* | `PATCH /personas/:persona_id` | `estudiantes.familia.update` |
| ListPeriodos | `GET /periodos` | `estudiantes.view` |
| CreatePeriodo *(W)* | `POST /periodos` | `estudiantes.matricula.update` |
| ChangeEstadoPeriodo *(W)* | `PATCH /periodos/:periodo_id/estado` | `estudiantes.matricula.update` |
| ListSecciones | `GET /periodos/:periodo_id/secciones` | `estudiantes.view` |
| CreateSeccion *(W)* | `POST /periodos/:periodo_id/secciones` | `estudiantes.matricula.update` |
| ListMatriculasPeriodo | `GET /periodos/:periodo_id/matriculas` (nómina) | `estudiantes.view` |
| ListMatriculasEstudiante | `GET /:id/matriculas` (historial) | `estudiantes.view` |
| MatricularEstudiante *(W)* | `POST /:id/matriculas` | `estudiantes.matricula.update` |
| CambiarSeccionMatricula *(W)* | `PATCH /matriculas/:matricula_id/seccion` | `estudiantes.matricula.update` |
| RetirarMatricula *(W)* | `PATCH /matriculas/:matricula_id/retiro` (RETIRO o TRASLADO) | `estudiantes.matricula.update` |
| ReingresarMatricula *(W)* | `PATCH /matriculas/:matricula_id/reingreso` | `estudiantes.matricula.update` |
| ConcluirMatricula *(W)* | `PATCH /matriculas/:matricula_id/conclusion` | `estudiantes.matricula.update` |

### Reglas de matrícula

Son reglas de integridad, no reglas académicas:

- **Periodo:** `PLANIFICADO → EN_CURSO → CERRADO`, solo hacia adelante. Un
  periodo `CERRADO` no admite secciones ni cambios de matrícula; la única
  excepción es concluirla.
- **Matrícula:**
  - Solo de estudiantes con registro `ACTIVO`.
  - Una por estudiante y periodo.
  - Si tiene sección, la sección debe ser del mismo periodo.
  - Si la sección tiene un límite de vacantes, se respeta. Las altas
    simultáneas se serializan con un bloqueo de fila.
- **Transiciones:**
  - `MATRICULADO → RETIRADO | TRASLADADO | CONCLUIDO`.
  - `RETIRADO | TRASLADADO → MATRICULADO` (reingreso).
  - Cada cambio deja un movimiento en `matricula_movimientos`, que es solo de
    inserción.
- **Fechas:** no pueden ser futuras ni anteriores a la matrícula o al retiro
  registrado.
- **Historial:** `ListMatriculasEstudiante` devuelve la trayectoria completa:
  una matrícula por periodo, con todos sus movimientos.

### Errores

Cada mensaje empieza con un código estable seguido de `:`. Nunca incluye
valores.

| Código gRPC → HTTP | Códigos |
|---|---|
| InvalidArgument → 400 | `validation_error`, `invalid_document`, `invalid_parentesco`, `invalid_date` |
| Unauthenticated → 401 | (sin token o token inválido) |
| PermissionDenied → 403 | `permission_denied` |
| NotFound → 404 | `student_not_found`, `persona_not_found`, `relacion_not_found`, `periodo_not_found`, `seccion_not_found`, `matricula_not_found` |
| AlreadyExists → 409 | `duplicate_codigo`, `duplicate_document`, `duplicate_codigo_siagie`, `duplicate_persona`, `duplicate_familiar`, `duplicate_progenitor`, `duplicate_apoderado`, `duplicate_periodo`, `duplicate_seccion`, `duplicate_matricula` |
| Aborted → 409 | `version_conflict`, `persona_conflict` |
| FailedPrecondition → 400 | `invalid_state`, `invalid_transition`, `periodo_cerrado`, `sin_vacantes`, `estudiante_inactivo`, `relacion_no_vigente`, `persona_mismatch`, `no_changes` |

## Auditoría y privacidad

- **`auditoria_estudiantes`:**
  - Solo inserción: un trigger impide `UPDATE`, `DELETE` y `TRUNCATE`.
  - Registra quién, qué acción, sobre qué estudiante o persona y los
    **nombres** de los campos. Nunca valores.
  - Acciones: `CREATE`, `UPDATE`, `CHANGE_ESTADO`, `PERSONAL_VIEW`,
    `PERSONAL_UPDATE`, `FAMILIA_VIEW`, `FAMILIA_UPDATE`, `MATRICULA_UPDATE`,
    `PERSONA_VIEW`, `PERSONA_UPDATE`.
- **Logs:**
  - El contenido de los 30 RPC está excluido de los logs:
    `IgnoreContentMethods` en el servicio y `logsafe` en el Gateway.
  - Una prueba falla si algún RPC queda fuera de esa lista.

## Cómo probar

```bash
go build ./... && go vet ./...
go test ./...                                  # unitarias (las de integración se omiten)

# Integración contra PostgreSQL real (estudiantes_db): crea sus propios datos y
# los borra al terminar. Requiere backend/.env y backend/.env.test cargados.
INTEGRATION_TEST=true go test ./internal/integration/...
```

## Decisiones pendientes del colegio

- **Egreso:**
  - No está implementado.
  - Falta confirmar qué condición académica acredita la culminación (por
    ejemplo, una matrícula `CONCLUIDO` en 5.° de secundaria) y qué documento la
    respalda.
  - Falta definir cómo se registran los egresados anteriores al sistema.
- **Columnas académicas heredadas de `estudiantes`** (`nivel`, `grado`,
  `seccion`, `turno`, `estado_matricula`):
  - Siguen siendo las que leen los consumidores actuales.
  - No se sincronizan con `matriculas`.
  - Hay que decidir a qué periodo pertenecen los datos actuales y cuándo se
    retiran (ADR-08).
- **Regla de un solo apoderado vigente:** está en la base; falta que la
  institución la confirme (ADR-06).
- **`colegio_procedencia` y `condicion`:** pendientes (ADR-09).
- **Mesa de Partes:** sus expedientes todavía no guardan `estudiante_id`.
  Pueden resolverlo con `POST /api/estudiantes/resolver`.
