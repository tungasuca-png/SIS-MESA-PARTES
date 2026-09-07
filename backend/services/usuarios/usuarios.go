package main

import (
	"flag"
	"fmt"

	"usuarios/internal/config"
	"usuarios/internal/interceptor"
	"usuarios/internal/server"
	"usuarios/internal/svc"
	"usuarios/usuarios"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/usuarios.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	// conf.UseEnv() es obligatorio para que ${JWT_SECRET} del YAML se
	// resuelva contra la variable de entorno y no quede como texto literal.
	conf.MustLoad(*configFile, &c, conf.UseEnv())
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		usuarios.RegisterUsuariosServer(grpcServer, server.NewUsuariosServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	s.AddUnaryInterceptors(interceptor.AuthenticationInterceptor(ctx.JWTValidator))
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
