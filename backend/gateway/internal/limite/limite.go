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
// Rutas con segmentos dinámicos ("/api/x/:id/accion"): la regla se declara
// con el patrón y el contador es POR PATRÓN, no por URL concreta; cambiar el
// id no crea una cubeta nueva (no evade el límite).
//
// Es un límite por instancia y en memoria (no hay un limitador distribuido en
// el proyecto): con varias réplicas del Gateway, cada una cuenta por separado
// (documentado en docs/infra/publicacion-gateway.md). Si se alcanza el tope
// de cubetas, las claves nuevas reciben 429 (falla cerrada, no abierta).
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

// Ruta: reglas de una ruta (método + path exacto, o patrón con segmentos
// ":nombre"). Una regla con PorMinuto 0 no se aplica.
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
		// Inicio de sesión y renovación de la sesión (A-04): por IP, contra
		// la prueba de contraseñas sobre muchas cuentas a la vez (Auth ya
		// bloquea CADA cuenta tras varios fallos, pero no limita por origen).
		// Valores holgados: el personal del colegio puede salir a Internet
		// con una misma IP pública (NAT) y la renovación es automática (una
		// por sesión cada ~15 min). Detrás de IIS exige ConfiarProxyLocal
		// (si no, todos los clientes comparten la IP del proxy).
		"POST /api/auth/login":   {PorIP: Regla{PorMinuto: 30, Rafaga: 10}},
		"POST /api/auth/refresh": {PorIP: Regla{PorMinuto: 60, Rafaga: 20}},
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
		// Registro manual con correo: por IP (altas automatizadas, envío
		// masivo de correos y prueba de códigos). Auth además limita los
		// registros por correo, la espera y los reenvíos, y los intentos por
		// código.
		"POST /api/auth/registro":           {PorIP: Regla{PorMinuto: 5, Rafaga: 3}},
		"POST /api/auth/registro/verificar": {PorIP: Regla{PorMinuto: 15, Rafaga: 5}},
		"POST /api/auth/registro/reenviar":  {PorIP: Regla{PorMinuto: 5, Rafaga: 3}},
		// Bandeja de verificación de identidad (personal autorizado). El
		// limitador compara rutas literales: verificar/rechazar (con :id) los
		// protege el permiso, comprobado en Auth y en Usuarios.
		// Valores tomados de los niveles existentes: consultar como los
		// reportes (60/20 por credencial); verificar, rechazar y reconciliar
		// como las exportaciones (10/5 por credencial: operaciones costosas y
		// manuales; un cotejo de identidad lleva bastante más de 6 s) y, para
		// verificar/rechazar, además por IP como el inicio de Google (20/10),
		// así que rotar tokens tampoco evade el límite. Contador por patrón:
		// cambiar el :id no lo reinicia.
		"GET /api/solicitantes/identidades":                {PorCredencial: Regla{PorMinuto: 60, Rafaga: 20}},
		"POST /api/solicitantes/identidades/:id/verificar": {PorCredencial: Regla{PorMinuto: 10, Rafaga: 5}, PorIP: Regla{PorMinuto: 20, Rafaga: 10}},
		"POST /api/solicitantes/identidades/:id/rechazar":  {PorCredencial: Regla{PorMinuto: 10, Rafaga: 5}, PorIP: Regla{PorMinuto: 20, Rafaga: 10}},
		"POST /api/solicitantes/identidades/reconciliar":   {PorCredencial: Regla{PorMinuto: 10, Rafaga: 5}},
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

// patron: regla de una ruta con segmentos dinámicos.
type patron struct {
	clave     string // "MÉTODO /ruta/:id/..." (también es la clave del contador)
	metodo    string
	segmentos []string
	ruta      Ruta
}

func (p patron) coincide(metodo string, segmentos []string) bool {
	if metodo != p.metodo || len(segmentos) != len(p.segmentos) {
		return false
	}
	for i, s := range p.segmentos {
		if strings.HasPrefix(s, ":") {
			if segmentos[i] == "" {
				return false
			}
			continue
		}
		if s != segmentos[i] {
			return false
		}
	}
	return true
}

// Limitador es seguro para uso concurrente.
type Limitador struct {
	rutas    map[string]Ruta
	patrones []patron
	// confiarProxyLocal: si la conexión llega desde loopback (el proxy inverso
	// en el mismo equipo), la IP del cliente es la última de X-Forwarded-For.
	confiarProxyLocal bool
	ahora             func() time.Time

	mu       sync.Mutex
	cubetas  map[string]*cubeta
	limpieza time.Time
}

func Nuevo(rutas map[string]Ruta, confiarProxyLocal bool) *Limitador {
	exactas := map[string]Ruta{}
	var patrones []patron
	for clave, ruta := range rutas {
		metodo, path, _ := strings.Cut(clave, " ")
		if !strings.Contains(path, "/:") {
			exactas[clave] = ruta
			continue
		}
		patrones = append(patrones, patron{clave: clave, metodo: metodo, segmentos: strings.Split(strings.Trim(path, "/"), "/"), ruta: ruta})
	}
	return &Limitador{rutas: exactas, patrones: patrones, confiarProxyLocal: confiarProxyLocal, ahora: time.Now, cubetas: map[string]*cubeta{}}
}

// reglas: ruta exacta o, si no, el primer patrón que coincide. Devuelve la
// clave del contador (el patrón, nunca la URL concreta).
func (l *Limitador) reglas(r *http.Request) (string, Ruta, bool) {
	ruta := r.Method + " " + r.URL.Path
	if reglas, ok := l.rutas[ruta]; ok {
		return ruta, reglas, true
	}
	if len(l.patrones) > 0 {
		segmentos := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		for _, p := range l.patrones {
			if p.coincide(r.Method, segmentos) {
				return p.clave, p.ruta, true
			}
		}
	}
	return "", Ruta{}, false
}

// Middleware de go-zero (rest.Middleware).
func (l *Limitador) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ruta, reglas, ok := l.reglas(r)
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
