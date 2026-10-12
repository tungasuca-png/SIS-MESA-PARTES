package logic

import (
	"context"

	"auth/auth"
	"auth/internal/interceptor"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetMyPermissionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMyPermissionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMyPermissionsLogic {
	return &GetMyPermissionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetMyPermissions devuelve la lista completa de permisos efectivos del
// usuario AUTENTICADO — self-only por construcción: el request no tiene
// ningún campo de identificador, así que no hay forma de que un cliente
// pida los permisos de otro usuario. Reutiliza tal cual
// AuthorizationService.GetPermissionsByUserID (la misma función que ya usa
// HasPermission internamente), que ya resuelve correctamente la unión de
// permisos de TODOS los roles reales del usuario — sin caché: cada llamada
// consulta la base de datos en el momento.
func (l *GetMyPermissionsLogic) GetMyPermissions(_ *auth.GetMyPermissionsRequest) (*auth.GetMyPermissionsResponse, error) {
	userID, ok := interceptor.UserIDFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}

	permissions, err := l.svcCtx.AuthorizationService.GetPermissionsByUserID(l.ctx, userID)
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo consultar los permisos")
	}

	return &auth.GetMyPermissionsResponse{Permissions: permissions}, nil
}
