package logic

import (
	"context"
	"errors"
	"time"

	"auth/auth"
	"auth/internal/correo"
	"auth/internal/security"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type EmailResendCodeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEmailResendCodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EmailResendCodeLogic {
	return &EmailResendCodeLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// EmailResendCode: código nuevo para un registro pendiente. El anterior deja
// de valer, los intentos vuelven a cero y la vigencia se renueva. Respeta la
// espera entre envíos y el máximo de envíos por registro. Si el envío falla,
// no cambia nada (el código anterior sigue vigente).
func (l *EmailResendCodeLogic) EmailResendCode(in *auth.EmailResendCodeRequest) (*auth.EmailResendCodeResponse, error) {
	if err := registroDisponible(l.svcCtx); err != nil {
		return nil, err
	}
	id, err := idRegistroValido(in.GetRegistrationId())
	if err != nil {
		return nil, err
	}
	lim := limitesDe(l.svcCtx.Config.RegistroCorreo)
	codigo, err := security.GenerarCodigoVerificacion()
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo generar el código")
	}
	var errCorreo error
	expira, err := l.svcCtx.RegistroCorreoRepository.Reenviar(l.ctx, id, l.svcCtx.CodigosVerificacion.Hash(id, codigo),
		lim.duracion, lim.espera, lim.envios, func(email string, existente bool) error {
			m := correo.CodigoVerificacion(email, codigo, lim.duracion)
			if existente {
				m = correo.CuentaExistente(email)
			}
			errCorreo = l.svcCtx.Correo.Enviar(l.ctx, m)
			return errCorreo
		})
	if err != nil {
		var e error
		if errCorreo != nil && errors.Is(err, errCorreo) {
			e = errEnvio(l.ctx, errCorreo)
		} else {
			e = errDelRegistro(err)
			if status.Code(e) == codes.Internal {
				l.Errorf("registro con correo: no se pudo reenviar el código: %v", err)
			}
		}
		eventoRegistro(l.ctx, l.svcCtx, "", "correo_reenvio", "RECHAZADO", motivoRegistro(e))
		return nil, e
	}
	eventoRegistro(l.ctx, l.svcCtx, "", "correo_reenvio", "OK", "")
	return &auth.EmailResendCodeResponse{
		ExpiresIn: segundos(time.Until(expira)),
		ResendIn:  segundos(lim.espera),
	}, nil
}
