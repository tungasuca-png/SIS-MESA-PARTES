package svc

import (
	"auth/authclient"
	"expedientes/expedientesclient"
	"gateway/internal/config"
	"usuarios/usuariosclient"

	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config            config.Config
	AuthClient        authclient.Auth
	ExpedientesClient expedientesclient.Expedientes
	UsuariosClient    usuariosclient.Usuarios
}

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
	}
}
