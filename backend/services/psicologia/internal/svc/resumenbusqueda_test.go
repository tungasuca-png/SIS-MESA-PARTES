package svc

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"estudiantes/estudiantesclient"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Dobles para Resumenes (lotes) y Buscar. Datos ficticios.
type estudiantesLote struct {
	estudiantesclient.Estudiantes
	lotes        [][]string
	autorizacion [][]string
	errBasic     error
	errBuscar    error
	buscarPedido *estudiantesclient.BuscarEstudiantesSeleccionRequest
	minimo       bool // respuesta mínima de Estudiantes para sistemas: id, nombre_completo, grado, sección
}

func (e *estudiantesLote) GetEstudiantesBasic(ctx context.Context, in *estudiantesclient.GetEstudiantesBasicRequest, _ ...grpc.CallOption) (*estudiantesclient.GetEstudiantesBasicResponse, error) {
	md, _ := metadata.FromOutgoingContext(ctx)
	e.autorizacion = append(e.autorizacion, md.Get("authorization"))
	e.lotes = append(e.lotes, append([]string(nil), in.GetIds()...))
	if e.errBasic != nil {
		return nil, e.errBasic
	}
	out := []*estudiantesclient.EstudianteBasico{}
	for _, id := range in.GetIds() {
		if id != estudianteNoExiste && e.minimo {
			out = append(out, &estudiantesclient.EstudianteBasico{Id: strings.ToUpper(id), NombreCompleto: "Nombre Apellido " + id[:4], Grado: "3", Seccion: "B"})
		} else if id != estudianteNoExiste {
			out = append(out, &estudiantesclient.EstudianteBasico{Id: strings.ToUpper(id), Nombres: "Nombre", Apellidos: "Apellido " + id[:4],
				NumeroDocumento: "45127823", Grado: "3", Seccion: "B"})
		}
	}
	return &estudiantesclient.GetEstudiantesBasicResponse{Estudiantes: out}, nil
}

func (e *estudiantesLote) BuscarEstudiantesSeleccion(ctx context.Context, in *estudiantesclient.BuscarEstudiantesSeleccionRequest, _ ...grpc.CallOption) (*estudiantesclient.BuscarEstudiantesSeleccionResponse, error) {
	md, _ := metadata.FromOutgoingContext(ctx)
	e.autorizacion = append(e.autorizacion, md.Get("authorization"))
	e.buscarPedido = in
	if e.errBuscar != nil {
		return nil, e.errBuscar
	}
	return &estudiantesclient.BuscarEstudiantesSeleccionResponse{Total: 1, Estudiantes: []*estudiantesclient.EstudianteSeleccion{
		{Id: estudianteExiste, NombreCompleto: "Ana María Quispe", Grado: "3", Seccion: "B"},
	}}, nil
}

func TestResumenes_UnaLlamadaPorLoteConIdsUnicos(t *testing.T) {
	est := &estudiantesLote{}
	v := NewEstudianteVerifier(est, proveedor(&authFalso{}, &reloj{t: time.Now()}))
	ctx, humano := ctxUsuario(t)
	nombres, err := v.Resumenes(ctx, []string{estudianteExiste, strings.ToUpper(estudianteExiste), estudianteNoExiste, "", estudianteExiste})
	if err != nil {
		t.Fatal(err)
	}
	if len(est.lotes) != 1 || len(est.lotes[0]) != 2 {
		t.Fatalf("se esperaba 1 llamada con 2 IDs únicos: %v", est.lotes)
	}
	if nombres[estudianteExiste] != "Nombre Apellido 1111" {
		t.Fatalf("nombre inesperado: %v", nombres)
	}
	if _, ok := nombres[estudianteNoExiste]; ok {
		t.Fatal("un estudiante inexistente no debe aparecer")
	}
	for _, n := range nombres {
		if strings.Contains(n, "45127823") {
			t.Fatal("solo se copia el nombre, nunca el documento")
		}
	}
	if len(est.autorizacion[0]) != 1 || est.autorizacion[0][0] != "Bearer svc-1" || strings.Contains(est.autorizacion[0][0], humano) {
		t.Fatalf("debe viajar solo el token de servicio: %v", est.autorizacion[0])
	}
}

func TestResumenes_LotesDeCienComoMaximo(t *testing.T) {
	est := &estudiantesLote{}
	v := NewEstudianteVerifier(est, proveedor(&authFalso{}, &reloj{t: time.Now()}))
	ids := make([]string, 0, 150)
	for i := 0; i < 150; i++ {
		ids = append(ids, fmt.Sprintf("%08x-0000-4000-8000-000000000000", i))
	}
	if _, err := v.Resumenes(context.Background(), ids); err != nil {
		t.Fatal(err)
	}
	if len(est.lotes) != 2 || len(est.lotes[0]) != 100 || len(est.lotes[1]) != 50 {
		t.Fatalf("lotes inesperados: %d llamadas", len(est.lotes))
	}
}

