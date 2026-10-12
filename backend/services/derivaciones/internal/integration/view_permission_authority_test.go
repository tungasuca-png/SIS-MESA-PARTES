package integration

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"derivaciones/derivaciones"
	"derivaciones/internal/config"
	"derivaciones/internal/interceptor"
	"derivaciones/internal/repository"
	"derivaciones/internal/security"
	"derivaciones/internal/server"
	"derivaciones/internal/svc"
	"expedientes/expedientesclient"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// scopedExpedientesClient modela SOLO el ALCANCE DE DATOS de
// Expedientes.GetExpediente (no permisos): el usuario del JWT reenviado
// accede si es el solicitante del expediente o si figura en su alcance
// interno (área/global). Así estas pruebas ejercen la separación
// permiso (Auth) / alcance (Expedientes).
type scopedExpedientesClient struct {
	expedientesclient.Expedientes
	solicitantes map[string]string          // expedienteID -> solicitanteID
	alcance      map[string]map[string]bool // userID -> expedienteID -> accesible (interno)
}

func (c *scopedExpedientesClient) GetExpediente(ctx context.Context, in *expedientesclient.GetExpedienteRequest, _ ...grpc.CallOption) (*expedientesclient.GetExpedienteResponse, error) {
	dueno, existe := c.solicitantes[in.Id]
	if !existe {
		return nil, status.Error(codes.NotFound, "expediente no encontrado")
	}
	md, _ := metadata.FromOutgoingContext(ctx)
	var sub string
	if h := md.Get("authorization"); len(h) > 0 {
		claims := &security.Claims{}
		_, _, _ = jwt.NewParser().ParseUnverified(strings.TrimPrefix(h[0], "Bearer "), claims)
		sub = claims.Subject
	}
	if sub == "" || (dueno != sub && !c.alcance[sub][in.Id]) {
		return nil, status.Error(codes.PermissionDenied, "sin acceso al expediente")
	}
	return &expedientesclient.GetExpedienteResponse{Expediente: &expedientesclient.Expediente{Id: in.Id, SolicitanteId: dueno}}, nil
}

