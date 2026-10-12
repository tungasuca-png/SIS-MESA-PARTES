# 03 — Relaciones actuales

Estado **verificado** en PostgreSQL 17.11 (catálogo, solo lectura, 2026-09-30) y relaciones lógicas entre bases comprobadas en el código.

> No existen claves foráneas entre bases distintas: cada microservicio tiene su propia base (`<servicio>_db`) y PostgreSQL no puede garantizar referencias entre ellas. Esas relaciones son **lógicas** (UUID guardado sin FK) y se validan —o no— en la aplicación.

## 1. Claves foráneas físicas (verificadas)

| Base | Tabla | Columnas | Referencia | ON DELETE |
|---|---|---|---|---|
| auth_db | `refresh_tokens` | usuario_id | `usuarios`(id) | CASCADE |
| auth_db | `role_permissions` | permission_id | `permissions`(id) | RESTRICT |
| auth_db | `role_permissions` | role_id | `roles`(id) | RESTRICT |
| auth_db | `user_profiles` | user_id | `usuarios`(id) | CASCADE |
| auth_db | `usuario_roles` | rol_id | `roles`(id) | RESTRICT |
| auth_db | `usuario_roles` | usuario_id | `usuarios`(id) | CASCADE |
| psicologia_db | `atencion_numero_counters` | expediente_id | `expedientes_psicologicos`(id) | RESTRICT |
| psicologia_db | `atencion_versiones` | atencion_id | `atenciones_psicologicas`(id) | RESTRICT |
| psicologia_db | `atencion_versiones` | version_anterior_id | `atencion_versiones`(id) | RESTRICT |
| psicologia_db | `atenciones_psicologicas` | expediente_id | `expedientes_psicologicos`(id) | RESTRICT |
| estudiantes_db | `auditoria_estudiantes` | estudiante_id | `estudiantes`(id) | RESTRICT |
| estudiantes_db | `estudiante_familiares` | estudiante_id | `estudiantes`(id) | RESTRICT |
| estudiantes_db | `estudiante_familiares` | parentesco | `cat_parentescos`(codigo) | RESTRICT |
| estudiantes_db | `estudiante_familiares` | persona_id | `personas`(id) | RESTRICT |
| estudiantes_db | `estudiantes` | tipo_documento | `cat_tipos_documento`(codigo) | RESTRICT |
| estudiantes_db | `matricula_movimientos` | matricula_id | `matriculas`(id) | RESTRICT |
| estudiantes_db | `matricula_movimientos` | seccion_anterior_id | `secciones`(id) | RESTRICT |
| estudiantes_db | `matricula_movimientos` | seccion_nueva_id | `secciones`(id) | RESTRICT |
| estudiantes_db | `matriculas` | estudiante_id | `estudiantes`(id) | RESTRICT |
| estudiantes_db | `matriculas` | periodo_id | `periodos_academicos`(id) | RESTRICT |
| estudiantes_db | `matriculas` | seccion_id, periodo_id | `secciones`(id, periodo_id) | RESTRICT |
| estudiantes_db | `personas` | tipo_documento | `cat_tipos_documento`(codigo) | RESTRICT |
| estudiantes_db | `secciones` | periodo_id | `periodos_academicos`(id) | RESTRICT |

Bases **sin ninguna FK**: `usuarios_db`, `expedientes_db`, `documentos_db` y `derivaciones_db` (una tabla principal cada una, que solo referencia a otras bases).

## 2. Relaciones lógicas entre bases (sin FK)

| Origen (base.tabla.columna) | Destino lógico | Validación en código (evidencia) | Huérfanos verificados (2026-09-30) |
|---|---|---|---|
| usuarios_db.usuarios.id | auth_db.usuarios.id (mismo UUID) | Usuarios.UpsertUsuario lo recibe del Gateway tras crear la cuenta (CreateUser) | 0 perfiles sin cuenta; **28 de 35 cuentas sin perfil** |
| expedientes_db.expedientes.solicitante_id | auth_db.usuarios.id | Se toma del JWT al crear; no se revalida después | **11 de 12** solicitantes distintos sin cuenta en Auth |
| documentos_db.documentos.expediente_id | expedientes_db.expedientes.id | Gateway/Documentos consultan Expedientes por gRPC antes de cargar | **5 de 26** expedientes referenciados no existen |
| documentos_db.documentos.subido_por | auth_db.usuarios.id | Del JWT | **5** actores sin cuenta |
| derivaciones_db.derivaciones.expediente_id | expedientes_db.expedientes.id | Derivaciones valida con Expedientes (ValidateDerivacion) | **6 de 23** expedientes referenciados no existen |
| derivaciones_db.derivaciones.registrado_por | auth_db.usuarios.id | Del JWT | **13** actores sin cuenta |
| psicologia_db.expedientes_psicologicos.estudiante_id | estudiantes_db.estudiantes.id | Psicología llama a Estudiantes.GetEstudiante al abrir (fail-closed) | 0 de 1 |
| psicologia_db (responsable_id, abierto_por, autor_id…) | auth_db / usuarios_db (usuario) | Responsable validado contra Usuarios al abrir | 0 |
| auditoria_psicologica.estudiante_id / usuario_id | estudiantes_db / auth_db | Solo registro de auditoría | No verificado (tabla de auditoría) |
| auditoria_estudiantes.usuario_id, matricula_movimientos.registrado_por | auth_db.usuarios.id | Del JWT | Sin filas |
| expedientes.area_actual, derivaciones.origen/destino | Códigos de rol de auth_db.roles | area_actual por CHECK con los códigos de rol; origen/destino son **texto libre** | No aplica |

> **Causa de los huérfanos: no confirmada.** La hipótesis más probable son restos de pruebas de integración: los tests de Expedientes, Documentos y Derivaciones crean y borran cuentas **directamente en `auth_db`** (`internal/integration/haspermission_*_integration_test.go`), y los registros de otras bases no se limpian en cascada. Verificarlo requiere revisar fechas y datos (no se hizo en modo solo lectura).

