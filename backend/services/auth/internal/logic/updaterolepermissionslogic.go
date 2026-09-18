package logic

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"

	"auth/auth"
	"auth/internal/repository"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UpdateRolePermissionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateRolePermissionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateRolePermissionsLogic {
	return &UpdateRolePermissionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// updateRolePermissionsUUIDPattern valida solo la FORMA de role_id/
// permission_ids (mismo patrón ya usado por HasPermissionLogic para
// user_id y por GetRolePermissionsLogic para role_id).
var updateRolePermissionsUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// UpdateRolePermissions (Paso 21C): reemplaza el conjunto COMPLETO de
// permisos de un rol — permission_ids es el estado FINAL deseado, nunca una
// lista de permisos a "agregar". Autenticación + "roles.view" ya resueltas
// por el interceptor (ProtectedMethod), igual que ListRoles/
// ListPermissions/GetRolePermissions — sin ningún chequeo de rol/permiso
// adicional acá (sección 3: nada de "if role == ADMIN").
func (l *UpdateRolePermissionsLogic) UpdateRolePermissions(in *auth.UpdateRolePermissionsRequest) (*auth.UpdateRolePermissionsResponse, error) {
	if in == nil || strings.TrimSpace(in.RoleId) == "" {
		return nil, status.Error(codes.InvalidArgument, "el role_id es obligatorio")
	}
	roleID := strings.TrimSpace(in.RoleId)
	if !updateRolePermissionsUUIDPattern.MatchString(roleID) {
		return nil, status.Error(codes.InvalidArgument, "el role_id no tiene un formato válido")
	}

	// Normaliza permission_ids: TrimSpace + formato UUID (InvalidArgument
	// si alguno no lo es) + deduplicado (sección 5: la BD nunca debe
	// terminar con duplicados; se elige normalizar en vez de rechazar,
	// coherente con el resto del proyecto, que ya recorta/normaliza campos
	// de texto en cada Logic en vez de rechazar variaciones inofensivas).
	seen := make(map[string]bool, len(in.PermissionIds))
	permissionIDs := make([]string, 0, len(in.PermissionIds))
	for _, raw := range in.PermissionIds {
		permissionID := strings.TrimSpace(raw)
		if !updateRolePermissionsUUIDPattern.MatchString(permissionID) {
			return nil, status.Error(codes.InvalidArgument, "uno de los permission_ids no tiene un formato válido")
		}
		if seen[permissionID] {
			continue
		}
		seen[permissionID] = true
		permissionIDs = append(permissionIDs, permissionID)
	}

	// Reemplazo atómico (BEGIN/validar-dentro-de-la-tx/DELETE/INSERT/COMMIT
	// — ver RolePermissionRepository.ReplacePermissionsByRoleID). Cualquier
	// rol o permission_id inexistente hace ROLLBACK completo: el rol nunca
	// queda con una modificación parcial.
	if err := l.svcCtx.RolePermissionRepository.ReplacePermissionsByRoleID(l.ctx, roleID, permissionIDs); err != nil {
		if errors.Is(err, repository.ErrRoleNotFound) {
			return nil, status.Error(codes.NotFound, "rol no encontrado")
		}
		if errors.Is(err, repository.ErrPermissionNotFound) {
			return nil, status.Error(codes.NotFound, "uno de los permisos indicados no existe")
		}
		return nil, status.Error(codes.Internal, "no se pudo actualizar los permisos del rol")
	}

	// Orden determinista para la respuesta (sección 10), independiente del
	// orden en que llegó el request.
	sort.Strings(permissionIDs)

	return &auth.UpdateRolePermissionsResponse{
		RoleId:        roleID,
		PermissionIds: permissionIDs,
	}, nil
}
