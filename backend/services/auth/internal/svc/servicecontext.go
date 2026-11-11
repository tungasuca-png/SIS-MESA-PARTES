package svc

import (
	"context"
	"log"
	"strings"
	"time"

	"auth/internal/authorization"
	"auth/internal/config"
	"auth/internal/correo"
	"auth/internal/repository"
	"auth/internal/security"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ServiceContext struct {
	Config                   config.Config
	DB                       *pgxpool.Pool
	UserRepository           *repository.UserRepository
	RoleRepository           *repository.RoleRepository
	PermissionRepository     *repository.PermissionRepository
	RolePermissionRepository *repository.RolePermissionRepository
	RefreshTokenRepository   *repository.RefreshTokenRepository
	AuthorizationService     *authorization.AuthorizationService
	JWTManager               *security.JWTManager
	// Sistemas consumidores (client credentials). ServiceTokenIssuer es nil
	// si ServiceJWTSecret no está configurado: IssueServiceToken responde
	// Unavailable y nada más cambia.
	ServiceClientRepository *repository.ServiceClientRepository
	ServiceTokenIssuer      *security.ServiceTokenIssuer
	// A + B: emisores por audiencia (estudiantes = ServiceTokenIssuer).
	EmisoresServicio security.EmisoresServicio
	// Acceso con Google (solo SOLICITANTES).
	GoogleRepository *repository.GoogleRepository
	Google           security.GoogleProvider
	// Registro manual con verificación del correo (solo SOLICITANTES). Correo
	// no configurado = el registro manual responde "no disponible".
	RegistroCorreoRepository *repository.RegistroCorreoRepository
	Correo                   correo.Enviador
	CodigosVerificacion      *security.CodigosVerificacion
	// Identidad declarada de los solicitantes públicos (verificación por
	// Secretaría, permiso solicitantes.identidad.verify).
	IdentidadRepository *repository.IdentidadRepository
}

func NewServiceContext(c config.Config) *ServiceContext {

	log.Printf(
		"CONFIG → Host=%q Port=%d User=%q Database=%q SSLMode=%q PasswordLength=%d",
		c.Postgres.Host,
		c.Postgres.Port,
		c.Postgres.User,
		c.Postgres.Database,
		c.Postgres.SSLMode,
		len(c.Postgres.Password),
	)

	// Construir la cadena de conexión a PostgreSQL
	dsn := postgresDSN(c.Postgres.User, c.Postgres.Password, c.Postgres.Host, c.Postgres.Port, c.Postgres.Database, c.Postgres.SSLMode)

	// Parsear la configuración de PostgreSQL
	dbConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatalf(
			"Error al parsear la configuración de PostgreSQL: %v",
			err,
		)
	}

	// Configuración del pool de conexiones
	dbConfig.MaxConns = 10
	dbConfig.MinConns = 2
	dbConfig.MaxConnLifetime = 1 * time.Hour
	dbConfig.MaxConnIdleTime = 30 * time.Minute

	// Crear contexto con timeout
	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	// Crear pool de conexiones
	db, err := pgxpool.NewWithConfig(ctx, dbConfig)
	if err != nil {
		log.Fatalf(
			"Error al inicializar el pool de conexiones: %v",
			err,
		)
	}

	// Verificar conexión real con PostgreSQL
	if err := db.Ping(ctx); err != nil {
		db.Close()

		log.Fatalf(
			"No se pudo conectar a PostgreSQL (Ping fallido): %v",
			err,
		)
	}

	log.Println("Conexión exitosa a PostgreSQL")

	serviceIssuer, err := security.NewServiceTokenIssuer(c.ServiceJWTSecret, c.JWTSecret, c.ServiceTokenIssuer, c.ServiceTokenAudience, c.ServiceTokenDuration)
	if err != nil {
		db.Close()
		log.Fatalf("Configuración de tokens de servicio inválida: %v", err)
	}
	if serviceIssuer == nil {
		log.Println("Tokens de servicio deshabilitados: SERVICE_JWT_SECRET no está configurado")
	} else {
		log.Println("Tokens de servicio habilitados (client credentials)")
	}
	emisores := security.EmisoresServicio{}
	if serviceIssuer != nil {
		emisores[security.AudienciaEstudiantes] = serviceIssuer
	}
	if strings.TrimSpace(c.ServiceJWTSecretDerivaciones) != "" {
		if strings.TrimSpace(c.ServiceJWTSecretDerivaciones) == strings.TrimSpace(c.ServiceJWTSecret) {
			db.Close()
			log.Fatalf("ServiceJWTSecretDerivaciones debe ser distinto de ServiceJWTSecret (un secreto por audiencia)")
		}
		emisorDeriv, err := security.NewServiceTokenIssuer(c.ServiceJWTSecretDerivaciones, c.JWTSecret, c.ServiceTokenIssuer, security.AudienciaDerivaciones, c.ServiceTokenDuration)
		if err != nil {
			db.Close()
			log.Fatalf("Configuración de tokens de servicio (derivaciones) inválida: %v", err)
		}
		emisores[security.AudienciaDerivaciones] = emisorDeriv
		log.Println("Tokens de servicio habilitados para la audiencia derivaciones")
	}

	google := security.NewGoogleOIDC(security.GoogleConfig{
		ClientID: c.Google.ClientID, ClientSecret: c.Google.ClientSecret,
		RedirectURL: c.Google.RedirectURL, Issuer: c.Google.Issuer,
	})
	if google.Configurado() {
		log.Println("Acceso con Google habilitado (solo SOLICITANTES)")
	} else {
		log.Println("Acceso con Google deshabilitado: GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET o GOOGLE_REDIRECT_URL no configurados")
	}

	enviador, err := correo.NewSMTP(c.RegistroCorreo.SMTP)
	if err != nil {
		db.Close()
		log.Fatalf("Configuración de correo (SMTP_*) inválida: %v", err)
	}
	if enviador.Configurado() {
		log.Println("Registro manual con verificación de correo habilitado (solo SOLICITANTES)")
	} else {
		log.Println("Registro manual deshabilitado: SMTP_SERVIDOR o SMTP_REMITENTE no configurados")
	}
	codigos, err := security.NewCodigosVerificacion(c.JWTSecret)
	if err != nil {
		db.Close()
		log.Fatalf("Códigos de verificación: %v", err)
	}

	return &ServiceContext{
		Config:                   c,
		DB:                       db,
		UserRepository:           repository.NewUserRepository(db),
		RoleRepository:           repository.NewRoleRepository(db),
		PermissionRepository:     repository.NewPermissionRepository(db),
		RolePermissionRepository: repository.NewRolePermissionRepository(db),
		RefreshTokenRepository:   repository.NewRefreshTokenRepository(db),
		AuthorizationService:     authorization.NewAuthorizationService(repository.NewUserRepository(db), repository.NewRoleRepository(db), repository.NewRolePermissionRepository(db)),
		JWTManager:               security.NewJWTManager(c.JWTSecret, c.AccessTokenDuration),
		ServiceClientRepository:  repository.NewServiceClientRepository(db),
		ServiceTokenIssuer:       serviceIssuer,
		EmisoresServicio:         emisores,
		GoogleRepository:         repository.NewGoogleRepository(db),
		Google:                   google,
		RegistroCorreoRepository: repository.NewRegistroCorreoRepository(db),
		Correo:                   enviador,
		CodigosVerificacion:      codigos,
		IdentidadRepository:      repository.NewIdentidadRepository(db),
	}
}

// Close cierra correctamente el pool de PostgreSQL.
func (sc *ServiceContext) Close() {
	if sc.DB != nil {
		sc.DB.Close()
		log.Println("Pool de conexiones de PostgreSQL cerrado correctamente")
	}
}