## 3. Restricciones CHECK (verificadas)

| Base | Tabla | Restricción | Definición |
|---|---|---|---|
| auth_db | `permissions` | chk_permissions_codigo | `CHECK ((length(TRIM(BOTH FROM codigo)) > 0))` |
| auth_db | `permissions` | chk_permissions_modulo | `CHECK ((length(TRIM(BOTH FROM modulo)) > 0))` |
| auth_db | `permissions` | chk_permissions_nombre | `CHECK ((length(TRIM(BOTH FROM nombre)) > 0))` |
| auth_db | `refresh_tokens` | chk_refresh_tokens_expiry | `CHECK ((expires_at > created_at))` |
| auth_db | `refresh_tokens` | chk_refresh_tokens_hash_length | `CHECK ((octet_length(token_hash) = 32))` |
| auth_db | `roles` | chk_roles_nombre | `CHECK ((length(TRIM(BOTH FROM nombre)) >= 2))` |
| auth_db | `user_profiles` | chk_user_profiles_apellidos | `CHECK ((length(TRIM(BOTH FROM apellidos)) > 0))` |
| auth_db | `user_profiles` | chk_user_profiles_nombres | `CHECK ((length(TRIM(BOTH FROM nombres)) > 0))` |
| auth_db | `user_profiles` | chk_user_profiles_numero_documento | `CHECK ((length(TRIM(BOTH FROM numero_documento)) > 0))` |
| auth_db | `user_profiles` | chk_user_profiles_tipo_documento | `CHECK ((length(TRIM(BOTH FROM tipo_documento)) > 0))` |
| auth_db | `usuarios` | chk_usuarios_dni | `CHECK (((dni IS NULL) OR ((dni)::text ~ '^[0-9]{8}$'::text)))` |
| auth_db | `usuarios` | chk_usuarios_failed_login_attempts | `CHECK ((failed_login_attempts >= 0))` |
| auth_db | `usuarios` | chk_usuarios_username | `CHECK ((length(TRIM(BOTH FROM username)) >= 3))` |
| usuarios_db | `usuarios` | chk_usuarios_apellidos | `CHECK ((length(TRIM(BOTH FROM apellidos)) > 0))` |
| usuarios_db | `usuarios` | chk_usuarios_nombres | `CHECK ((length(TRIM(BOTH FROM nombres)) > 0))` |
| usuarios_db | `usuarios` | chk_usuarios_tipo | `CHECK (((tipo_usuario)::text = ANY (ARRAY[('ADMIN'::character varying)::text, ('DIRECTOR'::character varying)::text, ('SUBDIRECTOR'::character varying)::text, ('SECRETARIA'::character varying)::text, ('DOCENTE'::character varying)::text, ('AUXILIAR'::character varying)::text, ('PSICOLOGO'::character varying)::text, ('SOLICITANTE'::character varying)::text])))` |
| expedientes_db | `expedientes` | chk_expedientes_area_actual | `CHECK (((area_actual)::text = ANY (ARRAY[('SECRETARIA'::character varying)::text, ('DIRECTOR'::character varying)::text, ('SUBDIRECTOR'::character varying)::text, ('DOCENTE'::character varying)::text, ('AUXILIAR'::character varying)::text, ('ADMIN'::character varying)::text])))` |
| expedientes_db | `expedientes` | chk_expedientes_asunto | `CHECK ((length(TRIM(BOTH FROM asunto)) > 0))` |
| expedientes_db | `expedientes` | chk_expedientes_estado | `CHECK (((estado)::text = ANY (ARRAY[('PENDIENTE'::character varying)::text, ('EN_PROCESO'::character varying)::text, ('ATENDIDO'::character varying)::text, ('OBSERVADO'::character varying)::text])))` |
| expedientes_db | `expedientes` | chk_expedientes_prioridad | `CHECK (((prioridad)::text = ANY (ARRAY[('NORMAL'::character varying)::text, ('URGENTE'::character varying)::text])))` |
| expedientes_db | `expedientes` | chk_expedientes_tipo | `CHECK (((tipo)::text = ANY (ARRAY[('SOLICITUD'::character varying)::text, ('TRAMITE'::character varying)::text, ('OFICIO'::character varying)::text, ('OTRO'::character varying)::text, ('CERTIFICADO'::character varying)::text, ('CONSTANCIA'::character varying)::text, ('PERMISO'::character varying)::text, ('JUSTIFICACION_FALTA'::character varying)::text, ('JUSTIFICACION_TARDANZA'::character varying)::text])))` |
| documentos_db | `documentos` | chk_documentos_estado | `CHECK (((estado)::text = ANY (ARRAY[('ACTIVO'::character varying)::text, ('ELIMINADO'::character varying)::text])))` |
| documentos_db | `documentos` | chk_documentos_extension | `CHECK (((extension)::text = ANY (ARRAY[('pdf'::character varying)::text, ('jpg'::character varying)::text, ('jpeg'::character varying)::text, ('png'::character varying)::text, ('doc'::character varying)::text, ('docx'::character varying)::text])))` |
| documentos_db | `documentos` | chk_documentos_nombre | `CHECK ((length(TRIM(BOTH FROM nombre)) > 0))` |
| documentos_db | `documentos` | chk_documentos_tamano | `CHECK ((tamano_bytes > 0))` |
| documentos_db | `documentos` | chk_documentos_tipo | `CHECK (((tipo_documento)::text = ANY (ARRAY[('ADJUNTO'::character varying)::text, ('PROVEIDO'::character varying)::text, ('ACTA'::character varying)::text, ('INFORME'::character varying)::text, ('FORMATO'::character varying)::text, ('OTRO'::character varying)::text])))` |
| derivaciones_db | `derivaciones` | chk_derivaciones_destino | `CHECK ((length(TRIM(BOTH FROM destino)) > 0))` |
| derivaciones_db | `derivaciones` | chk_derivaciones_motivo | `CHECK ((length(TRIM(BOTH FROM motivo)) > 0))` |
| derivaciones_db | `derivaciones` | chk_derivaciones_origen | `CHECK ((length(TRIM(BOTH FROM origen)) > 0))` |
| derivaciones_db | `derivaciones` | chk_derivaciones_tipo | `CHECK (((tipo)::text = ANY (ARRAY[('DERIVACION'::character varying)::text, ('NOTIFICACION'::character varying)::text, ('ASIGNACION'::character varying)::text, ('RECEPCION'::character varying)::text, ('APROBACION'::character varying)::text])))` |
| psicologia_db | `atencion_numero_counters` | chk_atencion_numero_counters_ultimo | `CHECK ((ultimo >= 0))` |
| psicologia_db | `atencion_versiones` | chk_atencion_versiones_cifrado | `CHECK (((contenido_cifrado IS NULL) = (cifrado_key_id IS NULL)))` |
| psicologia_db | `atencion_versiones` | chk_atencion_versiones_correccion | `CHECK (((numero_version <= 1) OR (version_anterior_id IS NOT NULL)))` |
| psicologia_db | `atencion_versiones` | chk_atencion_versiones_derivaciones | `CHECK ((jsonb_typeof(derivaciones) = 'array'::text))` |
| psicologia_db | `atencion_versiones` | chk_atencion_versiones_fecha_hora | `CHECK (((numero_version = 0) OR ((fecha_atencion IS NOT NULL) AND (hora_atencion IS NOT NULL))))` |
| psicologia_db | `atencion_versiones` | chk_atencion_versiones_motivos | `CHECK ((jsonb_typeof(motivos) = 'array'::text))` |
| psicologia_db | `atencion_versiones` | chk_atencion_versiones_numero | `CHECK ((numero_version >= 0))` |
| psicologia_db | `atencion_versiones` | chk_atencion_versiones_realizada_por | `CHECK ((realizada_por <@ ARRAY['DOCENTE'::text, 'AUXILIAR'::text, 'DIRECTIVO'::text, 'PSICOLOGO'::text]))` |
| psicologia_db | `atenciones_psicologicas` | chk_atenciones_psicologicas_anulacion | `CHECK ((((estado)::text <> 'ANULADA'::text) OR ((anulada_por IS NOT NULL) AND (anulada_en IS NOT NULL) AND (motivo_anulacion_cifrado IS NOT NULL))))` |
| psicologia_db | `atenciones_psicologicas` | chk_atenciones_psicologicas_cifrado | `CHECK (((motivo_anulacion_cifrado IS NULL) OR (cifrado_key_id IS NOT NULL)))` |
| psicologia_db | `atenciones_psicologicas` | chk_atenciones_psicologicas_estado | `CHECK (((estado)::text = ANY (ARRAY[('BORRADOR'::character varying)::text, ('REGISTRADA'::character varying)::text, ('ANULADA'::character varying)::text, ('DESCARTADA'::character varying)::text])))` |
| psicologia_db | `atenciones_psicologicas` | chk_atenciones_psicologicas_numero | `CHECK ((((estado)::text = ANY (ARRAY[('BORRADOR'::character varying)::text, ('DESCARTADA'::character varying)::text])) = (numero IS NULL)))` |
| psicologia_db | `atenciones_psicologicas` | chk_atenciones_psicologicas_numero_positivo | `CHECK (((numero IS NULL) OR (numero > 0)))` |
| psicologia_db | `atenciones_psicologicas` | chk_atenciones_psicologicas_registro | `CHECK ((((estado)::text <> ALL (ARRAY[('REGISTRADA'::character varying)::text, ('ANULADA'::character varying)::text])) OR ((registrada_por IS NOT NULL) AND (registrada_en IS NOT NULL) AND (version_vigente >= 1))))` |
| psicologia_db | `atenciones_psicologicas` | chk_atenciones_psicologicas_revision | `CHECK ((revision >= 0))` |
| psicologia_db | `atenciones_psicologicas` | chk_atenciones_psicologicas_version | `CHECK ((version_vigente >= 0))` |
| psicologia_db | `auditoria_psicologica` | chk_auditoria_psicologica_accion | `CHECK (((accion)::text = ANY (ARRAY[('CREATE'::character varying)::text, ('VIEW'::character varying)::text, ('UPDATE'::character varying)::text, ('CLOSE'::character varying)::text, ('REOPEN'::character varying)::text, ('DENIED'::character varying)::text, ('REGISTER'::character varying)::text, ('CORRECT'::character varying)::text, ('ANNUL'::character varying)::text, ('DISCARD'::character varying)::text, ('EXPORT'::character varying)::text])))` |
| psicologia_db | `auditoria_psicologica` | chk_auditoria_psicologica_recurso | `CHECK ((length(TRIM(BOTH FROM recurso)) > 0))` |
| psicologia_db | `auditoria_psicologica` | chk_auditoria_psicologica_recurso_valido | `CHECK (((recurso)::text = ANY (ARRAY[('EXPEDIENTE_PSICOLOGICO'::character varying)::text, ('ATENCION_PSICOLOGICA'::character varying)::text])))` |
| psicologia_db | `auditoria_psicologica` | chk_auditoria_psicologica_resultado | `CHECK (((resultado)::text = ANY (ARRAY[('PERMITIDO'::character varying)::text, ('DENEGADO'::character varying)::text])))` |
| psicologia_db | `catalogo_destinos_derivacion` | chk_catalogo_destinos_codigo | `CHECK (((codigo)::text ~ '^[A-Z_]+$'::text))` |
| psicologia_db | `catalogo_destinos_derivacion` | chk_catalogo_destinos_etiqueta | `CHECK ((length(TRIM(BOTH FROM etiqueta)) > 0))` |
| psicologia_db | `catalogo_motivos_atencion` | chk_catalogo_motivos_codigo | `CHECK (((codigo)::text ~ '^[A-Z_]+$'::text))` |
| psicologia_db | `catalogo_motivos_atencion` | chk_catalogo_motivos_etiqueta | `CHECK ((length(TRIM(BOTH FROM etiqueta)) > 0))` |
| psicologia_db | `expediente_codigo_counters` | chk_expediente_codigo_counters_ultimo | `CHECK ((ultimo >= 0))` |
| psicologia_db | `expedientes_psicologicos` | chk_expedientes_psicologicos_cierre | `CHECK ((((estado)::text <> 'CERRADO'::text) OR ((fecha_cierre IS NOT NULL) AND (motivo_cierre_cifrado IS NOT NULL))))` |
| psicologia_db | `expedientes_psicologicos` | chk_expedientes_psicologicos_cifrado | `CHECK ((((resumen_caso_cifrado IS NULL) AND (motivo_cierre_cifrado IS NULL)) OR (cifrado_key_id IS NOT NULL)))` |
| psicologia_db | `expedientes_psicologicos` | chk_expedientes_psicologicos_codigo | `CHECK (((codigo)::text ~ '^PSI-[0-9]{4}-[0-9]{6}$'::text))` |
| psicologia_db | `expedientes_psicologicos` | chk_expedientes_psicologicos_estado | `CHECK (((estado)::text = ANY (ARRAY[('ABIERTO'::character varying)::text, ('EN_SEGUIMIENTO'::character varying)::text, ('CERRADO'::character varying)::text])))` |
| estudiantes_db | `auditoria_estudiantes` | chk_auditoria_estudiantes_accion | `CHECK (((accion)::text = ANY ((ARRAY['CREATE'::character varying, 'UPDATE'::character varying, 'CHANGE_ESTADO'::character varying, 'PERSONAL_VIEW'::character varying, 'PERSONAL_UPDATE'::character varying, 'FAMILIA_VIEW'::character varying, 'FAMILIA_UPDATE'::character varying])::text[])))` |
| estudiantes_db | `auditoria_estudiantes` | chk_auditoria_estudiantes_campos | `CHECK ((campos <@ ARRAY['codigo'::text, 'codigo_siagie'::text, 'tipo_documento'::text, 'numero_documento'::text, 'nombres'::text, 'apellido_paterno'::text, 'apellido_materno'::text, 'apellidos'::text, 'fecha_nacimiento'::text, 'sexo'::text, 'pais_nacimiento'::text, 'ubigeo_nacimiento'::text, 'lengua_materna'::text, 'segunda_lengua'::text, 'foto_ref'::text, 'estado'::text, 'domicilio_direccion'::text, 'domicilio_departamento'::text, 'domicilio_provincia'::text, 'domicilio_distrito'::text, 'domicilio_ubigeo'::text, 'correo'::text, 'telefono'::text, 'familiares'::text]))` |
| estudiantes_db | `auditoria_estudiantes` | chk_auditoria_estudiantes_estados | `CHECK (((((accion)::text = 'CHANGE_ESTADO'::text) AND ((estado_anterior)::text = ANY ((ARRAY['ACTIVO'::character varying, 'INACTIVO'::character varying])::text[])) AND ((estado_nuevo)::text = ANY ((ARRAY['ACTIVO'::character varying, 'INACTIVO'::character varying])::text[])) AND ((estado_anterior)::text <> (estado_nuevo)::text)) OR (((accion)::text <> 'CHANGE_ESTADO'::text) AND (estado_anterior IS NULL) AND (estado_nuevo IS NULL))))` |
| estudiantes_db | `auditoria_estudiantes` | chk_auditoria_estudiantes_version | `CHECK ((version_resultante >= 1))` |
| estudiantes_db | `cat_parentescos` | chk_cat_parentescos_codigo | `CHECK (((codigo)::text ~ '^[A-Z_]{2,20}$'::text))` |
| estudiantes_db | `cat_tipos_documento` | chk_cat_tipos_documento_codigo | `CHECK (((codigo)::text ~ '^[A-Z_]{2,20}$'::text))` |
| estudiantes_db | `estudiante_familiares` | chk_estudiante_familiares_fechas | `CHECK (((hasta IS NULL) OR (desde IS NULL) OR (hasta >= desde)))` |
| estudiantes_db | `estudiante_familiares` | chk_estudiante_familiares_vigencia | `CHECK (((vigente AND (hasta IS NULL)) OR ((NOT vigente) AND (hasta IS NOT NULL))))` |
| estudiantes_db | `estudiantes` | chk_estudiantes_apellidos | `CHECK ((length(TRIM(BOTH FROM apellidos)) > 0))` |
| estudiantes_db | `estudiantes` | chk_estudiantes_apellidos_separados | `CHECK (((apellido_paterno IS NULL) OR ((length(TRIM(BOTH FROM apellido_paterno)) > 0) AND ((apellidos)::text = TRIM(BOTH FROM (((apellido_paterno)::text \|\| ' '::text) \|\| (COALESCE(apellido_materno, ''::character varying))::text))))))` |
| estudiantes_db | `estudiantes` | chk_estudiantes_codigo | `CHECK ((length(TRIM(BOTH FROM codigo)) > 0))` |
| estudiantes_db | `estudiantes` | chk_estudiantes_codigo_siagie | `CHECK (((codigo_siagie IS NULL) OR ((codigo_siagie)::text ~ '^[0-9]{14}$'::text)))` |
| estudiantes_db | `estudiantes` | chk_estudiantes_contacto | `CHECK ((((correo IS NULL) OR ((correo)::text ~ '^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$'::text)) AND ((telefono IS NULL) OR ((telefono)::text ~ '^\+?[0-9][0-9 -]{5,18}$'::text))))` |
| estudiantes_db | `estudiantes` | chk_estudiantes_documento | `CHECK (((((tipo_documento)::text = 'SIN_DOCUMENTO'::text) AND (numero_documento IS NULL)) OR (((tipo_documento)::text = 'DNI'::text) AND (numero_documento IS NOT NULL) AND ((numero_documento)::text ~ '^[0-9]{8}$'::text)) OR (((tipo_documento)::text = ANY ((ARRAY['CE'::character varying, 'PASAPORTE'::character varying])::text[])) AND (numero_documento IS NOT NULL) AND ((numero_documento)::text ~ '^[A-Z0-9]{4,12}$'::text))))` |
| estudiantes_db | `estudiantes` | chk_estudiantes_domicilio | `CHECK ((((domicilio_direccion IS NULL) OR (length(TRIM(BOTH FROM domicilio_direccion)) > 0)) AND ((domicilio_departamento IS NULL) OR (length(TRIM(BOTH FROM domicilio_departamento)) > 0)) AND ((domicilio_provincia IS NULL) OR (length(TRIM(BOTH FROM domicilio_provincia)) > 0)) AND ((domicilio_distrito IS NULL) OR (length(TRIM(BOTH FROM domicilio_distrito)) > 0)) AND ((domicilio_ubigeo IS NULL) OR (domicilio_ubigeo ~ '^[0-9]{6}$'::text))))` |
| estudiantes_db | `estudiantes` | chk_estudiantes_estado | `CHECK (((estado)::text = ANY ((ARRAY['ACTIVO'::character varying, 'INACTIVO'::character varying])::text[])))` |
| estudiantes_db | `estudiantes` | chk_estudiantes_estado_matricula | `CHECK (((estado_matricula)::text = ANY (ARRAY[('ACTIVO'::character varying)::text, ('RETIRADO'::character varying)::text])))` |
| estudiantes_db | `estudiantes` | chk_estudiantes_foto_ref | `CHECK (((foto_ref IS NULL) OR ((length(TRIM(BOTH FROM foto_ref)) > 0) AND ((foto_ref)::text !~* '^[a-z][a-z0-9+.-]*://'::text))))` |
| estudiantes_db | `estudiantes` | chk_estudiantes_lenguas | `CHECK ((((lengua_materna IS NULL) OR ((lengua_materna)::text ~ '^[A-Z0-9_]{1,10}$'::text)) AND ((segunda_lengua IS NULL) OR ((segunda_lengua)::text ~ '^[A-Z0-9_]{1,10}$'::text)) AND ((segunda_lengua IS NULL) OR ((segunda_lengua)::text IS DISTINCT FROM (lengua_materna)::text))))` |
| estudiantes_db | `estudiantes` | chk_estudiantes_nombres | `CHECK ((length(TRIM(BOTH FROM nombres)) > 0))` |
| estudiantes_db | `estudiantes` | chk_estudiantes_pais_nacimiento | `CHECK (((pais_nacimiento IS NULL) OR (pais_nacimiento ~ '^[A-Z]{2}$'::text)))` |
| estudiantes_db | `estudiantes` | chk_estudiantes_sexo | `CHECK (((sexo IS NULL) OR (sexo = ANY (ARRAY['M'::bpchar, 'F'::bpchar]))))` |
| estudiantes_db | `estudiantes` | chk_estudiantes_ubigeo_nacimiento | `CHECK (((ubigeo_nacimiento IS NULL) OR ((ubigeo_nacimiento ~ '^[0-9]{6}$'::text) AND (COALESCE(pais_nacimiento, ''::bpchar) = 'PE'::bpchar))))` |
| estudiantes_db | `estudiantes` | chk_estudiantes_version | `CHECK ((version >= 1))` |
| estudiantes_db | `matricula_movimientos` | chk_movimientos_motivo | `CHECK (((motivo IS NULL) OR (length(TRIM(BOTH FROM motivo)) > 0)))` |
| estudiantes_db | `matricula_movimientos` | chk_movimientos_tipo | `CHECK (((tipo)::text = ANY ((ARRAY['CAMBIO_SECCION'::character varying, 'CAMBIO_TURNO'::character varying, 'RETIRO'::character varying, 'REINGRESO'::character varying, 'TRASLADO_SALIDA'::character varying])::text[])))` |
| estudiantes_db | `matriculas` | chk_matriculas_estado | `CHECK (((estado_matricula)::text = ANY ((ARRAY['MATRICULADO'::character varying, 'RETIRADO'::character varying, 'TRASLADADO'::character varying, 'CONCLUIDO'::character varying])::text[])))` |
| estudiantes_db | `matriculas` | chk_matriculas_fechas | `CHECK (((fecha_retiro IS NULL) OR (fecha_matricula IS NULL) OR (fecha_retiro >= fecha_matricula)))` |
| estudiantes_db | `matriculas` | chk_matriculas_motivo | `CHECK (((motivo_retiro IS NULL) OR (length(TRIM(BOTH FROM motivo_retiro)) > 0)))` |
| estudiantes_db | `matriculas` | chk_matriculas_origen | `CHECK (((origen)::text = ANY ((ARRAY['REGISTRO'::character varying, 'MIGRACION_LEGADO'::character varying])::text[])))` |
| estudiantes_db | `matriculas` | chk_matriculas_retiro | `CHECK ((((estado_matricula)::text = ANY ((ARRAY['RETIRADO'::character varying, 'TRASLADADO'::character varying])::text[])) OR ((fecha_retiro IS NULL) AND (motivo_retiro IS NULL))))` |
| estudiantes_db | `matriculas` | chk_matriculas_version | `CHECK ((version >= 1))` |
| estudiantes_db | `periodos_academicos` | chk_periodos_academicos_anio | `CHECK (((anio >= 2000) AND (anio <= 2100)))` |
| estudiantes_db | `periodos_academicos` | chk_periodos_academicos_estado | `CHECK (((estado)::text = ANY ((ARRAY['PLANIFICADO'::character varying, 'EN_CURSO'::character varying, 'CERRADO'::character varying])::text[])))` |
| estudiantes_db | `periodos_academicos` | chk_periodos_academicos_fechas | `CHECK (((fecha_inicio IS NULL) OR (fecha_fin IS NULL) OR (fecha_fin >= fecha_inicio)))` |
| estudiantes_db | `periodos_academicos` | chk_periodos_academicos_nombre | `CHECK ((length(TRIM(BOTH FROM nombre)) > 0))` |
| estudiantes_db | `personas` | chk_personas_documento | `CHECK (((((tipo_documento)::text = 'SIN_DOCUMENTO'::text) AND (numero_documento IS NULL)) OR (((tipo_documento)::text = 'DNI'::text) AND (numero_documento IS NOT NULL) AND ((numero_documento)::text ~ '^[0-9]{8}$'::text)) OR (((tipo_documento)::text = ANY ((ARRAY['CE'::character varying, 'PASAPORTE'::character varying])::text[])) AND (numero_documento IS NOT NULL) AND ((numero_documento)::text ~ '^[A-Z0-9]{4,12}$'::text))))` |
| estudiantes_db | `personas` | chk_personas_textos | `CHECK (((length(TRIM(BOTH FROM nombres)) > 0) AND ((apellido_paterno IS NULL) OR (length(TRIM(BOTH FROM apellido_paterno)) > 0)) AND ((apellido_materno IS NULL) OR (length(TRIM(BOTH FROM apellido_materno)) > 0))))` |
| estudiantes_db | `personas` | chk_personas_version | `CHECK ((version >= 1))` |
| estudiantes_db | `secciones` | chk_secciones_nivel | `CHECK (((nivel)::text = ANY ((ARRAY['INICIAL'::character varying, 'PRIMARIA'::character varying, 'SECUNDARIA'::character varying])::text[])))` |
| estudiantes_db | `secciones` | chk_secciones_textos | `CHECK (((length(TRIM(BOTH FROM grado)) > 0) AND (length(TRIM(BOTH FROM seccion)) > 0) AND ((turno IS NULL) OR (length(TRIM(BOTH FROM turno)) > 0)) AND ((aula IS NULL) OR (length(TRIM(BOTH FROM aula)) > 0))))` |
| estudiantes_db | `secciones` | chk_secciones_vacantes | `CHECK (((vacantes IS NULL) OR (vacantes >= 0)))` |