func TestResumenes_ErroresSePropagan(t *testing.T) {
	v := NewEstudianteVerifier(&estudiantesLote{errBasic: status.Error(codes.PermissionDenied, "scope")}, proveedor(&authFalso{}, &reloj{t: time.Now()}))
	if _, err := v.Resumenes(context.Background(), []string{estudianteExiste}); status.Code(err) != codes.Unavailable {
		t.Fatalf("scope insuficiente debe llegar como Unavailable: %v", err)
	}
	v2 := NewEstudianteVerifier(&estudiantesLote{}, proveedor(&authFalso{err: status.Error(codes.Unavailable, "auth")}, &reloj{t: time.Now()}))
	if _, err := v2.Resumenes(context.Background(), []string{estudianteExiste}); status.Code(err) != codes.Unavailable {
		t.Fatalf("fallo de Auth debe ser Unavailable: %v", err)
	}
}

func TestBuscar_SoloTokenDeServicioYCamposMinimos(t *testing.T) {
	est := &estudiantesLote{}
	v := NewEstudianteVerifier(est, proveedor(&authFalso{}, &reloj{t: time.Now()}))
	ctx, humano := ctxUsuario(t)
	resp, err := v.Buscar(ctx, "45127823", 2, 10)
	if err != nil || resp.GetTotal() != 1 || len(resp.GetEstudiantes()) != 1 {
		t.Fatalf("búsqueda: %v %v", resp, err)
	}
	if est.buscarPedido.GetQ() != "45127823" || est.buscarPedido.GetPage() != 2 || est.buscarPedido.GetPageSize() != 10 {
		t.Fatalf("parámetros no transmitidos: %+v", est.buscarPedido)
	}
	if len(est.autorizacion[0]) != 1 || est.autorizacion[0][0] != "Bearer svc-1" || strings.Contains(est.autorizacion[0][0], humano) {
		t.Fatalf("debe viajar solo el token de servicio: %v", est.autorizacion[0])
	}
}

func TestBuscar_ErroresYModoAnterior(t *testing.T) {
	v := NewEstudianteVerifier(&estudiantesLote{errBuscar: status.Error(codes.PermissionDenied, "sin search.basic")}, proveedor(&authFalso{}, &reloj{t: time.Now()}))
	if _, err := v.Buscar(context.Background(), "quispe", 1, 20); status.Code(err) != codes.Unavailable {
		t.Fatalf("sin search.basic en el token debe ser Unavailable: %v", err)
	}
	v2 := NewEstudianteVerifier(&estudiantesLote{errBuscar: status.Error(codes.Internal, "falla")}, proveedor(&authFalso{}, &reloj{t: time.Now()}))
	if _, err := v2.Buscar(context.Background(), "quispe", 1, 20); status.Code(err) != codes.Internal {
		t.Fatalf("error interno se propaga: %v", err)
	}
	if _, err := NewEstudianteVerifier(&estudiantesLote{}, nil).Buscar(context.Background(), "quispe", 1, 20); status.Code(err) != codes.Unavailable {
		t.Fatalf("en el modo anterior la búsqueda no está disponible: %v", err)
	}
}

// Respuesta mínima de Estudiantes (solo nombre_completo, sin nombres ni
// apellidos separados): Psicología sigue resolviendo los nombres.
func TestRespuestaMinimaDeEstudiantes(t *testing.T) {
	est := &estudiantesLote{minimo: true}
	v := NewEstudianteVerifier(est, proveedor(&authFalso{}, &reloj{t: time.Now()}))
	nombres, err := v.Resumenes(context.Background(), []string{estudianteExiste, estudianteNoExiste})
	if err != nil || nombres[estudianteExiste] != "Nombre Apellido 1111" || len(nombres) != 1 {
		t.Fatalf("Resumenes con respuesta mínima: %v %v", nombres, err)
	}
	n, err := v.NombreCompleto(context.Background(), estudianteExiste)
	if err != nil || n != "Nombre Apellido 1111" {
		t.Fatalf("NombreCompleto con respuesta mínima: %q %v", n, err)
	}
	if ok, err := v.Exists(context.Background(), estudianteExiste); err != nil || !ok {
		t.Fatalf("Exists con respuesta mínima: %v %v", ok, err)
	}
}
