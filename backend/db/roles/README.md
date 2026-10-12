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
En las bases de Mesa de Partes y para los roles `<s>_tester`, los privilegios por
defecto (`ALTER DEFAULT PRIVILEGES`) ya cubren las tablas nuevas creadas por
`<s>_owner`.
