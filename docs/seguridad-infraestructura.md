# Seguridad de infraestructura (FASE 6N-3)

Correcciones C1 (contenido clínico cifrado), C2 (mínimo privilegio y secretos) y
C3 (exposición de red). Este documento describe lo implementado en DESARROLLO y los
requisitos para PRODUCCIÓN, que todavía no existe: no se inventa infraestructura.

## 1. Desarrollo (implementado)

```text
Navegador ── http://localhost:5173 (Vite, ::1)
   └─► Gateway 127.0.0.1:8888 (HTTP)
         └─► servicios gRPC 127.0.0.1:8080-8087
               └─► PostgreSQL 127.0.0.1:5433 (tg-postgres, volumen "pgdata")
etcd 127.0.0.1:2379 · Adminer 127.0.0.1:8081 (solo bajo demanda)
```

- Todo escucha **solo en loopback**: nada es alcanzable desde la LAN. Verificado con
  `Test-NetConnection` contra la IP de la LAN (5433, 8081, 2379, 2380, 8080-8087 y
  8888 cerrados).
- Infraestructura: `backend/docker-compose.dev.yml` (sin secretos). Adminer solo con
  `--profile tools` y se detiene al terminar.
- Superusuario de administración (`tg_admin`): usuario y contraseña en archivos de
  `TG_SECRETS_DIR` (fuera del repositorio), montados como secretos de Docker.
- Roles: `backend/db/roles/` (owner / migrator / tester / app por servicio).
- TLS hacia PostgreSQL: no aplica en desarrollo (misma máquina, loopback).
- Pendiente con permisos de administrador de Windows: limitar al perfil *Private*
  la regla de entrada "Docker Desktop Backend" (hoy también *Public*):

  ```powershell
  Get-NetFirewallRule -DisplayName 'Docker Desktop Backend' | Set-NetFirewallRule -Profile Private
  ```

  La regla "PostgreSQL Server" pertenece a una instalación nativa de PostgreSQL 18
  que no es de este proyecto (escucha solo en 127.0.0.1); no se modificó.

## 2. Producción (requisitos, no implementado)

```text
Internet ─► HTTPS (TLS 1.2+) ─► proxy inverso ─► Gateway (red interna)
                                                   └─► servicios internos (red interna)
                                                         └─► PostgreSQL (red interna, TLS)
```

- Solo el proxy inverso publica un puerto (443). **Nunca** se publican PostgreSQL,
  etcd, Adminer, Auth, Expedientes, Usuarios, Documentos, Derivaciones, Psicología ni
  Estudiantes.
- Adminer **no se despliega**. La administración se hace por túnel SSH con `psql`.
- HTTPS obligatorio en el Gateway público (el contenido clínico viaja por ese tramo).
- PostgreSQL: TLS obligatorio si atraviesa una red no confiable u otro host
  (`hostssl` en `pg_hba.conf`, `hostnossl ... reject`, servicios con
  `SSLMode: verify-full` y la CA indicada por variable de entorno). Certificados y
  claves privadas **fuera del repositorio**, montados como secretos de solo lectura.
- `<servicio>_tester` no existe en producción.
- etcd con autenticación y solo en la red interna.

## 3. Secretos

| Secreto | Dónde vive | Nunca en |
|---|---|---|
| `JWT_SECRET` | `backend/.env` (prod: secreto del orquestador) | código, YAML, pruebas, Git |
| `<S>_DB_PASSWORD` (roles de aplicación) | `backend/.env` | YAML, código, Git |
| `TEST_<S>_DB_URL` (roles de prueba) | `backend/.env.test` | código, Git |
| Migradores | `TG_SECRETS_DIR/migrators.env` | `.env` compartido, Git |
| `tg_admin` | `TG_SECRETS_DIR/pg_admin_*` | YAML, `.env`, código, pruebas, Compose |
| `CLINICAL_DATA_ENCRYPTION_KEY` | `backend/.env` + copia bajo custodia | Git, código, Gateway, frontend, PostgreSQL, logs |

Los secretos anteriores a la FASE 6N-3 (JWT y contraseñas de PostgreSQL) estuvieron
publicados en repositorios públicos: **todos fueron rotados** y los anteriores quedan
inválidos. En la FASE 6N-4 se preparó una historia de Git reescrita sin esos valores
(`SECRETO-ELIMINADO` en su lugar); publicarla requiere force-push, que afecta a
cualquier clon existente (debe volver a clonarse). GitHub puede conservar en caché
los commits anteriores accesibles por su SHA: la rotación es la mitigación efectiva.

Cuentas de demostración (`*.demo`, solo desarrollo): la contraseña de `admin.demo`
se cambió en la FASE 6N-4 (nueva en `TG_SECRETS_DIR/admin-demo-password`). Las demás
cuentas `*.demo` y `solicitante2.e2e` conservan una contraseña que estuvo publicada:
no deben existir en producción.

## 4. Clave clínica: custodia y procedimientos

Estado actual (FASE 6N-4), sin valores:

| Aspecto | Situación |
|---|---|
| Ubicación en uso | `backend/.env` (`CLINICAL_DATA_ENCRYPTION_KEY`, `CLINICAL_DATA_KEY_ID`), fuera de Git (`.gitignore`). |
| Copia | `TG_SECRETS_DIR/clinical-key-custodia.env`, **en el mismo equipo y disco**. No es una custodia secundaria real. |
| Quién tiene acceso | La cuenta de Windows del desarrollador y cualquier proceso que corra con ella, y los administradores del equipo. |
| Dónde NO está | Git (árbol e historial), README, logs, Docker Compose, código, frontend, Gateway, PostgreSQL. |
| Riesgo | Si el disco se pierde o se daña, se pierden a la vez la clave y su copia: las atenciones cifradas serían **irrecuperables**, aunque existan backups de la base. |

**Pendiente (decisión institucional D2):** una custodia secundaria fuera de este
equipo, por ejemplo:

1. Gestor de secretos institucional (bóveda con control de acceso y auditoría), o
2. Sobre sellado con la clave impresa/en medio extraíble cifrado, guardado en caja
   fuerte, con custodia dual (dos responsables) y registro de accesos.

La copia de custodia **nunca** se guarda junto a los backups de la base.

**Recuperación** (clave perdida en el equipo, custodia disponible): recuperar la
clave de la custodia → escribirla en `backend/.env` con el mismo `CLINICAL_DATA_KEY_ID`
→ iniciar Psicología: el canario confirma que es la clave correcta; si no lo es, el
servicio no inicia y no se toca ningún dato. Sin custodia no hay recuperación.

- **Pérdida de la clave = pérdida irreversible** de las atenciones cifradas.
- Rotación (no ejecutada todavía): generar una clave nueva con otro `key_id`, mover
  la anterior a `CLINICAL_DATA_PREVIOUS_KEYS`, crear su canario
  (`cmd/cifrar-contenido --inicializar-canario`) y volver a cifrar los datos. La
  inmutabilidad estricta de la 004 bloquea la reescritura de las versiones
  registradas: la rotación necesitará su propia migración con una transición
  controlada.
- Backups: los tomados ANTES de la migración 004 contienen texto claro y se
  conservan cifrados (GPG AES-256) fuera del repositorio hasta que se apruebe su
  destrucción.
