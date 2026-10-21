// Package errorsafe define cómo el Gateway responde los errores al navegador.
//
// Por defecto go-zero devuelve el texto completo del error gRPC. En los 4xx
// eso es útil y seguro: los servicios escriben ahí un código estable
// ("student_not_found: ...") que el frontend traduce, sin valores. En los
// 5xx, en cambio, el texto puede traer detalles internos (por ejemplo
// "transport: Error while dialing: dial tcp 127.0.0.1:8086 ..."), que no
// deben llegar a un navegador, menos aún desde la red local. Aquí:
//   - 4xx: exactamente la misma respuesta que go-zero (contrato intacto);
//   - 5xx: mismo código HTTP y mismo formato "rpc error: code = X desc = ...",
//     con un mensaje genérico; el error original queda solo en el log.
package errorsafe

import (
	"context"
	"net/http"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const MensajeNoDisponible = "servicio no disponible temporalmente"

// httpStatus replica el mapeo de go-zero (rest/internal/errcode.CodeFromGrpcError).
func httpStatus(code codes.Code) int {
	switch code {
	case codes.OK:
		return http.StatusOK
	case codes.InvalidArgument, codes.FailedPrecondition, codes.OutOfRange:
		return http.StatusBadRequest
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.NotFound:
		return http.StatusNotFound
	case codes.Canceled:
		return http.StatusRequestTimeout
	case codes.AlreadyExists, codes.Aborted:
		return http.StatusConflict
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	}
	return http.StatusInternalServerError
}

// Handler se registra con httpx.SetErrorHandlerCtx.
func Handler(ctx context.Context, err error) (int, any) {
	st, ok := status.FromError(err)
	if !ok {
		// Errores de parseo de la solicitud (httpx.Parse): mismo 400 que go-zero.
		return http.StatusBadRequest, err
	}
	code := httpStatus(st.Code())
	if code < http.StatusInternalServerError {
		return code, err
	}
	logx.WithContext(ctx).Errorf("error %d hacia un servicio: %v", code, err)
	return code, status.Error(st.Code(), MensajeNoDisponible)
}
