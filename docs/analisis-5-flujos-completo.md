# Análisis funcional completo — 5 flujos institucionales I.E. Tungasuca

> Fuente proporcionada íntegramente por el usuario (2026-09-08). Documento base
> para la Etapa 12 (`etapa-12-especificacion-funcional.md`). Se transcribe tal
> cual fue recibido, como registro de la fuente original.

Los 5 flujos: (1) Cumplimiento de Horas de Clase, (2) Solicitud de Certificado y
Constancia de Estudios, (3) Citación a Padres de Familia, (4) Permisos y
Justificaciones de Faltas y Tardanzas, (5) Monitoreo y Supervisión de Profesores.

## Nota sobre esta copia

El archivo original pegado por el usuario tenía problemas de codificación de
caracteres (acentos/ñ corruptos, ej. "AutenticaciÃ³n"). Esta copia conserva el
contenido literal para trazabilidad; la Etapa 12 (`etapa-12-especificacion-funcional.md`)
re-expresa el mismo contenido con la codificación correcta y en el formato
pedido por esa etapa.

---

## FLUJO 1 — Cumplimiento de Horas de Clase

**Objetivo**: Gestionar, verificar, corregir y consolidar el registro del
cumplimiento de las horas de clase impartidas por los profesores para la
elaboración del informe final enviado a Dirección.

**Actores**: Auxiliar (usuario interno/área administrativa), Profesor (usuario
interno/docente), Sistema, Subdirectora (área directiva), Dirección (área
directiva).

**Inicio**: El Auxiliar habilita/asigna el "Formato de cumplimiento de horas de
clase".

**Entrada**: Formato de cumplimiento de horas de clase; información del
dictado de horas ingresada por el Profesor.

**Flujo paso a paso**:
1. Auxiliar registra el formato de cumplimiento de horas de clase.
2. Auxiliar asigna/habilita el formato para el profesor.
3. Profesor ingresa al sistema.
4. Profesor consulta el formato.
5. Profesor completa la información requerida.
6. Profesor envía el formato a la Subdirectora.
7. Sistema registra y almacena el formato enviado.
8. Sistema notifica a la Subdirectora que el formato fue enviado.
9. Subdirectora revisa la información registrada.
10. Subdirectora verifica el cumplimiento de las horas de clase.
11. Subdirectora (decisión) evalúa si la información está completa y correcta.
12. Si NO: Subdirectora registra observaciones.
13. Sistema notifica al profesor las observaciones.
14. Profesor corrige o completa la información.
15. Profesor envía nuevamente el formato.
16. Si SÍ: Subdirectora consolida la información de cumplimiento de horas.
17. Subdirectora elabora el informe de cumplimiento de horas de clase.
18. Subdirectora envía el informe a Dirección.
19. Sistema genera y almacena el informe.
20. Sistema registra la entrega del informe.
21. Dirección recibe el informe.
22. Dirección revisa el informe.

**Decisión**: ¿La información está completa y correcta? (Subdirectora) → Sí:
consolida y elabora informe. No: registra observaciones, notifica al profesor,
quien corrige y reenvía.

**Documentos generados**: Formato de cumplimiento de horas de clase; registro
de observaciones; informe de cumplimiento de horas de clase.

**Derivaciones**: Profesor→Subdirectora (envía formato); Subdirectora→Profesor
(observaciones, vía notificación); Subdirectora→Dirección (informe
consolidado).

**Estados del trámite**: Registrado/Habilitado; Enviado; Con
Observaciones/Pendiente de corrección; Consolidado; Informe Elaborado; Informe
Entregado/Finalizado.

**Resultado final**: El informe consolidado es recibido y revisado formalmente
por Dirección.

**Reglas de negocio**:
- Explícita: el Auxiliar es responsable de habilitar el formato para el profesor.
- Explícita: la Subdirectora debe verificar y validar la información antes de
  elaborar el informe final.
- Explícita: si existen inconsistencias o faltantes, el formato se devuelve al
  profesor con observaciones notificadas.
- Inferida: el sistema no permite generar el informe final para Dirección
  hasta que la información del formato sea declarada completa y correcta por
  la Subdirectora.

**Información a almacenar**:
- Expediente: código de reporte, periodo/mes, estado general del trámite.
- Documento: tipo (Formato/Informe), nombre, fecha de registro, archivo/data
  de horas, observaciones registradas.
- Usuario: ID Auxiliar, ID Profesor, ID Subdirectora, ID Director.
- Derivación: origen, destino, fecha de envío, estado de revisión.

**Posible lógica digital**: Auxiliar (habilita) → Profesor (completa) →
Expediente (formato enviado) → Subdirectora (revisión) → decisión (¿observado?
→ notificación/corrección profesor) → Subdirectora (consolida e informe) →
documento (informe generado) → derivación (Dirección) → Dirección (revisión) →
cierre.

