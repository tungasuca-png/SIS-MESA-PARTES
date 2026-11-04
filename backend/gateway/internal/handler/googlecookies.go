package handler

import (
	"net/http"
	"strings"

	"gateway/internal/logic"
	"gateway/internal/svc"
)

// Cookies del flujo con Google: HttpOnly siempre, Path limitado a
// /api/auth/google, Secure si GOOGLE_COOKIE_SECURE=true (producción HTTPS).
func cookieGoogle(svcCtx *svc.ServiceContext, nombre, valor string, maxAge int, sameSite http.SameSite) *http.Cookie {
	return &http.Cookie{
		Name:     nombre,
		Value:    valor,
		Path:     logic.RutaCookiesGoogle,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   strings.EqualFold(strings.TrimSpace(svcCtx.Config.GoogleCookieSegura), "true"),
		SameSite: sameSite,
	}
}

// borrarCookieGoogle: los valores del flujo son de un solo uso.
func borrarCookieGoogle(svcCtx *svc.ServiceContext, nombre string, sameSite http.SameSite) *http.Cookie {
	return cookieGoogle(svcCtx, nombre, "", -1, sameSite)
}

// redirigir: 302 sin caché (las cabeceras de seguridad ya agregan no-store y
// Referrer-Policy: no-referrer).
func redirigir(w http.ResponseWriter, r *http.Request, destino string) {
	http.Redirect(w, r, destino, http.StatusFound)
}
