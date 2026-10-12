package logic

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"auth/auth"
	"auth/internal/repository"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UpdateUserRoleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateUserRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateUserRoleLogic {
	return &UpdateUserRoleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// updateUserRoleUUIDPattern valida solo la FORMA de user_id/role_id (mismo
// patrón ya usado por HasPermissionLogic/UpdateRolePermissionsLogic).
var updateUserRoleUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// UpdateUserRole (Usuario -> Rol, distinto de Rol -> Permisos de
// UpdateRolePermissions): reemplaza el ÚNICO rol de un usuario ya
// existente. Autenticación + "usuarios.change_role" ya resueltas por el
// interceptor (ProtectedMethod) — sin ningún chequeo de rol adicional acá.
func (l *UpdateUserRoleLogic) UpdateUserRole(in *auth.UpdateUserRoleRequest) (*auth.UpdateUserRoleResponse, error) {
	if in == nil || strings.TrimSpace(in.UserId) == "" {
		return nil, status.Error(codes.InvalidArgument, "el user_id es obligatorio")
	}
	userID := strings.TrimSpace(in.UserId)
	if !updateUserRoleUUIDPattern.MatchString(userID) {
		return nil, status.Error(codes.InvalidArgument, "el user_id no tiene un formato válido")
	}

	if strings.TrimSpace(in.RoleId) == "" {
		return nil, status.Error(codes.InvalidArgument, "el role_id es obligatorio")
	}
	roleID := strings.TrimSpace(in.RoleId)
	if !updateUserRoleUUIDPattern.MatchString(roleID) {
		return nil, status.Error(codes.InvalidArgument, "el role_id no tiene un formato válido")
	}

	// Reemplazo atómico (BEGIN/validar-dentro-de-la-tx/DELETE/INSERT/COMMIT
	// — ver UserRepository.ReplaceUserRole). Cualquier fallo de validación
	// hace ROLLBACK completo: el usuario nunca queda sin rol ni con dos.
	user, role, err := l.svcCtx.UserRepository.ReplaceUserRole(l.ctx, userID, roleID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, "usuario no encontrado")
		}
		if errors.Is(err, repository.ErrRoleNotFound) {
			return nil, status.Error(codes.NotFound, "rol no encontrado")
		}
		if errors.Is(err, repository.ErrUserInactive) {
			return nil, status.Error(codes.FailedPrecondition, "el usuario está inactivo")
		}
		if errors.Is(err, repository.ErrRoleInactive) {
			return nil, status.Error(codes.FailedPrecondition, "el rol está inactivo")
		}
		l.Errorf("cambiar rol del usuario %s: %v", userID, err)
		return nil, status.Error(codes.Internal, "no se pudo cambiar el rol del usuario")
	}

	return &auth.UpdateUserRoleResponse{
		UserId:     user.ID,
		RoleId:     role.ID,
		RoleNombre: role.Name,
	}, nil
}
