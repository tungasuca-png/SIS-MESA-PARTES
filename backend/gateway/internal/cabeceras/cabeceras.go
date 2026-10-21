// Package cabeceras agrega cabeceras HTTP de seguridad a todas las
// respuestas del Gateway. Es una API JSON que devuelve datos personales
// (expedientes, estudiantes, contenido clínico), así que:
//   - nosniff: el navegador no reinterpreta una respuesta como HTML/JS;
//   - no-store: ni el navegador ni un proxy intermedio guardan copias;
//   - DENY / frame-ancestors 'none': ninguna respuesta se muestra en un iframe;
//   - no-referrer: las URLs de la API (con IDs) no se filtran como Referer.
package cabeceras

import "net/http"

// Seguridad es un middleware de go-zero (rest.Middleware).
func Seguridad(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Cache-Control", "no-store")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", "frame-ancestors 'none'")
		h.Set("Referrer-Policy", "no-referrer")
		next(w, r)
	}
}
