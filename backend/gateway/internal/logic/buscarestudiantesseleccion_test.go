package logic

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"estudiantes/estudiantesclient"
	"gateway/internal/svc"
	"gateway/internal/types"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// estudiantesSeleccionFalso implementa solo BuscarEstudiantesSeleccion;
// cualquier otro método haría panic (no debe usarse). Datos ficticios.
type estudiantesSeleccionFalso struct {
	estudiantesclient.Estudiantes
	pedido *estudiantesclient.BuscarEstudiantesSeleccionRequest
	authz  []string
	err    error
}

func (e *estudiantesSeleccionFalso) BuscarEstudiantesSeleccion(ctx context.Context, in *estudiantesclient.BuscarEstudiantesSeleccionRequest, _ ...grpc.CallOption) (*estudiantesclient.BuscarEstudiantesSeleccionResponse, error) {
	md, _ := metadata.FromOutgoingContext(ctx)
	e.authz = md.Get("authorization")
	e.pedido = in
	if e.err != nil {
		return nil, e.err
	}
	return &estudiantesclient.BuscarEstudiantesSeleccionResponse{Total: 3, Estudiantes: []*estudiantesclient.EstudianteSeleccion{
		{Id: "11111111-1111-4111-8111-111111111111", NombreCompleto: "Ana María Quispe", Grado: "3", Seccion: "B"},
	}}, nil
}

func TestBuscarEstudiantesSeleccion_TransmiteTokenYSoloCamposMinimos(t *testing.T) {
	falso := &estudiantesSeleccionFalso{}
	l := NewBuscarEstudiantesSeleccionLogic(context.Background(), &svc.ServiceContext{EstudiantesClient: falso})
	resp, err := l.BuscarEstudiantesSeleccion(&types.BuscarEstudiantesSeleccionRequest{Authorization: "Bearer token.servicio.ficticio", Q: "45127823", Page: 2, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if falso.pedido.Q != "45127823" || falso.pedido.Page != 2 || falso.pedido.PageSize != 10 {
		t.Fatalf("parámetros no transmitidos: %+v", falso.pedido)
	}
	if len(falso.authz) != 1 || falso.authz[0] != "Bearer token.servicio.ficticio" {
		t.Fatal("el token del sistema debe reenviarse a Estudiantes, que lo valida")
	}
	cuerpo, _ := json.Marshal(resp)
	if string(cuerpo) != `{"estudiantes":[{"id":"11111111-1111-4111-8111-111111111111","nombre_completo":"Ana María Quispe","grado":"3","seccion":"B"}],"total":3}` {
		t.Fatalf("respuesta JSON inesperada: %s", cuerpo)
	}
	if strings.Contains(string(cuerpo), "45127823") {
		t.Fatal("la respuesta no debe contener el DNI")
	}
}

// Rechazos de Estudiantes (JWT humano, sin scope, token inválido) se propagan
// con su código, para que errorsafe responda 401/403 uniformes.
func TestBuscarEstudiantesSeleccion_PropagaRechazos(t *testing.T) {
	for _, c := range []codes.Code{codes.Unauthenticated, codes.PermissionDenied, codes.InvalidArgument} {
		falso := &estudiantesSeleccionFalso{err: status.Error(c, "rechazado")}
		l := NewBuscarEstudiantesSeleccionLogic(context.Background(), &svc.ServiceContext{EstudiantesClient: falso})
		if _, err := l.BuscarEstudiantesSeleccion(&types.BuscarEstudiantesSeleccionRequest{Q: "quispe"}); status.Code(err) != c {
			t.Fatalf("se esperaba %v: %v", c, err)
		}
	}
}
