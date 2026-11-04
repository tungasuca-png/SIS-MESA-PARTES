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

type GoogleCancelRegistrationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGoogleCancelRegistrationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GoogleCancelRegistrationLogic {
	return &GoogleCancelRegistrationLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// GoogleCancelRegistration: compensación cuando Usuarios Service rechaza el
// perfil (DNI ya registrado allí). Elimina SOLO la cuenta pendiente creada
// por este registro, que nunca tuvo sesión ni actividad.
func (l *GoogleCancelRegistrationLogic) GoogleCancelRegistration(in *auth.GoogleRegistrationRequest) (*auth.GoogleCancelResponse, error) {
	hash, ok := hashCodigo(in.GetLoginCode())
	if !ok {
		return nil, errCodigoInvalido()
	}
	usuarioID, err := l.svcCtx.GoogleRepository.CancelRegistration(l.ctx, hash)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrCodigoInvalido):
			return nil, errCodigoInvalido()
		case errors.Is(err, repository.ErrRegistroNoCancelable):
			return nil, status.Error(codes.FailedPrecondition, "el registro no se puede cancelar")
		}
		l.Errorf("compensación del registro con Google: %v", err)
		return nil, status.Error(codes.Internal, "no se pudo cancelar el registro")
	}
	evento(l.ctx, l.svcCtx, usuarioID, "google_registro_cancelado", "OK", "perfil_rechazado")
	return &auth.GoogleCancelResponse{Cancelled: true}, nil
}