## 4. Unicidad (restricciones e índices únicos, verificados)

| Base | Tabla | Nombre | Definición |
|---|---|---|---|
| auth_db | `permissions` | permissions_codigo_key | `permissions USING btree (codigo)` |
| auth_db | `refresh_tokens` | refresh_tokens_token_hash_key | `refresh_tokens USING btree (token_hash)` |
| auth_db | `roles` | uq_roles_nombre | `roles USING btree (nombre)` |
| auth_db | `user_profiles` | uq_user_profiles_document | `user_profiles USING btree (tipo_documento, numero_documento)` |
| auth_db | `user_profiles` | uq_user_profiles_user_id | `user_profiles USING btree (user_id)` |
| auth_db | `usuario_roles` | uq_usuario_roles_usuario | `usuario_roles USING btree (usuario_id)` |
| auth_db | `usuarios` | uq_usuarios_dni | `usuarios USING btree (dni) WHERE (dni IS NOT NULL)` |
| auth_db | `usuarios` | uq_usuarios_email | `usuarios USING btree (email)` |
| auth_db | `usuarios` | uq_usuarios_username | `usuarios USING btree (username)` |
| usuarios_db | `usuarios` | uq_usuarios_dni | `usuarios USING btree (dni) WHERE ((dni)::text <> ''::text)` |
| expedientes_db | `expedientes` | uq_expedientes_codigo | `expedientes USING btree (codigo)` |
| psicologia_db | `atencion_versiones` | uq_atencion_versiones_numero | `atencion_versiones USING btree (atencion_id, numero_version)` |
| psicologia_db | `atenciones_psicologicas` | uq_atenciones_psicologicas_numero | `atenciones_psicologicas USING btree (expediente_id, numero)` |
| psicologia_db | `expedientes_psicologicos` | uq_expedientes_psicologicos_codigo | `expedientes_psicologicos USING btree (codigo)` |
| psicologia_db | `expedientes_psicologicos` | uq_expedientes_psicologicos_estudiante | `expedientes_psicologicos USING btree (estudiante_id)` |
| estudiantes_db | `estudiante_familiares` | uq_estudiante_familiares_apoderado | `estudiante_familiares USING btree (estudiante_id) WHERE (vigente AND es_apoderado)` |
| estudiantes_db | `estudiante_familiares` | uq_estudiante_familiares_progenitor | `estudiante_familiares USING btree (estudiante_id, parentesco) WHERE (vigente AND ((parentesco)::text = ANY ((ARRAY['PADRE'::character varying, 'MADRE'::character varying])::text[])))` |
| estudiantes_db | `estudiante_familiares` | uq_estudiante_familiares_vigente | `estudiante_familiares USING btree (estudiante_id, persona_id) WHERE vigente` |
| estudiantes_db | `estudiantes` | uq_estudiantes_codigo | `estudiantes USING btree (codigo)` |
| estudiantes_db | `estudiantes` | uq_estudiantes_codigo_siagie | `estudiantes USING btree (codigo_siagie) WHERE (codigo_siagie IS NOT NULL)` |
| estudiantes_db | `estudiantes` | uq_estudiantes_documento | `estudiantes USING btree (tipo_documento, numero_documento) WHERE (numero_documento IS NOT NULL)` |
| estudiantes_db | `matriculas` | uq_matriculas_estudiante_periodo | `matriculas USING btree (estudiante_id, periodo_id)` |
| estudiantes_db | `periodos_academicos` | uq_periodos_academicos_anio | `periodos_academicos USING btree (anio)` |
| estudiantes_db | `personas` | uq_personas_documento | `personas USING btree (tipo_documento, numero_documento) WHERE (numero_documento IS NOT NULL)` |
| estudiantes_db | `secciones` | uq_secciones_id_periodo | `secciones USING btree (id, periodo_id)` |
| estudiantes_db | `secciones` | uq_secciones_periodo_grupo | `secciones USING btree (periodo_id, nivel, grado, seccion, turno) NULLS NOT DISTINCT` |

