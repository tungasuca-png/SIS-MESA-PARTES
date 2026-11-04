package logic

import (
	"context"
	"errors"
	"strings"
	"time"

	"auth/auth"
	"auth/internal/correo"
	"auth/internal/repository"
	"auth/internal/security"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type EmailRegisterLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEmailRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EmailRegisterLogic {
	return &EmailRegisterLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// EmailRegister: formulario del registro manual. No crea la cuenta: guarda
// un registro pendiente y envía el código. Si el correo ya tiene cuenta, la
// respuesta es idéntica, pero el correo recibido es un aviso (sin código).
// El DNI no se comprueba aquí (se comprueba al verificar, después de que la
// persona demuestre que controla el correo): así esta respuesta no revela
// qué DNI tienen cuenta.
func (l *EmailRegisterLogic) EmailRegister(in *auth.EmailRegisterRequest) (*auth.EmailRegisterResponse, error) {
	if err := registroDisponible(l.svcCtx); err != nil {
		return nil, err
	}
	email, err := normalizarCorreo(in.GetEmail())
	if err != nil {
		return nil, err
	}
	dni, err := normalizeDNI(in.GetDni())
	if err != nil {
		return nil, errRegistro(codes.InvalidArgument, "datos_invalidos", "El DNI debe tener 8 dígitos numéricos.")
	}
	nombres, paterno, materno := espacios(in.GetNombres()), espacios(in.GetApellidoPaterno()), espacios(in.GetApellidoMaterno())
	switch {
	case nombres == "" || paterno == "":
		return nil, errRegistro(codes.InvalidArgument, "datos_invalidos", "Nombres y apellido paterno son obligatorios.")
	case largo(nombres, 100) || largo(strings.TrimSpace(paterno+" "+materno), 100):
		return nil, errRegistro(codes.InvalidArgument, "datos_invalidos", "Nombres y apellidos no pueden superar los 100 caracteres.")
	}
	if err := validarPassword(in.GetPassword()); err != nil {
		return nil, errRegistro(codes.InvalidArgument, "clave_invalida", "La contraseña debe tener entre 8 caracteres y 72 bytes.")
	}
	version := strings.TrimSpace(in.GetTermsVersion())
	if !in.GetAcceptTerms() || version == "" || largo(version, 40) {
		return nil, errRegistro(codes.InvalidArgument, "terminos", "Debes aceptar los términos y la política de privacidad.")
	}

	lim := limitesDe(l.svcCtx.Config.RegistroCorreo)
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(in.GetPassword()), bcrypt.DefaultCost)
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo procesar la contraseña")
	}
	id, err := security.NuevoIDRegistro()
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo preparar el registro")
	}
	codigo, err := security.GenerarCodigoVerificacion()
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo generar el código")
	}
	sinCodigo, err := security.GenerarCodigoVerificacion()
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo generar el código")
	}
	codigos := l.svcCtx.CodigosVerificacion
	nuevo := repository.NuevoRegistroCorreo{
		ID: id, Email: email, DNI: dni, PasswordHash: string(passwordHash),
		Nombres: nombres, ApellidoPaterno: paterno, ApellidoMaterno: materno,
		HashCodigo: codigos.Hash(id, codigo), HashSinCodigo: codigos.Hash(id, "x"+sinCodigo),
		TerminosVersion: version, ExpiraEn: time.Now().Add(lim.duracion),
	}
	var errCorreo error
	err = l.svcCtx.RegistroCorreoRepository.Crear(l.ctx, nuevo, lim.porHora, func(existente bool) error {
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
				l.Errorf("registro con correo: no se pudo guardar el registro: %v", err)
			}
		}
		eventoRegistro(l.ctx, l.svcCtx, "", "correo_registro", "RECHAZADO", motivoRegistro(e))
		return nil, e
	}
	eventoRegistro(l.ctx, l.svcCtx, "", "correo_registro", "OK", "")
	return &auth.EmailRegisterResponse{
		RegistrationId: id,
		ExpiresIn:      segundos(lim.duracion),
		ResendIn:       segundos(lim.espera),
	}, nil
}
