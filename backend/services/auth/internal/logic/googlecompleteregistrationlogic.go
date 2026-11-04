package logic

import (
	"context"
	"errors"

	"auth/auth"
	"auth/internal/repository"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GoogleCompleteRegistrationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGoogleCompleteRegistrationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GoogleCompleteRegistrationLogic {
	return &GoogleCompleteRegistrationLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// GoogleCompleteRegistration: paso 2. El perfil ya existe en Usuarios
// Service: marca la identidad como completa, consume el código y emite la
// sesión normal (solo ahora).
func (l *GoogleCompleteRegistrationLogic) GoogleCompleteRegistration(in *auth.GoogleRegistrationRequest) (*auth.GoogleCompleteResponse, error) {
	hash, ok := hashCodigo(in.GetLoginCode())
	if !ok {
		return nil, errCodigoInvalido()
	}
	usuarioID, err := l.svcCtx.GoogleRepository.CompleteRegistration(l.ctx, hash)
	if err != nil {
		if errors.Is(err, repository.ErrCodigoInvalido) {
			return nil, errCodigoInvalido()
		}
		return nil, status.Error(codes.Internal, "no se pudo completar el registro")
	}
	user, role, err := cuentaSolicitanteDisponible(l.ctx, l.svcCtx, usuarioID)
	if err != nil {
		return nil, err
	}
	sesion, err := emitirSesion(l.ctx, l.svcCtx, user, role)
	if err != nil {
		return nil, err
	}
	evento(l.ctx, l.svcCtx, usuarioID, "google_registro", "OK", "")
	return &auth.GoogleCompleteResponse{Session: sesion}, nil
}
