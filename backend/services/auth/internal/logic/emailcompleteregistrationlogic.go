package logic

import (
	"context"

	"auth/auth"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type EmailCompleteRegistrationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEmailCompleteRegistrationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EmailCompleteRegistrationLogic {
	return &EmailCompleteRegistrationLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// EmailCompleteRegistration: el perfil ya existe en Usuarios Service. Desde
// aquí la cuenta puede iniciar sesión y el código deja de servir.
func (l *EmailCompleteRegistrationLogic) EmailCompleteRegistration(in *auth.EmailRegistrationRequest) (*auth.EmailCompleteResponse, error) {
	if err := registroDisponible(l.svcCtx); err != nil {
		return nil, err
	}
	id, err := idRegistroValido(in.GetRegistrationId())
	if err != nil {
		return nil, err
	}
	codigo, err := codigoValido(in.GetCode())
	if err != nil {
		return nil, err
	}
	codigos := l.svcCtx.CodigosVerificacion
	lim := limitesDe(l.svcCtx.Config.RegistroCorreo)
	usuarioID, dni, err := l.svcCtx.RegistroCorreoRepository.Completar(l.ctx, id, func(hash []byte) bool {
		return codigos.Coincide(hash, id, codigo)
	}, lim.intentos)
	if err != nil {
		e := errDelRegistro(err)
		if status.Code(e) == codes.Internal {
			l.Errorf("registro con correo: no se pudo completar: %v", err)
		}
		eventoRegistro(l.ctx, l.svcCtx, "", "correo_activacion", "RECHAZADO", motivoRegistro(e))
		return nil, e
	}
	eventoRegistro(l.ctx, l.svcCtx, usuarioID, "correo_activacion", "OK", "")
	return &auth.EmailCompleteResponse{Dni: dni}, nil
}
