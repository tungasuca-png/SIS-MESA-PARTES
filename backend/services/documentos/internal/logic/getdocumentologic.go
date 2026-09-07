package logic

import (
	"context"
	"errors"
	"strings"

	"documentos/documentos"
	"documentos/internal/interceptor"
	"documentos/internal/repository"
	"documentos/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetDocumentoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDocumentoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDocumentoLogic {
	return &GetDocumentoLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetDocumentoLogic) GetDocumento(in *documentos.GetDocumentoRequest) (*documentos.GetDocumentoResponse, error) {
	if _, _, ok := interceptor.UserFromContext(l.ctx); !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if in == nil || strings.TrimSpace(in.Id) == "" {
		return nil, status.Error(codes.InvalidArgument, "el identificador es obligatorio")
	}

	found, err := l.svcCtx.DocumentoRepository.GetMetadata(l.ctx, strings.TrimSpace(in.Id))
	if err != nil {
		if errors.Is(err, repository.ErrDocumentoNotFound) {
			return nil, status.Error(codes.NotFound, "documento no encontrado")
		}
		return nil, status.Error(codes.Internal, "no se pudo consultar el documento")
	}

	return &documentos.GetDocumentoResponse{Documento: toProto(found)}, nil
}
