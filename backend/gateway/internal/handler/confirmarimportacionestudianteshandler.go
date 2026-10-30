package handler

import (
	"net/http"

	"gateway/internal/logic"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func ConfirmarImportacionEstudiantesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req := types.ExcelEstudiantesRequest{Authorization: r.Header.Get("Authorization")}
		archivo, err := leerArchivoExcel(r)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := logic.NewConfirmarImportacionEstudiantesLogic(r.Context(), svcCtx)
		resp, err := l.ConfirmarImportacionEstudiantes(&req, archivo)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
