package logic

import (
	"context"

	"gateway/internal/svc"
	"gateway/internal/types"

	"auth/authclient"
	"usuarios/usuariosclient"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	MensajeCuentaActivada = "Tu cuenta fue activada. Inicia sesión con tu correo y tu contraseña. " +
		"Cuando la institución verifique tu identidad también podrás ingresar con tu DNI."
	mensajeDNIRegistrado = "Este DNI ya tiene una cuenta registrada. Inicia sesión con tus credenciales o comunícate con la institución."
)

type RegistroVerificarLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegistroVerificarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegistroVerificarLogic {
	return &RegistroVerificarLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// RegistroVerificar activa la cuenta. Auth y Usuarios tienen bases
// separadas (no hay transacción común), así que, como en el registro con
// Google:
//  1. Auth comprueba el código y crea la cuenta SOLICITANTE con el correo
//     verificado (o retoma la ya creada: reintento idempotente).
//  2. Usuarios crea el perfil con los datos que Auth guardó (no los del
//     navegador), con un token del nuevo usuario que nunca llega al
//     navegador. Si el DNI ya pertenece a OTRO perfil, se COMPENSA: Auth
//     elimina la cuenta recién creada (nunca tuvo sesión). Ante otro fallo
//     la cuenta queda pendiente y el mismo código vuelve a servir.
//  3. Auth marca el registro como completado: desde ahí se puede iniciar
//     sesión y el código deja de servir.
func (l *RegistroVerificarLogic) RegistroVerificar(req *types.RegistroVerificarRequest) (*types.RegistroVerificarResponse, error) {
	reg := &authclient.EmailRegistrationRequest{RegistrationId: req.RegistroId, Code: req.Codigo}
	// Antes de que Auth cree la cuenta: Usuarios debe poder crear el perfil.
	if err := usuariosListoParaRegistro(l.ctx, l.svcCtx, l.Logger, "registro_no_disponible"); err != nil {
		return nil, err
	}
	v, err := l.svcCtx.AuthClient.EmailVerify(l.ctx, &authclient.EmailVerifyRequest{RegistrationId: req.RegistroId, Code: req.Codigo})
	if err != nil {
		return nil, err
	}
	ctxPerfil := withAuthorization(l.ctx, "Bearer "+v.GetProfileToken())
	_, err = l.svcCtx.UsuariosClient.CreateMyProfile(ctxPerfil, &usuariosclient.CreateMyProfileRequest{
		Nombres: v.GetNombres(), Apellidos: v.GetApellidos(), DniDeclarado: v.GetDni(), Correo: v.GetEmail(),
	})
	switch status.Code(err) {
	case codes.OK:
	case codes.AlreadyExists:
		// ¿El perfil es de este mismo usuario (dos verificaciones casi
		// simultáneas)? Entonces no hay conflicto: no se compensa.
		_, gerr := l.svcCtx.UsuariosClient.GetUsuario(ctxPerfil, &usuariosclient.GetUsuarioRequest{Id: v.GetUserId()})
		switch status.Code(gerr) {
		case codes.OK:
		case codes.NotFound:
			l.compensar(reg)
			return nil, status.Error(codes.AlreadyExists, "registro_dni_registrado: "+mensajeDNIRegistrado)
		default:
			l.Errorf("registro con correo: no se pudo comprobar el perfil (%v); la cuenta queda pendiente", status.Code(gerr))
			return nil, errPerfilPendiente()
		}
	case codes.InvalidArgument:
		// Los datos los validó Auth; si Usuarios aun así los rechaza, el
		// registro no puede continuar con ellos.
		l.compensar(reg)
		return nil, status.Error(codes.InvalidArgument,
			"registro_datos_invalidos: No se pudieron registrar tus datos ("+status.Convert(err).Message()+"). Vuelve a registrarte.")
	default:
		l.Errorf("registro con correo: perfil no sincronizado (%v); la cuenta queda pendiente", status.Code(err))
		return nil, errPerfilPendiente()
	}

	if _, err := l.svcCtx.AuthClient.EmailCompleteRegistration(l.ctx, reg); err != nil {
		return nil, err
	}
	return &types.RegistroVerificarResponse{Mensaje: MensajeCuentaActivada, Correo: v.GetEmail()}, nil
}

func (l *RegistroVerificarLogic) compensar(reg *authclient.EmailRegistrationRequest) {
	if _, err := l.svcCtx.AuthClient.EmailCancelRegistration(l.ctx, reg); err != nil {
		l.Errorf("registro con correo: no se pudo compensar la cuenta pendiente: %v", status.Code(err))
	}
}

func errPerfilPendiente() error {
	return status.Error(codes.Unavailable, "registro_perfil_pendiente: No se pudo completar tu registro. Vuelve a ingresar el código en unos minutos.")
}
