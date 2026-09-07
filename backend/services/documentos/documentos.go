package main

import (
	"flag"
	"fmt"

	"documentos/documentos"
	"documentos/internal/config"
	"documentos/internal/interceptor"
	"documentos/internal/server"
	"documentos/internal/svc"
	"documentos/internal/validation"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/documentos.yaml", "the config file")

// maxMsgSize deja margen sobre validation.MaxTamanoDocumento para el resto
// de campos del mensaje protobuf (metadata, overhead de codificación).
const maxMsgSize = validation.MaxTamanoDocumento + 64*1024

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		documentos.RegisterDocumentosServer(grpcServer, server.NewDocumentosServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	s.AddOptions(grpc.MaxRecvMsgSize(maxMsgSize), grpc.MaxSendMsgSize(maxMsgSize))
	s.AddUnaryInterceptors(interceptor.AuthenticationInterceptor(ctx.JWTValidator))
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
