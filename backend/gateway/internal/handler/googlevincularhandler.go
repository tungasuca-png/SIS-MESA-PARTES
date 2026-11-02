package handler

import (
	"net/http"

	"gateway/internal/logic"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// GoogleVincularHandler: POST /api/auth/google/vincular (Bearer + clave
// actual). El código de vinculación queda en una cookie HttpOnly
// SameSite=Strict, nunca en la URL ni en el cuerpo de la respuesta.
func GoogleVincularHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GoogleVincularRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		resp, codigo, err := logic.NewGoogleVincularLogic(r.Context(), svcCtx).GoogleVincular(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		http.SetCookie(w, cookieGoogle(svcCtx, logic.CookieVinculoGoogle, codigo, int(resp.ExpiraEn), http.SameSiteStrictMode))
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
