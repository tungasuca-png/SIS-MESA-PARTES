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

// getRolePermissionsUUIDPattern valida solo la FORMA del role_id (mismo
// patrón ya usado por HasPermissionLogic para user_id) — evita que un
// role_id malformado dispare un error crudo de PostgreSQL en vez de un
// InvalidArgument claro.
var getRolePermissionsUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type GetRolePermissionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetRolePermissionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRolePermissionsLogic {
	return &GetRolePermissionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetRolePermissions (Paso 21B): permisos REALMENTE asignados a un rol
// (role_permissions), identificado exclusivamente por role_id real —
// nunca por role_name ni por una lista de permission_ids del cliente.
// Autenticación + "roles.view" ya resueltas por el interceptor
// (ProtectedMethod), igual que ListRoles/ListPermissions.
func (l *GetRolePermissionsLogic) GetRolePermissions(in *auth.GetRolePermissionsRequest) (*auth.GetRolePermissionsResponse, error) {
	if in == nil || strings.TrimSpace(in.RoleId) == "" {
		return nil, status.Error(codes.InvalidArgument, "el role_id es obligatorio")
	}
	roleID := strings.TrimSpace(in.RoleId)
	if !getRolePermissionsUUIDPattern.MatchString(roleID) {
		return nil, status.Error(codes.InvalidArgument, "el role_id no tiene un formato válido")
	}

	// Confirma que el rol exista de verdad ANTES de consultar sus permisos
	// (sección 13/19: aislamiento explícito por role_id, nunca devolver
	// permisos de otro rol por error) — RolePermissionRepository.
	// GetPermissionsByRoleID por sí solo no distingue "rol inexistente" de
	// "rol sin ningún permiso asignado" (ambos devuelven lista vacía), así
	// que esta validación es la única forma de responder NotFound cuando
	// corresponde.
	if _, err := l.svcCtx.RoleRepository.GetByID(l.ctx, roleID); err != nil {
		if errors.Is(err, repository.ErrRoleNotFound) {
			return nil, status.Error(codes.NotFound, "rol no encontrado")
		}
		return nil, status.Error(codes.Internal, "no se pudo consultar el rol")
	}

	permissions, err := l.svcCtx.RolePermissionRepository.GetPermissionsByRoleID(l.ctx, roleID)
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo consultar los permisos del rol")
	}

	items := make([]*auth.Permission, 0, len(permissions))
	for _, p := range permissions {
		items = append(items, &auth.Permission{
			Id:          p.ID,
			Codigo:      p.Codigo,
			Nombre:      p.Nombre,
			Descripcion: p.Descripcion,
			Modulo:      p.Modulo,
			Estado:      p.Estado,
		})
	}

	return &auth.GetRolePermissionsResponse{Permissions: items}, nil
}
