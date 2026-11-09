package logic

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"auth/authclient"
	"derivaciones/derivaciones"
	"derivaciones/internal/interceptor"
	"derivaciones/internal/repository"
	"derivaciones/internal/security"
	"derivaciones/internal/svc"

	"expedientes/expedientesclient"
	"expedientes/reportes"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Pruebas sin PostgreSQL. Datos ficticios; los "usuarios" de origen son
// nombres inventados para comprobar que nunca salen en la respuesta.

var hoyDeriv = time.Date(2026, 10, 30, 9, 0, 0, 0, reportes.Zona)

func filasDeriv() []repository.ReporteDerivFila {
	return []repository.ReporteDerivFila{
		{ExpedienteID: "e1", Tipo: "DERIVACION", Origen: "SECRETARIA", Destino: "DIRECTOR", Dia: "2026-10-26", Registrador: "u-1", Cantidad: 3},
		{ExpedienteID: "e2", Tipo: "DERIVACION", Origen: "secretaria.demo (Secretaría)", Destino: "Dirección", Dia: "2026-10-27", Registrador: "u-1", Cantidad: 1},
		{ExpedienteID: "e2", Tipo: "DERIVACION", Origen: "DIRECTOR", Destino: "SECRETARIA", Dia: "2026-10-28", Registrador: "u-2", Cantidad: 1},
		{ExpedienteID: "e1", Tipo: "DERIVACION", Origen: "e2esec_1 (Secretar�a)", Destino: "Direcci�n", Dia: "2026-10-28", Registrador: "u-2", Cantidad: 1},
		{ExpedienteID: "e1", Tipo: "NOTIFICACION", Origen: "juan.perez (Docente)", Destino: "Secretaría", Dia: "2026-10-29", Registrador: "u-3", Cantidad: 2},
		{ExpedienteID: "e2", Tipo: "APROBACION", Origen: "director.demo (Dirección)", Destino: "Secretaría", Dia: "2026-10-29", Registrador: "u-2", Cantidad: 1},
	}
}

var tiposDeriv = map[string]string{"e1": "CONSTANCIA", "e2": "PERMISO"}

func cantidadDeriv(cs []*derivaciones.ReporteDerivConteo, k string) int32 {
	for _, c := range cs {
		if c.Clave == k {
			return c.Cantidad
		}
	}
	return 0
}

func TestArmarReporteDerivaciones_NormalizaYCuenta(t *testing.T) {
	r, err := armarReporteDerivaciones(filasDeriv(), tiposDeriv, "DIA", "2026-10-26", "2026-10-30", true, hoyDeriv)
	if err != nil {
		t.Fatal(err)
	}
	// Solo DERIVACION: 3 + 1 + 1 + 1.
	if r.Total != 6 {
		t.Fatalf("total: %d", r.Total)
	}
	if cantidadDeriv(r.PorDestino, "DIRECTOR") != 4 || cantidadDeriv(r.PorDestino, "SECRETARIA") != 1 || cantidadDeriv(r.PorDestino, reportes.SinClasificar) != 1 {
		t.Fatalf("destino (etiquetas antiguas normalizadas): %v", r.PorDestino)
	}
	if cantidadDeriv(r.PorOrigen, "SECRETARIA") != 4 || cantidadDeriv(r.PorOrigen, reportes.SinClasificar) != 1 {
		t.Fatalf("origen (\"usuario (Rol)\" -> rol; texto dañado -> Sin clasificar): %v", r.PorOrigen)
	}
	if r.SinClasificar != 1 {
		t.Fatalf("sin clasificar: %d", r.SinClasificar)
	}
	if cantidadDeriv(r.PorTipoTramite, "CONSTANCIA") != 4 || cantidadDeriv(r.PorTipoTramite, "PERMISO") != 2 {
		t.Fatalf("por tipo de trámite: %v", r.PorTipoTramite)
	}
	if cantidadDeriv(r.OtrosMovimientos, "NOTIFICACION") != 2 || cantidadDeriv(r.OtrosMovimientos, "APROBACION") != 1 {
		t.Fatalf("otros movimientos: %v", r.OtrosMovimientos)
	}
	flujo := map[string]int32{}
	for _, f := range r.Flujo {
		flujo[f.Origen+">"+f.Destino] = f.Cantidad
	}
	if flujo["SECRETARIA>DIRECTOR"] != 4 || flujo["DIRECTOR>SECRETARIA"] != 1 || flujo[reportes.SinClasificar+">"+reportes.SinClasificar] != 1 {
		t.Fatalf("flujo: %v", flujo)
	}
	if len(r.Serie) != 5 || r.Serie[0].Cantidad != 3 || r.Serie[2].Cantidad != 2 || r.Serie[3].Cantidad != 0 {
		t.Fatalf("serie diaria (solo DERIVACION): %v", r.Serie)
	}
	// Registros por usuario: todos los tipos (actividad).
	if cantidadDeriv(r.RegistrosPorUsuario, "u-1") != 4 || cantidadDeriv(r.RegistrosPorUsuario, "u-3") != 2 {
		t.Fatalf("registros: %v", r.RegistrosPorUsuario)
	}
	// Derivaciones por usuario: SOLO el tipo DERIVACION (u-3 solo notificó;
	// u-2 derivó 2 veces y aprobó 1).
	if cantidadDeriv(r.DerivacionesPorUsuario, "u-1") != 4 || cantidadDeriv(r.DerivacionesPorUsuario, "u-2") != 2 ||
		cantidadDeriv(r.DerivacionesPorUsuario, "u-3") != 0 || cantidadDeriv(r.RegistrosPorUsuario, "u-2") != 3 {
		t.Fatalf("derivaciones por usuario: %v", r.DerivacionesPorUsuario)
	}
	// Nunca sale texto libre ni nombres de usuario.
	j, _ := json.Marshal(r)
	for _, prohibido := range []string{"juan.perez", "secretaria.demo", "director.demo", "e2esec", "Secretaría", "Dirección"} {
		if strings.Contains(string(j), prohibido) {
			t.Fatalf("la respuesta contiene %q", prohibido)
		}
	}
}

