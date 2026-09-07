package logic

import (
	"context"

	"documentos/documentosclient"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListAllDocumentosLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListAllDocumentosLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAllDocumentosLogic {
	return &ListAllDocumentosLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListAllDocumentosLogic) ListAllDocumentos(req *types.ListAllDocumentosRequest) (resp *types.ListAllDocumentosResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	listed, err := l.svcCtx.DocumentosClient.ListDocumentos(ctx, &documentosclient.ListDocumentosRequest{
		ExpedienteId:  req.ExpedienteId,
		TipoDocumento: req.TipoDocumento,
		Page:          req.Page,
		PageSize:      req.PageSize,
	})
	if err != nil {
		return nil, err
	}

	items := make([]types.DocumentoDTO, 0, len(listed.Documentos))
	for _, item := range listed.Documentos {
		items = append(items, toDocumentoDTO(item))
	}

	return &types.ListAllDocumentosResponse{Documentos: items, Total: listed.Total}, nil
}