---

## FLUJO 2 — Solicitud de Certificado y Constancia de Estudios

**Objetivo**: Permitir a padres de familia o solicitantes gestionar en línea
trámites de Certificados o Constancias de Estudios, con validación documental
por Secretaría y remisión final a Dirección.

**Actores**: Padre de Familia/Solicitante (externo); Sistema de Mesa de
Partes; Secretaría (administrativa); Dirección (directiva).

**Inicio**: El solicitante ingresa al sistema, inicia sesión y selecciona
"Nuevo trámite".

**Entrada**: Datos de identificación del solicitante; tipo de trámite
(Certificado o Constancia); formulario digital completado; documentos
requeridos adjuntos.

**Flujo paso a paso**:
1. Solicitante ingresa al sistema.
2. Inicia sesión/se identifica.
3. Selecciona "Nuevo trámite".
4. Selecciona tipo: Certificado o Constancia de Estudios.
5. Completa el formulario digital.
6. Adjunta documentos requeridos.
7. Envía la solicitud.
8. Sistema valida que la información esté completa (decisión).
9. Si NO: muestra observaciones/campos faltantes.
10. Solicitante corrige y reenvía.
11. Si SÍ: sistema registra automáticamente el trámite.
12. Sistema genera código de seguimiento (`#TR-2023-1234`).
13. Sistema cambia el estado a "Recibido".
14. Sistema notifica automáticamente a Secretaría.
15. Secretaría revisa digitalmente la solicitud.
16. Secretaría (decisión) verifica si cumple los requisitos.
17. Si NO: registra una observación.
18. Sistema notifica la observación al solicitante.
19. Solicitante corrige y reenvía.
20. Si SÍ: Secretaría deriva digitalmente el trámite a Dirección.
21. Sistema genera notificación.
22. Sistema actualiza el estado a "Derivado a Dirección".
23. Dirección recibe la solicitud.
24. Dirección revisa el trámite.
25. Sistema registra el estado y permite seguimiento del trámite.

**Documentos generados**: Formulario digital de solicitud; código de
seguimiento; documento de observaciones; expediente digital del trámite.

**Derivaciones**: Sistema→Secretaría (notifica, estado "Recibido");
Secretaría→Dirección (deriva tras aprobar requisitos).

**Estados**: Recibido; Observado; Derivado a Dirección; Finalizado/Atendido.

**Reglas de negocio**:
- Explícita: todo trámite genera automáticamente un código único de
  seguimiento (`#TR-YYYY-XXXX`).
- Explícita: es obligatoria la validación del sistema antes de enviar a
  Secretaría.
- Explícita: Secretaría debe comprobar el cumplimiento de requisitos antes de
  derivar a Dirección.
- Inferida: el solicitante puede subsanar y reenviar tanto por errores del
  sistema como por observaciones de Secretaría.

**Información a almacenar**:
- Expediente: código, tipo (Certificado/Constancia), fecha de creación, estado.
- Documento: requisitos adjuntos, formulario de datos, historial de
  observaciones.
- Usuario: DNI/identificador del solicitante, nombres, teléfono/correo.
- Derivación: fecha de envío a Secretaría, fecha de derivación a Dirección,
  responsable asignado.

---

## FLUJO 3 — Citación a Padres de Familia

**Objetivo**: Estructurar la emisión de citaciones a padres de familia,
registrando acuerdos, compromisos, actas y el seguimiento posterior.

**Actores**: Trabajador del Colegio (interno); Sistema; Padre de Familia
(externo); Responsable del Seguimiento (interno/administrativo o directivo).

**Inicio**: El trabajador del colegio identifica el motivo por el cual se
requiere citar al padre.

**Entrada**: Motivo de la citación; formulario digital de citación; acuerdos y
datos de la reunión.

**Flujo paso a paso**:
1. Trabajador identifica el motivo.
2. Registra una nueva citación.
3. Sistema muestra el formulario digital.
4. Trabajador completa el formulario.
5. Envía la citación.
6. Sistema registra la citación.
7. Sistema genera y registra la notificación de citación.
8. Padre de familia recibe la citación.
9. Padre acude al colegio en la fecha indicada.
10. Trabajador realiza la reunión.
11. Padre lee y firma el acta.
12. Padre asume el compromiso.
13. Trabajador registra los acuerdos.
14. Trabajador elabora el acta.
15. Sistema guarda el acta y el compromiso.
16. Sistema registra el seguimiento del compromiso.
17. Responsable del Seguimiento realiza seguimiento.
18. Responsable (decisión) verifica si se cumplió el compromiso.
19. Si SÍ: registra cumplimiento (fin).
20. Si NO: registra incumplimiento.
21. Responsable (decisión) evalúa si se requiere nueva citación.
22. Si SÍ: genera nueva citación (reinicia el flujo).
23. Si NO: registra el resultado del seguimiento (fin).

