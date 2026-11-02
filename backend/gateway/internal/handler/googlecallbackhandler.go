package handler

import (
	"net/http"

	"gateway/internal/logic"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// GoogleCallbackHandler: GET /api/auth/google/callback (redirect URI
// registrada en Google). La cookie del flujo se elimina SIEMPRE (un solo uso)
// y la respuesta es una redirección al frontend.
func GoogleCallbackHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GoogleCallbackRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		valor := ""
		if c, err := r.Cookie(logic.CookieFlujoGoogle); err == nil {
			valor = c.Value
		}
		http.SetCookie(w, borrarCookieGoogle(svcCtx, logic.CookieFlujoGoogle, http.SameSiteLaxMode))
		redirigir(w, r, logic.NewGoogleCallbackLogic(r.Context(), svcCtx).GoogleCallback(&req, valor))
	}
}
