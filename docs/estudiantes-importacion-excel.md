# Importación y exportación Excel de estudiantes

Módulo del servicio **Estudiantes**. Escribe únicamente en la base maestra `estudiantes_db`. El Gateway solo transporta los archivos.

## Formato oficial

Hoja `Estudiantes`. La primera fila debe tener exactamente estos encabezados, en este orden:

`Nro | TIPO.DOC | DNI | APELLIDO PATERNO | APELLIDO MATERNO | NOMBRES | CONDICION | ESTADO | NIVEL`

Al leer el archivo se toleran espacios extra y diferencias de mayúsculas en los encabezados.

| Columna | Regla |
|---|---|
| Nro | Solo numeración de referencia. No identifica al estudiante. |
| TIPO.DOC | `DNI`, `CE`, `PASAPORTE` o `SIN_DOCUMENTO` (catálogo `cat_tipos_documento`). Si TIPO.DOC y DNI están vacíos, se toma como `SIN_DOCUMENTO`. |
| DNI | Siempre se lee como texto: conserva ceros iniciales y nunca sale en notación científica. DNI: 8 dígitos. CE y PASAPORTE: de 4 a 12 letras o dígitos. |
| APELLIDO PATERNO | Obligatorio, hasta 60 caracteres. |
| APELLIDO MATERNO | Opcional, hasta 60 caracteres. |
| NOMBRES | Obligatorio, hasta 100 caracteres. Se conservan tildes y ñ. |
| CONDICION | Solo `ESTUDIANTE`. El maestro no tiene columna de condición: todo registro de `estudiantes` es un estudiante. |
| ESTADO | `ACTIVO` o `INACTIVO` (`chk_estudiantes_estado`). |
| NIVEL | `INICIAL`, `PRIMARIA` o `SECUNDARIA`, el mismo catálogo que las secciones. Se guarda en la columna heredada `estudiantes.nivel`. No crea matrículas. |

Si un texto empieza con `=`, se rechaza porque es una fórmula. Al exportar, todas las celdas se escriben como texto.

## Endpoints (Gateway)

| Método | Ruta | Permiso | Cuerpo / respuesta |
|---|---|---|---|
| GET | `/api/estudiantes/importacion/plantilla` | `estudiantes.create` | `{nombre, contenido_base64}` |
| POST | `/api/estudiantes/importacion/preview` | `estudiantes.create` | multipart: `archivo` → resumen y filas |
| POST | `/api/estudiantes/importacion/confirmar` | `estudiantes.create` | multipart: `archivo` + `huella` → resultado por fila |
| GET | `/api/estudiantes/exportar?estado=&nivel=` | `estudiantes.view` | `{nombre, contenido_base64, filas}` |

Límites, configurables en `Excel:` dentro de `etc/estudiantes.yaml`:

- Archivo: 1 MB.
- Importación: 2000 filas.
- Exportación: 10 000 filas. Si se supera, se pide filtrar.

## Identidad y duplicados

- **Con documento:** la clave es (tipo, número), que es el índice único `uq_estudiantes_documento`.
  - Un estudiante ya registrado **no se modifica**: aparece como «Ya registrado» y se omite.
  - Si un documento se repite dentro del archivo, la segunda aparición se rechaza indicando la fila original.
- **Sin documento:** la fila se importa. Si el nombre coincide con un estudiante registrado, o con otra fila sin documento del archivo, solo se **avisa**. Nunca se deduce la identidad por el nombre.
- **Código del maestro**, que es obligatorio y único:
  - Con documento: `DNI########`, `CE…` o `PAS…`.
  - Sin documento: `SD` + 12 caracteres hexadecimales derivados de la huella del archivo y la fila.
  - En ambos casos es determinista, así que confirmar dos veces el mismo archivo no duplica.
- **Concurrencia:** los índices únicos deciden. Si dos confirmaciones simultáneas llevan al mismo estudiante, la segunda lo recibe como «Omitido».

## Flujo

1. **Vista previa:** valida todo y no escribe nada. Devuelve la `huella`, que es el SHA-256 del archivo.
2. **Confirmación:** exige la misma huella, vuelve a leer y validar el archivo e inserta solo las filas válidas.
   - Usa la misma lógica que el alta individual (`validarMaestro` + `Create`).
   - Cada alta queda en `auditoria_estudiantes` como `CREATE` con el usuario.
   - La operación completa se registra en el log sin datos personales: usuario, huella y totales.
3. **Resultado por fila:** `INSERTADA`, `OMITIDA` o `RECHAZADA`.
   - La importación puede ser parcial, y el resultado lo muestra explícitamente.
   - Ante un error inesperado se detiene; reintentar es seguro.

La importación no toca matrículas, familiares, personas ni otros servicios.
