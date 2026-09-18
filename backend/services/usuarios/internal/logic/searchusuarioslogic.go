package logic

import (
	"context"
	"strings"
	"unicode/utf8"

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

// minQueryLength evita búsquedas indiscriminadas que devuelvan
// prácticamente todo el padrón de usuarios.
const minQueryLength = 3

type SearchUsuariosLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSearchUsuariosLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchUsuariosLogic {
	return &SearchUsuariosLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SearchUsuariosLogic) SearchUsuarios(in *usuarios.SearchUsuariosRequest) (*usuarios.SearchUsuariosResponse, error) {
	userID, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	// Permiso real (Paso 16B) + regla de negocio existente (CanListOrSearch),
	// AMBAS deben cumplirse — mismo criterio y misma razón que ListUsuarios.
	if err := authorization.RequirePermission(l.ctx, l.svcCtx.AuthClient, userID, "usuarios.view"); err != nil {
		return nil, err
	}
	if !authorization.CanListOrSearch(role) {
		return nil, status.Error(codes.PermissionDenied, "el rol no puede buscar usuarios")
	}

	query := strings.TrimSpace(in.GetQuery())
	if utf8.RuneCountInString(query) < minQueryLength {
		return nil, status.Error(codes.InvalidArgument, "la búsqueda debe tener al menos 3 caracteres")
	}

	tipoUsuario := strings.ToUpper(strings.TrimSpace(in.GetTipoUsuario()))
	if tipoUsuario != "" && !validation.IsValidTipoUsuario(tipoUsuario) {
		return nil, status.Error(codes.InvalidArgument, "el tipo de usuario no es válido")
	}

	filter := repository.ListFilter{
		TipoUsuario: tipoUsuario,
		Query:       query,
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
		l.Errorf("buscar usuarios: %v", err)
		return nil, status.Error(codes.Internal, "no se pudo buscar usuarios")
	}

	protoItems := make([]*usuarios.Usuario, 0, len(items))
	for _, item := range items {
		protoItems = append(protoItems, toProto(item))
	}

	return &usuarios.SearchUsuariosResponse{Usuarios: protoItems, Total: int32(total)}, nil
}
