package integration

import (
	"context"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"auth/auth"
	"auth/internal/authorization"
	"auth/internal/config"
	"auth/internal/interceptor"
	"auth/internal/repository"
	"auth/internal/security"
	"auth/internal/server"
	"auth/internal/svc"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// TestLoginPorDNI prueba el login DNI + clave de punta a punta (bcrypt real,
// PostgreSQL real, interceptores de producción): la credencial es el DNI (texto
// de 8 dígitos), nunca el username; los errores no revelan si el DNI existe;
// el JWT sigue identificando al usuario por su id interno.
func TestLoginPorDNI(t *testing.T) {
	if strings.ToLower(os.Getenv("INTEGRATION_TEST")) != "true" {
		t.Skip("set INTEGRATION_TEST=true to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, integrationDatabaseURL(t))
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	userRepo := repository.NewUserRepository(pool)
	roleRepo := repository.NewRoleRepository(pool)
	rpRepo := repository.NewRolePermissionRepository(pool)
	authz := authorization.NewAuthorizationService(userRepo, roleRepo, rpRepo)
	jwtManager := security.NewJWTManager("login-dni-test-secret", time.Hour)
	svcCtx := &svc.ServiceContext{
		Config:                   config.Config{RefreshTokenDuration: 24 * time.Hour},
		DB:                       pool,
		UserRepository:           userRepo,
		RoleRepository:           roleRepo,
		PermissionRepository:     repository.NewPermissionRepository(pool),
		RolePermissionRepository: rpRepo,
		RefreshTokenRepository:   repository.NewRefreshTokenRepository(pool),
		AuthorizationService:     authz,
		JWTManager:               jwtManager,
	}
	policies := interceptor.DefaultMethodPolicies()
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
		interceptor.AuthenticationInterceptor(jwtManager, policies, userRepo),
		interceptor.AuthorizationInterceptor(authz, policies),
	))
	auth.RegisterAuthServer(grpcServer, server.NewAuthServer(svcCtx))
	go func() { _ = grpcServer.Serve(listener) }()
	defer grpcServer.Stop()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	client := auth.NewAuthClient(conn)

	prefix := "dl" + strconv.FormatInt(time.Now().UnixNano()%1e8, 10) + "_"
	const clave = "Clave-Segura-123"
	hash, _ := bcrypt.GenerateFromPassword([]byte(clave), bcrypt.MinCost)
	var userIDs []string
	defer func() {
		c, cc := context.WithTimeout(context.Background(), 10*time.Second)
		defer cc()
		for _, id := range userIDs {
			_, _ = pool.Exec(c, "DELETE FROM refresh_tokens WHERE usuario_id = $1", id)
			_, _ = pool.Exec(c, "DELETE FROM usuario_roles WHERE usuario_id = $1", id)
			_, _ = pool.Exec(c, "DELETE FROM usuarios WHERE id = $1", id)
		}
	}()
	roleID := func(name string) string {
		var id string
		if err := pool.QueryRow(ctx, "SELECT id FROM roles WHERE nombre = $1", name).Scan(&id); err != nil {
			t.Fatalf("rol %s: %v", name, err)
		}
		return id
	}
	mkUser := func(suffix, role string, estado bool) (id, username, dni string) {
		username = prefix + suffix
		dni = nextTestDNI()
		if err := pool.QueryRow(ctx, `INSERT INTO usuarios (username, email, password_hash, estado, dni) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			username, username+"@test.local", string(hash), estado, dni).Scan(&id); err != nil {
			t.Fatalf("crear usuario: %v", err)
		}
		userIDs = append(userIDs, id)
		if _, err := pool.Exec(ctx, "INSERT INTO usuario_roles (usuario_id, rol_id) VALUES ($1,$2)", id, roleID(role)); err != nil {
			t.Fatalf("asignar rol: %v", err)
		}
		return
	}
	bearer := func(token string) context.Context {
		return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
	}

	docenteID, docenteUser, docenteDNI := mkUser("docente", "DOCENTE", true)

	t.Run("LoginValido_JWTSubEsUserID_Rol_Iat_Exp", func(t *testing.T) {
		resp, err := client.Login(ctx, &auth.LoginRequest{Dni: docenteDNI, Password: clave})
		if err != nil {
			t.Fatalf("login: %v", err)
		}
		if resp.AccessToken == "" || resp.RefreshToken == "" || resp.UserId != docenteID || resp.Role != "DOCENTE" {
			t.Fatalf("respuesta inesperada: %+v", resp)
		}
		claims := jwt.MapClaims{}
		if _, _, err := jwt.NewParser().ParseUnverified(resp.AccessToken, claims); err != nil {
			t.Fatalf("parse jwt: %v", err)
		}
		if claims["sub"] != docenteID {
			t.Fatalf("sub debe ser el user_id interno, no el DNI: %v", claims["sub"])
		}
		if claims["sub"] == docenteDNI {
			t.Fatal("el DNI no debe ser el sub")
		}
		if claims["role"] != "DOCENTE" || claims["iat"] == nil || claims["exp"] == nil {
			t.Fatalf("claims incompletos: %v", claims)
		}
		for k, v := range claims {
			if s, ok := v.(string); ok && (s == docenteDNI || s == clave) {
				t.Fatalf("el claim %q expone el DNI o la clave", k)
			}
		}
	})

	t.Run("DNIConEspacios_SeNormaliza", func(t *testing.T) {
		if _, err := client.Login(ctx, &auth.LoginRequest{Dni: "  " + docenteDNI + " ", Password: clave}); err != nil {
			t.Fatalf("un DNI con espacios alrededor debe normalizarse: %v", err)
		}
	})

	t.Run("ErroresNoRevelanSiElDNIExiste", func(t *testing.T) {
		_, errClaveMala := client.Login(ctx, &auth.LoginRequest{Dni: docenteDNI, Password: "otra-clave-x"})
		_, errNoExiste := client.Login(ctx, &auth.LoginRequest{Dni: "99999990", Password: "otra-clave-x"})
		if status.Code(errClaveMala) != codes.Unauthenticated || status.Code(errNoExiste) != codes.Unauthenticated {
			t.Fatalf("se esperaba Unauthenticated en ambos: %v / %v", errClaveMala, errNoExiste)
		}
		if status.Convert(errClaveMala).Message() != status.Convert(errNoExiste).Message() {
			t.Fatalf("el mensaje no debe distinguir DNI inexistente de clave incorrecta: %q vs %q",
				status.Convert(errClaveMala).Message(), status.Convert(errNoExiste).Message())
		}
	})

	t.Run("DNIInvalido_ErrorDeValidacion", func(t *testing.T) {
		for _, dni := range []string{"123", "1234ABCD", "123456789", "1234 678", "", "   "} {
			_, err := client.Login(ctx, &auth.LoginRequest{Dni: dni, Password: clave})
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("DNI %q: se esperaba InvalidArgument, got %v", dni, err)
			}
		}
	})

	t.Run("UsernameYEmail_NoSonCredenciales", func(t *testing.T) {
		for _, id := range []string{docenteUser, docenteUser + "@test.local"} {
			if _, err := client.Login(ctx, &auth.LoginRequest{Dni: id, Password: clave}); err == nil {
				t.Fatalf("%q no debe autenticar", id)
			}
		}
	})

	t.Run("UsuarioInactivo_Rechazado", func(t *testing.T) {
		_, _, dni := mkUser("inactivo", "DOCENTE", false)
		_, err := client.Login(ctx, &auth.LoginRequest{Dni: dni, Password: clave})
		if status.Code(err) != codes.Unauthenticated || status.Convert(err).Message() != "credenciales inválidas" {
			t.Fatalf("usuario inactivo: se esperaba credenciales inválidas, got %v", err)
		}
	})

	t.Run("UsuarioSinDNI_NoPuedeIniciarSesion", func(t *testing.T) {
		var id string
		u := prefix + "sindni"
		if err := pool.QueryRow(ctx, `INSERT INTO usuarios (username, email, password_hash) VALUES ($1,$2,$3) RETURNING id`,
			u, u+"@test.local", string(hash)).Scan(&id); err != nil {
			t.Fatalf("crear usuario: %v", err)
		}
		userIDs = append(userIDs, id)
		if _, err := client.Login(ctx, &auth.LoginRequest{Dni: "", Password: clave}); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("DNI vacío no debe coincidir con usuarios sin DNI: %v", err)
		}
	})

	t.Run("Permisos_TrasLogin_SonLosDelRol", func(t *testing.T) {
		resp, err := client.Login(ctx, &auth.LoginRequest{Dni: docenteDNI, Password: clave})
		if err != nil {
			t.Fatalf("login: %v", err)
		}
		got, err := client.GetMyPermissions(bearer(resp.AccessToken), &auth.GetMyPermissionsRequest{})
		if err != nil {
			t.Fatalf("GetMyPermissions: %v", err)
		}
		rows, err := pool.Query(ctx, `SELECT p.codigo FROM role_permissions rp JOIN permissions p ON p.id=rp.permission_id WHERE rp.role_id=$1 AND p.estado`, roleID("DOCENTE"))
		if err != nil {
			t.Fatalf("permisos BD: %v", err)
		}
		defer rows.Close()
		var want []string
		for rows.Next() {
			var c string
			_ = rows.Scan(&c)
			want = append(want, c)
		}
		gotCodes := append([]string(nil), got.Permissions...)
		sort.Strings(gotCodes)
		sort.Strings(want)
		if strings.Join(gotCodes, ",") != strings.Join(want, ",") {
			t.Fatalf("permisos tras login = %v, BD = %v", gotCodes, want)
		}
	})

	t.Run("RefreshSinDNI_YLogout", func(t *testing.T) {
		resp, err := client.Login(ctx, &auth.LoginRequest{Dni: docenteDNI, Password: clave})
		if err != nil {
			t.Fatalf("login: %v", err)
		}
		ref, err := client.RefreshToken(ctx, &auth.RefreshTokenRequest{RefreshToken: resp.RefreshToken})
		if err != nil || ref.AccessToken == "" || ref.RefreshToken == resp.RefreshToken {
			t.Fatalf("refresh (sin DNI/clave) con rotación: %+v err=%v", ref, err)
		}
		if _, err := client.RefreshToken(ctx, &auth.RefreshTokenRequest{RefreshToken: resp.RefreshToken}); status.Code(err) != codes.Unauthenticated {
			t.Fatalf("reutilizar el refresh anterior debe rechazarse: %v", err)
		}
		if _, err := client.Logout(bearer(ref.AccessToken), &auth.LogoutRequest{RefreshToken: ref.RefreshToken}); err != nil {
			t.Fatalf("logout: %v", err)
		}
	})

	t.Run("Register_ExigeDNIValidoYUnico", func(t *testing.T) {
		dni := nextTestDNI()
		u := prefix + "reg1"
		ok, err := client.Register(ctx, &auth.RegisterRequest{Dni: dni, Username: u, Email: u + "@test.local", Password: clave})
		if err != nil {
			t.Fatalf("register: %v", err)
		}
		userIDs = append(userIDs, ok.Id)
		var stored string
		if err := pool.QueryRow(ctx, "SELECT dni FROM usuarios WHERE id=$1", ok.Id).Scan(&stored); err != nil || stored != dni {
			t.Fatalf("dni guardado = %q err=%v", stored, err)
		}
		if _, err := client.Login(ctx, &auth.LoginRequest{Dni: dni, Password: clave}); err != nil {
			t.Fatalf("un usuario registrado debe poder entrar con su DNI: %v", err)
		}
		u2 := prefix + "reg2"
		_, err = client.Register(ctx, &auth.RegisterRequest{Dni: dni, Username: u2, Email: u2 + "@test.local", Password: clave})
		if status.Code(err) != codes.AlreadyExists {
			t.Fatalf("DNI duplicado: se esperaba AlreadyExists, got %v", err)
		}
		for _, bad := range []string{"", "123", "1234ABCD"} {
			_, err = client.Register(ctx, &auth.RegisterRequest{Dni: bad, Username: u2, Email: u2 + "@test.local", Password: clave})
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("DNI %q en Register: se esperaba InvalidArgument, got %v", bad, err)
			}
		}
	})

	t.Run("BD_RechazaDNIInvalidoYDuplicado", func(t *testing.T) {
		u := prefix + "dbdup"
		if _, err := pool.Exec(ctx, `INSERT INTO usuarios (username, email, password_hash, dni) VALUES ($1,$2,'x',$3)`, u, u+"@test.local", docenteDNI); err == nil {
			_, _ = pool.Exec(ctx, "DELETE FROM usuarios WHERE username=$1", u)
			t.Fatal("la BD debe rechazar un DNI duplicado")
		}
		if _, err := pool.Exec(ctx, `INSERT INTO usuarios (username, email, password_hash, dni) VALUES ($1,$2,'x','12A45678')`, u, u+"@test.local"); err == nil {
			_, _ = pool.Exec(ctx, "DELETE FROM usuarios WHERE username=$1", u)
			t.Fatal("la BD debe rechazar un DNI con formato inválido")
		}
	})
}
