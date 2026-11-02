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

type GoogleRegisterLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGoogleRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GoogleRegisterLogic {
	return &GoogleRegisterLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// GoogleRegister: paso 1 del autorregistro. Con el código REGISTRO y el DNI
// del formulario crea la cuenta SOLICITANTE (rol fijado aquí; el navegador no
// puede elegirlo) y su identidad Google, pendiente de perfil. Un DNI o correo
// ya registrados detienen todo sin crear nada y sin revelar a quién
// pertenecen. Devuelve un access token del nuevo usuario solo para que el
// Gateway cree su perfil en Usuarios Service; la sesión se emite al confirmar.
func (l *GoogleRegisterLogic) GoogleRegister(in *auth.GoogleRegisterRequest) (*auth.GoogleRegisterResponse, error) {
	hash, ok := hashCodigo(in.GetLoginCode())
	if !ok {
		return nil, errCodigoInvalido()
	}
	dni, err := normalizeDNI(in.GetDni())
	if err != nil {
		return nil, err
	}
	username, passwordHash, err := credencialesInternas()
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo preparar la cuenta")
	}
	reg, err := l.svcCtx.GoogleRepository.RegisterGoogleUser(l.ctx, hash, dni, username, passwordHash)
	if err != nil {
		var e error
		switch {
		case errors.Is(err, repository.ErrCodigoInvalido):
			e = errCodigoInvalido()
		case errors.Is(err, repository.ErrDNIExists):
			e = errGoogle(codes.AlreadyExists, "dni_registrado", MensajeDNIRegistrado)
		case errors.Is(err, repository.ErrEmailExists):
			e = errGoogle(codes.AlreadyExists, "correo_existente", MensajeCorreoExistente)
		case errors.Is(err, repository.ErrIdentidadVinculada):
			e = errGoogle(codes.AlreadyExists, "ya_vinculado", "Esta cuenta de Google ya está registrada. Inicia sesión con Google.")
		case errors.Is(err, repository.ErrDNIDistinto):
			e = errGoogle(codes.InvalidArgument, "dni_distinto", "Tu registro pendiente se inició con otro DNI. Escribe el mismo DNI para continuar.")
		case errors.Is(err, repository.ErrUsernameExists):
			e = status.Error(codes.Aborted, "no se pudo crear la cuenta; inténtalo nuevamente")
		default:
			l.Errorf("registro con Google: no se pudo crear la cuenta: %v", err)
			e = status.Error(codes.Internal, "no se pudo crear la cuenta")
		}
		evento(l.ctx, l.svcCtx, "", "google_registro", "RECHAZADO", motivoDe(e))
		return nil, e
	}
	token, _, err := l.svcCtx.JWTManager.GenerateAccessToken(reg.UsuarioID, reg.Username, repository.RolSolicitante)
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo generar el token")
	}
	return &auth.GoogleRegisterResponse{UserId: reg.UsuarioID, Dni: reg.DNI, Email: reg.Email, ProfileToken: token}, nil
}
