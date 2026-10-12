# 07 — Historial, auditoría y replicación

## 1. Lo que existe (verificado)

| Mecanismo | Dónde | Qué registra |
|---|---|---|
| `auditoria_psicologica` (append-only por trigger, sin UPDATE, DELETE ni TRUNCATE) | psicologia_db | Actor, acción, recurso, estudiante, resultado (PERMITIDO/DENEGADO); **sin contenido clínico** |
| `auditoria_estudiantes` (append-only, lista blanca de nombres de campos) | estudiantes_db | Actor, acción (CREATE, UPDATE, CHANGE_ESTADO, PERSONAL_VIEW/UPDATE, FAMILIA_VIEW/UPDATE), versión y campos; **nunca valores** |
| Versionado de atenciones (`atencion_versiones`, inmutable) | psicologia_db | Historial clínico completo, **cifrado** |
| `matricula_movimientos` (append-only) | estudiantes_db | Movimientos académicos |
| Relaciones familiares con vigencia (no se borran) | estudiantes_db | Historial de vínculos |
| `derivaciones` | derivaciones_db | Movimientos del expediente (funciona como historial de negocio) |
| Respaldos cifrados GPG (AES-256) antes de cada migración | Disco local, fuera del repositorio | Recuperación puntual (manual) |

**No existe:** auditoría en Auth (cambios de rol o permisos), Usuarios, Expedientes (solo `fecha_actualizacion`) ni Documentos (solo `subido_por`); tampoco correlación entre servicios, réplicas ni base histórica.

## 2. Las alternativas no son equivalentes

| | A. Auditoría en cada base | B. Base histórica por servicio | C. Plataforma central de eventos de auditoría | D. Réplica de lectura o espejo | E. Respaldos (recuperación ante desastres) |
|---|---|---|---|---|---|
| **Propósito** | Quién hizo qué, sobre qué registro | Consultar versiones pasadas completas | Vista transversal de actividad | Descargar consultas o alta disponibilidad | Recuperar tras una pérdida |
| **Datos** | Metadatos (actor, acción, campos) | Copias completas de filas | Eventos de metadatos de todos los servicios | Copia **exacta** de la base | Copia exacta en un instante |
| **Retención** | Larga (años) | Larga | Larga | Igual a la primaria | Según política (p. ej. 30 diarios, 12 mensuales) |
| **Consistencia** | Transaccional con el cambio | Diferida | Eventual | Casi en tiempo real (asíncrona) | Instantánea |
| **Almacenamiento** | Bajo | **Alto** (duplica todo) | Medio | Igual a la primaria | Medio (comprimido) |
| **Complejidad** | Baja (**ya existe** en 2 servicios) | Alta | Media–alta | Media | Baja–media |
| **Recuperación** | No recupera datos | Parcial | No recupera datos | Si la primaria falla (failover) | **Sí** |
| **Seguridad y privacidad** | Controlada por el dueño | **Duplica datos personales y clínicos** | Riesgo de volverse una vía lateral de acceso | Tiene **todos** los datos: mismo control que la primaria | Cifrados; custodia de la clave |
| **Riesgo de duplicación** | Nulo | **Alto** | Bajo si solo guarda metadatos | Total (por diseño) | Total (por diseño) |
| **Acceso** | Rol del servicio y auditores del dominio | Nadie más que el dueño | Solo auditoría o seguridad | Solo lectura, mismos permisos | Solo operación |

**Conclusiones:**
- La réplica (D) y los respaldos (E) **no son auditoría**: copian el estado, no dicen quién cambió qué.
- La auditoría (A/C) **no recupera** datos.
- Una base histórica (B) duplica datos personales: **no se recomienda**.

## 3. Estrategia propuesta

1. **A en todos los servicios con escrituras** (Auth, Usuarios, Expedientes, Documentos, Derivaciones), con el patrón existente: tabla append-only por base, trigger anti-modificación, rol de aplicación solo con INSERT/SELECT y **solo metadatos**.
2. **Formato común de evento**, para poder agregarlos después:

   | Campo | Descripción |
   |---|---|
   | id | UUID |
   | servicio, entidad, entidad_id | Qué se tocó |
   | operacion | CREATE, UPDATE, VIEW, DELETE_LOGICO, VINCULAR… |
   | ocurrido_en | timestamptz |
   | actor_id, actor_tipo | usuario, sistema o proceso |
   | origen | api, migración, proceso programado |
   | correlation_id | Mismo ID propagado desde el Gateway por metadata gRPC |
   | resultado | PERMITIDO, DENEGADO, ERROR |
   | motivo_codigo | Código corto, nunca texto libre del usuario |
   | version_resultante / evento_relacionado_id | Enlace a la versión o al evento previo |
   | campos | Nombres de campos, nunca valores |

3. **C (opcional, posterior):** un colector que lee las auditorías de cada servicio (por RPC o *outbox*) **solo con metadatos**. **Psicología queda excluida** del colector central, o aporta solo eventos sin `estudiante_id` ni recurso clínico, para no crear una vía lateral a información psicológica.
4. **E formal:**
   - `pg_dump` cifrado por base, con una **prueba de restauración** periódica (ya se hace de forma manual antes de las migraciones).
   - WAL archiving para recuperación a un punto en el tiempo en producción.
   - Clave de respaldo y clave clínica en custodia separada (ya documentado en `docs/seguridad-infraestructura.md` §4).
5. **D solo si hay necesidad:** réplica de solo lectura para reportes o alta disponibilidad en producción, con los mismos controles de acceso y TLS. No se habilita ahora.
6. **Nunca en logs generales:** contraseñas, tokens, contenido clínico, documentos completos. Ya está implementado para Estudiantes, Psicología y Gateway (`IgnoreContentMethods`, `logsafe`); falta en el cliente gRPC de Psicología hacia Estudiantes.

## 4. Datos clínicos

- Siguen con su auditoría propia y el cifrado AES-256-GCM (**existe**).
- Ni una base histórica ni el colector central pueden contener texto clínico.
- El acceso a la auditoría psicológica se limita a los roles del dominio clínico y al auditor designado.
