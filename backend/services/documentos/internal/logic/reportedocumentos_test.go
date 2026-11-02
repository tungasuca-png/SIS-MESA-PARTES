package logic

import (
	"context"
	"strings"
	"testing"
	"time"

	"auth/authclient"
	"documentos/documentos"
	"documentos/internal/interceptor"
	"documentos/internal/repository"
	"documentos/internal/security"
	"documentos/internal/svc"

	"expedientes/expedientesclient"
	"expedientes/reportes"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var hoyDoc = time.Date(2026, 10, 30, 9, 0, 0, 0, reportes.Zona)

func cantidadDoc(cs []*documentos.ReporteDocConteo, k string) int32 {
	for _, c := range cs {
		if c.Clave == k {
			return c.Cantidad
		}
	}
	return 0
}

func TestArmarReporteDocumentos(t *testing.T) {
	filas := []repository.ReporteDocFila{
		{ExpedienteID: "e1", TipoDocumento: "ADJUNTO", Dia: "2026-10-05", Registrador: "u-1", Cantidad: 4},
		{ExpedienteID: "e2", TipoDocumento: "PROVEIDO", Dia: "2026-10-20", Registrador: "u-2", Cantidad: 1},
		{ExpedienteID: "e1", TipoDocumento: "ADJUNTO", Dia: "2026-10-20", Registrador: "u-1", Cantidad: 2},
	}
	tipos := map[string]string{"e1": "CONSTANCIA", "e2": "PERMISO"}
	r, err := armarReporteDocumentos(filas, tipos, "SEMANA", "2026-10-01", "2026-10-30", true, hoyDoc)
	if err != nil {
		t.Fatal(err)
	}
	if r.Total != 7 || cantidadDoc(r.PorTipoDocumento, "ADJUNTO") != 6 || cantidadDoc(r.PorTipoDocumento, "PROVEIDO") != 1 || len(r.PorTipoDocumento) != 6 {
		t.Fatalf("por tipo documental (catálogo completo): %v", r.PorTipoDocumento)
	}
	if cantidadDoc(r.PorTipoTramite, "CONSTANCIA") != 6 || cantidadDoc(r.PorTipoTramite, "PERMISO") != 1 {
		t.Fatalf("por tipo de trámite: %v", r.PorTipoTramite)
	}
	// Semanas del 28/09 al 26/10 (lunes): 5 puntos.
	if len(r.Serie) != 5 || r.Serie[0].Clave != "2026-09-28" || cantidadDoc(r.Serie, "2026-10-05") != 4 || cantidadDoc(r.Serie, "2026-10-19") != 3 {
		t.Fatalf("serie semanal: %v", r.Serie)
	}
	if cantidadDoc(r.RegistrosPorUsuario, "u-1") != 6 {
		t.Fatalf("registros: %v", r.RegistrosPorUsuario)
	}
	sin, _ := armarReporteDocumentos(filas, tipos, "MES", "", "", false, hoyDoc)
	if len(sin.RegistrosPorUsuario) != 0 {
		t.Fatal("sin incluir_registros no se devuelven IDs")
	}
}

type authDocFalso struct {
	authclient.Auth
	permitido map[string]bool
}

func (a *authDocFalso) HasPermission(_ context.Context, in *authclient.HasPermissionRequest, _ ...grpc.CallOption) (*authclient.HasPermissionResponse, error) {
	return &authclient.HasPermissionResponse{Allowed: a.permitido[in.Permission]}, nil
}

type expAlcanceDocFalso struct {
	expedientesclient.Expedientes
	err   error
	authz []string
}

func (e *expAlcanceDocFalso) ReporteAlcanceExpedientes(ctx context.Context, _ *expedientesclient.ReporteAlcanceRequest, _ ...grpc.CallOption) (*expedientesclient.ReporteAlcanceResponse, error) {
	md, _ := metadata.FromOutgoingContext(ctx)
	e.authz = md.Get("authorization")
	if e.err != nil {
		return nil, e.err
	}
	return &expedientesclient.ReporteAlcanceResponse{}, nil
}

const secretoDoc = "secreto-de-prueba-reporte-documentos"

func reporteDocComo(t *testing.T, auth *authDocFalso, exp *expAlcanceDocFalso, in *documentos.ReporteDocumentosRequest) (*documentos.ReporteDocumentosResponse, error) {
	t.Helper()
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, security.Claims{Role: "DIRECTOR", RegisteredClaims: jwt.RegisteredClaims{
		Subject: "00000000-0000-4000-8000-000000000002", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))}}).SignedString([]byte(secretoDoc))
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+tok))
	sc := &svc.ServiceContext{AuthClient: auth, ExpedientesClient: exp, DocumentoRepository: repository.NewDocumentoRepository(nil)}
	var resp *documentos.ReporteDocumentosResponse
	_, err := interceptor.AuthenticationInterceptor(security.NewJWTValidator(secretoDoc))(ctx, in, &grpc.UnaryServerInfo{},
		func(ctx context.Context, req any) (any, error) {
			r, err := NewReporteDocumentosLogic(ctx, sc).ReporteDocumentos(req.(*documentos.ReporteDocumentosRequest))
			resp = r
			return r, err
		})
	return resp, err
}

func TestReporteDocumentos_PermisoValidacionYAlcance(t *testing.T) {
	if _, err := reporteDocComo(t, &authDocFalso{permitido: map[string]bool{"documentos.view": true}}, &expAlcanceDocFalso{}, &documentos.ReporteDocumentosRequest{}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("sin reportes.view: %v", err)
	}
	con := &authDocFalso{permitido: map[string]bool{"reportes.view": true}}
	if _, err := reporteDocComo(t, con, &expAlcanceDocFalso{}, &documentos.ReporteDocumentosRequest{Hasta: "1999-01-01"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("fecha inválida: %v", err)
	}
	if _, err := reporteDocComo(t, con, &expAlcanceDocFalso{err: status.Error(codes.InvalidArgument, "tipo")}, &documentos.ReporteDocumentosRequest{TipoTramite: "X"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("propaga la validación de Expedientes: %v", err)
	}
	exp := &expAlcanceDocFalso{}
	r, err := reporteDocComo(t, con, exp, &documentos.ReporteDocumentosRequest{})
	if err != nil || r.Total != 0 || len(exp.authz) != 1 || !strings.HasPrefix(exp.authz[0], "Bearer ") {
		t.Fatalf("alcance vacío y token reenviado: %v %v", r, err)
	}
}
