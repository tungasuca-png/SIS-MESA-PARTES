package handler

import (
	"net/http"

	"gateway/internal/logic"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func ConfirmarDatosEstudiantesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req := types.DatosArchivoRequest{Authorization: r.Header.Get("Authorization")}
		solicitud, err := leerArchivoDatos(r)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := logic.NewConfirmarDatosEstudiantesLogic(r.Context(), svcCtx)
		resp, err := l.ConfirmarDatosEstudiantes(&req, solicitud)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
