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

type GetUsuarioLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUsuarioLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUsuarioLogic {
	return &GetUsuarioLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetUsuarioLogic) GetUsuario(in *usuarios.GetUsuarioRequest) (*usuarios.GetUsuarioResponse, error) {
	requesterID, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if in == nil || strings.TrimSpace(in.Id) == "" {
		return nil, status.Error(codes.InvalidArgument, "el identificador es obligatorio")
	}

	id := strings.TrimSpace(in.Id)
	if !validation.IsValidUUID(id) {
		return nil, status.Error(codes.InvalidArgument, "el identificador no es un UUID válido")
	}
	if !authorization.CanViewFullProfile(role, requesterID, id) {
		return nil, status.Error(codes.PermissionDenied, "no tiene acceso al perfil completo de este usuario")
	}

	found, err := l.svcCtx.UsuarioRepository.FindByID(l.ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrUsuarioNotFound) {
			return nil, status.Error(codes.NotFound, "usuario no encontrado")
		}
		l.Errorf("consultar usuario %s: %v", id, err)
		return nil, status.Error(codes.Internal, "no se pudo consultar el usuario")
	}

	return &usuarios.GetUsuarioResponse{Usuario: toProto(found)}, nil
}
