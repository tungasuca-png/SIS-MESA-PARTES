package logic

import (
	"context"
	"os"
	"testing"
	"time"

	"derivaciones/derivaciones"
	"derivaciones/internal/interceptor"
	"derivaciones/internal/repository"
	"derivaciones/internal/security"
	"derivaciones/internal/svc"

	"expedientes/expedientesclient"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Privacidad del detalle de derivaciones (GetDerivacion) contra un
// PostgreSQL DESECHABLE con la migración de Derivaciones
// (DERIVACIONES_DB_AISLADA_URL). Auth y Expedientes se simulan con la regla
// REAL de Expedientes: un solicitante recibe NotFound «expediente no
// encontrado» tanto para un expediente ajeno como para uno inexistente.

// expedientesPropiedadFalso: dueño de cada expediente; decide por el JWT
// reenviado (como Expedientes.GetExpediente).
type expedientesPropiedadFalso struct {
	expedientesclient.Expedientes
	duenos map[string]string
}

func (e *expedientesPropiedadFalso) GetExpediente(ctx context.Context, in *expedientesclient.GetExpedienteRequest, _ ...grpc.CallOption) (*expedientesclient.GetExpedienteResponse, error) {
	md, _ := metadata.FromOutgoingContext(ctx)
	if len(md.Get("authorization")) == 0 {
		return nil, status.Error(codes.Unauthenticated, "falta el token de acceso")
	}
	claims, err := security.NewJWTValidator(secretoDeriv).Validate(md.Get("authorization")[0][len("Bearer "):])
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "token de acceso inválido")
	}
	dueno, ok := e.duenos[in.Id]
	if !ok || (claims.Role == "SOLICITANTE" && dueno != claims.Subject) {
		return nil, status.Error(codes.NotFound, "expediente no encontrado")
	}
	return &expedientesclient.GetExpedienteResponse{Expediente: &expedientesclient.Expediente{Id: in.Id, SolicitanteId: dueno}}, nil
}

func tokenDeriv(sub, rol string) string {
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, security.Claims{Role: rol, RegisteredClaims: jwt.RegisteredClaims{
		Subject: sub, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))}}).SignedString([]byte(secretoDeriv))
	return tok
}

func getDerivacionComo(t *testing.T, sc *svc.ServiceContext, token, id string) (*derivaciones.GetDerivacionResponse, error) {
	t.Helper()
	ctx := context.Background()
	if token != "" {
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+token))
	}
	var resp *derivaciones.GetDerivacionResponse
	_, err := interceptor.AuthenticationInterceptor(security.NewJWTValidator(secretoDeriv))(ctx, &derivaciones.GetDerivacionRequest{Id: id},
		&grpc.UnaryServerInfo{FullMethod: "/derivaciones.Derivaciones/GetDerivacion"},
		func(ctx context.Context, req any) (any, error) {
			r, err := NewGetDerivacionLogic(ctx, sc).GetDerivacion(req.(*derivaciones.GetDerivacionRequest))
			resp = r
			return r, err
		})
	return resp, err
}

func TestGetDerivacion_AjenaIndistinguibleDeInexistente(t *testing.T) {
	url := os.Getenv("DERIVACIONES_DB_AISLADA_URL")
	if url == "" {
		t.Skip("DERIVACIONES_DB_AISLADA_URL no definida: solo contra un PostgreSQL desechable")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	repo := repository.NewDerivacionRepository(pool)
	const (
		solA = "00000000-0000-4000-8000-00000000000a"
		solB = "00000000-0000-4000-8000-00000000000b"
		sec  = "00000000-0000-4000-8000-00000000000c"
		expA = "10000000-0000-4000-8000-00000000000a"
		expB = "10000000-0000-4000-8000-00000000000b"
	)
	ctx := context.Background()
	dA, err := repo.Create(ctx, expA, "DERIVACION", "SECRETARIA", "DIRECTOR", "Motivo interno A", "", sec)
	if err != nil {
		t.Fatal(err)
	}
	dB, err := repo.Create(ctx, expB, "DERIVACION", "SECRETARIA", "DIRECTOR", "Motivo interno B", "", sec)
	if err != nil {
		t.Fatal(err)
	}
	exp := &expedientesPropiedadFalso{duenos: map[string]string{expA: solA, expB: solB}}
	solicitante := &svc.ServiceContext{DerivacionRepository: repo, ExpedientesClient: exp,
		AuthClient: &authDerivFalso{permitido: map[string]bool{"derivaciones.view_own": true}}}
	personal := &svc.ServiceContext{DerivacionRepository: repo, ExpedientesClient: exp,
		AuthClient: &authDerivFalso{permitido: map[string]bool{"derivaciones.view": true}}}

	// 1. El solicitante consulta su propia derivación (proyección reducida).
	r, err := getDerivacionComo(t, solicitante, tokenDeriv(solA, "SOLICITANTE"), dA.ID)
	if err != nil || r.Derivacion.Id != dA.ID || r.Derivacion.Motivo != "" || r.Derivacion.RegistradoPor != "" {
		t.Fatalf("derivación propia: %+v %v", r, err)
	}
	// 2-4. Ajena e inexistente: misma respuesta, sin datos del expediente.
	_, ajena := getDerivacionComo(t, solicitante, tokenDeriv(solA, "SOLICITANTE"), dB.ID)
	_, inexistente := getDerivacionComo(t, solicitante, tokenDeriv(solA, "SOLICITANTE"), "20000000-0000-4000-8000-0000000000ff")
	if status.Code(ajena) != codes.NotFound || status.Code(inexistente) != codes.NotFound {
		t.Fatalf("ajena %v / inexistente %v", ajena, inexistente)
	}
	if status.Convert(ajena).Message() != status.Convert(inexistente).Message() || status.Convert(ajena).Message() != "derivación no encontrada" {
		t.Fatalf("respuestas distinguibles: %q vs %q", status.Convert(ajena).Message(), status.Convert(inexistente).Message())
	}
	// B tampoco ve la de A.
	if _, err := getDerivacionComo(t, solicitante, tokenDeriv(solB, "SOLICITANTE"), dA.ID); status.Convert(err).Message() != "derivación no encontrada" {
		t.Fatalf("B consulta la de A: %v", err)
	}
	// 5. El personal (view general) consulta cualquiera, con todos los datos.
	r, err = getDerivacionComo(t, personal, tokenDeriv(sec, "SECRETARIA"), dB.ID)
	if err != nil || r.Derivacion.Motivo != "Motivo interno B" {
		t.Fatalf("SECRETARIA: %+v %v", r, err)
	}
	// El personal ante una derivación inexistente: sin cambios (NotFound propio).
	if _, err := getDerivacionComo(t, personal, tokenDeriv(sec, "SECRETARIA"), "20000000-0000-4000-8000-0000000000ff"); status.Code(err) != codes.NotFound {
		t.Fatalf("personal, inexistente: %v", err)
	}
	// 6. Sin sesión.
	if _, err := getDerivacionComo(t, solicitante, "", dA.ID); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("sin sesión: %v", err)
	}
	// Sin ningún permiso de derivaciones: 403 (antes de consultar nada).
	sinPermiso := &svc.ServiceContext{DerivacionRepository: repo, ExpedientesClient: exp, AuthClient: &authDerivFalso{permitido: map[string]bool{}}}
	if _, err := getDerivacionComo(t, sinPermiso, tokenDeriv(solA, "SOLICITANTE"), dA.ID); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("sin permiso: %v", err)
	}
}
