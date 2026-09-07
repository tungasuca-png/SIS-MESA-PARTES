package logic

import (
	"context"
	"encoding/base64"

	"documentos/documentosclient"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UploadDocumentoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUploadDocumentoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UploadDocumentoLogic {
	return &UploadDocumentoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UploadDocumentoLogic) UploadDocumento(req *types.UploadDocumentoRequest) (resp *types.UploadDocumentoResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	contenido, err := base64.StdEncoding.DecodeString(req.Contenido)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "el contenido no es base64 válido")
	}

	created, err := l.svcCtx.DocumentosClient.UploadDocumento(ctx, &documentosclient.UploadDocumentoRequest{
		ExpedienteId:  req.ExpedienteId,
		Nombre:        req.Nombre,
		TipoDocumento: req.TipoDocumento,
		Extension:     req.Extension,
		Contenido:     contenido,
	})
	if err != nil {
		return nil, err
	}

	return &types.UploadDocumentoResponse{Documento: toDocumentoDTO(created.Documento)}, nil
}
