# Publicación segura del Gateway (referencia, NO aplicada)

Este documento es una referencia. **No se modificó ninguna configuración de
red**: el Gateway sigue escuchando en `127.0.0.1:8888`.

## Estado actual (verificado el 30/10/2026)

| Componente | Dirección | Expuesto fuera del equipo |
|---|---|---|
| Gateway | `127.0.0.1:8888` | No |
| Servicios gRPC (Auth, Usuarios, Expedientes, Documentos, Derivaciones, Psicología, Estudiantes) | `127.0.0.1:808x` | No |
| PostgreSQL (`tg-postgres`) | `127.0.0.1:5433` | No |
| etcd | `127.0.0.1:2379` | No |
| Adminer | `127.0.0.1:8081` | No (no debe existir en producción) |

- El firewall de Windows está activo en todos los perfiles.
- No hay proxy inverso, dominio ni certificado TLS en el repositorio.
- CORS solo admite `http://localhost:5173`.

## Faltantes para publicar (bloqueos)

1. **Dominio** (por ejemplo `api.<dominio-del-colegio>`) con DNS que apunte al
   equipo o servidor de publicación.
2. **Certificado TLS** válido para ese dominio (Let's Encrypt o el que provea
   la institución), con su clave privada fuera del repositorio.
3. **Host del proxy inverso** (nginx, Caddy o IIS + ARR) y quién lo administra.
4. **Origen del frontend en producción**, para ajustar CORS (hoy solo localhost).
5. Decidir quién consumirá la API (red local o Internet) para definir las reglas
   del firewall.

Sin estos datos no se debe publicar nada. **No** cambiar el Gateway a `0.0.0.0`.

## Arquitectura objetivo

```text
Cliente ─► 443/HTTPS ─► proxy inverso (TLS) ─► 127.0.0.1:8888 Gateway ─► servicios en loopback
```

- Solo el proxy publica un puerto: **443**. El Gateway y los servicios siguen
  en loopback.
- El proxy y el Gateway corren en el mismo equipo. Así el Gateway confía en
  `X-Forwarded-For` solo si la conexión viene de loopback
  (`ConfiarProxyLocal: true`); de lo contrario, todos los clientes compartirían
  la IP del proxy para los límites de tasa.

### Ejemplo de nginx (plantilla; reemplazar los marcadores)

```nginx
server {
    listen 443 ssl http2;
    server_name <DOMINIO>;

    ssl_certificate     <RUTA_CERTIFICADO>;      # fuera del repositorio
    ssl_certificate_key <RUTA_CLAVE_PRIVADA>;    # fuera del repositorio
    ssl_protocols TLSv1.2 TLSv1.3;
    add_header Strict-Transport-Security "max-age=31536000" always;

    client_max_body_size 5m;                     # el Gateway limita cada ruta

    # No registrar cuerpos ni el encabezado Authorization.
    access_log /var/log/nginx/tungasuca_access.log;  # formato por defecto: sin cuerpo

    location /api/ {
        proxy_pass http://127.0.0.1:8888;
        proxy_set_header Host              $host;
        proxy_set_header X-Forwarded-For   $remote_addr;   # reemplaza, no agrega
        proxy_set_header X-Forwarded-Proto https;
        proxy_read_timeout 200s;                 # importaciones largas (190 s)
    }
}

server {
    listen 80;
    server_name <DOMINIO>;
    return 301 https://$host$request_uri;        # sin servir nada por HTTP
}
```

### Firewall (Windows, ejemplo)

```powershell
# Solo 443 hacia el proxy; nada hacia 8888, 808x, 5433, 2379 ni 8081.
New-NetFirewallRule -DisplayName "Tungasuca HTTPS" -Direction Inbound -Protocol TCP -LocalPort 443 -Action Allow -Profile Domain,Private
```

### Configuración del Gateway al publicar

- `ConfiarProxyLocal: true` en `etc/gateway-api.yaml`, solo con el proxy en el
  mismo equipo.
- Ajustar CORS (`gateway.go`, `rest.WithCors`) al origen real del frontend.
- Mantener `Host: 127.0.0.1`.

## Riesgos pendientes

- **Límites por instancia:** los límites de tasa se guardan en memoria. Si hay
  varias réplicas del Gateway o un reinicio, se reinician los contadores. El
  proxy puede añadir su propio límite (`limit_req`) como segunda capa.
- **Revocación inmediata:** un token de servicio emitido sigue siendo válido
  hasta que vence (5 min), aunque se revoque el cliente.
- **Login humano:** `/api/auth/login` no tiene límite de tasa en el Gateway; se
  recomienda agregarlo en el proxy o en una fase posterior.
- **Copias y logs del proxy:** no activar el registro de cuerpos (`$request_body`)
  ni de encabezados.
