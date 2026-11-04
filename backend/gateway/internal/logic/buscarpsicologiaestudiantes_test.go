package logic

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"gateway/internal/svc"
	"gateway/internal/types"
	"psicologia/psicologiaclient"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// psicologiaBusquedaFalso implementa solo BuscarEstudiantes; cualquier otro
// método haría panic (no debe usarse).
type psicologiaBusquedaFalso struct {
	psicologiaclient.Psicologia
	pedido *psicologiaclient.BuscarEstudiantesRequest
	authz  []string
}

func (p *psicologiaBusquedaFalso) BuscarEstudiantes(ctx context.Context, in *psicologiaclient.BuscarEstudiantesRequest, _ ...grpc.CallOption) (*psicologiaclient.BuscarEstudiantesResponse, error) {
	md, _ := metadata.FromOutgoingContext(ctx)
	p.authz = md.Get("authorization")
	p.pedido = in
	return &psicologiaclient.BuscarEstudiantesResponse{Total: 7, Estudiantes: []*psicologiaclient.EstudianteBusqueda{
		{Id: "11111111-1111-4111-8111-111111111111", NombreCompleto: "Ana María Quispe", Grado: "3", Seccion: "B"},
	}}, nil
}

func TestBuscarPsicologiaEstudiantes_TransmiteYMapeaSoloCamposMinimos(t *testing.T) {
	falso := &psicologiaBusquedaFalso{}
	l := NewBuscarPsicologiaEstudiantesLogic(context.Background(), &svc.ServiceContext{PsicologiaClient: falso})
	resp, err := l.BuscarPsicologiaEstudiantes(&types.BuscarPsicologiaEstudiantesRequest{Authorization: "Bearer jwt.ficticio.x", Q: "45127823", Page: 2, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if falso.pedido.Q != "45127823" || falso.pedido.Page != 2 || falso.pedido.PageSize != 10 {
		t.Fatalf("parámetros no transmitidos: %+v", falso.pedido)
	}
	if len(falso.authz) != 1 || falso.authz[0] != "Bearer jwt.ficticio.x" {
		t.Fatal("el JWT del usuario debe reenviarse a Psicología, que decide los permisos")
	}
	cuerpo, _ := json.Marshal(resp)
	if string(cuerpo) != `{"estudiantes":[{"id":"11111111-1111-4111-8111-111111111111","nombre_completo":"Ana María Quispe","grado":"3","seccion":"B"}],"total":7}` {
		t.Fatalf("respuesta JSON inesperada: %s", cuerpo)
	}
	if strings.Contains(string(cuerpo), "45127823") {
		t.Fatal("la respuesta no debe contener el DNI")
	}
}

func TestExpedientePsicologicoDTO_ResumenDelEstudiante(t *testing.T) {
	con := toExpedientePsicologicoDTO(&psicologiaclient.ExpedientePsicologico{Id: "e1", Estudiante: &psicologiaclient.EstudianteResumen{NombreCompleto: "Ana", Resolucion: "RESUELTO"}})
	if con.Estudiante == nil || con.Estudiante.NombreCompleto != "Ana" || con.Estudiante.Resolucion != "RESUELTO" {
		t.Fatalf("resumen no mapeado: %+v", con.Estudiante)
	}
	sin, _ := json.Marshal(toExpedientePsicologicoDTO(&psicologiaclient.ExpedientePsicologico{Id: "e1"}))
	if strings.Contains(string(sin), `"estudiante"`) {
		t.Fatalf("sin resumen el campo se omite (compatibilidad): %s", sin)
	}
}
