// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package handler

import (
	"net/http"

	"gateway/internal/svc"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// MensajeRegistroDeshabilitado: el autorregistro público con usuario y clave
// se retiró; los solicitantes se registran con Google (y completan su DNI) o
// reciben sus credenciales de la institución.
const MensajeRegistroDeshabilitado = "El registro público fue deshabilitado. Usa «Continuar con Google» o solicita tus credenciales a la institución educativa."

// RegisterHandler: POST /api/auth/register responde 410 Gone sin llamar a
// ningún servicio. No elimina usuarios ni datos existentes.
func RegisterHandler(_ *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJsonCtx(r.Context(), w, http.StatusGone, map[string]string{"message": MensajeRegistroDeshabilitado})
	}
}