## 5. Índices no únicos (verificados)

| Base | Tabla | Índice | Definición |
|---|---|---|---|
| auth_db | `permissions` | idx_permissions_modulo | `permissions USING btree (modulo)` |
| auth_db | `refresh_tokens` | idx_refresh_tokens_active_lookup | `refresh_tokens USING btree (token_hash, expires_at) WHERE (revoked_at IS NULL)` |
| auth_db | `refresh_tokens` | idx_refresh_tokens_usuario_id | `refresh_tokens USING btree (usuario_id)` |
| auth_db | `role_permissions` | idx_role_permissions_permission_id | `role_permissions USING btree (permission_id)` |
| auth_db | `usuario_roles` | idx_usuario_roles_rol_id | `usuario_roles USING btree (rol_id)` |
| auth_db | `usuarios` | idx_usuarios_estado | `usuarios USING btree (estado)` |
| auth_db | `usuarios` | idx_usuarios_last_login_at | `usuarios USING btree (last_login_at)` |
| auth_db | `usuarios` | idx_usuarios_locked_until | `usuarios USING btree (locked_until)` |
| usuarios_db | `usuarios` | idx_usuarios_apellidos | `usuarios USING btree (apellidos)` |
| usuarios_db | `usuarios` | idx_usuarios_tipo_usuario | `usuarios USING btree (tipo_usuario)` |
| expedientes_db | `expedientes` | idx_expedientes_area_actual | `expedientes USING btree (area_actual)` |
| expedientes_db | `expedientes` | idx_expedientes_estado | `expedientes USING btree (estado)` |
| expedientes_db | `expedientes` | idx_expedientes_fecha_registro | `expedientes USING btree (fecha_registro)` |
| expedientes_db | `expedientes` | idx_expedientes_solicitante_id | `expedientes USING btree (solicitante_id)` |
| documentos_db | `documentos` | idx_documentos_estado | `documentos USING btree (estado)` |
| documentos_db | `documentos` | idx_documentos_expediente_id | `documentos USING btree (expediente_id)` |
| derivaciones_db | `derivaciones` | idx_derivaciones_expediente_id | `derivaciones USING btree (expediente_id)` |
| derivaciones_db | `derivaciones` | idx_derivaciones_tipo | `derivaciones USING btree (tipo)` |
| psicologia_db | `atencion_versiones` | idx_atencion_versiones_atencion | `atencion_versiones USING btree (atencion_id, numero_version)` |
| psicologia_db | `atenciones_psicologicas` | idx_atenciones_psicologicas_autor_estado | `atenciones_psicologicas USING btree (autor_id, estado)` |
| psicologia_db | `atenciones_psicologicas` | idx_atenciones_psicologicas_expediente_estado | `atenciones_psicologicas USING btree (expediente_id, estado)` |
| psicologia_db | `auditoria_psicologica` | idx_auditoria_psicologica_fecha | `auditoria_psicologica USING btree (fecha_hora)` |
| psicologia_db | `auditoria_psicologica` | idx_auditoria_psicologica_recurso | `auditoria_psicologica USING btree (recurso, recurso_id)` |
| psicologia_db | `auditoria_psicologica` | idx_auditoria_psicologica_usuario_fecha | `auditoria_psicologica USING btree (usuario_id, fecha_hora)` |
| psicologia_db | `expedientes_psicologicos` | idx_expedientes_psicologicos_estado | `expedientes_psicologicos USING btree (estado)` |
| psicologia_db | `expedientes_psicologicos` | idx_expedientes_psicologicos_fecha_apertura | `expedientes_psicologicos USING btree (fecha_apertura)` |
| psicologia_db | `expedientes_psicologicos` | idx_expedientes_psicologicos_responsable | `expedientes_psicologicos USING btree (responsable_id)` |
| estudiantes_db | `auditoria_estudiantes` | idx_auditoria_estudiantes_estudiante_fecha | `auditoria_estudiantes USING btree (estudiante_id, fecha_hora)` |
| estudiantes_db | `auditoria_estudiantes` | idx_auditoria_estudiantes_usuario_fecha | `auditoria_estudiantes USING btree (usuario_id, fecha_hora)` |
| estudiantes_db | `estudiante_familiares` | idx_estudiante_familiares_persona | `estudiante_familiares USING btree (persona_id)` |
| estudiantes_db | `estudiantes` | idx_estudiantes_apellidos | `estudiantes USING btree (apellidos)` |
| estudiantes_db | `estudiantes` | idx_estudiantes_estado_matricula | `estudiantes USING btree (estado_matricula)` |
| estudiantes_db | `estudiantes` | idx_estudiantes_nivel_grado_seccion | `estudiantes USING btree (nivel, grado, seccion)` |
| estudiantes_db | `estudiantes` | idx_estudiantes_texto_busqueda_trgm | `estudiantes USING gin (texto_busqueda gin_trgm_ops)` |
| estudiantes_db | `matricula_movimientos` | idx_matricula_movimientos_matricula | `matricula_movimientos USING btree (matricula_id, created_at)` |
| estudiantes_db | `matriculas` | idx_matriculas_periodo_seccion | `matriculas USING btree (periodo_id, seccion_id)` |