// TestVisibilidadDerivacionesPermisoEsLaAutoridad prueba que ver derivaciones
// lo deciden derivaciones.view (alcance general) y derivaciones.view_own
// (solo expedientes propios) según Auth — nunca el nombre del rol — y que el
// alcance de datos de Expedientes se sigue aplicando encima del permiso.
func TestVisibilidadDerivacionesPermisoEsLaAutoridad(t *testing.T) {
	requireRealJWTSecret(t)
	if strings.ToLower(os.Getenv("INTEGRATION_TEST")) != "true" {
		t.Skip("set INTEGRATION_TEST=true to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, integrationDatabaseURL(t))
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	base := time.Now().UnixNano()
	expA, expB, expC := fixedUUID(base, 1), fixedUUID(base, 2), fixedUUID(base, 3)
	solNone := fixedUUID(base, 10)  // SOLICITANTE sin ningún permiso de derivaciones
	solOwn := fixedUUID(base, 11)   // SOLICITANTE con view_own, dueño de expA
	solOther := fixedUUID(base, 12) // SOLICITANTE con view_own, dueño de expB
	solView := fixedUUID(base, 13)  // SOLICITANTE con view (general), dueño de expC
	admin := fixedUUID(base, 14)    // ADMIN con view
	adminSin := fixedUUID(base, 15) // ADMIN sin permisos de derivaciones
	docente := fixedUUID(base, 16)  // DOCENTE con view, alcance solo expA
	dirSin := fixedUUID(base, 17)   // DIRECTOR sin permisos de derivaciones
	dirOwn := fixedUUID(base, 18)   // DIRECTOR solo con view_own, sin ser dueño de expA
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM derivaciones WHERE expediente_id = ANY($1)", []string{expA, expB, expC})
	}()

	repo := repository.NewDerivacionRepository(pool)
	seed := func(exp string) string {
		d, err := repo.Create(ctx, exp, "DERIVACION", "SECRETARIA", "DIRECTOR", "motivo interno", "URGENTE", fixedUUID(base, 50))
		if err != nil {
			t.Fatalf("seed derivación: %v", err)
		}
		return d.ID
	}
	dA, dB := seed(expA), seed(expB)
	seed(expC)

	svcCtx := &svc.ServiceContext{
		Config:               config.Config{JWTSecret: realSharedJWTSecret},
		DB:                   pool,
		DerivacionRepository: repo,
		JWTValidator:         security.NewJWTValidator(realSharedJWTSecret),
		AuthClient: &permAuthClient{grants: map[string]map[string]bool{
			solOwn:   {"derivaciones.view_own": true},
			solOther: {"derivaciones.view_own": true},
			solView:  {"derivaciones.view": true},
			admin:    {"derivaciones.view": true},
			docente:  {"derivaciones.view": true},
			dirOwn:   {"derivaciones.view_own": true},
		}},
		ExpedientesClient: &scopedExpedientesClient{
			solicitantes: map[string]string{expA: solOwn, expB: solOther, expC: solView},
			alcance: map[string]map[string]bool{
				admin:    {expA: true, expB: true, expC: true},
				adminSin: {expA: true, expB: true, expC: true},
				docente:  {expA: true},
				dirSin:   {expA: true},
				dirOwn:   {expA: true},
			},
		},
	}
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(interceptor.AuthenticationInterceptor(svcCtx.JWTValidator)))
	derivaciones.RegisterDerivacionesServer(grpcServer, server.NewDerivacionesServer(svcCtx))
	go func() { _ = grpcServer.Serve(listener) }()
	defer grpcServer.Stop()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	client := derivaciones.NewDerivacionesClient(conn)

	authFor := func(userID, role string) context.Context {
		claims := security.Claims{Username: "t", Role: role, RegisteredClaims: jwt.RegisteredClaims{
			Subject: userID, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		}}
		tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(realSharedJWTSecret))
		return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+tok)
	}
	wantCode := func(t *testing.T, err error, want codes.Code, what string) {
		t.Helper()
		if status.Code(err) != want {
			t.Fatalf("%s: se esperaba %v, got %v", what, want, err)
		}
	}

	t.Run("Caso1_SolicitanteSinPermisos_List_y_Get_Denegados", func(t *testing.T) {
		resp, err := client.ListDerivaciones(authFor(solNone, "SOLICITANTE"), &derivaciones.ListDerivacionesRequest{ExpedienteId: expA})
		wantCode(t, err, codes.PermissionDenied, "List sin permisos")
		if resp != nil {
			t.Fatal("no debía filtrarse ningún dato")
		}
		_, err = client.ListDerivaciones(authFor(solNone, "SOLICITANTE"), &derivaciones.ListDerivacionesRequest{})
		wantCode(t, err, codes.PermissionDenied, "List sin permisos y sin expediente_id")
		g, err := client.GetDerivacion(authFor(solNone, "SOLICITANTE"), &derivaciones.GetDerivacionRequest{Id: dA})
		wantCode(t, err, codes.PermissionDenied, "Get sin permisos")
		if g != nil {
			t.Fatal("no debía filtrarse ningún dato")
		}
	})

	t.Run("Caso2_ViewOwn_Propio_Permitido_Ajeno_Denegado_VistaReducida", func(t *testing.T) {
		c := authFor(solOwn, "SOLICITANTE")
		l, err := client.ListDerivaciones(c, &derivaciones.ListDerivacionesRequest{ExpedienteId: expA})
		if err != nil || l.Total != 1 {
			t.Fatalf("List propio: se esperaba 1 derivación, got %v err=%v", l, err)
		}
		g, err := client.GetDerivacion(c, &derivaciones.GetDerivacionRequest{Id: dA})
		if err != nil {
			t.Fatalf("Get propio: %v", err)
		}
		for _, d := range []*derivaciones.Derivacion{l.Derivaciones[0], g.Derivacion} {
			if d.Origen != "SECRETARIA" || d.Destino != "DIRECTOR" || d.Tipo != "DERIVACION" || d.Id != dA {
				t.Fatalf("la vista reducida debe conservar el recorrido: %+v", d)
			}
			if d.Motivo != "" || d.Condicion != "" || d.RegistradoPor != "" {
				t.Fatalf("view_own no debe exponer motivo/condición/registrado_por: %+v", d)
			}
		}
		_, err = client.ListDerivaciones(c, &derivaciones.ListDerivacionesRequest{ExpedienteId: expB})
		wantCode(t, err, codes.PermissionDenied, "List de expediente ajeno con view_own")
		_, err = client.GetDerivacion(c, &derivaciones.GetDerivacionRequest{Id: dB})
		wantCode(t, err, codes.PermissionDenied, "Get de derivación ajena con view_own")
		_, err = client.ListDerivaciones(c, &derivaciones.ListDerivacionesRequest{})
		wantCode(t, err, codes.InvalidArgument, "view_own no habilita el listado global")
		_, err = client.ListDerivaciones(c, &derivaciones.ListDerivacionesRequest{ExpedienteId: fixedUUID(base, 99)})
		wantCode(t, err, codes.NotFound, "expediente inexistente")
	})

	t.Run("Caso3_SolicitanteConView_UsaAlcanceGeneral_SinAmpliarlo", func(t *testing.T) {
		c := authFor(solView, "SOLICITANTE")
		l, err := client.ListDerivaciones(c, &derivaciones.ListDerivacionesRequest{ExpedienteId: expC})
		if err != nil || l.Total != 1 {
			t.Fatalf("List con view sobre expediente dentro de su alcance: %v err=%v", l, err)
		}
		if d := l.Derivaciones[0]; d.Motivo == "" || d.Condicion == "" || d.RegistradoPor == "" {
			t.Fatalf("view (general) conserva la vista completa: %+v", d)
		}
		_, err = client.ListDerivaciones(c, &derivaciones.ListDerivacionesRequest{ExpedienteId: expA})
		wantCode(t, err, codes.PermissionDenied, "view no elimina el alcance de datos")
		_, err = client.GetDerivacion(c, &derivaciones.GetDerivacionRequest{Id: dA})
		wantCode(t, err, codes.PermissionDenied, "Get: view no elimina el alcance de datos")
	})

	t.Run("Caso4_AdminConView_AlcanceGlobal_VistaCompleta", func(t *testing.T) {
		c := authFor(admin, "ADMIN")
		for _, exp := range []string{expA, expB, expC} {
			if l, err := client.ListDerivaciones(c, &derivaciones.ListDerivacionesRequest{ExpedienteId: exp}); err != nil || l.Total != 1 {
				t.Fatalf("ADMIN con view debe listar %s: %v err=%v", exp, l, err)
			}
		}
		g, err := client.GetDerivacion(c, &derivaciones.GetDerivacionRequest{Id: dB})
		if err != nil || g.Derivacion.Motivo != "motivo interno" || g.Derivacion.RegistradoPor != fixedUUID(base, 50) {
			t.Fatalf("ADMIN con view: vista completa esperada, got %+v err=%v", g, err)
		}
		if _, err := client.ListDerivaciones(c, &derivaciones.ListDerivacionesRequest{}); err != nil {
			t.Fatalf("view habilita el listado global: %v", err)
		}
	})

	t.Run("Caso5_InternoSinPermisos_Denegado_AunqueSeaAdmin", func(t *testing.T) {
		for _, c := range []struct{ user, role string }{{adminSin, "ADMIN"}, {dirSin, "DIRECTOR"}} {
			_, err := client.ListDerivaciones(authFor(c.user, c.role), &derivaciones.ListDerivacionesRequest{ExpedienteId: expA})
			wantCode(t, err, codes.PermissionDenied, c.role+" sin permisos: List")
			_, err = client.GetDerivacion(authFor(c.user, c.role), &derivaciones.GetDerivacionRequest{Id: dA})
			wantCode(t, err, codes.PermissionDenied, c.role+" sin permisos: Get")
		}
	})

	t.Run("Caso6_PermisoSinRolEspecial_DocenteConView", func(t *testing.T) {
		c := authFor(docente, "DOCENTE")
		if l, err := client.ListDerivaciones(c, &derivaciones.ListDerivacionesRequest{ExpedienteId: expA}); err != nil || l.Total != 1 {
			t.Fatalf("DOCENTE con view dentro de su alcance: %v err=%v", l, err)
		}
		if _, err := client.GetDerivacion(c, &derivaciones.GetDerivacionRequest{Id: dA}); err != nil {
			t.Fatalf("Get DOCENTE con view: %v", err)
		}
		_, err := client.ListDerivaciones(c, &derivaciones.ListDerivacionesRequest{ExpedienteId: expB})
		wantCode(t, err, codes.PermissionDenied, "DOCENTE fuera de su alcance")
	})

	t.Run("ViewOwn_ExigePropiedadReal_NoAlcancePorArea", func(t *testing.T) {
		// DIRECTOR con solo view_own tiene alcance sobre expA, pero no es su
		// dueño: view_own exige propiedad real, no alcance por área.
		_, err := client.ListDerivaciones(authFor(dirOwn, "DIRECTOR"), &derivaciones.ListDerivacionesRequest{ExpedienteId: expA})
		wantCode(t, err, codes.PermissionDenied, "view_own sobre expediente no propio")
		_, err = client.GetDerivacion(authFor(dirOwn, "DIRECTOR"), &derivaciones.GetDerivacionRequest{Id: dA})
		wantCode(t, err, codes.PermissionDenied, "view_own sobre derivación de expediente no propio")
	})
}
