# Roles de PostgreSQL (FASE 6N-3, C2)

Mínimo privilegio: ningún servicio se conecta como superusuario ni como dueño
de sus tablas. Este directorio NO contiene contraseñas.

| Rol | Login | Uso | Privilegios |
|---|---|---|---|
| `tg_admin` | sí | Administración humana (bootstrap del contenedor) | SUPERUSER. Credenciales solo en `TG_SECRETS_DIR` (fuera del repositorio). Nunca en YAML, `.env`, código, pruebas ni Compose. |
| `<s>_owner` | no | Dueño de la base `<s>_db` y de sus objetos | Todos sobre su base |
| `<s>_migrator` | sí | Solo migraciones | Miembro de `<s>_owner` **sin herencia**: debe ejecutar `SET ROLE <s>_owner`. Contraseña en `TG_SECRETS_DIR/migrators.env`. |
| `<s>_tester` | sí | Solo pruebas de integración en desarrollo | DML sobre su base. Credenciales en `backend/.env.test` (fuera de Git). No debe existir en producción. |
| `auth_app`, `<s>_user` | sí | Ejecución de cada servicio | Solo DML mínimo (ver `02_propiedad_y_permisos.sql`). Contraseña en `backend/.env` (`<S>_DB_PASSWORD`). |

`<s>` ∈ auth, psicologia, estudiantes, expedientes, documentos, derivaciones, usuarios.

## Aplicar (como tg_admin)

```powershell
docker exec -i tg-postgres psql -U tg_admin -d postgres -v ON_ERROR_STOP=1 < 01_roles.sql
docker exec -i tg-postgres psql -U tg_admin -d postgres -v ON_ERROR_STOP=1 < 02_propiedad_y_permisos.sql
```

Ambos scripts son idempotentes y `02` puede ejecutarse en cualquier momento.
El orden depende de la situación:

| Situación | Orden |
|---|---|
| **Instalación nueva** (servidor vacío) | `01` → `02` → migraciones de cada servicio → `02` |
| **Actualización** (bases existentes) | `01` → migraciones nuevas → `02` |

En una instalación nueva, la primera pasada de `02` crea las bases que falten
(ya con su `<s>_owner`), retira `CONNECT` a PUBLIC y concede `CONNECT` a cada
rol de servicio por su nombre, de modo que `<s>_migrator` pueda aplicar las
migraciones. La segunda pasada concede los privilegios sobre las tablas que
esas migraciones crearon (todo GRANT sobre una tabla concreta es condicional).
No hace falta ninguna concesión manual.

Roles de una sola función (A + B y reconciliación): `expedientes_publicador`,
`derivaciones_proyector` y `reconciliador_movimientos`. `expedientes_user` no
tiene DELETE sobre ninguna tabla de `expedientes_db` (solo bajas lógicas).

Las contraseñas se asignan aparte, enviando `ALTER ROLE <rol> PASSWORD '...'` por
**stdin** a `psql` (nunca como argumento de línea de comandos ni en un archivo
versionado), y se guardan solo en `backend/.env`, `backend/.env.test` o
`TG_SECRETS_DIR`.

## Migraciones

Siempre con el migrador del servicio y actuando como dueño, para que los objetos
nuevos pertenezcan a `<s>_owner`:

```powershell
docker exec -i tg-postgres psql -U psicologia_migrator -d psicologia_db -v ON_ERROR_STOP=1 `
  -c "SET ROLE psicologia_owner" -f - < ..\..\services\psicologia\migrations\003_cifrado_contenido.sql
```

(Dentro del contenedor la conexión local no pide contraseña; desde el host,
usar la contraseña de `TG_SECRETS_DIR/migrators.env`.)

Toda migración nueva debe conceder explícitamente a `auth_app` / `<s>_user` los
privilegios que necesite: el rol de aplicación ya no es dueño de nada.

**Y además debe añadirlos a `02_propiedad_y_permisos.sql`** (bloque condicional
con `to_regclass`): ese script hace `REVOKE ALL ON ALL TABLES ... FROM auth_app`
y, si una tabla no figura en él, reejecutarlo le quita a `auth_app` lo que la
migración le concedió. Hoy cubre las migraciones de Auth hasta la 025
(019/020 `service_clients*`, 022 `user_auth_providers`, 023
`oauth_login_codes`, `auth_eventos` y la función
`cancelar_registro_google_pendiente(uuid)`, 024 `registros_correo` y la
función `cancelar_registro_correo_pendiente(uuid)`, 025
`identidad_solicitantes`).

## Orden de aplicación

1. `01_roles.sql` (tg_admin): crea `<s>_owner`, `<s>_migrator`, `<s>_tester` y
   los roles de aplicación. Idempotente.
2. Contraseñas por stdin (ver arriba).
3. Bases nuevas: crearlas con dueño `<s>_owner` y las extensiones
   (`pgcrypto`, `citext`) como tg_admin.
4. `02_propiedad_y_permisos.sql` (tg_admin): dueños y privilegios mínimos.
   Idempotente y **reejecutable en cualquier momento**: deja a `auth_app`
   exactamente con el mínimo declarado, también para las tablas de
   migraciones posteriores que ya existan.
5. Migraciones de cada servicio, en orden numérico, con `<s>_migrator` +
   `SET ROLE <s>_owner`. Cada una concede lo suyo a la aplicación.
6. Tras añadir una migración con permisos nuevos: actualizar el bloque
   correspondiente de `02_propiedad_y_permisos.sql` en el mismo cambio.

Privilegios esperados de `auth_app` en `auth_db` (cualquier otro es un error):

| Tabla | Privilegios |
|---|---|
| `usuarios` | SELECT, INSERT, UPDATE (**nunca DELETE**) |
| `usuario_roles`, `role_permissions` | SELECT, INSERT, DELETE |
| `refresh_tokens`, `permissions` | SELECT, INSERT, UPDATE |
| `roles` | SELECT |
| `user_profiles` | ninguno |
| `service_clients` | SELECT, INSERT, UPDATE |
| `service_client_permissions` | SELECT, INSERT, DELETE |
| `user_auth_providers`, `oauth_login_codes` | SELECT, INSERT, UPDATE |
| `auth_eventos` | INSERT (sin lectura) |
| `registros_correo`, `identidad_solicitantes` | SELECT, INSERT, UPDATE |
| funciones `cancelar_registro_google_pendiente(uuid)` y `cancelar_registro_correo_pendiente(uuid)` | EXECUTE (SECURITY DEFINER, dueño `auth_owner`; nadie más) |
En las bases de Mesa de Partes y para los roles `<s>_tester`, los privilegios por
defecto (`ALTER DEFAULT PRIVILEGES`) ya cubren las tablas nuevas creadas por
`<s>_owner`. Excepción: `auth_tester` no tiene `DELETE` sobre `registros_correo`
ni `identidad_solicitantes` (migración Auth 026; el script lo vuelve a retirar si
se reejecuta). Puede seguir eliminando sus usuarios de prueba: las acciones
`ON DELETE` de las claves foráneas se ejecutan como el dueño de la tabla.
