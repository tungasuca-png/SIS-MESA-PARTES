package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"auth/authclient"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestClasificar(t *testing.T) {
	casos := map[codes.Code]decision{
		codes.OK: publicar, codes.FailedPrecondition: fallar, codes.InvalidArgument: fallar,
		codes.Aborted: reintentar, codes.Unavailable: reintentar, codes.DeadlineExceeded: reintentar,
		codes.Unauthenticated: reintentar, codes.Unimplemented: reintentar, codes.PermissionDenied: reintentar,
		codes.Internal: reintentar, codes.ResourceExhausted: reintentar,
	}
	for c, esperado := range casos {
		var err error
		if c != codes.OK {
			err = status.Error(c, "x")
		}
		if got := Clasificar(err); got != esperado {
			t.Errorf("%v: %v, esperado %v", c, got, esperado)
		}
	}
	if Clasificar(errors.New("conexión rechazada")) != reintentar {
		t.Error("un error de red es transitorio")
	}
}

func TestBackoff(t *testing.T) {
	for intentos, base := range map[int]time.Duration{1: time.Second, 2: 2 * time.Second, 5: 16 * time.Second, 50: 5 * time.Minute} {
		for i := 0; i < 50; i++ {
			d := Backoff(intentos)
			if d < time.Duration(float64(base)*0.8) || d > time.Duration(float64(base)*1.2) {
				t.Fatalf("intentos=%d: %s fuera de %s ±20%%", intentos, d, base)
			}
		}
	}
}

type authFalso struct {
	authclient.Auth
	llamadas int
	falla    bool
}

func (a *authFalso) IssueServiceToken(_ context.Context, in *authclient.IssueServiceTokenRequest, _ ...grpc.CallOption) (*authclient.IssueServiceTokenResponse, error) {
	a.llamadas++
	if a.falla {
		return nil, status.Error(codes.Unavailable, "auth caído")
	}
	return &authclient.IssueServiceTokenResponse{AccessToken: "tok", ExpiresIn: 300}, nil
}

func TestTokensAuth_CacheRenovacionYFallo(t *testing.T) {
	a := &authFalso{}
	reloj := time.Now()
	tk := NewTokensAuth(a, "cli", "secreto")
	tk.ahora = func() time.Time { return reloj }
	for i := 0; i < 3; i++ {
		if _, err := tk.Token(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if a.llamadas != 1 {
		t.Fatalf("el token debe reutilizarse: %d llamadas", a.llamadas)
	}
	reloj = reloj.Add(250 * time.Second) // dentro de los 60 s previos al vencimiento
	_, _ = tk.Token(context.Background())
	if a.llamadas != 2 {
		t.Fatalf("debe renovarse antes del vencimiento: %d llamadas", a.llamadas)
	}
	tk.Invalidar()
	_, _ = tk.Token(context.Background())
	if a.llamadas != 3 {
		t.Fatalf("Invalidar debe forzar la renovación: %d", a.llamadas)
	}
	a.falla = true
	tk.Invalidar()
	if _, err := tk.Token(context.Background()); status.Code(err) != codes.Unavailable {
		t.Fatalf("con Auth caído debe devolver el error: %v", err)
	}
}

func TestAvisar_NoBloqueaYEsSeguroConNil(t *testing.T) {
	var nada *Publicador
	nada.Avisar()
	p := New(nil, nil, nil, Config{})
	for i := 0; i < 5; i++ {
		p.Avisar() // el canal tiene capacidad 1: no debe bloquear
	}
}
