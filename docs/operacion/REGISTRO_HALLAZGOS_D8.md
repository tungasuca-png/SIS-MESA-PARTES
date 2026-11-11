# Registro de hallazgos — release D8 (actualizado 2026-11-10)

Estados: **CORREGIDO** (código o documento cambiado y probado), **PREPARADO** (procedimiento listo,
no aplicado), **PENDIENTE** (necesita decisión o autorización). Nada está desplegado en el servidor.

| Id | Prioridad | Hallazgo | Estado | Evidencia |
|---|---|---|---|---|
| C-01 | Bloqueante (publicador) | Nadie tiene `integraciones.manage`; no se puede crear el cliente técnico del publicador | PENDIENTE (decisión institucional) | Base desechable: 0 roles con el permiso; solo existen PSICOLOGO y SOLICITANTE. Propuesta: `C-01_PROPUESTA_PERMISOS.md` |
| H-03 | Alta | Con el publicador deshabilitado, el seguimiento no muestra los movimientos nuevos | PENDIENTE (decidir orden) | `historial_expediente` lee `movimientos_proyeccion`, que solo llena el publicador. `RELEASE_D8.md` §1 |
| SO-01 | Crítica | Windows Server sin parches desde 2021-08 (KB5005552) | PREPARADO | `Get-HotFix` por SSH. `HARDENING_SRV-TUNGASUCA.md` §2 |
| H-01 | Alta | Compilación con `cd` encadenados; rutas rotas | CORREGIDO | Bloque §2 ejecutado de punta a punta en un clon limpio de `ba18882` (79 s): 10 binarios con `vcs.revision` y `vcs.modified=false`, `VERSION.txt`, `SHA256SUMS`. Probado: no sobrescribe un release existente y se niega a compilar desde un checkout con archivos sin seguimiento |
| H-02 | Alta | El respaldo dependía de una credencial DPAPI que no autentica | CORREGIDO (procedimiento) | §4.2 y §4.3 ejecutados contra una base desechable: 7 dumps + globales, validación, SHA256 y `RESPALDO_VERIFICADO.txt`; con contraseña incorrecta se detiene sin crear la carpeta. La prueba destapó que `$B` pisaba `$b` (PowerShell no distingue mayúsculas): corregido. **En producción falta** que alguien conozca la contraseña del superusuario |
| M-03 | Media | Pruebas sin usuarios institucionales definidos | CORREGIDO (documento) / PENDIENTE (cuentas) | `RELEASE_D8.md` §5.1: roles, permisos verificados en el código, matriz de 14 casos, todos PENDIENTES |
| B-01 | Baja → corregida | Cinco validadores de JWT humano no rechazaban tokens con `token_type` | CORREGIDO | derivaciones, documentos, expedientes, psicologia, usuarios: `security/jwt.go` + `jwt_tokentype_test.go` (rechaza `service`, `refresh` y tokens vencidos; acepta el humano) |
| M-02 | Media | Roles `*_tester` con LOGIN en producción; `01_roles.sql` les devolvía LOGIN al reejecutarse | CORREGIDO (script) / PREPARADO (producción) | `01_roles.sql` ya no fuerza LOGIN; probado: un `NOLOGIN` sobrevive a la reejecución. `HARDENING` §6 |
| M-01 | Media | Sin HTTPS; sin CSP en la SPA | PREPARADO / PENDIENTE (acceso público) | 0 certificados por SSH. Plantilla `iis/web.config`; opciones en `HARDENING` §4 |
| B-02 | Baja | PostgreSQL escucha en `*` | PREPARADO | `pg_hba` solo localhost: sin dependencias remotas. `HARDENING` §5 |
| B-03 | Baja | `Default Web Site` activo en `*:80` | PREPARADO | Sin aplicaciones; solo `iisstart`. `HARDENING` §3 |
| C-03 | Media | DSN sin escapar | CORREGIDO (`6bf912d`) | Pruebas `TestPostgresDSN*` en los 7 módulos |
| C-04 | Media | Lease del publicador sin validar | CORREGIDO (`89f70b7`) | `TestConfigValidar` |

## Migraciones (base desechable PostgreSQL 18, 127.0.0.1:55433, eliminada al terminar)

- Instalación completa en orden: `01` (×2) → `02` → las 54 migraciones de los 7 servicios → `02` (×2). Todas `rc=0`.
- Idempotencia: las cinco migraciones D8 se reaplicaron dos veces, con `rc=0`.
- `TestReconciliacionD8Integracion` (código real): PASS.
- 36 pruebas de permisos y triggers por rol: 36 PASS. Cubren la guardia 005, append-only, el outbox (solo nace PENDIENTE, fencing, lease ≤ 10 min, purga), el mínimo privilegio del publicador y del reconciliador, `SECURITY DEFINER` con `search_path` fijo, la proyección en orden, y la audiencia inmutable y el bloqueo de `derivaciones.proyectar` para humanos.
- Concurrencia del publicador con el SQL real de `publicador.go`: solo reclama la cabeza; no roba un lease vigente; `SKIP LOCKED` no espera (91 ms).
- Reversión: `rollback_expedientes.sql` y `rollback_derivaciones.sql` restablecen el comportamiento de `85dcee7` sin perder el append-only; reaplicar 005 y 002 restablece las guardias.
- **En producción no se ha aplicado ninguna.**
