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
	// A + B: secreto EXCLUSIVO de la audiencia derivaciones (publicador de
	// Expedientes). Distinto de JWTSecret y de ServiceJWTSecret. Vacío = esa
	// audiencia queda deshabilitada.
	ServiceJWTSecretDerivaciones string `json:",optional"`
	// Acceso con Google (OpenID Connect) solo para SOLICITANTES. Sin
	// ClientID/ClientSecret/RedirectURL el acceso con Google responde
	// "no configurado" y nada más cambia.
	Google GoogleConfig `json:",optional"`
	// Registro manual de SOLICITANTES con verificación del correo. Sin
	// SMTP.Servidor y SMTP.Remitente el registro manual responde "no
	// disponible" y nada más cambia.
	RegistroCorreo RegistroCorreoConfig `json:",optional"`
}

// RegistroCorreoConfig: límites del registro manual. Los valores en cero
// toman el valor por defecto (ver logic/registrocorreo.go).
type RegistroCorreoConfig struct {
	DuracionCodigo      time.Duration `json:",optional"` // vigencia del código (10m)
	MaxIntentos         int           `json:",optional"` // intentos por código (5)
	EsperaReenvio       time.Duration `json:",optional"` // entre envíos (60s)
	MaxEnvios           int           `json:",optional"` // envíos por registro (5)
	MaxRegistrosPorHora int           `json:",optional"` // registros por correo y hora (5)
	SMTP                SMTPConfig    `json:",optional"`
}

// SMTPConfig: proveedor de correo, desde backend/.env (SMTP_*). Nunca
// credenciales en el código ni en el YAML.
type SMTPConfig struct {
	Servidor        string `json:",optional"` // host:puerto
	Usuario         string `json:",optional"`
	Clave           string `json:",optional"`
	Remitente       string `json:",optional"` // dirección del remitente
	NombreRemitente string `json:",optional"`
	// Seguridad: "starttls" (por defecto, puerto 587), "tls" (465) o
	// "ninguna" (solo servidores locales de prueba, p. ej. Mailpit).
	Seguridad string `json:",optional"`
}

// GoogleConfig: valores desde backend/.env (GOOGLE_*). Issuer vacío = Google.
type GoogleConfig struct {
	ClientID     string `json:",optional"`
	ClientSecret string `json:",optional"`
	RedirectURL  string `json:",optional"`
	Issuer       string `json:",optional"`
}
