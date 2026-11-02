// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"sort"
	"strings"
	"time"

	"gateway/internal/reportes"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ReporteActividadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReporteActividadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReporteActividadLogic {
	return &ReporteActividadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// actividad: resultado ya agrupado por ROL (sin IDs ni nombres).
type actividad struct {
	roles                []reportes.ActividadRol
	alcance, areaAlcance string
	generado             time.Time
}

// calcularActividad combina los conteos por usuario de Expedientes,
// Derivaciones y Documentos (cada servicio valida "reportes.view" y su
// alcance) y los agrupa por el ROL de cada usuario. Los IDs no salen de aquí.
func calcularActividad(ctx context.Context, svcCtx *svc.ServiceContext, authorization, desde, hasta, tipo string) (*actividad, error) {
	exp, err := consultarReporte(ctx, svcCtx, authorization, filtrosReporte{Desde: desde, Hasta: hasta, Tipo: tipo, IncluirRegistros: true})
	if err != nil {
		return nil, err
	}
	der, err := consultarDerivaciones(ctx, svcCtx, authorization, desde, hasta, tipo, "", true)
	if err != nil {
		return nil, err
	}
	doc, err := consultarDocumentos(ctx, svcCtx, authorization, desde, hasta, tipo, "", true)
	if err != nil {
		return nil, err
	}
	// porMov: todos los movimientos; porDer: solo los de tipo DERIVACION.
	porExp, porMov, porDer, porDoc := map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
	ids := map[string]bool{}
	for _, c := range exp.RegistrosPorUsuario {
		id := strings.ToLower(c.Clave)
		porExp[id] += int(c.Cantidad)
		ids[id] = true
	}
	for _, c := range der.RegistrosPorUsuario {
		id := strings.ToLower(c.Clave)
		porMov[id] += int(c.Cantidad)
		ids[id] = true
	}
	for _, c := range der.DerivacionesPorUsuario {
		porDer[strings.ToLower(c.Clave)] += int(c.Cantidad)
	}
	for _, c := range doc.RegistrosPorUsuario {
		id := strings.ToLower(c.Clave)
		porDoc[id] += int(c.Cantidad)
		ids[id] = true
	}
	lista := make([]string, 0, len(ids))
	for id := range ids {
		lista = append(lista, id)
	}
	sort.Strings(lista)
	roles, err := rolesDeUsuarios(ctx, svcCtx, authorization, lista)
	if err != nil {
		return nil, err
	}
	return &actividad{
		roles:   reportes.AgruparActividadPorRol(porExp, porMov, porDer, porDoc, roles),
		alcance: exp.Alcance, areaAlcance: exp.AreaAlcance, generado: generadoLima(exp.Generado),
	}, nil
}

// ReporteActividad: actividad institucional agrupada por ROL (nunca por
// persona): expedientes registrados, movimientos registrados (de ellos,
// derivaciones entre áreas) y documentos subidos en el periodo.
func (l *ReporteActividadLogic) ReporteActividad(req *types.ReportePeriodoRequest) (resp *types.ReporteActividadResponse, err error) {
	a, err := calcularActividad(l.ctx, l.svcCtx, req.Authorization, req.Desde, req.Hasta, req.Tipo)
	if err != nil {
		return nil, err
	}
	out := make([]types.ActividadRolDTO, 0, len(a.roles))
	for _, r := range a.roles {
		out = append(out, types.ActividadRolDTO{
			Rol: r.Rol, ExpedientesRegistrados: int32(r.Expedientes), MovimientosRegistrados: int32(r.Movimientos), DerivacionesRegistradas: int32(r.Derivaciones),
			DocumentosSubidos: int32(r.Documentos), Total: int32(r.Total),
		})
	}
	return &types.ReporteActividadResponse{Roles: out, Alcance: a.alcance, AreaAlcance: a.areaAlcance, Generado: a.generado.Format(time.RFC3339)}, nil
}
