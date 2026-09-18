package logic

import (
	"context"
	"regexp"
	"strings"

	"auth/auth"
	"auth/internal/interceptor"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type HasPermissionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewHasPermissionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HasPermissionLogic {
	return &HasPermissionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// hasPermissionUUIDPattern valida solo la FORMA del user_id (mismo patrón
// que ya usan Derivaciones/Documentos/Usuarios Service en su paquete
// internal/validation) — no es una segunda implementación de autorización,
// es una validación de transporte antes de llamar a AuthorizationService.
var hasPermissionUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// HasPermission expone por gRPC la MISMA decisión que ya toma
// AuthorizationService.HasPermission() (usuario -> roles reales -> permisos
// reales, ya sembrados en PostgreSQL) — no consulta usuario_roles,
// role_permissions ni permissions por su cuenta, y no reimplementa ninguna
// regla de roles/permisos.
//
// Seguridad (decisión del PASO 11, sin mecanismos nuevos): el RPC exige
// autenticación igual que Logout (AuthenticatedMethod en
// DefaultMethodPolicies) y SOLO permite consultar el permiso del propio
// usuario autenticado — el user_id del request debe coincidir con el sub
// del JWT ya validado por AuthenticationInterceptor. Así se evita que un
// usuario autenticado use este RPC para averiguar los permisos de otro.
func (l *HasPermissionLogic) HasPermission(in *auth.HasPermissionRequest) (*auth.HasPermissionResponse, error) {
	authenticatedUserID, ok := interceptor.UserIDFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}

	if in == nil || strings.TrimSpace(in.UserId) == "" {
		return nil, status.Error(codes.InvalidArgument, "el user_id es obligatorio")
	}
	if strings.TrimSpace(in.Permission) == "" {
		return nil, status.Error(codes.InvalidArgument, "el permission es obligatorio")
	}

	userID := strings.TrimSpace(in.UserId)
	permission := strings.TrimSpace(in.Permission)

	if !hasPermissionUUIDPattern.MatchString(userID) {
		return nil, status.Error(codes.InvalidArgument, "el user_id no tiene un formato válido")
	}

	if userID != authenticatedUserID {
		return nil, status.Error(codes.PermissionDenied, "no puede consultar el permiso de otro usuario")
	}

	allowed, err := l.svcCtx.AuthorizationService.HasPermission(l.ctx, userID, permission)
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo verificar el permiso")
	}

	return &auth.HasPermissionResponse{Allowed: allowed}, nil
}
