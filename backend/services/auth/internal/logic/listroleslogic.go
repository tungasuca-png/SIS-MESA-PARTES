package logic

import (
	"context"

	"auth/auth"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListRolesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListRolesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRolesLogic {
	return &ListRolesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListRoles (Paso 21B): catálogo real de roles, SOLO LECTURA — la
// autenticación (JWT) y la autorización ("roles.view") ya las resuelve por
// completo el par AuthenticationInterceptor/AuthorizationInterceptor
// (DefaultMethodPolicies: ProtectedMethod("roles.view")), así que esta
// lógica no repite ningún chequeo de permiso.
func (l *ListRolesLogic) ListRoles(in *auth.ListRolesRequest) (*auth.ListRolesResponse, error) {
	roles, err := l.svcCtx.RoleRepository.List(l.ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo consultar los roles")
	}

	items := make([]*auth.Role, 0, len(roles))
	for _, r := range roles {
		items = append(items, &auth.Role{
			Id:          r.ID,
			Nombre:      r.Name,
			Descripcion: r.Descripcion,
			Estado:      r.Estado,
		})
	}

	return &auth.ListRolesResponse{Roles: items}, nil
}
