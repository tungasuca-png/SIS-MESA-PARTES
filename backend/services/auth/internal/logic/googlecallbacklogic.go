package logic

import (
	"context"
	"errors"
	"strings"

	"auth/auth"
	"auth/internal/repository"
	"auth/internal/security"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	ResultadoLogin     = "LOGIN"
	ResultadoRegistro  = "REGISTRO"
	ResultadoVinculado = "VINCULADO"
)

type GoogleCallbackLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGoogleCallbackLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GoogleCallbackLogic {
	return &GoogleCallbackLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// GoogleCallback canjea el código de Google (con el verificador PKCE), valida
// el ID token (firma, issuer, audience, expiración y nonce) y decide el
// flujo. Nunca emite la sesión aquí: devuelve un código de un solo uso.
func (l *GoogleCallbackLogic) GoogleCallback(in *auth.GoogleCallbackRequest) (*auth.GoogleCallbackResponse, error) {
	if in == nil || strings.TrimSpace(in.Code) == "" || in.Verifier == "" || in.Nonce == "" {
		return nil, errGoogle(codes.InvalidArgument, "solicitud_invalida", "La respuesta de Google está incompleta. Vuelve a intentarlo.")
	}
	ident, err := l.svcCtx.Google.Exchange(l.ctx, in.Code, in.Verifier, in.Nonce)
	if err != nil {
		e := errProveedor(err)
		evento(l.ctx, l.svcCtx, "", "google_callback", "RECHAZADO", motivoDe(e))
		return nil, e
	}
	if !ident.EmailVerified || ident.Email == "" {
		evento(l.ctx, l.svcCtx, "", "google_callback", "RECHAZADO", "email_no_verificado")
		return nil, errGoogle(codes.FailedPrecondition, "email_no_verificado",
			"Google no confirmó tu correo. Verifícalo en tu cuenta de Google e inténtalo de nuevo.")
	}
	if in.LinkCode != "" {
		return l.vincular(ident, in.LinkCode)
	}

	identidad, err := l.svcCtx.GoogleRepository.FindIdentity(l.ctx, ident.Subject)
	switch {
	case err == nil && identidad.RegistroCompleto:
		// A. Login de una identidad ya vinculada.
		user, _, err := cuentaSolicitanteDisponible(l.ctx, l.svcCtx, identidad.UsuarioID)
		if err != nil {
			evento(l.ctx, l.svcCtx, identidad.UsuarioID, "google_login", "RECHAZADO", motivoDe(err))
			return nil, err
		}
		codigo, err := nuevoCodigo(l.ctx, l.svcCtx, repository.NuevoCodigo{Purpose: repository.PropositoLogin, UsuarioID: user.ID}, duracionCodigoLogin)
		if err != nil {
			return nil, err
		}
		_ = l.svcCtx.GoogleRepository.TouchIdentity(l.ctx, identidad.ID)
		evento(l.ctx, l.svcCtx, user.ID, "google_login", "OK", "")
		return &auth.GoogleCallbackResponse{Result: ResultadoLogin, LoginCode: codigo}, nil

	case err == nil:
		// B'. Registro pendiente (cuenta creada, perfil sin confirmar): retoma
		// el formulario con los mismos datos.
		if _, _, err := cuentaSolicitanteDisponible(l.ctx, l.svcCtx, identidad.UsuarioID); err != nil {
			evento(l.ctx, l.svcCtx, identidad.UsuarioID, "google_registro", "RECHAZADO", motivoDe(err))
			return nil, err
		}
		codigo, err := nuevoCodigo(l.ctx, l.svcCtx, repository.NuevoCodigo{
			Purpose: repository.PropositoRegistro, UsuarioID: identidad.UsuarioID, Subject: identidad.Subject,
			Email: identidad.Email, Nombres: ident.GivenName, Apellidos: ident.FamilyName,
		}, duracionCodigoRegistro)
		if err != nil {
			return nil, err
		}
		return &auth.GoogleCallbackResponse{Result: ResultadoRegistro, LoginCode: codigo}, nil

	case errors.Is(err, repository.ErrIdentidadNoEncontrada):
		// B. Identidad nueva. Un correo ya registrado NO se vincula ni se
		// fusiona: la persona entra con su cuenta y vincula desde su perfil.
		if _, err := l.svcCtx.UserRepository.FindByEmail(l.ctx, ident.Email); err == nil {
			evento(l.ctx, l.svcCtx, "", "google_registro", "RECHAZADO", "correo_existente")
			return nil, errGoogle(codes.AlreadyExists, "correo_existente", MensajeCorreoExistente)
		} else if !errors.Is(err, repository.ErrUserNotFound) {
			return nil, status.Error(codes.Internal, "no se pudo procesar el acceso con Google")
		}
		codigo, err := nuevoCodigo(l.ctx, l.svcCtx, repository.NuevoCodigo{
			Purpose: repository.PropositoRegistro, Subject: ident.Subject, Email: ident.Email,
			Nombres: ident.GivenName, Apellidos: ident.FamilyName,
		}, duracionCodigoRegistro)
		if err != nil {
			return nil, err
		}
		evento(l.ctx, l.svcCtx, "", "google_registro_iniciado", "OK", "")
		return &auth.GoogleCallbackResponse{Result: ResultadoRegistro, LoginCode: codigo}, nil
	}
	return nil, status.Error(codes.Internal, "no se pudo procesar el acceso con Google")
}

// vincular: C. Vinculación explícita autorizada por un código VINCULACION
// (emitido tras reautenticar con la clave).
func (l *GoogleCallbackLogic) vincular(ident *security.GoogleIdentity, linkCode string) (*auth.GoogleCallbackResponse, error) {
	hash, ok := hashCodigo(linkCode)
	if !ok {
		return nil, errCodigoInvalido()
	}
	usuarioID, err := l.svcCtx.GoogleRepository.LinkIdentity(l.ctx, hash, ident.Subject, ident.Email)
	if err != nil {
		var e error
		switch {
		case errors.Is(err, repository.ErrCodigoInvalido):
			e = errGoogle(codes.Unauthenticated, "codigo_invalido", "La solicitud de vinculación venció. Vuelve a intentarlo desde tu perfil.")
		case errors.Is(err, repository.ErrIdentidadVinculada):
			e = errGoogle(codes.AlreadyExists, "ya_vinculado", "Esta cuenta de Google ya está vinculada a otra cuenta del sistema.")
		case errors.Is(err, repository.ErrCuentaYaVinculada):
			e = errGoogle(codes.AlreadyExists, "cuenta_ya_vinculada", "Tu cuenta ya tiene una cuenta de Google vinculada.")
		case errors.Is(err, repository.ErrRolNoPermitido), errors.Is(err, repository.ErrCuentaNoDisponible):
			e = errGoogle(codes.PermissionDenied, "cuenta_no_disponible", MensajeCuentaNoDisponible)
		default:
			e = status.Error(codes.Internal, "no se pudo vincular la cuenta de Google")
		}
		evento(l.ctx, l.svcCtx, "", "google_vinculacion", "RECHAZADO", motivoDe(e))
		return nil, e
	}
	evento(l.ctx, l.svcCtx, usuarioID, "google_vinculacion", "OK", "")
	return &auth.GoogleCallbackResponse{Result: ResultadoVinculado}, nil
}
