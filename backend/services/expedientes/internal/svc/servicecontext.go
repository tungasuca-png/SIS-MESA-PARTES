package svc

import (
	"strings"

	"context"
	"derivaciones/derivacionesclient"
	"expedientes/internal/outbox"
	"log"
	"time"

	"auth/authclient"
	"expedientes/internal/config"
	"expedientes/internal/repository"
	"expedientes/internal/security"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config               config.Config
	DB                   *pgxpool.Pool
	ExpedienteRepository *repository.ExpedienteRepository
	JWTValidator         *security.JWTValidator

	// AuthClient consulta Auth.HasPermission (Paso 13B) — el permiso real
	// se combina con las reglas de negocio existentes (ownership/área/
	// estado), nunca las reemplaza.
	AuthClient authclient.Auth

	// A + B: nil si el publicador está deshabilitado (Avisar es seguro con nil).
	Publicador   *outbox.Publicador
	PublicadorDB *pgxpool.Pool
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

	authClient := authclient.NewAuth(zrpc.MustNewClient(c.AuthRpc))
	sc := &ServiceContext{
		Config:               c,
		DB:                   db,
		ExpedienteRepository: repository.NewExpedienteRepository(db),
		JWTValidator:         security.NewJWTValidator(c.JWTSecret),
		AuthClient:           authClient,
	}
	p := c.Publicador
	if !p.Habilitado {
		log.Println("ATENCIÓN: publicador del outbox DESHABILITADO: los movimientos quedarán pendientes de proyectar")
		return sc
	}
	// D4: fail-fast si falta cualquier credencial del publicador.
	if strings.TrimSpace(p.ClientID) == "" || strings.TrimSpace(p.ClientSecret) == "" ||
		strings.TrimSpace(p.Postgres.User) == "" || strings.TrimSpace(p.Postgres.Password) == "" ||
		strings.TrimSpace(p.DerivacionesRpc.Target) == "" {
		log.Fatalf("Publicador habilitado sin credenciales completas (EXPEDIENTES_PUBLICADOR_CLIENT_ID/SECRET, EXPEDIENTES_PUBLICADOR_DB_PASSWORD, DerivacionesRpc): el servicio no arranca")
	}
	cfgPub := outbox.Config{
		Lote:          max(p.Lote, 1),
		Lease:         time.Duration(max(p.LeaseSegundos, 15)) * time.Second,
		Intervalo:     time.Duration(max(p.IntervaloMs, 100)) * time.Millisecond,
		MaxIntentos:   max(p.MaxIntentos, 1),
		TimeoutRPC:    time.Duration(max(p.TimeoutRPCMs, 500)) * time.Millisecond,
		RetencionDias: p.RetencionDias,
	}
	// Fail-fast: con un lease fuera de rango el publicador arrancaría sin
	// publicar nunca (la base rechaza el reclamo o Ciclo no procesa nada).
	aviso, err := cfgPub.Validar()
	if err != nil {
		log.Fatalf("Configuración del publicador inválida: %v", err)
	}
	if aviso != "" {
		log.Printf("ATENCIÓN publicador: %s", aviso)
	}
	sc.PublicadorDB = abrirPool(p.Postgres, 3)
	sc.Publicador = outbox.New(sc.PublicadorDB,
		derivacionesclient.NewDerivaciones(zrpc.MustNewClient(p.DerivacionesRpc)),
		outbox.NewTokensAuth(authClient, p.ClientID, p.ClientSecret),
		cfgPub)
	log.Println("Publicador del outbox habilitado")
	return sc
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
