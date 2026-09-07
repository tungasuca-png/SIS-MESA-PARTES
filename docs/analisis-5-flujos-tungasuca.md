# Análisis funcional — 5 flujos institucionales I.E. Tungasuca

> Fuente proporcionada por el usuario (2026-09-08). Usado como base para el diseño de
> Documentos Service (y futuros: Derivaciones, Seguimiento, Notificaciones, Reportes).
> Los 5 flujos: (1) Cumplimiento de Horas de Clase, (2) Solicitud de Certificado y
> Constancia de Estudios, (3) Citación a Padres de Familia, (4) Permisos y
> Justificaciones de Faltas y Tardanzas, (5) Monitoreo y Supervisión de Profesores.

## Relevante para Documentos Service

Cita textual del mapeo a microservicios provisto:

> **Documentos Service**: Carga de adjuntos, generación de Proveídos (PDF), Actas,
> Informes y Formato de Horas.

Documentos identificados por flujo (Matriz de Documentos del análisis):

| Documento | Quién lo genera | Quién lo recibe | Flujo |
|---|---|---|---|
| Formato de Cumplimiento de Horas | Auxiliar/Profesor | Subdirectora | F1 |
| Informe de Cumplimiento de Horas | Subdirectora | Dirección | F1 |
| Formulario de Citación | Trabajador del Colegio | Padre de Familia | F3 |
| Acta de Reunión / Compromiso | Trabajador/Padre | Responsable de Seguimiento | F3 |
| Proveído Digital (PDF) | Subdirección/Sistema | Profesor | F4 |
| Ficha de Monitoreo/Evaluación | Subdirectora | Sistema/Profesor | F5 |

Nota: la "Solicitud de Certificado/Constancia" (F2) y la "Solicitud de Permiso/
Justificación" (F4) ya se modelan como el propio Expediente en este sistema
(Expedientes Service), no como un documento separado.

Regla de negocio consolidada citada: "Las resoluciones favorables de permisos o
justificaciones deben generar automáticamente un proveído en formato PDF
estandarizado." — Documentos Service no genera el PDF automáticamente en esta
etapa (eso requeriría una plantilla/motor de generación no definido aún); se deja
como carga manual del proveído ya elaborado, documentado como pendiente.

El análisis completo (los 5 flujos con actores, decisiones, estados, reglas y
matrices consolidadas) fue provisto en el chat y no se transcribe aquí en su
totalidad; este archivo referencia solo la parte usada para el diseño actual.
