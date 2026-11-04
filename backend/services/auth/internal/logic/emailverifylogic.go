package logic

import (
	"context"
	"strings"

	"auth/auth"
	"auth/internal/repository"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type EmailVerifyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEmailVerifyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EmailVerifyLogic {
	return &EmailVerifyLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// EmailVerify: paso 1 de la activación. Con el código correcto crea la
// cuenta SOLICITANTE con el correo verificado (o retoma la ya creada si falta
// el perfil) y devuelve un access token del nuevo usuario SOLO para que el
// Gateway cree su perfil. La cuenta no puede iniciar sesión hasta
// EmailCompleteRegistration. Un código incorrecto cuenta un intento.
func (l *EmailVerifyLogic) EmailVerify(in *auth.EmailVerifyRequest) (*auth.EmailVerifyResponse, error) {
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
	username, err := usernameSolicitante()
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo preparar la cuenta")
	}
	codigos := l.svcCtx.CodigosVerificacion
	lim := limitesDe(l.svcCtx.Config.RegistroCorreo)
	v, err := l.svcCtx.RegistroCorreoRepository.Verificar(l.ctx, id, func(hash []byte) bool {
		return codigos.Coincide(hash, id, codigo)
	}, lim.intentos, username)
	if err != nil {
		e := errDelRegistro(err)
		if status.Code(e) == codes.Internal {
			l.Errorf("registro con correo: no se pudo verificar: %v", err)
		}
		eventoRegistro(l.ctx, l.svcCtx, "", "correo_verificacion", "RECHAZADO", motivoRegistro(e))
		return nil, e
	}
	token, _, err := l.svcCtx.JWTManager.GenerateAccessToken(v.UsuarioID, v.Username, repository.RolSolicitante)
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo generar el token")
	}
	eventoRegistro(l.ctx, l.svcCtx, v.UsuarioID, "correo_verificacion", "OK", "")
	return &auth.EmailVerifyResponse{
		UserId:       v.UsuarioID,
		Dni:          v.DNI,
		Email:        v.Email,
		Nombres:      v.Nombres,
		Apellidos:    strings.TrimSpace(v.ApellidoPaterno + " " + v.ApellidoMaterno),
		ProfileToken: token,
	}, nil
}
