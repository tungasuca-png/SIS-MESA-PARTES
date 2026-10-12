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

	// JWTSecret debe ser el mismo secreto de Auth Service: Estudiantes no
	// emite tokens, solo valida la firma del access token que el Gateway
	// reenvía (cuando exista esa integración — FASE 5B).
	JWTSecret string

	// AuthRpc: Auth.HasPermission. Configurado y wireado en el ServiceContext
	// desde esta fase, pero NINGÚN RPC de logic lo invoca todavía — dejarlo
	// listo para que FASE 5B solo agregue authorization.RequirePermission(...)
	// en cada logic, sin tocar config/svc otra vez (mismo criterio ya usado
	// al crear Psicología).
	AuthRpc zrpc.RpcClientConf
}