**Documentos generados**: Formulario digital de citación; notificación de
citación; acta de reunión firmada; documento/registro de compromiso; registro
de seguimiento/incumplimiento.

**Derivaciones**: Trabajador→Padre (notificación de citación);
Trabajador→Sistema/Responsable de Seguimiento (transferencia de acta y
compromisos).

**Estados**: Citación Registrada/Notificada; Reunión Realizada; Acta
Guardada/Compromiso Registrado; En Seguimiento; Compromiso Cumplido;
Compromiso Incumplido; Finalizado.

**Reglas de negocio**:
- Explícita: la reunión debe concluir con lectura, firma del acta y
  aceptación del compromiso por el padre.
- Explícita: un compromiso no finaliza tras la reunión; requiere una etapa
  explícita de seguimiento.
- Explícita: si hay incumplimiento, el responsable puede reabrir el ciclo
  generando directamente una nueva citación.
- Inferida: los compromisos deben guardarse asociados al historial del
  estudiante/padre.

**Información a almacenar**:
- Expediente: código de citación, ID estudiante, ID padre, estado del
  compromiso.
- Documento: formulario de citación, acta firmada, ficha de compromiso,
  historial de seguimiento.
- Usuario: ID trabajador emisor, ID padre, ID responsable de seguimiento.
- Derivación: fecha/hora pactada de reunión, fecha límite de verificación de
  compromiso.

---

## FLUJO 4 — Permisos y Justificaciones de Faltas y Tardanzas

**Objetivo**: Estandarizar la recepción, evaluación administrativa,
autorización directiva y notificación docente de solicitudes de permisos y
justificaciones presentadas por los padres de familia.

**Actores**: Padre de Familia/Solicitante (externo); Sistema de Mesa de
Partes; Secretaría (administrativa); Dirección (directiva); Subdirección
(directiva); Profesor (interno/docente).

**Inicio**: El solicitante inicia sesión y selecciona "Nuevo trámite".

**Entrada**: Selección del estudiante; tipo de solicitud (Permiso/
Justificación de Falta/Justificación de Tardanza); motivo y datos declarados;
documentos sustentatorios.

**Flujo paso a paso**:
1. Solicitante ingresa e inicia sesión.
2. Selecciona "Nuevo trámite".
3. Selecciona tipo: Permiso/Justificación Falta/Justificación Tardanza.
4. Selecciona estudiante.
5. Completa formulario digital.
6. Adjunta documentos sustentatorios y envía.
7. Sistema valida la información (decisión).
8. Si NO: solicita completar/corregir; el padre corrige y reenvía.
9. Si SÍ: registra el trámite y genera número de seguimiento (`#TR-2023-5678`).
10. Sistema establece estado "Recibido" y notifica a Secretaría.
11. Secretaría revisa la solicitud (decisión).
12. Si NO está correctamente presentada: registra observación; el sistema
    notifica al padre; el padre corrige; Secretaría vuelve a revisar.
13. Si SÍ: actualiza estado a "Derivado a Dirección".
14. Dirección recibe y revisa la solicitud (decisión).
15. Si NO aprueba: registra motivo de desaprobación; estado "Desaprobado";
    fin.
16. Si SÍ aprueba: deriva digitalmente a Subdirección.
17. Subdirección recibe y revisa la solicitud.
18. Subdirección aprueba la solicitud.
19. Sistema registra la aprobación (estado "Aprobado").
20. Sistema genera digitalmente el proveído (PDF).
21. Sistema notifica al profesor encargado.
22. Sistema registra todo el historial del trámite.
23. Profesor recibe la notificación.
24. Profesor consulta la información y queda informado.
25. Sistema registra todo el historial y finaliza.

**Decisiones**:
- ¿La información está completa? (Sistema) → Sí: "Recibido", notifica a
  Secretaría. No: exige corrección.
- ¿La solicitud está correctamente presentada? (Secretaría) → Sí: "Derivado a
  Dirección". No: notifica observación al padre.
- ¿Se aprueba la solicitud? (Dirección) → Sí: deriva a Subdirección para
  formalización. No: registra motivo, "Desaprobado" (fin).

**Documentos generados**: Formulario de Permiso/Justificación; código de
seguimiento; registro de observación; proveído digital firmado/generado
(PDF); registro de desaprobación.

**Derivaciones**: Sistema→Secretaría (alerta); Secretaría→Dirección
(trámite formalizado); Dirección→Subdirección (derivación aprobatoria);
Subdirección/Sistema→Profesor (notificación e inclusión en módulo docente).

