package handler

import (
	"net/http"
	"net/url"
	"strings"

	"gateway/internal/logic"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// GoogleIniciarHandler: GET /api/auth/google/iniciar[?modo=vincular].
// Guarda state, nonce y verificador PKCE en una cookie HttpOnly y redirige a
// Google. En modo vincular exige la cookie de vinculación (creada por
// POST /api/auth/google/vincular tras reautenticar con la clave).
func GoogleIniciarHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GoogleIniciarRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		frontend := svcCtx.Config.FrontendURL
		vincular := strings.EqualFold(strings.TrimSpace(req.Modo), "vincular")
		vinculo := ""
		if vincular {
			c, err := r.Cookie(logic.CookieVinculoGoogle)
			if err != nil || c.Value == "" {
				redirigir(w, r, logic.DestinoPerfilGoogle(frontend, url.Values{"google_error": {"codigo_invalido"}}))
				return
			}
			vinculo = c.Value
			http.SetCookie(w, borrarCookieGoogle(svcCtx, logic.CookieVinculoGoogle, http.SameSiteStrictMode))
		}
		destino, flujo, err := logic.NewGoogleIniciarLogic(r.Context(), svcCtx).GoogleIniciar(&req, vinculo)
		if err != nil {
			clave := logic.ClaveErrorGoogle(err)
			if vincular {
				redirigir(w, r, logic.DestinoPerfilGoogle(frontend, url.Values{"google_error": {clave}}))
			} else {
				redirigir(w, r, logic.DestinoLoginGoogle(frontend, url.Values{"error": {clave}}))
			}
			return
		}
		http.SetCookie(w, cookieGoogle(svcCtx, logic.CookieFlujoGoogle, flujo, logic.DuracionFlujoGoogle, http.SameSiteLaxMode))
		redirigir(w, r, destino)
	}
}
