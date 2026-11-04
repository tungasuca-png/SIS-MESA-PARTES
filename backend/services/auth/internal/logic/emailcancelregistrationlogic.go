package logic

import (
	"context"

	"auth/auth"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type EmailCancelRegistrationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEmailCancelRegistrationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EmailCancelRegistrationLogic {
	return &EmailCancelRegistrationLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// EmailCancelRegistration: compensación cuando Usuarios Service rechaza el
// perfil (p. ej. DNI ya registrado allí). Elimina la cuenta creada por ESTE
// registro mediante la función cancelar_registro_correo_pendiente, que
// vuelve a comprobar que nunca tuvo sesión ni otro rol; anula el registro.
func (l *EmailCancelRegistrationLogic) EmailCancelRegistration(in *auth.EmailRegistrationRequest) (*auth.EmailCancelResponse, error) {
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
	usuarioID, err := l.svcCtx.RegistroCorreoRepository.Cancelar(l.ctx, id, func(hash []byte) bool {
		return codigos.Coincide(hash, id, codigo)
	}, lim.intentos)
	if err != nil {
		e := errDelRegistro(err)
		if status.Code(e) == codes.Internal {
			l.Errorf("registro con correo: no se pudo cancelar: %v", err)
		}
		eventoRegistro(l.ctx, l.svcCtx, "", "correo_registro_cancelado", "RECHAZADO", motivoRegistro(e))
		return nil, e
	}
	eventoRegistro(l.ctx, l.svcCtx, usuarioID, "correo_registro_cancelado", "OK", "")
	return &auth.EmailCancelResponse{Cancelled: true}, nil
}
