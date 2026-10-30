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
	// Tokens de sistemas consumidores (client credentials), emitidos por Auth
	// con una clave propia. Sin ServiceJWTSecret solo se aceptan usuarios.
	ServiceJWTSecret     string `json:",optional"`
	ServiceTokenIssuer   string `json:",optional"`
	ServiceTokenAudience string `json:",optional"`

	// Excel: importación/exportación de estudiantes (opcional).
	Excel ExcelConfig `json:",optional"`

	// Datos: límites del motor universal de importación/exportación (opcional).
	Datos DatosConfig `json:",optional"`
}

// ExcelConfig: límites de la importación/exportación Excel de estudiantes.
// Un valor ausente o 0 usa el predeterminado (ver logic.limitesExcel).
type ExcelConfig struct {
	// MaxBytes: tamaño máximo del .xlsx importado (predeterminado 1 MB; el
	// Gateway además limita el cuerpo HTTP con su MaxBytes).
	MaxBytes int64 `json:",optional"`
	// MaxFilasImportacion: estudiantes por archivo (predeterminado 2000).
	MaxFilasImportacion int `json:",optional"`
	// MaxFilasExportacion: estudiantes por exportación (predeterminado 10000).
	MaxFilasExportacion int `json:",optional"`
}

// DatosConfig: límites del motor de datos. 0 = predeterminado del motor
// (3 MB, 5000 registros) y 10000 registros por exportación.
type DatosConfig struct {
	MaxBytes            int64 `json:",optional"`
	MaxFilas            int   `json:",optional"`
	MaxFilasExportacion int   `json:",optional"`
}
