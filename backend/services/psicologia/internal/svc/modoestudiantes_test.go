package svc

import (
	"context"
	"strings"
	"testing"

	"estudiantes/estudiantesclient"

	"github.com/zeromicro/go-zero/core/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestResolverModoEstudiantes(t *testing.T) {
	const id, sec = clientIDPrueba, clientSecretPrueba
	casos := []struct {
		nombre                  string
		mode, cid, csec, legado string
		esperado                ModoEstudiantes
		falla                   bool
	}{
		{"producción con credenciales", service.ProMode, id, sec, "", ModoServicio, false},
		{"producción sin credenciales", service.ProMode, "", "", "", 0, true},
		{"producción con fallback pedido", service.ProMode, "", "", "true", 0, true},
		{"desarrollo con credenciales", service.DevMode, id, sec, "", ModoServicio, false},
		{"desarrollo sin credenciales ni fallback (deshabilitado por defecto)", service.DevMode, "", "", "", 0, true},
		{"desarrollo con fallback=false", service.DevMode, "", "", "false", 0, true},
		{"desarrollo con fallback explícito", service.DevMode, "", "", "true", ModoUsuarioLegado, false},
		{"test con fallback explícito", service.TestMode, "", "", "TRUE", ModoUsuarioLegado, false},
		{"credenciales tienen prioridad sobre el fallback", service.DevMode, id, sec, "true", ModoServicio, false},
		{"solo client_id", service.DevMode, id, "", "true", 0, true},
		{"solo client_secret", service.ProMode, "", sec, "", 0, true},
		{"fallback con valor inválido", service.DevMode, "", "", "si", 0, true},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			modo, err := ResolverModoEstudiantes(c.mode, c.cid, c.csec, c.legado)
			if c.falla {
				if err == nil {
					t.Fatalf("debió impedir el arranque, devolvió %v", modo)
				}
				if strings.Contains(err.Error(), sec) || strings.Contains(err.Error(), id) {
					t.Fatal("el mensaje de error contiene una credencial")
				}
				return
			}
			if err != nil || modo != c.esperado {
				t.Fatalf("se esperaba %v, se obtuvo %v (%v)", c.esperado, modo, err)
			}
		})
	}
}

// estudiantesLegado: solo GetEstudiante (modo anterior).
type estudiantesLegado struct {
	estudiantesclient.Estudiantes
	autorizacion []string
}

func (e *estudiantesLegado) GetEstudiante(ctx context.Context, in *estudiantesclient.GetEstudianteRequest, _ ...grpc.CallOption) (*estudiantesclient.GetEstudianteResponse, error) {
	md, _ := metadata.FromOutgoingContext(ctx)
	e.autorizacion = md.Get("authorization")
	if in.GetId() != estudianteExiste {
		return nil, status.Error(codes.NotFound, "estudiante no encontrado")
	}
	return &estudiantesclient.GetEstudianteResponse{Estudiante: &estudiantesclient.Estudiante{Id: in.GetId(), Nombres: "Ana María", Apellidos: "Quispe Huamán"}}, nil
}

// Modo anterior (solo desarrollo con fallback explícito): reenvía el JWT del
// usuario y no pide token de servicio.
func TestVerificador_ModoAnteriorExplicito(t *testing.T) {
	est := &estudiantesLegado{}
	v := NewEstudianteVerifier(est, nil)
	ctx, humano := ctxUsuario(t)
	ctx = metadata.NewOutgoingContext(ctx, metadata.MD{}) // sin metadata previa
	if ok, err := v.Exists(ctx, estudianteExiste); err != nil || !ok {
		t.Fatalf("existente: %v %v", ok, err)
	}
	if len(est.autorizacion) != 1 || est.autorizacion[0] != "Bearer "+humano {
		t.Fatal("en el modo anterior debe reenviarse exactamente el JWT del usuario")
	}
	if ok, err := v.Exists(ctx, estudianteNoExiste); err != nil || ok {
		t.Fatalf("inexistente: %v %v", ok, err)
	}
}
