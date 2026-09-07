package svc

import (
	"context"
	"fmt"
	"log"
	"time"

	"auth/internal/authorization"
	"auth/internal/config"
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
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.Postgres.User,
		c.Postgres.Password,
		c.Postgres.Host,
		c.Postgres.Port,
		c.Postgres.Database,
		c.Postgres.SSLMode,
	)

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
	}
}

// Close cierra correctamente el pool de PostgreSQL.
func (sc *ServiceContext) Close() {
	if sc.DB != nil {
		sc.DB.Close()
		log.Println("Pool de conexiones de PostgreSQL cerrado correctamente")
	}
}
