package logic

import (
	"context"

	"documentos/documentosclient"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListDocumentosByExpedienteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListDocumentosByExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListDocumentosByExpedienteLogic {
	return &ListDocumentosByExpedienteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListDocumentosByExpedienteLogic) ListDocumentosByExpediente(req *types.ListDocumentosRequest) (resp *types.ListDocumentosResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	listed, err := l.svcCtx.DocumentosClient.ListDocumentos(ctx, &documentosclient.ListDocumentosRequest{
		ExpedienteId: req.ExpedienteId,
	})
	if err != nil {
		return nil, err
	}

	items := make([]types.DocumentoDTO, 0, len(listed.Documentos))
	for _, item := range listed.Documentos {
		items = append(items, toDocumentoDTO(item))
	}

	return &types.ListDocumentosResponse{Documentos: items}, nil
}
