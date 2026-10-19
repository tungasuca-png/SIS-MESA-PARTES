package main

import (
	"flag"
	"fmt"

	"psicologia/internal/config"
	"psicologia/internal/interceptor"
	"psicologia/internal/server"
	"psicologia/internal/svc"
	"psicologia/psicologia"

	"estudiantes/estudiantes"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/psicologia.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	// conf.UseEnv() es obligatorio para que ${JWT_SECRET} y
	// ${PSICOLOGIA_DB_PASSWORD} del YAML se resuelvan contra variables de
	// entorno y no queden como texto literal.
	conf.MustLoad(*configFile, &c, conf.UseEnv())

	// Antes de crear el cliente de Estudiantes: go-zero registra en el
	// cliente gRPC la solicitud de cada llamada fallida y la respuesta de las
	// lentas. GetEstudiante devuelve nombres y datos académicos de menores:
	// se conserva la línea de error/duración, sin el contenido (mismo
	// criterio que gateway/internal/logsafe).
	zrpc.DontLogClientContentForMethod(estudiantes.Estudiantes_GetEstudiante_FullMethodName)

	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		psicologia.RegisterPsicologiaServer(grpcServer, server.NewPsicologiaServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	s.AddUnaryInterceptors(interceptor.AuthenticationInterceptor(ctx.JWTValidator))
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
