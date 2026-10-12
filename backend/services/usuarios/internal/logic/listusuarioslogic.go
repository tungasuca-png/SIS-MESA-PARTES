package logic

import (
	"context"
	"strings"

	"usuarios/internal/authorization"
	"usuarios/internal/interceptor"
	"usuarios/internal/repository"
	"usuarios/internal/svc"
	"usuarios/internal/validation"
	"usuarios/usuarios"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

type ListUsuariosLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListUsuariosLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListUsuariosLogic {
	return &ListUsuariosLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListUsuariosLogic) ListUsuarios(in *usuarios.ListUsuariosRequest) (*usuarios.ListUsuariosResponse, error) {
	userID, _, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	// La autoridad es el permiso real: Auth.HasPermission("usuarios.view"). Ya
	// no se exige además el nombre del rol (antes: role == ADMIN) — un rol al
	// que ADMIN le asigne usuarios.view desde Roles y permisos debe poder
	// listar.
	if err := authorization.RequirePermission(l.ctx, l.svcCtx.AuthClient, userID, "usuarios.view"); err != nil {
		return nil, err
	}

	tipoUsuario := strings.ToUpper(strings.TrimSpace(in.GetTipoUsuario()))
	if tipoUsuario != "" && !validation.IsValidTipoUsuario(tipoUsuario) {
		return nil, status.Error(codes.InvalidArgument, "el tipo de usuario no es válido")
	}

	filter := repository.ListFilter{
		TipoUsuario: tipoUsuario,
		Page:        int(in.GetPage()),
		PageSize:    int(in.GetPageSize()),
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 || filter.PageSize > maxPageSize {
		filter.PageSize = defaultPageSize
	}

	items, total, err := l.svcCtx.UsuarioRepository.List(l.ctx, filter)
	if err != nil {
		l.Errorf("listar usuarios: %v", err)
		return nil, status.Error(codes.Internal, "no se pudo listar los usuarios")
	}

	protoItems := make([]*usuarios.Usuario, 0, len(items))
	for _, item := range items {
		protoItems = append(protoItems, toProto(item))
	}

	return &usuarios.ListUsuariosResponse{Usuarios: protoItems, Total: int32(total)}, nil
}
