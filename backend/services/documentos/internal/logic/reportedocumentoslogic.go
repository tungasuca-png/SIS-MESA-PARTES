package logic

import (
	"context"
	"sort"
	"strings"
	"time"

	"documentos/documentos"
	"documentos/internal/authorization"
	"documentos/internal/interceptor"
	"documentos/internal/repository"
	"documentos/internal/svc"

	"expedientes/expedientesclient"
	"expedientes/reportes"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Reportes (Fase 2): ver docs/reportes/modulo-reportes.md.

var ordenTiposDocumento = []string{"ADJUNTO", "PROVEIDO", "ACTA", "INFORME", "FORMATO", "OTRO"}

var ordenTiposTramiteDoc = []string{
	"CERTIFICADO", "CONSTANCIA", "PERMISO", "JUSTIFICACION_FALTA", "JUSTIFICACION_TARDANZA",
	"SOLICITUD", "TRAMITE", "OFICIO", "OTRO",
}

// ahoraReporteDoc se reemplaza en pruebas.
var ahoraReporteDoc = time.Now

type ReporteDocumentosLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReporteDocumentosLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReporteDocumentosLogic {
	return &ReporteDocumentosLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func invalidoReporteDoc(msg string) error {
	return status.Error(codes.InvalidArgument, "validation_error: "+msg)
}

func (l *ReporteDocumentosLogic) ReporteDocumentos(in *documentos.ReporteDocumentosRequest) (*documentos.ReporteDocumentosResponse, error) {
	userID, _, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if err := authorization.RequirePermission(l.ctx, l.svcCtx.AuthClient, userID, "reportes.view"); err != nil {
		return nil, err
	}
	ahora := ahoraReporteDoc().In(reportes.Zona)
	desde, hasta := strings.TrimSpace(in.GetDesde()), strings.TrimSpace(in.GetHasta())
	if err := reportes.ValidarPeriodo(desde, hasta, ahora); err != nil {
		return nil, invalidoReporteDoc(err.Error())
	}
	granularidad, err := reportes.Granularidad(in.GetGranularidad())
	if err != nil {
		return nil, invalidoReporteDoc(err.Error())
	}

	// Alcance: expedientes ACTIVOS visibles (Expedientes aplica permiso, rol
	// y tipo de trámite). Se reenvía el token del usuario.
	outCtx := l.ctx
	if token, ok := interceptor.TokenFromContext(l.ctx); ok {
		outCtx = metadata.AppendToOutgoingContext(l.ctx, "authorization", "Bearer "+token)
	}
	alcance, err := l.svcCtx.ExpedientesClient.ReporteAlcanceExpedientes(outCtx, &expedientesclient.ReporteAlcanceRequest{Tipo: in.GetTipoTramite()})
	if err != nil {
		return nil, err
	}
	tipoDe := make(map[string]string, len(alcance.GetExpedientes()))
	ids := make([]string, 0, len(alcance.GetExpedientes()))
	for _, e := range alcance.GetExpedientes() {
		tipoDe[e.GetId()] = e.GetTipo()
		ids = append(ids, e.GetId())
	}

	filas, err := l.svcCtx.DocumentoRepository.Reporte(l.ctx, repository.ReporteDocFiltro{
		ExpedienteIDs: ids, Desde: desde, Hasta: hasta, ConRegistrador: in.GetIncluirRegistros(),
	})
	if err != nil {
		l.Errorf("reporte de documentos: %v", err)
		return nil, status.Error(codes.Internal, "no se pudo generar el reporte")
	}
	resp, err := armarReporteDocumentos(filas, tipoDe, granularidad, desde, hasta, in.GetIncluirRegistros(), ahora)
	if err != nil {
		return nil, invalidoReporteDoc(err.Error())
	}
	resp.Generado = ahora.Format(time.RFC3339)
	resp.Alcance, resp.AreaAlcance = alcance.GetAlcance(), alcance.GetAreaAlcance()
	return resp, nil
}

func conteosDoc(cs []reportes.Conteo) []*documentos.ReporteDocConteo {
	out := make([]*documentos.ReporteDocConteo, 0, len(cs))
	for _, c := range cs {
		out = append(out, &documentos.ReporteDocConteo{Clave: c.Clave, Cantidad: int32(c.Cantidad)})
	}
	return out
}

func armarReporteDocumentos(filas []repository.ReporteDocFila, tipoDe map[string]string, granularidad, desde, hasta string, conRegistros bool, ahora time.Time) (*documentos.ReporteDocumentosResponse, error) {
	porTipoDoc, porTipoTramite, porDia, porUsuario := map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
	total := 0
	for _, f := range filas {
		total += f.Cantidad
		porTipoDoc[f.TipoDocumento] += f.Cantidad
		porTipoTramite[tipoDe[f.ExpedienteID]] += f.Cantidad
		porDia[f.Dia] += f.Cantidad
		if conRegistros && f.Registrador != "" {
			porUsuario[f.Registrador] += f.Cantidad
		}
	}
	serie, err := reportes.Serie(porDia, granularidad, desde, hasta, ahora)
	if err != nil {
		return nil, err
	}
	resp := &documentos.ReporteDocumentosResponse{
		Total:            int32(total),
		PorTipoDocumento: conteosDoc(reportes.Ordenar(ordenTiposDocumento, porTipoDoc)),
		PorTipoTramite:   conteosDoc(reportes.Ordenar(ordenTiposTramiteDoc, porTipoTramite)),
		Serie:            conteosDoc(serie),
		Granularidad:     granularidad,
	}
	if conRegistros {
		claves := make([]string, 0, len(porUsuario))
		for k := range porUsuario {
			claves = append(claves, k)
		}
		sort.Strings(claves)
		for _, k := range claves {
			resp.RegistrosPorUsuario = append(resp.RegistrosPorUsuario, &documentos.ReporteDocConteo{Clave: k, Cantidad: int32(porUsuario[k])})
		}
	}
	return resp, nil
}
