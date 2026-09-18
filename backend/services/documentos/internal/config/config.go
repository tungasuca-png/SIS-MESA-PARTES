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
	// Documentos Service no emite tokens, solo valida la firma del access
	// token que el Gateway reenvía como metadata "authorization".
	JWTSecret string

	// AuthRpc: destino de Auth Service para consultar Auth.HasPermission
	// (Paso 14B) — mismo patrón zrpc ya usado por Expedientes y el Gateway.
	AuthRpc zrpc.RpcClientConf

	// ExpedientesRpc: destino de Expedientes Service (Paso 19B) — usado
	// SOLO para resolver el ownership real de "documentos.view_own" en
	// GetDocumento (Expediente.SolicitanteID), mismo cliente generado que
	// ya usa el Gateway (expedientesclient), sin crear una arquitectura
	// nueva.
	ExpedientesRpc zrpc.RpcClientConf
}
