package svc

import (
	"context"
	"log"
	"time"

	"auth/authclient"
	"derivaciones/internal/config"
	"derivaciones/internal/repository"
	"derivaciones/internal/security"
	"expedientes/expedientesclient"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config               config.Config
	DB                   *pgxpool.Pool
	DerivacionRepository *repository.DerivacionRepository
	JWTValidator         *security.JWTValidator

	// A + B: consumidor de movimientos (pool separado con derivaciones_proyector).
	ProyectorDB          *pgxpool.Pool
	ProyeccionRepository *repository.ProyeccionRepository
	ServiceValidator     *security.ServiceTokenValidator

	// AuthClient consulta Auth.HasPermission (Paso 15B) — el permiso real
	// se combina con las reglas de negocio existentes de
	// internal/authorization, nunca las reemplaza.
	AuthClient authclient.Auth

	// ExpedientesClient (Paso 20A): resuelve el ownership/área REAL del
	// expediente (Expediente.SolicitanteID / AreaActual) para
	// ListDerivaciones cuando se indica expediente_id — mismo cliente
	// generado que ya usa el Gateway y Documentos, sin arquitectura nueva.
	ExpedientesClient expedientesclient.Expedientes
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

	dsn := postgresDSN(c.Postgres.User, c.Postgres.Password, c.Postgres.Host, c.Postgres.Port, c.Postgres.Database, c.Postgres.SSLMode)

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

	serviceValidator, err := security.NewServiceTokenValidator(c.ServiceJWTSecret, c.JWTSecret, c.ServiceTokenIssuer, c.ServiceTokenAudience)
	if err != nil {
		log.Fatalf("Configuración de tokens de servicio inválida: %v", err)
	}
	var proyectorDB *pgxpool.Pool
	var proyeccion *repository.ProyeccionRepository
	if serviceValidator != nil {
		if c.Proyector.User == "" {
			log.Fatalf("ServiceJWTSecret configurado sin la conexión Proyector (rol derivaciones_proyector)")
		}
		proyectorDB = abrirPool(c.Proyector, 4)
		proyeccion = repository.NewProyeccionRepository(proyectorDB)
		log.Println("AplicarMovimiento habilitado (audiencia derivaciones)")
	} else {
		log.Println("AplicarMovimiento deshabilitado: sin ServiceJWTSecret")
	}

	return &ServiceContext{
		Config:               c,
		DB:                   db,
		DerivacionRepository: repository.NewDerivacionRepository(db),
		JWTValidator:         security.NewJWTValidator(c.JWTSecret),
		ProyectorDB:          proyectorDB,
		ProyeccionRepository: proyeccion,
		ServiceValidator:     serviceValidator,
		AuthClient:           authclient.NewAuth(zrpc.MustNewClient(c.AuthRpc)),
		ExpedientesClient:    expedientesclient.NewExpedientes(zrpc.MustNewClient(c.ExpedientesRpc)),
	}
}

// abrirPool abre un pool adicional (falla el arranque si no conecta).
func abrirPool(p config.PostgresConfig, maxConns int32) *pgxpool.Pool {
	dsn := postgresDSN(p.User, p.Password, p.Host, p.Port, p.Database, p.SSLMode)
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatalf("Error al parsear la conexión de %s: %v", p.User, err)
	}
	cfg.MaxConns = maxConns
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err == nil {
		err = pool.Ping(ctx)
	}
	if err != nil {
		log.Fatalf("No se pudo conectar a PostgreSQL como %s: %v", p.User, err)
	}
	return pool
}

// Close cierra correctamente el pool de PostgreSQL.
func (sc *ServiceContext) Close() {
	if sc.DB != nil {
		sc.DB.Close()
		log.Println("Pool de conexiones de PostgreSQL cerrado correctamente")
	}
}
