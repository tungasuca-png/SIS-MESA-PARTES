// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package handler

import (
	"net/http"

	"gateway/internal/svc"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// MensajeRegistroDeshabilitado: este contrato antiguo (usuario, clave y rol)
// se retiró; los solicitantes se registran con Google o con su correo
// verificado (POST /api/auth/registro), o reciben sus credenciales de la
// institución.
const MensajeRegistroDeshabilitado = "Este método de registro fue deshabilitado. Usa «Continuar con Google» o «Crear cuenta con correo» en la pantalla de inicio de sesión."

// RegisterHandler: POST /api/auth/register responde 410 Gone sin llamar a
// ningún servicio. No elimina usuarios ni datos existentes.
func RegisterHandler(_ *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJsonCtx(r.Context(), w, http.StatusGone, map[string]string{"message": MensajeRegistroDeshabilitado})
	}
}
