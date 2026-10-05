package svc

import (
	"context"
	"fmt"
	"log"
	"time"

	"maestros/internal/config"
	"maestros/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ServiceContext struct {
	Config               config.Config
	DB                   *pgxpool.Pool
	PersonaRepository    *repository.PersonaRepository
	EstudianteRepository *repository.EstudianteRepository
}

func NewServiceContext(c config.Config) *ServiceContext {
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
		log.Fatalf(
			"Error al parsear configuración PostgreSQL: %v",
			err,
		)
	}

	dbConfig.MaxConns = 10
	dbConfig.MinConns = 2
	dbConfig.MaxConnLifetime = time.Hour
	dbConfig.MaxConnIdleTime = 30 * time.Minute

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	db, err := pgxpool.NewWithConfig(ctx, dbConfig)
	if err != nil {
		log.Fatalf(
			"Error al inicializar PostgreSQL: %v",
			err,
		)
	}

	if err := db.Ping(ctx); err != nil {
		db.Close()

		log.Fatalf(
			"No se pudo conectar a maestros_db: %v",
			err,
		)
	}

	log.Println("Conexión exitosa a maestros_db")

	return &ServiceContext{
		Config: c,
		DB:     db,

		PersonaRepository: repository.NewPersonaRepository(
			db,
		),

		EstudianteRepository: repository.NewEstudianteRepository(
			db,
		),
	}
}

func (sc *ServiceContext) Close() {
	if sc.DB != nil {
		sc.DB.Close()
	}
}
