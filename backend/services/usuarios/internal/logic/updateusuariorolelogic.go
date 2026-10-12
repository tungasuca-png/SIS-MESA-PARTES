package logic

import (
	"context"
	"errors"
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

type UpdateUsuarioRoleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateUsuarioRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateUsuarioRoleLogic {
	return &UpdateUsuarioRoleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpdateUsuarioRole sincroniza ÚNICAMENTE tipo_usuario tras un cambio de rol
// YA confirmado por Auth Service — este RPC nunca decide el rol, solo
// refleja uno que Auth ya validó y persistió. Se autoriza con el MISMO
// permiso real que protege esa decisión ("usuarios.change_role", vía
// Auth.HasPermission), nunca con un chequeo de rol local: no se reemplaza
// esa autorización por "role == ADMIN".
func (l *UpdateUsuarioRoleLogic) UpdateUsuarioRole(in *usuarios.UpdateUsuarioRoleRequest) (*usuarios.UpdateUsuarioRoleResponse, error) {
	actorID, _, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if err := authorization.RequirePermission(l.ctx, l.svcCtx.AuthClient, actorID, "usuarios.change_role"); err != nil {
		return nil, err
	}

	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "la solicitud es obligatoria")
	}
	id := strings.TrimSpace(in.Id)
	if !validation.IsValidUUID(id) {
		return nil, status.Error(codes.InvalidArgument, "el identificador no es un UUID válido")
	}
	tipoUsuario := strings.ToUpper(strings.TrimSpace(in.TipoUsuario))
	if !validation.IsValidTipoUsuario(tipoUsuario) {
		return nil, status.Error(codes.InvalidArgument, "el tipo de usuario no es válido")
	}

	saved, err := l.svcCtx.UsuarioRepository.UpdateTipoUsuario(l.ctx, id, tipoUsuario)
	if err != nil {
		if errors.Is(err, repository.ErrUsuarioNotFound) {
			return nil, status.Error(codes.NotFound, "el usuario no tiene perfil registrado en Usuarios Service")
		}
		l.Errorf("sincronizar tipo_usuario del usuario %s: %v", id, err)
		return nil, status.Error(codes.Internal, "no se pudo sincronizar el tipo de usuario")
	}

	return &usuarios.UpdateUsuarioRoleResponse{Usuario: toProto(saved)}, nil
}
