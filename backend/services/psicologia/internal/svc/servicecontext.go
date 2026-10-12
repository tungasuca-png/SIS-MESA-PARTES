package svc

import (
	"context"
	"fmt"
	"log"
	"time"

	"auth/authclient"
	"psicologia/internal/clinicalcrypto"
	"psicologia/internal/config"
	"psicologia/internal/repository"
	"psicologia/internal/security"
	"usuarios/usuariosclient"

	"estudiantes/estudiantesclient"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zeromicro/go-zero/zrpc"
)

// EstudianteVerifier es el punto de extensión para comprobar que un
// estudiante_id existe en la fuente oficial de estudiantes.
//
// FASE 5C: implementado sobre Estudiantes Service real
// (estudianteServiceVerifier, ver estudianteverifier.go), inyectado en
// NewServiceContext. Si algún test de integración necesita volver a dejarlo
// en nil (p. ej. para probar el comportamiento previo a esta fase) o
// sustituirlo por un doble, sigue siendo válido: OpenExpediente solo salta la
// verificación cuando el campo es nil.
type EstudianteVerifier interface {
	Exists(ctx context.Context, estudianteID string) (bool, error)
}

// EstudianteDirectory obtiene el nombre del estudiante para el PDF de la Hoja
// de Atención (misma implementación real que EstudianteVerifier, con el JWT
// del usuario). nil = el PDF no puede generarse (fail-closed).
type EstudianteDirectory interface {
	NombreCompleto(ctx context.Context, estudianteID string) (string, error)
}

type ServiceContext struct {
	Config         config.Config
	DB             *pgxpool.Pool
	ExpedienteRepo *repository.ExpedienteRepository
	AtencionRepo   *repository.AtencionRepository
	AuditoriaRepo  *repository.AuditoriaRepository
	JWTValidator   *security.JWTValidator
	AuthClient     authclient.Auth
	UsuariosClient usuariosclient.Usuarios
	Estudiantes    EstudianteVerifier
	EstudiantesDir EstudianteDirectory
}

func NewServiceContext(c config.Config) *ServiceContext {
	// Nunca se imprime la contraseña, solo su longitud (patrón existente).
	log.Printf(
		"CONFIG → Host=%q Port=%d User=%q Database=%q SSLMode=%q PasswordLength=%d",
		c.Postgres.Host, c.Postgres.Port, c.Postgres.User, c.Postgres.Database,
		c.Postgres.SSLMode, len(c.Postgres.Password),
	)

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.Postgres.User, c.Postgres.Password, c.Postgres.Host, c.Postgres.Port,
		c.Postgres.Database, c.Postgres.SSLMode,
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

	// Cifrado del contenido clínico (FASE 6N-3, C1). Sin clave válida, o si la
	// clave no descifra su canario, el servicio NO inicia: no existe modo en
	// texto claro. Los mensajes nunca incluyen la clave.
	cifrador, err := clinicalcrypto.New(c.ClinicalData.EncryptionKey, c.ClinicalData.KeyID, c.ClinicalData.PreviousKeys)
	if err != nil {
		db.Close()
		log.Fatalf("Psicología no inicia: %v (definir CLINICAL_DATA_ENCRYPTION_KEY y CLINICAL_DATA_KEY_ID)", err)
	}
	if err := repository.VerificarCanario(ctx, db, cifrador); err != nil {
		db.Close()
		log.Fatalf("Psicología no inicia: %v", err)
	}
	log.Printf("Cifrado clínico listo (clave activa %q)", cifrador.ActiveKeyID())

	estudiantes := NewEstudianteVerifier(estudiantesclient.NewEstudiantes(zrpc.MustNewClient(c.EstudiantesRpc)))
	return &ServiceContext{
		Config:         c,
		DB:             db,
		ExpedienteRepo: repository.NewExpedienteRepository(db, cifrador),
		AtencionRepo:   repository.NewAtencionRepository(db, cifrador),
		AuditoriaRepo:  repository.NewAuditoriaRepository(db),
		JWTValidator:   security.NewJWTValidator(c.JWTSecret),
		AuthClient:     authclient.NewAuth(zrpc.MustNewClient(c.AuthRpc)),
		UsuariosClient: usuariosclient.NewUsuarios(zrpc.MustNewClient(c.UsuariosRpc)),
		Estudiantes:    estudiantes,
		EstudiantesDir: estudiantes,
	}
}

// Close cierra correctamente el pool de PostgreSQL.
func (sc *ServiceContext) Close() {
	if sc.DB != nil {
		sc.DB.Close()
		log.Println("Pool de conexiones de PostgreSQL cerrado correctamente")
	}
}
