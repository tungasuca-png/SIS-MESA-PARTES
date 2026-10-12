package main

import (
	"flag"
	"fmt"

	"estudiantes/estudiantes"
	"estudiantes/internal/config"
	"estudiantes/internal/interceptor"
	"estudiantes/internal/server"
	"estudiantes/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/estudiantes.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	// conf.UseEnv() es obligatorio para que ${JWT_SECRET} y
	// ${ESTUDIANTES_DB_PASSWORD} del YAML se resuelvan contra variables de
	// entorno y no queden como texto literal.
	conf.MustLoad(*configFile, &c, conf.UseEnv())
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		estudiantes.RegisterEstudiantesServer(grpcServer, server.NewEstudiantesServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	s.AddUnaryInterceptors(interceptor.AuthenticationInterceptor(ctx.JWTValidator))
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
