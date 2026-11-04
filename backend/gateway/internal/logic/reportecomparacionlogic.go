// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"strings"
	"time"

	"expedientes/expedientesclient"

	"gateway/internal/reportes"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ReporteComparacionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReporteComparacionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReporteComparacionLogic {
	return &ReporteComparacionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

type comparacion struct {
	resultado            reportes.Comparacion
	a, b                 *expedientesclient.ReporteExpedientesResponse
	filtros              reportes.Filtros
	desdeB, hastaB       string
	alcance, areaAlcance string
	generado             time.Time
}

// calcularComparacion consulta el MISMO reporte de Expedientes para ambos
// periodos (mismos filtros, permiso y alcance) y compara. Las variaciones
// solo se calculan con al menos 10 expedientes en cada periodo.
func calcularComparacion(ctx context.Context, svcCtx *svc.ServiceContext, authorization string, f filtrosReporte, modo, desdeB, hastaB string) (*comparacion, error) {
	f.Desde, f.Hasta = strings.TrimSpace(f.Desde), strings.TrimSpace(f.Hasta)
	db, hb, err := reportes.PeriodoB(f.Desde, f.Hasta, modo, strings.TrimSpace(desdeB), strings.TrimSpace(hastaB))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "validation_error: "+err.Error())
	}
	a, err := consultarReporte(ctx, svcCtx, authorization, f)
	if err != nil {
		return nil, err
	}
	fb := f
	fb.Desde, fb.Hasta = db, hb
	b, err := consultarReporte(ctx, svcCtx, authorization, fb)
	if err != nil {
		return nil, err
	}
	resumen := func(r *expedientesclient.ReporteExpedientesResponse) reportes.ResumenPeriodo {
		return reportes.ResumenPeriodo{Total: int(r.Total), Pendientes: int(r.Pendientes),
			PorEstado: conteosExp(r.PorEstado), PorTipo: conteosExp(r.PorTipo), PorPrioridad: conteosExp(r.PorPrioridad), PorArea: conteosExp(r.PorArea)}
	}
	return &comparacion{
		resultado: reportes.Comparar(resumen(a), resumen(b)), a: a, b: b, filtros: filtrosImpresos(f),
		desdeB: db, hastaB: hb, alcance: a.Alcance, areaAlcance: a.AreaAlcance, generado: generadoLima(a.Generado),
	}, nil
}

func (l *ReporteComparacionLogic) ReporteComparacion(req *types.ReporteComparacionRequest) (resp *types.ReporteComparacionResponse, err error) {
	c, err := calcularComparacion(l.ctx, l.svcCtx, req.Authorization, filtrosReporte{
		Desde: req.Desde, Hasta: req.Hasta, Estado: req.Estado, Tipo: req.Tipo, Prioridad: req.Prioridad, Area: req.Area,
	}, req.Modo, req.DesdeB, req.HastaB)
	if err != nil {
		return nil, err
	}
	filas := make([]types.ComparacionFilaDTO, 0, len(c.resultado.Filas))
	for _, f := range c.resultado.Filas {
		filas = append(filas, types.ComparacionFilaDTO{
			Grupo: f.Grupo, Clave: f.Clave, Actual: int32(f.Actual), Anterior: int32(f.Anterior),
			TieneVariacion: f.TieneVariacion, Variacion: f.Variacion,
		})
	}
	return &types.ReporteComparacionResponse{
		DesdeA: c.filtros.Desde, HastaA: c.filtros.Hasta, DesdeB: c.desdeB, HastaB: c.hastaB,
		TotalA: c.a.Total, TotalB: c.b.Total, Comparable: c.resultado.Comparable, Minimo: reportes.MinimoComparacion,
		Mensaje: c.resultado.Mensaje, Filas: filas, Alcance: c.alcance, AreaAlcance: c.areaAlcance, Generado: c.generado.Format(time.RFC3339),
	}, nil
}
