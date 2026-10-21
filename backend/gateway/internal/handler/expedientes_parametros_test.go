package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gateway/internal/errorsafe"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// Seguridad: GET /api/expedientes con parámetros mal formados responde 400
// sin exponer el error interno de Go (strconv) y sin llegar al servicio
// (svcCtx nil: si el handler llamara a la lógica, la prueba fallaría).
func TestListExpedientes_ParametrosMalFormados400Generico(t *testing.T) {
	httpx.SetErrorHandlerCtx(errorsafe.Handler)
	t.Cleanup(func() { httpx.SetErrorHandlerCtx(nil) })

	for _, q := range []string{"page=abc", "page=1.5", "page_size=99999999999", "page=-99999999999", "page_size=1e9", "page=%27%20OR%201=1--"} {
		w := httptest.NewRecorder()
		ListExpedientesHandler(nil)(w, httptest.NewRequest(http.MethodGet, "/api/expedientes?"+q, nil))
		body := w.Body.String()
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: código %d, se esperaba 400", q, w.Code)
		}
		if strings.Contains(body, "strconv") || strings.Contains(body, "parsing") || !strings.Contains(body, errorsafe.MensajeSolicitudInvalida) {
			t.Errorf("%s: el cuerpo expone detalles internos: %q", q, body)
		}
	}
}
