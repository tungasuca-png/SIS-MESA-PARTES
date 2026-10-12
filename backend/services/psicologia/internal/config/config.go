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

// ClinicalDataConfig: clave de cifrado del contenido clínico (FASE 6N-3, C1).
// Solo desde variables de entorno (CLINICAL_DATA_*), nunca en el YAML ni en
// el repositorio. Los campos son "optional" para que la ausencia se reporte
// con un mensaje propio (sin valores) y el servicio no inicie.
type ClinicalDataConfig struct {
	EncryptionKey string `json:",optional"`
	KeyID         string `json:",optional"`
	PreviousKeys  string `json:",optional"`
}

type Config struct {
	zrpc.RpcServerConf
	Postgres PostgresConfig

	ClinicalData ClinicalDataConfig

	// JWTSecret debe ser el mismo secreto de Auth Service: Psicología no emite
	// tokens, solo valida la firma del access token que el Gateway reenvía.
	JWTSecret string

	// AuthRpc: Auth.HasPermission, la única autoridad de permisos.
	AuthRpc zrpc.RpcClientConf

	// UsuariosRpc: validar que responsable_id corresponde a un usuario real
	// (Usuarios.GetUsuariosBasic). No se copia ningún dato de usuarios.
	UsuariosRpc zrpc.RpcClientConf

	// EstudiantesRpc (FASE 5C): fuente real de estudiantes, usada por
	// svc.EstudianteVerifier para confirmar que estudiante_id existe antes de
	// abrir un expediente.
	EstudiantesRpc zrpc.RpcClientConf
}
