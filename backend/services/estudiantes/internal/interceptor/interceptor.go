package interceptor

import (
	"context"
	"strings"
	"time"

	"estudiantes/internal/security"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey struct{}

type userContext struct {
	UserID string
	Role   string
	// Token conserva el JWT crudo ya validado, para poder reenviarlo como
	// metadata saliente al consultar Auth.HasPermission (Paso 16B) — Auth
	// Service exige autenticación en ese RPC, igual que en Logout, y sin
	// esto no habría nada que reenviarle. Mismo patrón ya usado en
	// Expedientes/Documentos/Derivaciones.
	Token string
	// Service: identidad de un SISTEMA consumidor (client credentials). Sus
	// permisos son los del scope del token; nunca se consulta Auth ni se
	// reenvía su token (ver authorization.HasPermission).
	Service *ServiceIdentity
}

// ServiceIdentity: sistema consumidor autenticado con un token de servicio.
type ServiceIdentity struct {
	ID       string // sub: UUID del sistema en Auth
	ClientID string
	Permisos map[string]bool
	Method   string // RPC invocado (FullMethod)
}

// ServiceMethods: únicos RPC que un sistema consumidor puede invocar. El
// resto (familia, importación/exportación, plantillas, historial,
// duplicados, cambios de estado y de matrícula, datos personales en
// escritura) es exclusivo de usuarios. Cada uno sigue exigiendo además su
// permiso real dentro de su logic.
var ServiceMethods = map[string]bool{
	"/estudiantes.Estudiantes/GetEstudiante":            true,
	"/estudiantes.Estudiantes/SearchEstudiantes":        true,
	"/estudiantes.Estudiantes/GetEstudiantesBasic":      true,
	"/estudiantes.Estudiantes/ResolveEstudiante":        true,
	"/estudiantes.Estudiantes/GetEstudiantePersonal":    true,
	"/estudiantes.Estudiantes/CreateEstudiante":         true,
	"/estudiantes.Estudiantes/UpdateEstudiante":         true,
	"/estudiantes.Estudiantes/ListMatriculasEstudiante": true,
	"/estudiantes.Estudiantes/MatricularEstudiante":     true,
	"/estudiantes.Estudiantes/ListPeriodos":             true,
	"/estudiantes.Estudiantes/ListSecciones":            true,
}

// AuthenticationInterceptor exige un access token válido en la metadata
// "authorization" para todas las operaciones: Estudiantes Service no tiene
// métodos públicos. Acepta tokens de usuario (como siempre) y, si
// serviceValidator no es nil, tokens de sistemas consumidores.
func AuthenticationInterceptor(validator *security.JWTValidator, serviceValidator ...*security.ServiceTokenValidator) grpc.UnaryServerInterceptor {
	var services *security.ServiceTokenValidator
	if len(serviceValidator) > 0 {
		services = serviceValidator[0]
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		raw, err := bearerToken(ctx)
		if err != nil {
			return nil, err
		}
		if claims, err := validator.Validate(raw); err == nil {
			return handler(context.WithValue(ctx, contextKey{}, userContext{UserID: claims.Subject, Role: claims.Role, Token: raw}), req)
		}
		if services != nil {
			if claims, err := services.Validate(raw); err == nil {
				return handleService(ctx, req, info, handler, claims)
			}
		}
		return nil, status.Error(codes.Unauthenticated, "token de acceso inválido")
	}
}

// handleService aplica la lista de métodos permitidos y registra cada acceso
// del sistema (quién, qué operación, cuándo, resultado). Nunca registra el
// token, el contenido de la solicitud ni datos personales.
func handleService(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler, claims *security.ServiceClaims) (any, error) {
	inicio := time.Now()
	identity := &ServiceIdentity{ID: claims.Subject, ClientID: claims.ClientID, Permisos: claims.Permisos(), Method: info.FullMethod}

	var resp any
	var err error
	if ServiceMethods[info.FullMethod] {
		resp, err = handler(context.WithValue(ctx, contextKey{}, userContext{UserID: claims.Subject, Service: identity}), req)
	} else {
		err = status.Error(codes.PermissionDenied, "permission_denied: operación no disponible para sistemas consumidores")
	}

	logx.WithContext(ctx).Infow("acceso de sistema consumidor",
		logx.Field("sistema_id", identity.ID),
		logx.Field("client_id", identity.ClientID),
		logx.Field("operacion", info.FullMethod),
		logx.Field("resultado", status.Code(err).String()),
		logx.Field("fecha_hora", inicio.UTC().Format(time.RFC3339)),
		logx.Field("duracion_ms", time.Since(inicio).Milliseconds()),
	)
	return resp, err
}

func bearerToken(ctx context.Context) (string, error) {
	values, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "falta el token de acceso")
	}
	authValues := values.Get("authorization")
	if len(authValues) == 0 {
		return "", status.Error(codes.Unauthenticated, "falta el token de acceso")
	}
	parts := strings.Fields(authValues[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", status.Error(codes.Unauthenticated, "token de acceso inválido")
	}
	return parts[1], nil
}

// UserFromContext expone la identidad autenticada a la capa de logic. Para
// un sistema consumidor, userID es el UUID del sistema y role queda vacío
// (los registros de auditoría guardan ese UUID como autor).
func UserFromContext(ctx context.Context) (userID, role string, ok bool) {
	u, ok := ctx.Value(contextKey{}).(userContext)
	if !ok || strings.TrimSpace(u.UserID) == "" {
		return "", "", false
	}
	return u.UserID, u.Role, true
}

// ServiceFromContext: identidad del sistema consumidor, si la llamada la hizo
// un sistema y no un usuario.
func ServiceFromContext(ctx context.Context) (*ServiceIdentity, bool) {
	u, ok := ctx.Value(contextKey{}).(userContext)
	if !ok || u.Service == nil {
		return nil, false
	}
	return u.Service, true
}

// TokenFromContext expone el JWT crudo ya validado (Paso 16B) — solo para
// reenviarlo a Auth Service al consultar HasPermission, nunca para volver
// a validarlo acá. Un token de sistema nunca se reenvía.
func TokenFromContext(ctx context.Context) (string, bool) {
	u, ok := ctx.Value(contextKey{}).(userContext)
	if !ok || u.Service != nil || strings.TrimSpace(u.Token) == "" {
		return "", false
	}
	return u.Token, true
}
