# Documentos Service

Microservicio independiente responsable de los documentos asociados a un expediente:
adjuntos cargados por el solicitante (sustentos) y documentos generados por el
personal interno (proveídos, actas, informes, formatos).

Basado en el análisis funcional de los 5 flujos institucionales (ver
`docs/analisis-5-flujos-tungasuca.md` en la raíz del repo): "Documentos Service:
Carga de adjuntos, generación de Proveídos (PDF), Actas, Informes y Formato de Horas."

## Responsabilidad

- Guardar el archivo (metadata + contenido binario) de un documento.
- Consultar metadata, descargar contenido, listar por expediente, dar de baja
  (lógica) un documento.
- Validar el JWT emitido por Auth Service (no emite tokens propios).

Lo que **no** hace: generar automáticamente el PDF de un proveído (requeriría un
motor de plantillas no definido aún — se documenta como pendiente), ni verificar
contra Expedientes Service si el `expediente_id` indicado realmente existe o
pertenece al solicitante (ver limitación en `internal/authorization`).

## Arquitectura

```
React/Vite
    ↓
API Gateway :8888
    ↓ gRPC
Documentos Service :8084
    ↓
documentos_db (PostgreSQL, aislada — sin FK hacia auth_db/expedientes_db/usuarios_db)
```

## Puerto y base de datos

- gRPC: `0.0.0.0:8084`
- PostgreSQL: `documentos_db`, usuario `documentos_user` (mismo servidor Postgres
  que los demás servicios, base y rol propios).

## Variables de entorno

- `JWT_SECRET`: debe ser idéntico al usado por Auth Service (y por Expedientes/
  Usuarios), o los tokens emitidos por Auth serán rechazados aquí.

## gRPC

`UploadDocumento`, `GetDocumento`, `DownloadDocumento`, `ListDocumentos`,
`DeleteDocumento` — ver `documentos_service.proto`.

## Autorización (modelo propio, no integrado con Auth todavía)

- `UploadDocumento`: cualquier usuario autenticado; un `SOLICITANTE` solo puede
  subir tipo `ADJUNTO`, el personal interno puede subir cualquier tipo.
- `GetDocumento` / `DownloadDocumento` / `ListDocumentos`: cualquier usuario
  autenticado (limitación conocida: no verifica dueño real del expediente).
- `DeleteDocumento`: solo personal interno.

## Límite de tamaño

8 MiB por archivo (`internal/validation.MaxTamanoDocumento`). El límite de mensaje
gRPC (`MaxRecvMsgSize`/`MaxSendMsgSize`) se configura en `documentos.go` con margen
sobre ese valor.

## Cómo ejecutar

```bash
JWT_SECRET="..." go run documentos.go -f etc/documentos.yaml
```

## Cómo probar

```bash
go build ./... && go vet ./... && go test ./...
INTEGRATION_TEST=true go test ./internal/integration/...
```

## Relación con otros servicios

- **Auth Service**: fuente del JWT; Documentos solo lo valida.
- **Expedientes Service**: dueño del `expediente_id` que Documentos referencia
  (sin FK, sin llamada cross-service en esta etapa — ver limitación arriba).
- **Gateway**: expone `/api/documentos` reenviando el header `Authorization`.
