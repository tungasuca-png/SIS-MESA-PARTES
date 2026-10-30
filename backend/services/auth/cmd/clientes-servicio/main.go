// clientes-servicio: administración de sistemas consumidores (client
// credentials) desde el servidor. Llama a los RPC de Auth, que solo escucha
// en 127.0.0.1, con el access token de un usuario que tenga el permiso
// "integraciones.manage" (variable AUTH_ADMIN_TOKEN; nunca como argumento,
// para que no quede en el historial de la consola).
//
//	go run ./cmd/clientes-servicio crear -nombre "Sistema de Biblioteca" -permisos estudiantes.basic.view
//	go run ./cmd/clientes-servicio listar
//	go run ./cmd/clientes-servicio revocar -id cli_0123...
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"auth/authclient"

	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc/metadata"
)

func main() {
	if len(os.Args) < 2 {
		uso()
	}
	fs := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	target := fs.String("auth", "127.0.0.1:8080", "dirección del servicio Auth")
	nombre := fs.String("nombre", "", "nombre del sistema (crear)")
	permisos := fs.String("permisos", "", "permisos separados por comas (crear)")
	id := fs.String("id", "", "id o client_id del sistema (revocar)")
	_ = fs.Parse(os.Args[2:])

	token := strings.TrimSpace(os.Getenv("AUTH_ADMIN_TOKEN"))
	if token == "" {
		fallar("defina AUTH_ADMIN_TOKEN con el access token de un usuario con integraciones.manage")
	}
	client := authclient.NewAuth(zrpc.MustNewClient(zrpc.RpcClientConf{Target: *target, Timeout: 10000}))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)

	switch os.Args[1] {
	case "crear":
		resp, err := client.CreateServiceClient(ctx, &authclient.CreateServiceClientRequest{
			Nombre:   *nombre,
			Permisos: strings.Split(*permisos, ","),
		})
		comprobar(err)
		imprimir(resp.Client)
		fmt.Println()
		fmt.Println("client_secret (se muestra UNA sola vez; guárdelo en el gestor de secretos del sistema consumidor):")
		fmt.Println(resp.ClientSecret)
	case "listar":
		resp, err := client.ListServiceClients(ctx, &authclient.ListServiceClientsRequest{})
		comprobar(err)
		imprimir(resp.Clients)
	case "revocar":
		resp, err := client.RevokeServiceClient(ctx, &authclient.RevokeServiceClientRequest{Id: *id})
		comprobar(err)
		imprimir(resp.Client)
	default:
		uso()
	}
}

func imprimir(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}

func comprobar(err error) {
	if err != nil {
		fallar(err.Error())
	}
}

func fallar(msg string) {
	fmt.Fprintln(os.Stderr, "error:", msg)
	os.Exit(1)
}

func uso() {
	fallar("uso: clientes-servicio crear -nombre NOMBRE -permisos p1,p2 | listar | revocar -id ID")
}