## 6. Triggers y funciones (verificados)

| Base | Tabla | Trigger |
|---|---|---|
| psicologia_db | `atencion_versiones` | `trg_atencion_versiones_inmutables BEFORE DELETE OR UPDATE ON public.atencion_versiones FOR EACH ROW EXECUTE FUNCTION atencion_versiones_inmutables()` |
| psicologia_db | `auditoria_psicologica` | `trg_auditoria_psicologica_no_mod BEFORE DELETE OR UPDATE ON public.auditoria_psicologica FOR EACH ROW EXECUTE FUNCTION auditoria_psicologica_append_only()` |
| psicologia_db | `auditoria_psicologica` | `trg_auditoria_psicologica_no_truncate BEFORE TRUNCATE ON public.auditoria_psicologica FOR EACH STATEMENT EXECUTE FUNCTION auditoria_psicologica_append_only()` |
| estudiantes_db | `auditoria_estudiantes` | `trg_auditoria_estudiantes_no_mod BEFORE DELETE OR UPDATE ON public.auditoria_estudiantes FOR EACH ROW EXECUTE FUNCTION auditoria_estudiantes_append_only()` |
| estudiantes_db | `auditoria_estudiantes` | `trg_auditoria_estudiantes_no_truncate BEFORE TRUNCATE ON public.auditoria_estudiantes FOR EACH STATEMENT EXECUTE FUNCTION auditoria_estudiantes_append_only()` |
| estudiantes_db | `estudiante_familiares` | `trg_estudiante_familiares_updated_at BEFORE UPDATE ON public.estudiante_familiares FOR EACH ROW EXECUTE FUNCTION tg_actualizar_updated_at()` |
| estudiantes_db | `estudiantes` | `trg_estudiantes_actualizar_version BEFORE UPDATE ON public.estudiantes FOR EACH ROW EXECUTE FUNCTION estudiantes_actualizar_version()` |
| estudiantes_db | `matricula_movimientos` | `trg_matricula_movimientos_no_mod BEFORE DELETE OR UPDATE ON public.matricula_movimientos FOR EACH ROW EXECUTE FUNCTION matricula_movimientos_append_only()` |
| estudiantes_db | `matricula_movimientos` | `trg_matricula_movimientos_no_truncate BEFORE TRUNCATE ON public.matricula_movimientos FOR EACH STATEMENT EXECUTE FUNCTION matricula_movimientos_append_only()` |
| estudiantes_db | `matriculas` | `trg_matriculas_actualizar_version BEFORE UPDATE ON public.matriculas FOR EACH ROW EXECUTE FUNCTION estudiantes_actualizar_version()` |
| estudiantes_db | `periodos_academicos` | `trg_periodos_academicos_updated_at BEFORE UPDATE ON public.periodos_academicos FOR EACH ROW EXECUTE FUNCTION tg_actualizar_updated_at()` |
| estudiantes_db | `personas` | `trg_personas_actualizar_version BEFORE UPDATE ON public.personas FOR EACH ROW EXECUTE FUNCTION estudiantes_actualizar_version()` |
| estudiantes_db | `secciones` | `trg_secciones_updated_at BEFORE UPDATE ON public.secciones FOR EACH ROW EXECUTE FUNCTION tg_actualizar_updated_at()` |

