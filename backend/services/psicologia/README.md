# Psicología Service

Microservicio del módulo de Psicología: **expediente psicológico permanente por
estudiante** (FASE 1: base del módulo). Ver el diseño completo y las decisiones
en [`docs/modulo-psicologia-diseno.md`](../../../docs/modulo-psicologia-diseno.md).

No es el expediente administrativo de Mesa de Partes: son conceptos distintos
(`expedientes` vs `expedientes_psicologicos`).

## Responsabilidad

- Es dueño de `expedientes_psicologicos`, su código correlativo y la bitácora
  `auditoria_psicologica`.
- **No** es dueño de usuarios, roles, permisos ni estudiantes. Solo guarda
  `estudiante_id` (UUID, sin FK) y `responsable_id` / `abierto_por` (UUID de Auth).
- No emite tokens: valida el JWT de Auth con el mismo `JWT_SECRET`.
- No decide autorización por nombre de rol: consulta `Auth.HasPermission` con la
  identidad del JWT.

## Arquitectura

```
React → Gateway :8888 → Psicología :8086 → psicologia_db
                          ├─ Auth :8080      (HasPermission, fail-closed)
                          └─ Usuarios :8083  (validar responsable_id)
```

- gRPC: `127.0.0.1:8086` (solo loopback; el único llamador es el Gateway).
- PostgreSQL: `psicologia_db`, usuario `psicologia_user` (mismo servidor 5433).

## Base de datos (una sola vez)

Roles y privilegios: ver `backend/db/roles/README.md` (FASE 6N-3, C2). Ningún
servicio usa un superusuario: `psicologia_user` solo tiene DML mínimo (sin
`DELETE`), las tablas pertenecen a `psicologia_owner` y las migraciones las aplica
`psicologia_migrator` actuando como dueño. La contraseña de `psicologia_user` vive
en `backend/.env` (`PSICOLOGIA_DB_PASSWORD`); nunca en un YAML.

```bash
# migraciones, en orden, como migrador (nunca como psicologia_user ni como tg_admin)
for m in 001_create_psicologia_base 002_create_atenciones 003_cifrado_contenido; do
  (echo "SET ROLE psicologia_owner;"; cat migrations/$m.sql) |     docker exec -i tg-postgres psql -U psicologia_migrator -d psicologia_db -v ON_ERROR_STOP=1
done
```

Un trigger rechaza `UPDATE/DELETE/TRUNCATE` sobre `auditoria_psicologica` incluso
para el dueño, y otro impide modificar las versiones registradas de una atención.

Además hay dos migraciones de otros servicios necesarias para el rol y los
permisos: `auth/migrations/010_seed_psicologia_role_and_permissions.sql` y
`usuarios/migrations/002_add_psicologo_tipo_usuario.sql`.

## Cifrado del contenido clínico (FASE 6N-3, C1)

Los textos libres de la Hoja de Atención (y su motivo de corrección), el motivo de
anulación, el resumen del caso y el motivo de cierre se guardan **cifrados** con
AES-256-GCM (`internal/clinicalcrypto`): formato `0x01 ‖ nonce(12) ‖ cifrado ‖
etiqueta(16)`, `cifrado_key_id` por fila y datos autenticados que ligan cada
bloque a su fila (un bloque copiado o alterado no se descifra). Fecha, hora y las
categorías (realizada por, motivos, derivaciones) quedan en claro: son códigos de
catálogo que usan el listado y los reportes agregados.

- Clave: `CLINICAL_DATA_ENCRYPTION_KEY` (base64 de 32 bytes) y `CLINICAL_DATA_KEY_ID`
  en `backend/.env`; `CLINICAL_DATA_PREVIOUS_KEYS` solo durante una rotación. Solo
  Psicología la conoce (nunca el Gateway, el frontend, PostgreSQL ni los logs).
- Canario: al arrancar, Psicología descifra `cifrado_canario` para cada clave
  configurada. Sin clave, con una clave incorrecta o sin canario, **no inicia**. El
  canario lo crea `cmd/cifrar-contenido --inicializar-canario`, nunca el servicio.
- `cmd/cifrar-contenido` (`--dry-run`, `--aplicar`, `--verificar`, `--huella`,
  `--revertir`) migró los datos existentes; `migrations/004_eliminar_texto_plano.sql`
  eliminó después las columnas en claro. Se configura solo por variables de entorno
  (`PSICOLOGIA_MIGRATOR_DB_URL` y las `CLINICAL_DATA_*`) y nunca imprime contenido.
- **Si se pierde la clave, las atenciones cifradas son irrecuperables**: debe existir
  una copia bajo custodia institucional, separada de los backups de la base.

## Ejecutar

`backend/run-all.ps1` lo levanta junto con el resto (lee `backend/.env`). Manual:

```bash
cd backend/services/psicologia
# con las variables de backend/.env cargadas en el entorno
go run psicologia.go -f etc/psicologia.yaml
```

Pruebas de integración: `INTEGRATION_TEST=true` más `backend/.env` y
`backend/.env.test` (`TEST_PSICOLOGIA_DB_URL`) cargados en el entorno. Usan una
clave de cifrado aleatoria por corrida.

## RPC

`OpenExpediente`, `GetExpediente`, `GetExpedienteByEstudiante`,
`ListExpedientes`, `UpdateExpediente`, `CloseExpediente`, `ReopenExpediente`.

## Pruebas

```bash
INTEGRATION_TEST=true go test ./...
```

Usan `psicologia_db` real y el servicio real (JWT, lógica, repositorio); Auth y
Usuarios se sustituyen por dobles que conceden permisos por `userID`. Los datos
de prueba se eliminan al terminar (la limpieza de la bitácora la hace un
superusuario, porque la aplicación no puede borrarla).

## Dependencias pendientes

- **Estudiantes Service** (no existe): `svc.EstudianteVerifier` es el punto de
  extensión; mientras sea `nil` solo se valida el formato del `estudiante_id`.
