package logic

import (
	"context"

	"auth/auth"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListPermissionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListPermissionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPermissionsLogic {
	return &ListPermissionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListPermissions (Paso 21B): catálogo GLOBAL de permisos (tabla
// permissions completa), no los permisos del usuario autenticado — eso ya
// lo resuelve HasPermission/GetPermissionsByUserID, sin relación con este
// RPC. Autenticación + "roles.view" ya resueltas por el interceptor
// (ProtectedMethod), igual que ListRoles.
func (l *ListPermissionsLogic) ListPermissions(in *auth.ListPermissionsRequest) (*auth.ListPermissionsResponse, error) {
	permissions, err := l.svcCtx.PermissionRepository.List(l.ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo consultar los permisos")
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

	return &auth.ListPermissionsResponse{Permissions: items}, nil
}
