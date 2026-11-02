// Package limite aplica límites de tasa (token bucket, en memoria) a las
// rutas expuestas a sistemas consumidores y a las búsquedas de estudiantes:
//   - POST /api/auth/token: por dirección IP (fuerza bruta de credenciales);
//   - búsquedas/resolución de estudiantes: por credencial (hash SHA-256 del
//     encabezado Authorization; un token de servicio identifica a un cliente)
//     y, para la API de sistemas, además por IP (rotar tokens no evade el
//     límite).
//
// Al excederse responde 429 con el mismo formato de error que el resto del
// Gateway ("rpc error: code = ResourceExhausted desc = ...") y Retry-After.
// Nunca registra el token, el cuerpo ni la consulta: solo la ruta.
//
// Es un límite por instancia: con varias réplicas del Gateway, cada una
// cuenta por separado (documentado en docs/infra/publicacion-gateway.md).
package limite

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
	"golang.org/x/time/rate"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MensajeLimite: texto del 429 (sin detalles de la regla ni del cliente).
const MensajeLimite = "rate_limited: demasiadas solicitudes; intente nuevamente en unos segundos"

const (
	inactividadMaxima = 10 * time.Minute // cubetas sin uso se descartan
	intervaloLimpieza = time.Minute
	maxCubetas        = 50000 // tope de memoria; al llenarse, las claves nuevas reciben 429
)

// Regla: tasa sostenida por minuto y ráfaga permitida para una clave.
type Regla struct {
	PorMinuto int
	Rafaga    int
}

// Ruta: reglas de una ruta (método + path exactos). Una regla con PorMinuto 0
// no se aplica.
type Ruta struct {
	PorIP         Regla
	PorCredencial Regla
}

// RutasPorDefecto: límites holgados para el uso legítimo (un sistema renueva
// su token cada ~4 min; un usuario busca con debounce de 300 ms) y estrictos
// para el abuso.
func RutasPorDefecto() map[string]Ruta {
	return map[string]Ruta{
		"POST /api/auth/token": {PorIP: Regla{PorMinuto: 10, Rafaga: 5}},
		// Acceso con Google: por IP (automatización de altas y canjes) y, en
		// la vinculación (clave actual), también por credencial; Auth además
		// cuenta los intentos fallidos de clave y bloquea la cuenta.
		"GET /api/auth/google/iniciar":   {PorIP: Regla{PorMinuto: 20, Rafaga: 10}},
		"GET /api/auth/google/callback":  {PorIP: Regla{PorMinuto: 20, Rafaga: 10}},
		"POST /api/auth/google/canjear":  {PorIP: Regla{PorMinuto: 20, Rafaga: 10}},
		"POST /api/auth/google/registro": {PorIP: Regla{PorMinuto: 10, Rafaga: 5}},
		"POST /api/auth/google/vincular": {
			PorIP:         Regla{PorMinuto: 10, Rafaga: 5},
			PorCredencial: Regla{PorMinuto: 5, Rafaga: 3},
		},
		"GET /api/auth/google/estado": {PorCredencial: Regla{PorMinuto: 30, Rafaga: 10}},
		"POST /api/estudiantes/seleccion/buscar": {
			PorIP:         Regla{PorMinuto: 120, Rafaga: 30},
			PorCredencial: Regla{PorMinuto: 60, Rafaga: 20},
		},
		"POST /api/estudiantes/resolver":          {PorCredencial: Regla{PorMinuto: 60, Rafaga: 20}},
		"POST /api/estudiantes/basic-batch":       {PorCredencial: Regla{PorMinuto: 120, Rafaga: 40}},
		"POST /api/psicologia/estudiantes/buscar": {PorCredencial: Regla{PorMinuto: 60, Rafaga: 20}},
		// Reportes: el Dashboard consulta el reporte cada 10 s (2 llamadas);
		// la exportación genera archivos y es más costosa.
		"GET /api/reportes/expedientes":          {PorCredencial: Regla{PorMinuto: 60, Rafaga: 20}},
		"GET /api/reportes/expedientes/exportar": {PorCredencial: Regla{PorMinuto: 10, Rafaga: 5}},
		// Fase 2: consultas por pestaña y exportaciones (la actividad y la
		// comparación consultan varios servicios).
		"GET /api/reportes/derivaciones":          {PorCredencial: Regla{PorMinuto: 60, Rafaga: 20}},
		"GET /api/reportes/documentos":            {PorCredencial: Regla{PorMinuto: 60, Rafaga: 20}},
		"GET /api/reportes/actividad":             {PorCredencial: Regla{PorMinuto: 30, Rafaga: 10}},
		"GET /api/reportes/comparacion":           {PorCredencial: Regla{PorMinuto: 30, Rafaga: 10}},
		"GET /api/reportes/derivaciones/exportar": {PorCredencial: Regla{PorMinuto: 10, Rafaga: 5}},
		"GET /api/reportes/documentos/exportar":   {PorCredencial: Regla{PorMinuto: 10, Rafaga: 5}},
		"GET /api/reportes/actividad/exportar":    {PorCredencial: Regla{PorMinuto: 10, Rafaga: 5}},
		"GET /api/reportes/comparacion/exportar":  {PorCredencial: Regla{PorMinuto: 10, Rafaga: 5}},
	}
}

