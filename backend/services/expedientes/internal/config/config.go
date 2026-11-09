package config

import "github.com/zeromicro/go-zero/zrpc"

type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
}

type Config struct {
	zrpc.RpcServerConf
	Postgres PostgresConfig

	// JWTSecret debe ser el mismo secreto configurado en Auth Service:
	// Expedientes no emite ni renueva tokens, solo valida la firma del
	// access token que el Gateway reenvía como metadata "authorization".
	JWTSecret string

	// AuthRpc: destino de Auth Service para consultar Auth.HasPermission
	// (mismo patrón zrpc ya usado por el Gateway para este mismo servicio).
	AuthRpc zrpc.RpcClientConf

	// A + B: publicador del outbox (ver internal/outbox). Habilitado y sin
	// credenciales = el servicio NO arranca (D4).
	Publicador PublicadorConfig `json:",optional"`
}

type PublicadorConfig struct {
	Habilitado bool `json:",default=false"`
	// Conexión propia con el rol expedientes_publicador.
	Postgres        PostgresConfig     `json:",optional"`
	DerivacionesRpc zrpc.RpcClientConf `json:",optional"`
	// Client credentials (audiencia derivaciones): solo por entorno.
	ClientID      string `json:",optional"`
	ClientSecret  string `json:",optional"`
	Lote          int    `json:",default=10"`
	LeaseSegundos int    `json:",default=60"`
	IntervaloMs   int    `json:",default=2000"`
	MaxIntentos   int    `json:",default=20"`
	TimeoutRPCMs  int    `json:",default=3000"`
	RetencionDias int    `json:",default=30"`
}