func TestArmarReporteDerivaciones_SinRegistrosNiDatos(t *testing.T) {
	r, err := armarReporteDerivaciones(filasDeriv(), tiposDeriv, "MES", "", "", false, hoyDeriv)
	if err != nil || len(r.RegistrosPorUsuario) != 0 || len(r.DerivacionesPorUsuario) != 0 {
		t.Fatalf("sin incluir_registros no se devuelven IDs: %v %v", r.RegistrosPorUsuario, err)
	}
	vacio, err := armarReporteDerivaciones(nil, nil, "MES", "", "", false, hoyDeriv)
	if err != nil || vacio.Total != 0 || len(vacio.PorDestino) != len(reportes.OrdenAreas) || len(vacio.Serie) != 0 {
		t.Fatalf("vacío: %+v %v", vacio, err)
	}
}

type authDerivFalso struct {
	authclient.Auth
	permitido map[string]bool
}

func (a *authDerivFalso) HasPermission(_ context.Context, in *authclient.HasPermissionRequest, _ ...grpc.CallOption) (*authclient.HasPermissionResponse, error) {
	return &authclient.HasPermissionResponse{Allowed: a.permitido[in.Permission]}, nil
}

type expedientesAlcanceFalso struct {
	expedientesclient.Expedientes
	err      error
	authz    []string
	tipo     string
	devuelve []*expedientesclient.ReporteExpedienteRef
}

func (e *expedientesAlcanceFalso) ReporteAlcanceExpedientes(ctx context.Context, in *expedientesclient.ReporteAlcanceRequest, _ ...grpc.CallOption) (*expedientesclient.ReporteAlcanceResponse, error) {
	md, _ := metadata.FromOutgoingContext(ctx)
	e.authz, e.tipo = md.Get("authorization"), in.GetTipo()
	if e.err != nil {
		return nil, e.err
	}
	return &expedientesclient.ReporteAlcanceResponse{Expedientes: e.devuelve}, nil
}

const secretoDeriv = "secreto-de-prueba-reporte-derivaciones"

func reporteDerivComo(t *testing.T, auth *authDerivFalso, exp *expedientesAlcanceFalso, in *derivaciones.ReporteDerivacionesRequest) (*derivaciones.ReporteDerivacionesResponse, error) {
	t.Helper()
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, security.Claims{Role: "SECRETARIA", RegisteredClaims: jwt.RegisteredClaims{
		Subject: "00000000-0000-4000-8000-000000000001", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))}}).SignedString([]byte(secretoDeriv))
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+tok))
	sc := &svc.ServiceContext{AuthClient: auth, ExpedientesClient: exp, DerivacionRepository: repository.NewDerivacionRepository(nil)}
	var resp *derivaciones.ReporteDerivacionesResponse
	_, err := interceptor.AuthenticationInterceptor(security.NewJWTValidator(secretoDeriv), nil)(ctx, in, &grpc.UnaryServerInfo{},
		func(ctx context.Context, req any) (any, error) {
			r, err := NewReporteDerivacionesLogic(ctx, sc).ReporteDerivaciones(req.(*derivaciones.ReporteDerivacionesRequest))
			resp = r
			return r, err
		})
	return resp, err
}

func TestReporteDerivaciones_PermisoValidacionYAlcance(t *testing.T) {
	sin := &authDerivFalso{permitido: map[string]bool{"derivaciones.view": true}}
	if _, err := reporteDerivComo(t, sin, &expedientesAlcanceFalso{}, &derivaciones.ReporteDerivacionesRequest{}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("sin reportes.view: %v", err)
	}
	con := &authDerivFalso{permitido: map[string]bool{"reportes.view": true}}
	for nombre, in := range map[string]*derivaciones.ReporteDerivacionesRequest{
		"fecha":        {Desde: "30/10/2026"},
		"orden":        {Desde: "2026-10-10", Hasta: "2026-10-01"},
		"granularidad": {Granularidad: "ANIO"},
	} {
		if _, err := reporteDerivComo(t, con, &expedientesAlcanceFalso{}, in); status.Code(err) != codes.InvalidArgument {
			t.Errorf("%s: %v", nombre, err)
		}
	}
	// Expedientes rechaza (p. ej. tipo inválido o sin permiso): se propaga.
	if _, err := reporteDerivComo(t, con, &expedientesAlcanceFalso{err: status.Error(codes.PermissionDenied, "x")}, &derivaciones.ReporteDerivacionesRequest{}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("propaga el rechazo de Expedientes: %v", err)
	}
	// Sin expedientes visibles: reporte vacío sin consultar la base (repo con db nil).
	exp := &expedientesAlcanceFalso{}
	r, err := reporteDerivComo(t, con, exp, &derivaciones.ReporteDerivacionesRequest{TipoTramite: "PERMISO"})
	if err != nil || r.Total != 0 {
		t.Fatalf("alcance vacío: %v %v", r, err)
	}
	if exp.tipo != "PERMISO" || len(exp.authz) != 1 || !strings.HasPrefix(exp.authz[0], "Bearer ") {
		t.Fatal("debe reenviar el token del usuario y el tipo de trámite a Expedientes")
	}
}
