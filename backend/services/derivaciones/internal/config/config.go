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

	// A + B: tokens de servicio del publicador de Expedientes. Secreto
	// EXCLUSIVO de la audiencia derivaciones (SERVICE_JWT_SECRET_DERIVACIONES,
	// distinto de JWT_SECRET y del secreto de Estudiantes). Vacío = el RPC
	// AplicarMovimiento queda deshabilitado.
	ServiceJWTSecret     string `json:",optional"`
	ServiceTokenIssuer   string `json:",optional"`
	ServiceTokenAudience string `json:",optional"`

	// Conexión del consumidor (rol derivaciones_proyector): único rol que
	// puede insertar en movimientos_proyeccion. Obligatoria si hay secreto.
	Proyector PostgresConfig `json:",optional"`
}