**Estados**: Recibido; Observado; Derivado a Dirección; Aprobado; Desaprobado.

**Reglas de negocio**:
- Explícita: la denegación en Dirección corta el proceso inmediatamente
  registrando el motivo.
- Explícita: la aprobación requiere el visto bueno de Dirección **y** la
  ratificación/proveído de Subdirección.
- Explícita: todo el flujo debe quedar registrado en una trazabilidad/
  historial del trámite.
- Inferida: el docente no toma decisiones de aprobación; solo actúa como
  receptor notificado del resultado.

**Información a almacenar**:
- Expediente: código, estudiante asociado, tipo de falta/permiso, estado.
- Documento: adjuntos del padre, proveído digital PDF, registro de motivo de
  rechazo.
- Usuario: ID padre, ID Secretaría, ID Director, ID Subdirectora, ID Profesor.
- Derivación: trazabilidad completa con fechas y decisiones de Secretaría,
  Dirección y Subdirección.

---

## FLUJO 5 — Monitoreo y Supervisión de Profesores

**Objetivo**: Digitalizar la ejecución, evaluación, registro de observaciones
y retroalimentación de las sesiones de monitoreo pedagógico realizadas por la
Subdirectora a los profesores.

**Actores**: Subdirectora (directiva); Sistema; Profesor (interno/docente).

**Inicio**: La Subdirectora inicia sesión y selecciona "Nuevo Monitoreo".

**Entrada**: Docente seleccionado; sesión o curso seleccionado; criterios de
evaluación completados; observaciones y recomendaciones (si aplican).

**Flujo paso a paso**:
1. Subdirectora inicia sesión.
2. Selecciona "Nuevo Monitoreo".
3. Selecciona al profesor a monitorear.
4. Selecciona la sesión o curso.
5. Sistema muestra el formulario digital de monitoreo.
6. Subdirectora observa la sesión.
7. Evalúa los criterios de monitoreo.
8. Registra los resultados de la evaluación.
9. Subdirectora (decisión) evalúa si existen observaciones o aspectos por
   mejorar.
10. Si SÍ: registra observaciones y recomendaciones.
11. Si NO: continúa con el cierre del monitoreo.
12. Finaliza el monitoreo.
13. Sistema guarda el monitoreo.
14. Sistema registra la información del monitoreo.
15. Sistema actualiza el historial del profesor.
16. Sistema (decisión) evalúa si se debe notificar al profesor.
17. Si SÍ: envía notificación al profesor.
18. Profesor recibe la notificación.
19. Profesor consulta el resultado y las observaciones (fin).
20. Si NO se notifica: finaliza el proceso directamente.

**Documentos generados**: Formulario digital de monitoreo; ficha de
evaluación pedagógica; ficha de observaciones y recomendaciones; historial de
desempeño del profesor.

**Derivaciones**: Subdirectora→Sistema (carga de evaluación y cierre);
Sistema→Profesor (notificación condicional del resultado).

**Estados**: En Proceso/Monitoreo Iniciado; Evaluado; Finalizado/Guardado;
Notificado al Profesor.

**Reglas de negocio**:
- Explícita: el monitoreo impacta directamente en el historial acumulativo
  del profesor.
- Explícita: es opcional/condicional el envío de la notificación del
  resultado al docente, según regla del sistema.
- Inferida: la Subdirectora es el único rol facultado para abrir y calificar
  pautas de supervisión pedagógica.

**Información a almacenar**:
- Expediente: ID monitoreo, profesor evaluado, curso/sesión, fecha/hora.
- Documento: ficha de criterios evaluados, detalle de recomendaciones/
  observaciones.
- Usuario: ID Subdirectora, ID Profesor.
- Derivación: flag de notificación enviada (sí/no), fecha de lectura por el
  docente.

---

## Matrices ya provistas por la fuente (se reutilizan en la Etapa 12)

- **Matriz General de Procesos** (flujo, proceso, actor principal, entrada,
  resultado, áreas involucradas).
- **Matriz de Actores** (actor, tipo, procesos donde participa, acciones).
- **Matriz de Documentos** (documento, quién lo genera, quién lo recibe,
  proceso, momento).
- **Matriz de Estados** (estado, cuándo aparece, quién lo modifica, siguiente
  estado).
- **Reglas de negocio consolidadas** por categoría: Registro, Expedientes,
  Documentos, Derivaciones, Revisiones, Aprobaciones, Observaciones, Atención,
  Cierre.
- **Mapeo de lógica hacia microservicios** (Auth, Usuarios, Expedientes,
  Documentos, Derivaciones, Seguimiento, Notificaciones, Reportes).

Estas matrices se incorporan (reformateadas, no reinterpretadas) dentro de
`etapa-12-especificacion-funcional.md`.
