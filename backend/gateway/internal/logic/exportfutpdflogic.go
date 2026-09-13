package logic

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"documentos/documentosclient"
	"expedientes/expedientesclient"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ExportFutPdfLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewExportFutPdfLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExportFutPdfLogic {
	return &ExportFutPdfLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ExportFutPdf reconstruye el cargo del FUT como PDF vectorial (Etapa
// post-4, pedido explícito: "el formato no tiene buena resolución"). Antes
// se generaba en el navegador como una imagen JPEG rasterizada embebida en
// un PDF armado a mano — por eso el texto se veía borroso al hacer zoom o
// imprimir. Ahora el mismo contenido se dibuja con texto vectorial real en
// el backend (github.com/go-pdf/fpdf), a partir de los mismos datos que ya
// usaba el PDF anterior: nada de datos nuevos ni de reglas nuevas.
//
// La autorización se obtiene gratis de GetExpediente (Expedientes Service
// ya exige ser dueño del área o el solicitante del expediente, salvo
// ADMIN/SECRETARIA) — mismo patrón de composición en el Gateway que ya usan
// derivar/rechazar/corregir/resolver.
func (l *ExportFutPdfLogic) ExportFutPdf(req *types.ExportFutPdfRequest) (resp *types.ExportFutPdfResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	found, err := l.svcCtx.ExpedientesClient.GetExpediente(ctx, &expedientesclient.GetExpedienteRequest{
		Id: req.Id,
	})
	if err != nil {
		return nil, err
	}
	expediente := found.Expediente

	datosSolicitante := parseDatosSolicitanteFut(expediente.Descripcion)
	if datosSolicitante == nil {
		return nil, status.Error(codes.FailedPrecondition, "este expediente no tiene datos de FUT Digital")
	}

	var nombresDocumentos []string
	var firmaPNG []byte
	if listado, err := l.svcCtx.DocumentosClient.ListDocumentos(ctx, &documentosclient.ListDocumentosRequest{
		ExpedienteId: expediente.Id,
	}); err == nil {
		for _, doc := range listado.Documentos {
			if doc.Nombre == "firma.png" {
				if descargado, err := l.svcCtx.DocumentosClient.DownloadDocumento(ctx, &documentosclient.DownloadDocumentoRequest{
					Id: doc.Id,
				}); err == nil {
					firmaPNG = descargado.Contenido
				}
				continue
			}
			nombresDocumentos = append(nombresDocumentos, doc.Nombre)
		}
	}
	// Si ListDocumentos falla (p. ej. el usuario ya no tiene acceso a ese
	// detalle), el cargo se genera igual, solo que sin adjuntos/firma — mismo
	// comportamiento de degradación que tenía exportFutDelExpediente() en el
	// frontend.

	pdfBytes, err := buildFutCargoPdf(futCargoData{
		datosSolicitanteFut: *datosSolicitante,
		Sumilla:             expediente.Asunto,
		Codigo:              expediente.Codigo,
		Fecha:               formatFechaFut(expediente.FechaRegistro),
		Documentos:          nombresDocumentos,
		FirmaPNG:            firmaPNG,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo generar el PDF del FUT")
	}

	return &types.ExportFutPdfResponse{
		Nombre:          fmt.Sprintf("FUT-%s.pdf", expediente.Codigo),
		ContenidoBase64: base64.StdEncoding.EncodeToString(pdfBytes),
	}, nil
}

// formatFechaFut muestra solo la fecha (DD/MM/AAAA), igual que
// formatDate() del frontend (utils/format.js) — fecha_registro llega como
// timestamp ISO completo desde Expedientes Service.
func formatFechaFut(fechaRegistro string) string {
	fecha := strings.SplitN(fechaRegistro, "T", 2)[0]
	partes := strings.Split(fecha, "-")
	if len(partes) != 3 {
		return fechaRegistro
	}
	return fmt.Sprintf("%s/%s/%s", partes[2], partes[1], partes[0])
}
