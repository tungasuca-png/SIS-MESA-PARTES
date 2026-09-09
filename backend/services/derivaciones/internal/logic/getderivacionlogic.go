package logic

import (
	"context"
	"errors"
	"strings"

	"derivaciones/derivaciones"
	"derivaciones/internal/interceptor"
	"derivaciones/internal/repository"
	"derivaciones/internal/svc"
	"derivaciones/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetDerivacionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDerivacionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDerivacionLogic {
	return &GetDerivacionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetDerivacionLogic) GetDerivacion(in *derivaciones.GetDerivacionRequest) (*derivaciones.GetDerivacionResponse, error) {
	_, _, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if in == nil || strings.TrimSpace(in.Id) == "" || !validation.IsValidUUID(strings.TrimSpace(in.Id)) {
		return nil, status.Error(codes.InvalidArgument, "el identificador no es válido")
	}

	found, err := l.svcCtx.DerivacionRepository.Get(l.ctx, strings.TrimSpace(in.Id))
	if err != nil {
		if errors.Is(err, repository.ErrDerivacionNotFound) {
			return nil, status.Error(codes.NotFound, "derivación no encontrada")
		}
		return nil, status.Error(codes.Internal, "no se pudo consultar la derivación")
	}

	return &derivaciones.GetDerivacionResponse{Derivacion: toProto(found)}, nil
}
