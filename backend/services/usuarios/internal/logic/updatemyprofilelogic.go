package logic

import (
	"context"
	"errors"
	"strings"

	"usuarios/internal/interceptor"
	"usuarios/internal/repository"
	"usuarios/internal/svc"
	"usuarios/internal/validation"
	"usuarios/usuarios"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// maxTelefonoLength refleja exactamente el límite de la columna
// "telefono VARCHAR(20)" (ver migrations/001_create_usuarios.sql) — no es
// una regla de negocio nueva, es el mismo límite que la base ya impone.
const maxTelefonoLength = 20

type UpdateMyProfileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateMyProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateMyProfileLogic {
	return &UpdateMyProfileLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpdateMyProfile (Paso 22E): edita ÚNICAMENTE telefono/correo/direccion del
// perfil del usuario AUTENTICADO. La identidad sale exclusivamente de
// interceptor.UserFromContext (JWT ya validado) — el request no tiene, ni
// debe tener, ningún campo de identificador: no hay forma de que un cliente
// indique "actualiza a otro usuario" (sin IDOR posible por diseño del
// contrato). No requiere "usuarios.view" ni ninguna otra regla de
// authorization: autoconsulta/autoedición ya está permitida sin ese
// permiso, mismo criterio que GetUsuario.
func (l *UpdateMyProfileLogic) UpdateMyProfile(in *usuarios.UpdateMyProfileRequest) (*usuarios.UpdateMyProfileResponse, error) {
	userID, _, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "la solicitud es obligatoria")
	}

	telefono := strings.TrimSpace(in.Telefono)
	if len(telefono) > maxTelefonoLength {
		return nil, status.Error(codes.InvalidArgument, "el teléfono no puede superar los 20 caracteres")
	}

	correo := strings.TrimSpace(in.Correo)
	if correo != "" && !validation.IsValidEmail(correo) {
		return nil, status.Error(codes.InvalidArgument, "el correo no tiene un formato válido")
	}

	direccion := strings.TrimSpace(in.Direccion)

	updated, err := l.svcCtx.UsuarioRepository.UpdateOwnProfile(l.ctx, userID, telefono, correo, direccion)
	if err != nil {
		if errors.Is(err, repository.ErrUsuarioNotFound) {
			return nil, status.Error(codes.NotFound, "todavía no tienes un perfil registrado")
		}
		l.Errorf("actualizar perfil propio %s: %v", userID, err)
		return nil, status.Error(codes.Internal, "no se pudo actualizar el perfil")
	}

	return &usuarios.UpdateMyProfileResponse{Usuario: toProto(updated)}, nil
}
