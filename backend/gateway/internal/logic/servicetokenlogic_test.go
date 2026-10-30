package logic

import (
	"context"
	"testing"

	"auth/authclient"
	"gateway/internal/svc"
	"gateway/internal/types"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// authTokenFalso implementa solo IssueServiceToken; cualquier otro método
// del cliente de Auth haría panic (no debe usarse).
type authTokenFalso struct {
	authclient.Auth
	pedido *authclient.IssueServiceTokenRequest
}

func (a *authTokenFalso) IssueServiceToken(_ context.Context, in *authclient.IssueServiceTokenRequest, _ ...grpc.CallOption) (*authclient.IssueServiceTokenResponse, error) {
	a.pedido = in
	return &authclient.IssueServiceTokenResponse{AccessToken: "jwt.ficticio.firma", ExpiresIn: 300, Scope: "estudiantes.view"}, nil
}

func TestServiceToken_GrantTypeObligatorio(t *testing.T) {
	falso := &authTokenFalso{}
	l := NewServiceTokenLogic(context.Background(), &svc.ServiceContext{AuthClient: falso})
	_, err := l.ServiceToken(&types.ServiceTokenRequest{GrantType: "password", ClientId: "cli_x", ClientSecret: "cs_x"})
	if status.Code(err) != codes.InvalidArgument || falso.pedido != nil {
		t.Fatalf("se esperaba 400 sin consultar Auth: %v", err)
	}
}

func TestServiceToken_TransmiteCredencialesYRespuesta(t *testing.T) {
	falso := &authTokenFalso{}
	l := NewServiceTokenLogic(context.Background(), &svc.ServiceContext{AuthClient: falso})
	resp, err := l.ServiceToken(&types.ServiceTokenRequest{GrantType: "client_credentials", ClientId: "cli_x", ClientSecret: "cs_x"})
	if err != nil {
		t.Fatal(err)
	}
	if falso.pedido.ClientId != "cli_x" || falso.pedido.ClientSecret != "cs_x" {
		t.Fatal("credenciales no transmitidas a Auth")
	}
	if resp.TokenType != "Bearer" || resp.ExpiresIn != 300 || resp.Scope != "estudiantes.view" || resp.AccessToken == "" {
		t.Fatalf("respuesta inesperada: %+v", resp)
	}
}
