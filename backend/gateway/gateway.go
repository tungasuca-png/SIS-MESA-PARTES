// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package main

import (
	"flag"
	"fmt"

	"gateway/internal/config"
	"gateway/internal/handler"
	"gateway/internal/logsafe"
	"gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/gateway-api.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())

	// Antes de crear los clientes gRPC: oculta el contenido (no los errores) de las
	// llamadas con contraseñas, tokens o datos clínicos.
	logsafe.ProtectSensitiveRPCLogs()

	server := rest.MustNewServer(
		c.RestConf,
		rest.WithCors("http://localhost:5173"),
		rest.WithCorsHeaders(
			"Content-Type",
			"Authorization",
		),
	)

	defer server.Stop()

	// Reemplaza al LogHandler estándar (Middlewares.Log=false en el YAML).
	server.Use(logsafe.AccessLog)

	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
