package svc

import (
	"auth/authclient"
	"derivaciones/derivacionesclient"
	"documentos/documentosclient"
	"estudiantes/estudiantesclient"
	"expedientes/expedientesclient"
	"gateway/internal/config"
	"psicologia/psicologiaclient"
	"usuarios/usuariosclient"

	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
)

type ServiceContext struct {
	Config             config.Config
	AuthClient         authclient.Auth
	ExpedientesClient  expedientesclient.Expedientes
	UsuariosClient     usuariosclient.Usuarios
	DocumentosClient   documentosclient.Documentos
	DerivacionesClient derivacionesclient.Derivaciones
	PsicologiaClient   psicologiaclient.Psicologia
	EstudiantesClient  estudiantesclient.Estudiantes
}

// documentosMaxMsgSize deja margen sobre el limite de archivo de Documentos
// Service (5 MiB, ver documentos/internal/validation.MaxTamanoDocumento)
// para el resto de campos del mensaje protobuf.
const documentosMaxMsgSize = 5*1024*1024 + 64*1024

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config: c,
		AuthClient: authclient.NewAuth(
			zrpc.MustNewClient(c.AuthRpc),
		),
		ExpedientesClient: expedientesclient.NewExpedientes(
			zrpc.MustNewClient(c.ExpedientesRpc),
		),
		UsuariosClient: usuariosclient.NewUsuarios(
			zrpc.MustNewClient(c.UsuariosRpc),
		),
		DocumentosClient: documentosclient.NewDocumentos(
			zrpc.MustNewClient(c.DocumentosRpc, zrpc.WithDialOption(grpc.WithDefaultCallOptions(
				grpc.MaxCallRecvMsgSize(documentosMaxMsgSize),
				grpc.MaxCallSendMsgSize(documentosMaxMsgSize),
			))),
		),
		DerivacionesClient: derivacionesclient.NewDerivaciones(
			zrpc.MustNewClient(c.DerivacionesRpc),
		),
		PsicologiaClient: psicologiaclient.NewPsicologia(
			zrpc.MustNewClient(c.PsicologiaRpc),
		),
		EstudiantesClient: estudiantesclient.NewEstudiantes(
			zrpc.MustNewClient(c.EstudiantesRpc),
		),
	}
}
