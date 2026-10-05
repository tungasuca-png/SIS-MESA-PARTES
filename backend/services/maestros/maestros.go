package main

import (
	"flag"
	"fmt"

	"maestros/internal/config"
	"maestros/internal/server"
	"maestros/internal/svc"
	"maestros/maestros"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String(
	"f",
	"etc/maestros.yaml",
	"the config file",
)

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	ctx := svc.NewServiceContext(c)
	defer ctx.Close()

	s := zrpc.MustNewServer(
		c.RpcServerConf,
		func(grpcServer *grpc.Server) {
			maestros.RegisterMaestrosServer(
				grpcServer,
				server.NewMaestrosServer(ctx),
			)

			if c.Mode == service.DevMode ||
				c.Mode == service.TestMode {
				reflection.Register(grpcServer)
			}
		},
	)

	defer s.Stop()

	fmt.Printf(
		"Starting maestros rpc server at %s...\n",
		c.ListenOn,
	)

	s.Start()
}
