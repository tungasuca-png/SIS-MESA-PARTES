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

const (
	ResultadoSesion            = "SESION"
	ResultadoRegistroPendiente = "REGISTRO_PENDIENTE"
)

type GoogleExchangeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGoogleExchangeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GoogleExchangeLogic {
	return &GoogleExchangeLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// GoogleExchange: un código LOGIN se consume (una sola vez) y se canjea por la
// sesión normal; un código REGISTRO NO se consume: devuelve los datos que
// Google verificó para el formulario obligatorio (el registro lo consume).
func (l *GoogleExchangeLogic) GoogleExchange(in *auth.GoogleExchangeRequest) (*auth.GoogleExchangeResponse, error) {
	hash, ok := hashCodigo(in.GetLoginCode())
	if !ok {
		return nil, errCodigoInvalido()
	}
	codigo, err := l.svcCtx.GoogleRepository.ConsumeCode(l.ctx, hash, repository.PropositoLogin)
	if err == nil {
		// Se vuelven a comprobar estado, bloqueo y rol al emitir la sesión.
		user, role, err := cuentaSolicitanteDisponible(l.ctx, l.svcCtx, codigo.UsuarioID)
		if err != nil {
			evento(l.ctx, l.svcCtx, codigo.UsuarioID, "google_login", "RECHAZADO", motivoDe(err))
			return nil, err
		}
		sesion, err := emitirSesion(l.ctx, l.svcCtx, user, role)
		if err != nil {
			return nil, err
		}
		return &auth.GoogleExchangeResponse{Result: ResultadoSesion, Session: sesion}, nil
	}
	if !errors.Is(err, repository.ErrCodigoInvalido) {
		return nil, status.Error(codes.Internal, "no se pudo procesar el acceso con Google")
	}
	codigo, err = l.svcCtx.GoogleRepository.PeekCode(l.ctx, hash, repository.PropositoRegistro)
	if err != nil {
		if errors.Is(err, repository.ErrCodigoInvalido) {
			return nil, errCodigoInvalido()
		}
		return nil, status.Error(codes.Internal, "no se pudo procesar el acceso con Google")
	}
	return &auth.GoogleExchangeResponse{
		Result: ResultadoRegistroPendiente, Email: codigo.Email, Nombres: codigo.Nombres, Apellidos: codigo.Apellidos,
	}, nil
}
