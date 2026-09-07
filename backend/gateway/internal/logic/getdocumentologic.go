package logic

import (
	"context"

	"documentos/documentosclient"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetDocumentoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetDocumentoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDocumentoLogic {
	return &GetDocumentoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetDocumentoLogic) GetDocumento(req *types.GetDocumentoRequest) (resp *types.GetDocumentoResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	found, err := l.svcCtx.DocumentosClient.GetDocumento(ctx, &documentosclient.GetDocumentoRequest{
		Id: req.Id,
	})
	if err != nil {
		return nil, err
	}

	return &types.GetDocumentoResponse{Documento: toDocumentoDTO(found.Documento)}, nil
}
