// Package logsafe agrupa la protección de logs del Gateway: nada sensible
// (contraseñas, JWT, refresh tokens, cabeceras, cuerpos) debe llegar a un log.
package logsafe

import (
	"net/http"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

// statusRecorder observa el código HTTP que escribe el handler sin alterar
// nada de la respuesta: WriteHeader/Write/Flush se delegan tal cual en el
// ResponseWriter original (y Unwrap conserva http.ResponseController).
type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wrote {
		r.status = code
		r.wrote = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wrote {
		r.status = http.StatusOK
		r.wrote = true
	}
	return r.ResponseWriter.Write(b)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// AccessLog reemplaza al LogHandler estándar de go-zero (desactivado con
// `Middlewares: Log: false` en gateway-api.yaml). El estándar, ante un 5xx,
// vuelca el request completo (cabeceras, incluido Authorization, y los
// primeros 1 KB del cuerpo: contraseñas, refresh tokens…).
//
// Este middleware registra ÚNICAMENTE: método, ruta (sin query string), código
// HTTP y duración, p. ej. "[HTTP] POST /api/auth/login 401 23ms". Nunca lee ni
// registra cabeceras, cookies ni cuerpo del request o de la respuesta.
func AccessLog(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		defer func() {
			if p := recover(); p != nil {
				// Un panic terminará en el Recover de go-zero como 500: se registra
				// igual (sin el valor del panic) y se relanza sin alterarlo.
				logRequest(r, http.StatusInternalServerError, time.Since(start))
				panic(p)
			}
			logRequest(r, rec.status, time.Since(start))
		}()

		next(rec, r)
	}
}

func logRequest(r *http.Request, status int, elapsed time.Duration) {
	logger := logx.WithContext(r.Context()).WithDuration(elapsed)
	msg := "[HTTP] " + sanitize(r.Method) + " " + sanitize(r.URL.Path) + " " + statusText(status) + " " + elapsed.Round(time.Millisecond).String()
	if status >= http.StatusInternalServerError {
		logger.Error(msg)
		return
	}
	logger.Info(msg)
}

func statusText(code int) string {
	// strconv evitado a propósito: solo dígitos, sin dependencias.
	d := [3]byte{byte('0' + code/100%10), byte('0' + code/10%10), byte('0' + code%10)}
	return string(d[:])
}

// sanitize impide inyección de líneas en el log: la ruta es dato del cliente.
func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return '?'
		}
		return r
	}, s)
}
