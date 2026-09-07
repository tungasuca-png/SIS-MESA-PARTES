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
}