type cubeta struct {
	lim   *rate.Limiter
	visto time.Time
}

// Limitador es seguro para uso concurrente.
type Limitador struct {
	rutas map[string]Ruta
	// confiarProxyLocal: si la conexión llega desde loopback (el proxy inverso
	// en el mismo equipo), la IP del cliente es la última de X-Forwarded-For.
	confiarProxyLocal bool
	ahora             func() time.Time

	mu       sync.Mutex
	cubetas  map[string]*cubeta
	limpieza time.Time
}

func Nuevo(rutas map[string]Ruta, confiarProxyLocal bool) *Limitador {
	return &Limitador{rutas: rutas, confiarProxyLocal: confiarProxyLocal, ahora: time.Now, cubetas: map[string]*cubeta{}}
}

// Middleware de go-zero (rest.Middleware).
func (l *Limitador) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ruta := r.Method + " " + r.URL.Path
		reglas, ok := l.rutas[ruta]
		if !ok {
			next(w, r)
			return
		}
		permitido := true
		if reglas.PorIP.PorMinuto > 0 {
			permitido = l.permitir(ruta+"|ip:"+l.ipCliente(r), reglas.PorIP)
		}
		if permitido && reglas.PorCredencial.PorMinuto > 0 {
			permitido = l.permitir(ruta+"|"+l.credencial(r), reglas.PorCredencial)
		}
		if !permitido {
			logx.WithContext(r.Context()).Infow("límite de solicitudes excedido", logx.Field("ruta", ruta))
			w.Header().Set("Retry-After", strconv.Itoa(10))
			httpx.ErrorCtx(r.Context(), w, status.Error(codes.ResourceExhausted, MensajeLimite))
			return
		}
		next(w, r)
	}
}

func (l *Limitador) permitir(clave string, regla Regla) bool {
	ahora := l.ahora()
	l.mu.Lock()
	defer l.mu.Unlock()
	if ahora.Sub(l.limpieza) >= intervaloLimpieza {
		for k, c := range l.cubetas {
			if ahora.Sub(c.visto) > inactividadMaxima {
				delete(l.cubetas, k)
			}
		}
		l.limpieza = ahora
	}
	c, ok := l.cubetas[clave]
	if !ok {
		if len(l.cubetas) >= maxCubetas {
			return false
		}
		c = &cubeta{lim: rate.NewLimiter(rate.Limit(float64(regla.PorMinuto)/60), regla.Rafaga)}
		l.cubetas[clave] = c
	}
	c.visto = ahora
	return c.lim.AllowN(ahora, 1)
}

// credencial: hash del encabezado Authorization (nunca el token en claro);
// sin encabezado, la IP.
func (l *Limitador) credencial(r *http.Request) string {
	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if authz == "" {
		return "anon:" + l.ipCliente(r)
	}
	h := sha256.Sum256([]byte(authz))
	return "cred:" + hex.EncodeToString(h[:16])
}

func (l *Limitador) ipCliente(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if l.confiarProxyLocal && ip != nil && ip.IsLoopback() {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			partes := strings.Split(xff, ",")
			if ultima := net.ParseIP(strings.TrimSpace(partes[len(partes)-1])); ultima != nil {
				return ultima.String()
			}
		}
	}
	if ip != nil {
		return ip.String()
	}
	return host
}
