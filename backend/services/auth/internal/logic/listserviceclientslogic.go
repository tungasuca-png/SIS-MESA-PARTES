package logic

import (
	"context"

	"auth/auth"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListServiceClientsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListServiceClientsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListServiceClientsLogic {
	return &ListServiceClientsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListServiceClientsLogic) ListServiceClients(in *auth.ListServiceClientsRequest) (*auth.ListServiceClientsResponse, error) {
	clients, err := l.svcCtx.ServiceClientRepository.List(l.ctx)
	if err != nil {
		l.Errorf("listar sistemas consumidores: %v", err)
		return nil, status.Error(codes.Internal, "no se pudieron listar los sistemas")
	}
	out := make([]*auth.ServiceClient, 0, len(clients))
	for _, c := range clients {
		out = append(out, serviceClientProto(c))
	}
	return &auth.ListServiceClientsResponse{Clients: out}, nil
}
