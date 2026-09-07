package logic

import (
	"context"
	"errors"
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

type UpsertUsuarioLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpsertUsuarioLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertUsuarioLogic {
	return &UpsertUsuarioLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpsertUsuarioLogic) UpsertUsuario(in *usuarios.UpsertUsuarioRequest) (*usuarios.UpsertUsuarioResponse, error) {
	_, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if !authorization.CanUpsert(role) {
		return nil, status.Error(codes.PermissionDenied, "el rol no puede administrar perfiles de usuario")
	}
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "la solicitud es obligatoria")
	}

	// El id proviene de Auth Service: este servicio nunca genera identidades.
	id := strings.TrimSpace(in.Id)
	if !validation.IsValidUUID(id) {
		return nil, status.Error(codes.InvalidArgument, "el identificador no es un UUID válido")
	}

	nombres := strings.TrimSpace(in.Nombres)
	apellidos := strings.TrimSpace(in.Apellidos)
	if nombres == "" || apellidos == "" {
		return nil, status.Error(codes.InvalidArgument, "nombres y apellidos son obligatorios")
	}
	if utf8.RuneCountInString(nombres) > 100 || utf8.RuneCountInString(apellidos) > 100 {
		return nil, status.Error(codes.InvalidArgument, "nombres y apellidos no pueden superar los 100 caracteres")
	}

	tipoUsuario := strings.ToUpper(strings.TrimSpace(in.TipoUsuario))
	if !validation.IsValidTipoUsuario(tipoUsuario) {
		return nil, status.Error(codes.InvalidArgument, "el tipo de usuario no es válido")
	}

	saved, err := l.svcCtx.UsuarioRepository.Upsert(l.ctx, &repository.Usuario{
		ID:          id,
		Nombres:     nombres,
		Apellidos:   apellidos,
		DNI:         strings.TrimSpace(in.Dni),
		Telefono:    strings.TrimSpace(in.Telefono),
		Correo:      strings.TrimSpace(in.Correo),
		Direccion:   strings.TrimSpace(in.Direccion),
		TipoUsuario: tipoUsuario,
	})
	if err != nil {
		if errors.Is(err, repository.ErrDniExists) {
			return nil, status.Error(codes.AlreadyExists, "el DNI ya está registrado para otro usuario")
		}
		l.Errorf("guardar perfil de usuario %s: %v", id, err)
		return nil, status.Error(codes.Internal, "no se pudo guardar el perfil del usuario")
	}

	return &usuarios.UpsertUsuarioResponse{Usuario: toProto(saved)}, nil
}
