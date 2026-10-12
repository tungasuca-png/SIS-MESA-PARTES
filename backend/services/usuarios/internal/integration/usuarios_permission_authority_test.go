package integration

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"auth/authclient"
	"usuarios/internal/config"
	"usuarios/internal/interceptor"
	"usuarios/internal/repository"
	"usuarios/internal/security"
	"usuarios/internal/server"
	"usuarios/internal/svc"
	"usuarios/usuarios"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// grantsAuthClient decide SOLO por el user_id (permisos concedidos a ese
// usuario), nunca por el nombre del rol del JWT: modela "Auth calcula los
// permisos del usuario a partir de lo que ADMIN configuró en su rol".
type grantsAuthClient struct {
	*scriptedAuthClient
	grants map[string]map[string]bool
}

func (c *grantsAuthClient) HasPermission(_ context.Context, in *authclient.HasPermissionRequest, _ ...grpc.CallOption) (*authclient.HasPermissionResponse, error) {
	return &authclient.HasPermissionResponse{Allowed: c.grants[in.UserId][in.Permission]}, nil
}

// TestUsuariosPermisoEsLaAutoridad prueba que listar/buscar (usuarios.view)
// y administrar perfiles (usuarios.change_role) dependen únicamente del
// permiso que Auth reporta, no del nombre del rol del JWT.
func TestUsuariosPermisoEsLaAutoridad(t *testing.T) {
	if strings.ToLower(os.Getenv("INTEGRATION_TEST")) != "true" {
		t.Skip("set INTEGRATION_TEST=true to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, integrationDatabaseURL(t))
	if err != nil {
		t.Fatalf("create PostgreSQL pool: %v", err)
	}
	defer pool.Close()

	testID := time.Now().UnixNano()
	directorVer := testUUID(testID, 1)      // DIRECTOR con usuarios.view
	directorSin := testUUID(testID, 2)      // DIRECTOR sin permisos de usuarios
	directorRol := testUUID(testID, 3)      // DIRECTOR con usuarios.change_role
	adminSinPermisos := testUUID(testID, 4) // ADMIN sin ningún permiso de usuarios
	perfilID := testUUID(testID, 5)
	defer cleanup(t, pool, perfilID)

	authClient := &grantsAuthClient{
		scriptedAuthClient: &scriptedAuthClient{},
		grants: map[string]map[string]bool{
			directorVer: {"usuarios.view": true},
			directorRol: {"usuarios.change_role": true},
		},
	}
	svcCtx := &svc.ServiceContext{
		Config:            config.Config{JWTSecret: testSecret},
		DB:                pool,
		UsuarioRepository: repository.NewUsuarioRepository(pool),
		JWTValidator:      security.NewJWTValidator(testSecret),
		AuthClient:        authClient,
	}
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(interceptor.AuthenticationInterceptor(svcCtx.JWTValidator)))
	usuarios.RegisterUsuariosServer(grpcServer, server.NewUsuariosServer(svcCtx))
	go func() { _ = grpcServer.Serve(listener) }()
	defer grpcServer.Stop()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	client := usuarios.NewUsuariosClient(conn)

	t.Run("Caso1_DirectorConUsuariosView_PuedeListarYBuscar", func(t *testing.T) {
		c := authContext(directorVer, "DIRECTOR")
		if _, err := client.ListUsuarios(c, &usuarios.ListUsuariosRequest{}); err != nil {
			t.Fatalf("DIRECTOR con usuarios.view debía poder listar: %v", err)
		}
		if _, err := client.SearchUsuarios(c, &usuarios.SearchUsuariosRequest{Query: "abc"}); err != nil {
			t.Fatalf("DIRECTOR con usuarios.view debía poder buscar: %v", err)
		}
	})

	t.Run("Caso2_DirectorSinUsuariosView_403", func(t *testing.T) {
		c := authContext(directorSin, "DIRECTOR")
		if _, err := client.ListUsuarios(c, &usuarios.ListUsuariosRequest{}); status.Code(err) != codes.PermissionDenied {
			t.Fatalf("list: se esperaba PermissionDenied, got %v", err)
		}
		if _, err := client.SearchUsuarios(c, &usuarios.SearchUsuariosRequest{Query: "abc"}); status.Code(err) != codes.PermissionDenied {
			t.Fatalf("search: se esperaba PermissionDenied, got %v", err)
		}
	})

	t.Run("Caso3_DirectorConChangeRole_PuedeAdministrarPerfil", func(t *testing.T) {
		c := authContext(directorRol, "DIRECTOR")
		resp, err := client.UpsertUsuario(c, &usuarios.UpsertUsuarioRequest{
			Id: perfilID, Nombres: "TEST Perm", Apellidos: "TEST Autoridad", TipoUsuario: "DOCENTE",
		})
		if err != nil {
			t.Fatalf("DIRECTOR con usuarios.change_role debía poder: %v", err)
		}
		if resp.Usuario.Id != perfilID {
			t.Fatalf("id inesperado %s", resp.Usuario.Id)
		}
		if _, err := client.UpdateUsuarioRole(c, &usuarios.UpdateUsuarioRoleRequest{Id: perfilID, TipoUsuario: "AUXILIAR"}); err != nil {
			t.Fatalf("UpdateUsuarioRole con usuarios.change_role: %v", err)
		}
	})

	t.Run("Caso4_DirectorSinChangeRole_403_SinMutacion", func(t *testing.T) {
		// directorVer tiene usuarios.view pero NO usuarios.change_role.
		c := authContext(directorVer, "DIRECTOR")
		if _, err := client.UpsertUsuario(c, &usuarios.UpsertUsuarioRequest{
			Id: perfilID, Nombres: "HACK", Apellidos: "HACK", TipoUsuario: "ADMIN",
		}); status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied, got %v", err)
		}
		if _, err := client.UpdateUsuarioRole(c, &usuarios.UpdateUsuarioRoleRequest{Id: perfilID, TipoUsuario: "ADMIN"}); status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied en UpdateUsuarioRole, got %v", err)
		}
		got, err := client.GetUsuario(authContext(directorRol, "DIRECTOR"), &usuarios.GetUsuarioRequest{Id: perfilID})
		if err == nil && (got.Usuario.Nombres != "TEST Perm" || got.Usuario.TipoUsuario != "AUXILIAR") {
			t.Fatalf("el perfil no debía mutar: %+v", got.Usuario)
		}
	})

	t.Run("Caso5_AdminSinPermiso_NoEsAutorizadoPorSerAdmin", func(t *testing.T) {
		c := authContext(adminSinPermisos, "ADMIN")
		if _, err := client.ListUsuarios(c, &usuarios.ListUsuariosRequest{}); status.Code(err) != codes.PermissionDenied {
			t.Fatalf("list: ADMIN sin usuarios.view debía recibir PermissionDenied, got %v", err)
		}
		if _, err := client.SearchUsuarios(c, &usuarios.SearchUsuariosRequest{Query: "abc"}); status.Code(err) != codes.PermissionDenied {
			t.Fatalf("search: se esperaba PermissionDenied, got %v", err)
		}
		if _, err := client.UpsertUsuario(c, &usuarios.UpsertUsuarioRequest{
			Id: perfilID, Nombres: "X", Apellidos: "Y", TipoUsuario: "ADMIN",
		}); status.Code(err) != codes.PermissionDenied {
			t.Fatalf("upsert: se esperaba PermissionDenied, got %v", err)
		}
	})
}
