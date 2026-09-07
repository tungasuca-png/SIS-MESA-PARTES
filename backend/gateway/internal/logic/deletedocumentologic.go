package logic

import (
	"context"

	"documentos/documentosclient"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteDocumentoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteDocumentoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteDocumentoLogic {
	return &DeleteDocumentoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteDocumentoLogic) DeleteDocumento(req *types.DeleteDocumentoRequest) (resp *types.DeleteDocumentoResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	deleted, err := l.svcCtx.DocumentosClient.DeleteDocumento(ctx, &documentosclient.DeleteDocumentoRequest{
		Id: req.Id,
	})
	if err != nil {
		return nil, err
	}

	return &types.DeleteDocumentoResponse{Success: deleted.Success}, nil
}
