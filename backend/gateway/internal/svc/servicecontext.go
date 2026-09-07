package svc

import (
	"auth/authclient"
	"documentos/documentosclient"
	"expedientes/expedientesclient"
	"gateway/internal/config"
	"usuarios/usuariosclient"

	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
)

type ServiceContext struct {
	Config            config.Config
	AuthClient        authclient.Auth
	ExpedientesClient expedientesclient.Expedientes
	UsuariosClient    usuariosclient.Usuarios
	DocumentosClient  documentosclient.Documentos
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
	}
}
