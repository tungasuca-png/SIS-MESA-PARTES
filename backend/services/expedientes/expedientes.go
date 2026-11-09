package main

import (
	"context"
	"flag"
	"fmt"

	"auth/auth"
	derivacionespb "derivaciones/derivaciones"
	"expedientes/expedientes"
	"expedientes/internal/config"
	"expedientes/internal/interceptor"
	"expedientes/internal/server"
	"expedientes/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/proc"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/expedientes.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())

	// Antes de crear los clientes: go-zero registra en el cliente gRPC la
	// solicitud de cada llamada fallida. Client credentials lleva el
	// client_secret del publicador; AplicarMovimiento, el contenido del
	// movimiento (motivo, actor). Se conserva la línea de error/duración, sin
	// el contenido (mismo criterio que psicologia.go y gateway/internal/logsafe).
	zrpc.DontLogClientContentForMethod(auth.Auth_IssueServiceToken_FullMethodName)
	zrpc.DontLogClientContentForMethod(derivacionespb.Derivaciones_AplicarMovimiento_FullMethodName)

	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		expedientes.RegisterExpedientesServer(grpcServer, server.NewExpedientesServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	s.AddUnaryInterceptors(interceptor.AuthenticationInterceptor(ctx.JWTValidator))

	// A + B: el publicador del outbox corre en este mismo proceso y se detiene
	// con él (los leases en curso vencen y otra instancia los retoma).
	if ctx.Publicador != nil {
		publicadorCtx, detener := context.WithCancel(context.Background())
		proc.AddShutdownListener(detener)
		defer detener()
		go ctx.Publicador.Correr(publicadorCtx)
	}
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
