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
	// Derivaciones no emite ni renueva tokens, solo valida la firma del
	// access token que el Gateway reenvía como metadata "authorization".
	JWTSecret string

	// AuthRpc: destino de Auth Service para consultar Auth.HasPermission
	// (Paso 15B) — mismo patrón zrpc ya usado por Expedientes y Documentos.
	AuthRpc zrpc.RpcClientConf

	// ExpedientesRpc (Paso 20A): destino de Expedientes Service — usado
	// SOLO para resolver el ownership/área real de ListDerivaciones cuando
	// se indica expediente_id (Expediente.SolicitanteID / AreaActual),
	// mismo cliente generado que ya usa el Gateway y Documentos (Paso
	// 19B), sin crear una arquitectura nueva.
	ExpedientesRpc zrpc.RpcClientConf
}
