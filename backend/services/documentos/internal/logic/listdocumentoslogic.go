package logic

import (
	"context"
	"strings"

	"documentos/documentos"
	"documentos/internal/authorization"
	"documentos/internal/interceptor"
	"documentos/internal/repository"
	"documentos/internal/svc"
	"documentos/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
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
	userID, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}

	filter := repository.ListFilter{}
	if in != nil {
		filter.ExpedienteID = strings.TrimSpace(in.ExpedienteId)
		filter.TipoDocumento = strings.ToUpper(strings.TrimSpace(in.TipoDocumento))
		filter.Page = int(in.Page)
		filter.PageSize = int(in.PageSize)
	}
	if filter.ExpedienteID != "" && !validation.IsValidUUID(filter.ExpedienteID) {
		return nil, status.Error(codes.InvalidArgument, "el expediente_id no es válido")
	}
	if filter.TipoDocumento != "" && !validation.IsValidTipoDocumento(filter.TipoDocumento) {
		return nil, status.Error(codes.InvalidArgument, "el tipo de documento no es válido")
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 || filter.PageSize > maxPageSize {
		filter.PageSize = defaultPageSize
	}

	// Sin autorizacion para ver todo y sin filtrar por un expediente puntual:
	// se acota a lo que el propio usuario subio (ver limitacion documentada
	// en internal/authorization).
	if filter.ExpedienteID == "" && !authorization.CanViewAll(role) {
		filter.SubidoPor = userID
	}

	items, total, err := l.svcCtx.DocumentoRepository.List(l.ctx, filter)
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo listar los documentos")
	}

	protoItems := make([]*documentos.Documento, 0, len(items))
	for _, item := range items {
		protoItems = append(protoItems, toProto(item))
	}

	return &documentos.ListDocumentosResponse{Documentos: protoItems, Total: int32(total)}, nil
}
