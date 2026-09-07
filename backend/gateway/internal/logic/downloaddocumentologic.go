package logic

import (
	"context"
	"encoding/base64"

	"documentos/documentosclient"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DownloadDocumentoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDownloadDocumentoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DownloadDocumentoLogic {
	return &DownloadDocumentoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DownloadDocumentoLogic) DownloadDocumento(req *types.DownloadDocumentoRequest) (resp *types.DownloadDocumentoResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	found, err := l.svcCtx.DocumentosClient.DownloadDocumento(ctx, &documentosclient.DownloadDocumentoRequest{
		Id: req.Id,
	})
	if err != nil {
		return nil, err
	}

	return &types.DownloadDocumentoResponse{
		Documento: toDocumentoDTO(found.Documento),
		Contenido: base64.StdEncoding.EncodeToString(found.Contenido),
	}, nil
}
