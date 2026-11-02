package logic

import (
	"context"
	"sort"
	"strings"
	"time"

	"derivaciones/derivaciones"
	"derivaciones/internal/authorization"
	"derivaciones/internal/interceptor"
	"derivaciones/internal/repository"
	"derivaciones/internal/svc"

	"expedientes/expedientesclient"
	"expedientes/reportes"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Reportes (Fase 2): ver docs/reportes/modulo-reportes.md.

const tipoDerivacion = "DERIVACION"

// Otros tipos de movimiento: solo se cuentan (origen/destino son texto libre).
var ordenOtrosMovimientos = []string{"NOTIFICACION", "ASIGNACION", "RECEPCION", "APROBACION"}

var ordenTiposTramite = []string{
	"CERTIFICADO", "CONSTANCIA", "PERMISO", "JUSTIFICACION_FALTA", "JUSTIFICACION_TARDANZA",
	"SOLICITUD", "TRAMITE", "OFICIO", "OTRO",
}

// ahoraReporteDeriv se reemplaza en pruebas.
var ahoraReporteDeriv = time.Now

type ReporteDerivacionesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReporteDerivacionesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReporteDerivacionesLogic {
	return &ReporteDerivacionesLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func invalidoReporte(msg string) error {
	return status.Error(codes.InvalidArgument, "validation_error: "+msg)
}

func (l *ReporteDerivacionesLogic) ReporteDerivaciones(in *derivaciones.ReporteDerivacionesRequest) (*derivaciones.ReporteDerivacionesResponse, error) {
	userID, _, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if err := authorization.RequirePermission(l.ctx, l.svcCtx.AuthClient, userID, "reportes.view"); err != nil {
		return nil, err
	}
	ahora := ahoraReporteDeriv().In(reportes.Zona)
	desde, hasta := strings.TrimSpace(in.GetDesde()), strings.TrimSpace(in.GetHasta())
	if err := reportes.ValidarPeriodo(desde, hasta, ahora); err != nil {
		return nil, invalidoReporte(err.Error())
	}
	granularidad, err := reportes.Granularidad(in.GetGranularidad())
	if err != nil {
		return nil, invalidoReporte(err.Error())
	}

	// Alcance: expedientes ACTIVOS visibles para el usuario (Expedientes
	// aplica permiso, rol y tipo de trámite). Se reenvía el token del usuario.
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

	filas, err := l.svcCtx.DerivacionRepository.Reporte(l.ctx, repository.ReporteDerivFiltro{
		ExpedienteIDs: ids, Desde: desde, Hasta: hasta, ConRegistrador: in.GetIncluirRegistros(),
	})
	if err != nil {
		l.Errorf("reporte de derivaciones: %v", err)
		return nil, status.Error(codes.Internal, "no se pudo generar el reporte")
	}
	resp, err := armarReporteDerivaciones(filas, tipoDe, granularidad, desde, hasta, in.GetIncluirRegistros(), ahora)
	if err != nil {
		return nil, invalidoReporte(err.Error())
	}
	resp.Generado = ahora.Format(time.RFC3339)
	resp.Alcance, resp.AreaAlcance = alcance.GetAlcance(), alcance.GetAreaAlcance()
	return resp, nil
}

func conteosDeriv(cs []reportes.Conteo) []*derivaciones.ReporteDerivConteo {
	out := make([]*derivaciones.ReporteDerivConteo, 0, len(cs))
	for _, c := range cs {
		out = append(out, &derivaciones.ReporteDerivConteo{Clave: c.Clave, Cantidad: int32(c.Cantidad)})
	}
	return out
}

// armarReporteDerivaciones normaliza y agrega. Solo el tipo DERIVACION se
// analiza por áreas; los demás tipos solo se cuentan.
func armarReporteDerivaciones(filas []repository.ReporteDerivFila, tipoDe map[string]string, granularidad, desde, hasta string, conRegistros bool, ahora time.Time) (*derivaciones.ReporteDerivacionesResponse, error) {
	porDestino, porOrigen, porTipo, porDia, otros, porUsuario := map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
	flujo := map[[2]string]int{}
	derivPorUsuario := map[string]int{} // solo tipo DERIVACION
	total, sinClasificar := 0, 0
	for _, f := range filas {
		if conRegistros && f.Registrador != "" {
			porUsuario[f.Registrador] += f.Cantidad
		}
		if f.Tipo != tipoDerivacion {
			otros[f.Tipo] += f.Cantidad
			continue
		}
		if conRegistros && f.Registrador != "" {
			derivPorUsuario[f.Registrador] += f.Cantidad
		}
		origen, destino := reportes.NormalizarArea(f.Origen), reportes.NormalizarArea(f.Destino)
		total += f.Cantidad
		porOrigen[origen] += f.Cantidad
		porDestino[destino] += f.Cantidad
		flujo[[2]string{origen, destino}] += f.Cantidad
		porTipo[tipoDe[f.ExpedienteID]] += f.Cantidad
		porDia[f.Dia] += f.Cantidad
		if origen == reportes.SinClasificar || destino == reportes.SinClasificar {
			sinClasificar += f.Cantidad
		}
	}
	serie, err := reportes.Serie(porDia, granularidad, desde, hasta, ahora)
	if err != nil {
		return nil, err
	}
	resp := &derivaciones.ReporteDerivacionesResponse{
		Total:            int32(total),
		PorDestino:       conteosDeriv(reportes.Ordenar(reportes.OrdenAreas, porDestino)),
		PorOrigen:        conteosDeriv(reportes.Ordenar(reportes.OrdenAreas, porOrigen)),
		PorTipoTramite:   conteosDeriv(reportes.Ordenar(ordenTiposTramite, porTipo)),
		Serie:            conteosDeriv(serie),
		Granularidad:     granularidad,
		OtrosMovimientos: conteosDeriv(reportes.Ordenar(ordenOtrosMovimientos, otros)),
		SinClasificar:    int32(sinClasificar),
	}
	for _, o := range reportes.OrdenAreas {
		for _, d := range reportes.OrdenAreas {
			if n := flujo[[2]string{o, d}]; n > 0 {
				resp.Flujo = append(resp.Flujo, &derivaciones.ReporteDerivFlujo{Origen: o, Destino: d, Cantidad: int32(n)})
			}
		}
	}
	if conRegistros {
		resp.RegistrosPorUsuario = conteosPorClave(porUsuario)
		resp.DerivacionesPorUsuario = conteosPorClave(derivPorUsuario)
	}
	return resp, nil
}

// conteosPorClave: conteos ordenados por clave (para respuestas estables).
func conteosPorClave(m map[string]int) []*derivaciones.ReporteDerivConteo {
	claves := make([]string, 0, len(m))
	for k := range m {
		claves = append(claves, k)
	}
	sort.Strings(claves)
	out := make([]*derivaciones.ReporteDerivConteo, 0, len(claves))
	for _, k := range claves {
		out = append(out, &derivaciones.ReporteDerivConteo{Clave: k, Cantidad: int32(m[k])})
	}
	return out
}
