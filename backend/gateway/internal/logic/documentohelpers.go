package logic

import (
	"context"

	"documentos/documentosclient"
	"gateway/internal/svc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// requireDocumentoProveido (Etapa 4): verifica que ya exista un documento
// tipo=PROVEIDO para el expediente antes de dejar avanzar una acción que
// institucionalmente lo exige — el cierre de F2 (ResolverExpediente) y el
// envío del proveído al Docente en F4 (Subdirección -> Docente). No genera
// el documento (eso sigue siendo una carga manual en Documentos Service,
// mismo mecanismo ya existente — Etapa 4 pide reutilizarlo, no inventar un
// generador nuevo).
func requireDocumentoProveido(ctx context.Context, svcCtx *svc.ServiceContext, expedienteID string) error {
	listado, err := svcCtx.DocumentosClient.ListDocumentos(ctx, &documentosclient.ListDocumentosRequest{
		ExpedienteId:  expedienteID,
		TipoDocumento: "PROVEIDO",
	})
	if err != nil {
		return err
	}
	if listado.Total == 0 {
		return status.Error(
			codes.FailedPrecondition,
			"todavía no se cargó el documento final (proveído) para este expediente",
		)
	}
	return nil
}
