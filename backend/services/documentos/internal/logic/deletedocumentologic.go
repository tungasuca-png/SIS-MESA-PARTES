package logic

import (
	"context"
	"errors"
	"strings"

	"documentos/documentos"
	"documentos/internal/authorization"
	"documentos/internal/interceptor"
	"documentos/internal/repository"
	"documentos/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type DeleteDocumentoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteDocumentoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteDocumentoLogic {
	return &DeleteDocumentoLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *DeleteDocumentoLogic) DeleteDocumento(in *documentos.DeleteDocumentoRequest) (*documentos.DeleteDocumentoResponse, error) {
	_, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if !authorization.CanDelete(role) {
		return nil, status.Error(codes.PermissionDenied, "el rol no puede eliminar documentos")
	}
	if in == nil || strings.TrimSpace(in.Id) == "" {
		return nil, status.Error(codes.InvalidArgument, "el identificador es obligatorio")
	}

	if err := l.svcCtx.DocumentoRepository.Delete(l.ctx, strings.TrimSpace(in.Id)); err != nil {
		if errors.Is(err, repository.ErrDocumentoNotFound) {
			return nil, status.Error(codes.NotFound, "documento no encontrado")
		}
		return nil, status.Error(codes.Internal, "no se pudo eliminar el documento")
	}

	return &documentos.DeleteDocumentoResponse{Success: true}, nil
}
