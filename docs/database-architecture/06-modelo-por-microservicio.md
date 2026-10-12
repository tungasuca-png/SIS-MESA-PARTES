# 06 — Modelo por microservicio

Qué **posee** y qué **consume** cada servicio en la arquitectura objetivo. Regla general: **ningún servicio lee ni escribe tablas de otro**. Hoy esto ya se cumple en el código productivo; solo lo incumplen los tests (P-16).

## 1. Responsabilidades

| Servicio | Posee (fuente de verdad) | Consume (solo UUID + lo mínimo) | Cambios propuestos |
|---|---|---|---|
| **Maestro escolar** (hoy Estudiantes) | Personas, estudiantes, matrículas, familia, autorizaciones, egresos, vínculo persona↔cuenta, invitaciones | Auth: HasPermission, alta de cuentas por invitación | `persona_id`; tablas de [05](05-modelo-maestro-propuesto.md); escrituras de matrícula; eventos |
| **Auth** | Cuentas, credenciales, roles, permisos, refresh tokens, identidades externas (Google) | Maestro: validación del vínculo al emitir el JWT (`pid`) | Retirar `user_profiles`; agregar `persona_id` (proyección) e `identidades_externas`; limpiar tokens |
| **Usuarios** | Perfil **laboral** del personal (cargo, área) | Auth (rol), Maestro (persona) | `persona_id`; retirar la copia de dni, nombres y contacto (R-01/R-03) y `tipo_usuario` (R-02) |
| **Expedientes** (Mesa de Partes/FUT) | Expedientes, estado, área, instantánea de lo declarado | Auth (solicitante), Maestro (estudiante/persona) | `expediente_solicitante_snapshot`; `estudiante_id` opcional; catálogo de áreas |
| **Documentos** | Archivos y metadatos | Expedientes | Decidir almacenamiento (ADR-10); sin DELETE |
| **Derivaciones** | Movimientos entre áreas | Expedientes, catálogo de áreas | Códigos de área en vez de texto libre |
| **Psicología** | Expedientes y atenciones psicológicas (cifrado), su auditoría | Maestro: `GetEstudiante` / `GetEstudiantesBasic` (solo identidad básica); Usuarios (responsable) | Sin cambios de modelo; proteger los logs de cliente gRPC (pendiente previo) |
| **Gateway** | Nada (sin base) | Todos | Rutas de invitación, portal familiar y de egresados, OAuth |

## 2. Contratos de integración

| Consumidor | Necesita | Contrato (existe / propuesto) |
|---|---|---|
| Psicología | Existencia y nombre del estudiante | `GetEstudiante` (**existe**); en lotes, `GetEstudiantesBasic` (**existe**) |
| Expedientes | Resolver al estudiante de un trámite | `ResolveEstudiante` por código o documento (**existe**) |
| Portal de familias | Lista de estudiantes autorizados para la persona de la sesión | `ListEstudiantesAutorizados(persona_id)` (propuesto) |
| Portal de egresados | Verificar el egreso; datos propios | `VerificarEgreso`, `GetMiEgreso` (propuesto) |
| Auth | Confirmar el vínculo persona↔cuenta | `GetPersonaDeCuenta(usuario_id)` (propuesto) o el evento `persona_cuenta.vinculada` |
| Reportes (futuro) | Datos agregados | Vistas de solo lectura en el maestro o una réplica (ver [07](07-historial-auditoria-y-replicacion.md)) |

## 3. Disponibilidad, reintentos y consistencia

| Situación | Comportamiento propuesto |
|---|---|
| Maestro caído al **abrir** un expediente psicológico o registrar un trámite | **Fail-closed** para crear (ya ocurre en Psicología): no se registra con un estudiante no verificado |
| Maestro caído al **mostrar** nombres en listados | Se degrada: se muestra el UUID o "no disponible"; nunca se inventa un nombre |
| Reintentos | Solo en lecturas idempotentes, con backoff (go-zero ya reintenta la conexión). Las escrituras usan `expected_version` e idempotencia |
| Eventos (opcional, fase posterior) | `persona.fusionada`, `persona_cuenta.vinculada/revocada`, `egreso.validado`, `relacion_familiar.cerrada`. Entrega **al menos una vez**, `event_id` para deduplicar, *outbox* transaccional en la base del emisor |
| Correcciones de identidad | Los consumidores guardan solo UUID: un cambio de nombre o documento no requiere sincronización. Solo una **fusión** exige remapear UUID (evento más proceso idempotente en cada consumidor) |
| Proyecciones locales (caché) | Solo si una medición lo justifica. Datos: UUID, nombre para mostrar y código; vigencia corta (TTL) o invalidación por evento; nunca documentos ni datos personales ampliados |
| Instantáneas | Solo con valor probatorio (lo declarado en un trámite): inmutables y documentadas, sin sincronización |

## 4. Sin tecnologías nuevas innecesarias

Todo lo anterior se implementa con go-zero, gRPC y PostgreSQL. **No** se propone un broker en la primera etapa: el *outbox* puede leerse por *polling* desde el propio servicio consumidor mediante un RPC `ListEventosDesde(cursor)`. Un broker solo se evaluaría si el volumen lo requiere.
