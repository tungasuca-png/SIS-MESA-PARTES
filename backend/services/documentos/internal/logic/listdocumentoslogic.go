package logic

import (
	"context"
	"strings"

	"documentos/documentos"
	"documentos/internal/interceptor"
	"documentos/internal/svc"
	"documentos/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListDocumentosLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListDocumentosLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListDocumentosLogic {
	return &ListDocumentosLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListDocumentosLogic) ListDocumentos(in *documentos.ListDocumentosRequest) (*documentos.ListDocumentosResponse, error) {
	if _, _, ok := interceptor.UserFromContext(l.ctx); !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	expedienteID := ""
	if in != nil {
		expedienteID = strings.TrimSpace(in.ExpedienteId)
	}
	if !validation.IsValidUUID(expedienteID) {
		return nil, status.Error(codes.InvalidArgument, "el expediente_id no es válido")
	}

	items, err := l.svcCtx.DocumentoRepository.ListByExpediente(l.ctx, expedienteID)
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo listar los documentos")
	}

	protoItems := make([]*documentos.Documento, 0, len(items))
	for _, item := range items {
		protoItems = append(protoItems, toProto(item))
	}

	return &documentos.ListDocumentosResponse{Documentos: protoItems}, nil
}
