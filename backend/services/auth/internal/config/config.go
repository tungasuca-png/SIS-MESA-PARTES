package config

import (
	"time"

	"github.com/zeromicro/go-zero/zrpc"
)

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
	Postgres             PostgresConfig
	JWTSecret            string
	AccessTokenDuration  time.Duration
	RefreshTokenDuration time.Duration
	// Tokens de sistemas consumidores (client credentials). Sin
	// ServiceJWTSecret quedan deshabilitados; debe ser distinto de JWTSecret.
	ServiceJWTSecret     string        `json:",optional"`
	ServiceTokenIssuer   string        `json:",optional"`
	ServiceTokenAudience string        `json:",optional"`
	ServiceTokenDuration time.Duration `json:",optional"`
	// Acceso con Google (OpenID Connect) solo para SOLICITANTES. Sin
	// ClientID/ClientSecret/RedirectURL el acceso con Google responde
	// "no configurado" y nada más cambia.
	Google GoogleConfig `json:",optional"`
}

// GoogleConfig: valores desde backend/.env (GOOGLE_*). Issuer vacío = Google.
type GoogleConfig struct {
	ClientID     string `json:",optional"`
	ClientSecret string `json:",optional"`
	RedirectURL  string `json:",optional"`
	Issuer       string `json:",optional"`
}
