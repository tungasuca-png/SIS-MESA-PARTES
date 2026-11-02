package logic

import (
	"context"
	"regexp"
	"strings"
	"unicode/utf8"

	"gateway/internal/svc"
	"gateway/internal/types"

	"auth/authclient"
	"usuarios/usuariosclient"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var reDNIRegistro = regexp.MustCompile(`^[0-9]{8}$`)

type GoogleRegistroLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGoogleRegistroLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GoogleRegistroLogic {
	return &GoogleRegistroLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// GoogleRegistro: formulario obligatorio del autorregistro con Google.
// Auth y Usuarios tienen bases separadas (no hay transacción común), así que:
//  1. Auth crea la cuenta SOLICITANTE pendiente (o retoma la existente) y
//     rechaza un DNI/correo ya registrados sin crear nada.
//  2. Usuarios crea el perfil (el DNI vuelve a validarse con su índice
//     único). Si allí el DNI ya existe, se COMPENSA: Auth elimina la cuenta
//     pendiente (nunca tuvo sesión). Ante otro fallo, la cuenta queda
//     pendiente y el mismo formulario se puede reenviar (idempotente).
//  3. Solo entonces Auth emite la sesión normal.
func (l *GoogleRegistroLogic) GoogleRegistro(req *types.GoogleRegistroRequest) (*types.LoginResponse, error) {
	nombres := strings.Join(strings.Fields(req.Nombres), " ")
	paterno := strings.Join(strings.Fields(req.ApellidoPaterno), " ")
	materno := strings.Join(strings.Fields(req.ApellidoMaterno), " ")
	apellidos := strings.TrimSpace(paterno + " " + materno)
	dni := strings.TrimSpace(req.Dni)
	switch {
	case strings.TrimSpace(req.Codigo) == "":
		return nil, status.Error(codes.InvalidArgument, "google_codigo_invalido: El enlace de registro venció. Vuelve a intentarlo.")
	case nombres == "" || paterno == "":
		return nil, status.Error(codes.InvalidArgument, "google_datos_invalidos: Nombres y apellido paterno son obligatorios.")
	case utf8.RuneCountInString(nombres) > 100 || utf8.RuneCountInString(apellidos) > 100:
		return nil, status.Error(codes.InvalidArgument, "google_datos_invalidos: Nombres y apellidos no pueden superar los 100 caracteres.")
	case !reDNIRegistro.MatchString(dni):
		return nil, status.Error(codes.InvalidArgument, "google_datos_invalidos: El DNI debe tener 8 dígitos numéricos.")
	case len(strings.TrimSpace(req.Telefono)) > 20:
		return nil, status.Error(codes.InvalidArgument, "google_datos_invalidos: El teléfono no puede superar los 20 caracteres.")
	}

	reg, err := l.svcCtx.AuthClient.GoogleRegister(l.ctx, &authclient.GoogleRegisterRequest{LoginCode: req.Codigo, Dni: dni})
	if err != nil {
		return nil, err
	}
	// Perfil con el DNI y el correo que Auth confirmó (no los del navegador),
	// usando un token del nuevo usuario que nunca llega al navegador.
	ctxPerfil := withAuthorization(l.ctx, "Bearer "+reg.ProfileToken)
	_, err = l.svcCtx.UsuariosClient.CreateMyProfile(ctxPerfil, &usuariosclient.CreateMyProfileRequest{
		Nombres: nombres, Apellidos: apellidos, Dni: reg.Dni, Telefono: strings.TrimSpace(req.Telefono),
		Correo: reg.Email, Direccion: strings.TrimSpace(req.Direccion),
	})
	switch status.Code(err) {
	case codes.OK:
	case codes.AlreadyExists:
		// DNI registrado en Usuarios aunque no en Auth: compensación.
		if _, cerr := l.svcCtx.AuthClient.GoogleCancelRegistration(l.ctx, &authclient.GoogleRegistrationRequest{LoginCode: req.Codigo}); cerr != nil {
			l.Errorf("registro con Google: no se pudo compensar la cuenta pendiente: %v", status.Code(cerr))
		}
		return nil, status.Error(codes.AlreadyExists, "google_dni_registrado: "+MensajeDNIRegistradoGoogle)
	case codes.InvalidArgument:
		return nil, status.Error(codes.InvalidArgument, "google_datos_invalidos: "+status.Convert(err).Message())
	default:
		l.Errorf("registro con Google: perfil no sincronizado (%v); la cuenta queda pendiente", status.Code(err))
		return nil, status.Error(codes.Unavailable, "google_perfil_pendiente: No se pudo completar tu registro. Inténtalo nuevamente.")
	}

	fin, err := l.svcCtx.AuthClient.GoogleCompleteRegistration(l.ctx, &authclient.GoogleRegistrationRequest{LoginCode: req.Codigo})
	if err != nil {
		return nil, err
	}
	return sesionDTO(fin.GetSession()), nil
}
