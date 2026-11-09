package logic

import (
	"context"

	"expedientes/expedientes"
	"expedientes/internal/interceptor"
	"expedientes/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UpdateAreaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateAreaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAreaLogic {
	return &UpdateAreaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpdateArea queda DESHABILITADO (A + B): cambiaba el área sin validar la
// transición ni registrar el movimiento. El único camino para cambiar de área
// es DerivarExpediente (o RechazarExpediente). Se conserva el RPC para que un
// cliente antiguo reciba un error claro en lugar de Unimplemented.
func (l *UpdateAreaLogic) UpdateArea(in *expedientes.UpdateAreaRequest) (*expedientes.UpdateAreaResponse, error) {
	if _, _, ok := interceptor.UserFromContext(l.ctx); !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	return nil, status.Error(codes.FailedPrecondition, "el cambio directo de área está deshabilitado: use la derivación del expediente")
}
