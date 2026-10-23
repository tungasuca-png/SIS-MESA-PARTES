package handler

import (
	"net/http"

	"gateway/internal/logic"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func AnalizarDatosEstudiantesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req := types.DatosArchivoRequest{Authorization: r.Header.Get("Authorization")}
		solicitud, err := leerArchivoDatos(r)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := logic.NewAnalizarDatosEstudiantesLogic(r.Context(), svcCtx)
		resp, err := l.AnalizarDatosEstudiantes(&req, solicitud)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
