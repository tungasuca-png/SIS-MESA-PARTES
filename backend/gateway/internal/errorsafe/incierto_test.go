package errorsafe

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// A + B: el resultado incierto de una operación con Idempotency-Key se informa
// con su mensaje seguro (el usuario debe saber que puede reintentar sin duplicar).
func TestHandler_ResultadoIncierto_ConservaMensajeSeguro(t *testing.T) {
	for _, c := range []codes.Code{codes.DeadlineExceeded, codes.Unavailable} {
		code, body := Handler(context.Background(), status.Error(c, MensajeResultadoIncierto))
		if code != httpStatus(c) {
			t.Fatalf("%v: HTTP %d", c, code)
		}
		if st, _ := status.FromError(body.(error)); st.Message() != MensajeResultadoIncierto {
			t.Fatalf("%v: mensaje %q", c, st.Message())
		}
	}
}

// Cualquier otro 5xx sigue sin detalles internos.
func TestHandler_OtroError5xx_SigueEnmascarado(t *testing.T) {
	code, body := Handler(context.Background(), status.Error(codes.DeadlineExceeded, "context deadline exceeded en 127.0.0.1:8082"))
	if code != http.StatusGatewayTimeout {
		t.Fatalf("HTTP %d", code)
	}
	if st, _ := status.FromError(body.(error)); st.Message() != MensajeNoDisponible {
		t.Fatalf("debe enmascararse: %q", st.Message())
	}
}

// La cabecera Idempotency-Key obligatoria ausente produce un 400 legible.
func TestHandler_CabeceraObligatoriaConGuion(t *testing.T) {
	code, body := Handler(context.Background(), errors.New(`field "Idempotency-Key" is not set`))
	if code != http.StatusBadRequest || body.(error).Error() != `field "Idempotency-Key" is not set` {
		t.Fatalf("HTTP %d %v", code, body)
	}
}
