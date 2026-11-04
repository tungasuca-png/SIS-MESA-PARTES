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

// usuariosListoParaRegistro: Usuarios debe poder crear el perfil del
// registro público (base disponible y migración 003 aplicada) ANTES de que
// Auth cree o prepare nada (registro manual y Google). Comprobación
// funcional (Usuarios consulta la columna dni_declarado), no de conectividad.
// Si no está listo: 503 genérico, sin detalles internos y sin crear nada.
func usuariosListoParaRegistro(ctx context.Context, svcCtx *svc.ServiceContext, log logx.Logger, clave string) error {
	r, err := svcCtx.UsuariosClient.PublicRegistrationReady(ctx, &usuariosclient.PublicRegistrationReadyRequest{})
	if err == nil && r.GetReady() {
		return nil
	}
	if err != nil {
		log.Errorf("registro público: no se pudo consultar a Usuarios (%v)", status.Code(err))
	} else {
		log.Errorf("registro público: Usuarios no está preparado para crear perfiles")
	}
	return status.Error(codes.Unavailable, clave+": El registro no está disponible en este momento. Inténtalo más tarde.")
}

// MensajeCodigoEnviado: igual exista o no el correo (no revela cuentas).
const MensajeCodigoEnviado = "Si el correo puede registrarse, te enviamos un código de verificación. " +
	"Revisa también la carpeta de correo no deseado."

type RegistroCorreoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegistroCorreoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegistroCorreoLogic {
	return &RegistroCorreoLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// RegistroCorreo: formulario del registro manual. Auth valida todo, guarda
// el registro pendiente y envía el código; aquí no se crea nada. El rol no
// se recibe (siempre SOLICITANTE).
func (l *RegistroCorreoLogic) RegistroCorreo(req *types.RegistroCorreoRequest) (*types.RegistroCorreoResponse, error) {
	// Sin Usuarios listo no se guarda el registro ni se envía el código.
	if err := usuariosListoParaRegistro(l.ctx, l.svcCtx, l.Logger, "registro_no_disponible"); err != nil {
		return nil, err
	}
	r, err := l.svcCtx.AuthClient.EmailRegister(l.ctx, &authclient.EmailRegisterRequest{
		Email:           req.Correo,
		Password:        req.Clave,
		Dni:             req.Dni,
		Nombres:         req.Nombres,
		ApellidoPaterno: req.ApellidoPaterno,
		ApellidoMaterno: req.ApellidoMaterno,
		AcceptTerms:     req.AceptaTerminos,
		TermsVersion:    req.VersionTerminos,
	})
	if err != nil {
		return nil, err
	}
	return &types.RegistroCorreoResponse{
		RegistroId: r.GetRegistrationId(),
		ExpiraEn:   r.GetExpiresIn(),
		ReenviarEn: r.GetResendIn(),
		Mensaje:    MensajeCodigoEnviado,
	}, nil
}
