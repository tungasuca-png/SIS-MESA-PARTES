package logic

import (
	"context"
	"encoding/base64"

	"documentos/documentosclient"
	"expedientes/expedientesclient"
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

// DownloadDocumento (Etapa 4): Documentos Service, por sí solo, no valida
// de quién es el expediente al que pertenece un documento (limitación ya
// documentada en su propio internal/authorization — un SOLICITANTE podía
// descargar el documento de CUALQUIER expediente si conocía su id). Se
// cierra acá reutilizando la validación que ya existe en
// GetExpediente (Etapa 1): si el expediente no es del actor (ni tiene
// CanViewAll/su propia área), se deniega antes de entregar el contenido.
// Aplica igual para todo rol, no solo SOLICITANTE — mismo criterio ya
// usado en el resto de acciones sobre expedientes.
func (l *DownloadDocumentoLogic) DownloadDocumento(req *types.DownloadDocumentoRequest) (resp *types.DownloadDocumentoResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	meta, err := l.svcCtx.DocumentosClient.GetDocumento(ctx, &documentosclient.GetDocumentoRequest{Id: req.Id})
	if err != nil {
		return nil, err
	}
	if _, err := l.svcCtx.ExpedientesClient.GetExpediente(ctx, &expedientesclient.GetExpedienteRequest{
		Id: meta.Documento.ExpedienteId,
	}); err != nil {
		return nil, err
	}

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
