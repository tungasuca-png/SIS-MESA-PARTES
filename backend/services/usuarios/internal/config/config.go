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
	// Usuarios Service no emite tokens, solo valida la firma del access
	// token que el Gateway reenvía como metadata "authorization".
	JWTSecret string

	// AuthRpc: destino de Auth Service para consultar Auth.HasPermission
	// (Paso 16B) — mismo patrón zrpc ya usado por Expedientes/Documentos/
	// Derivaciones.
	AuthRpc zrpc.RpcClientConf
}