| Base | Función propia | Argumentos | Volatilidad |
|---|---|---|---|
| psicologia_db | `atencion_versiones_inmutables` |  | VOLATILE |
| psicologia_db | `auditoria_psicologica_append_only` |  | VOLATILE |
| estudiantes_db | `auditoria_estudiantes_append_only` |  | VOLATILE |
| estudiantes_db | `estudiantes_actualizar_version` |  | VOLATILE |
| estudiantes_db | `estudiantes_edad` | nacimiento date, referencia date | IMMUTABLE |
| estudiantes_db | `estudiantes_normalizar` | texto text | IMMUTABLE |
| estudiantes_db | `matricula_movimientos_append_only` |  | VOLATILE |
| estudiantes_db | `tg_actualizar_updated_at` |  | VOLATILE |

Vistas, secuencias y tipos ENUM propios: **ninguno** en las 7 bases (verificado). Los "catálogos" se implementan con tablas o con CHECK.

## 7. Extensiones instaladas (verificadas)

| Base | Extensiones |
|---|---|
| auth_db | citext 1.6, pgcrypto 1.3, plpgsql 1.0 |
| usuarios_db | citext 1.6, plpgsql 1.0 |
| expedientes_db | pgcrypto 1.3, plpgsql 1.0 |
| documentos_db | pgcrypto 1.3, plpgsql 1.0 |
| derivaciones_db | pgcrypto 1.3, plpgsql 1.0 |
| psicologia_db | plpgsql 1.0 |
| estudiantes_db | pg_trgm 1.6, pgcrypto 1.3, plpgsql 1.0, unaccent 1.1 |

## 8. Dependencias entre servicios (gRPC, verificadas en código)

| Servicio | Llama a | Para qué |
|---|---|---|
| Gateway | Auth, Expedientes, Usuarios, Documentos, Derivaciones, Psicología, Estudiantes | Transporte HTTP → gRPC (autorización en cada servicio) |
| Expedientes | Auth | HasPermission |
| Documentos | Auth, Expedientes | HasPermission; validar expediente |
| Derivaciones | Auth, Expedientes | HasPermission; validar expediente/derivación |
| Usuarios | Auth | HasPermission |
| Psicología | Auth, Usuarios, Estudiantes | HasPermission; validar responsable; existencia y nombre del estudiante |
| Estudiantes | Auth | HasPermission |

No se usan colas ni eventos: toda la integración es **síncrona por gRPC** (no hay broker en `docker-compose.dev.yml`).
