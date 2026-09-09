package svc

import (
	"context"
	"fmt"
	"log"
	"time"

	"derivaciones/internal/config"
	"derivaciones/internal/repository"
	"derivaciones/internal/security"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ServiceContext struct {
	Config               config.Config
	DB                   *pgxpool.Pool
	DerivacionRepository *repository.DerivacionRepository
	JWTValidator         *security.JWTValidator
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

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.Postgres.User,
		c.Postgres.Password,
		c.Postgres.Host,
		c.Postgres.Port,
		c.Postgres.Database,
		c.Postgres.SSLMode,
	)

	dbConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatalf("Error al parsear la configuración de PostgreSQL: %v", err)
	}

	dbConfig.MaxConns = 10
	dbConfig.MinConns = 2
	dbConfig.MaxConnLifetime = 1 * time.Hour
	dbConfig.MaxConnIdleTime = 30 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := pgxpool.NewWithConfig(ctx, dbConfig)
	if err != nil {
		log.Fatalf("Error al inicializar el pool de conexiones: %v", err)
	}

	if err := db.Ping(ctx); err != nil {
		db.Close()
		log.Fatalf("No se pudo conectar a PostgreSQL (Ping fallido): %v", err)
	}

	log.Println("Conexión exitosa a PostgreSQL")

	return &ServiceContext{
		Config:               c,
		DB:                   db,
		DerivacionRepository: repository.NewDerivacionRepository(db),
		JWTValidator:         security.NewJWTValidator(c.JWTSecret),
	}
}

// Close cierra correctamente el pool de PostgreSQL.
func (sc *ServiceContext) Close() {
	if sc.DB != nil {
		sc.DB.Close()
		log.Println("Pool de conexiones de PostgreSQL cerrado correctamente")
	}
}
