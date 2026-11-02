package logic

import (
	"context"
	"sort"
	"strings"
	"time"

	"expedientes/expedientes"
	"expedientes/internal/authorization"
	"expedientes/internal/estados"
	"expedientes/internal/interceptor"
	"expedientes/internal/repository"
	"expedientes/internal/svc"
	"expedientes/reportes"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Reportes (Fase 1): estadísticas agregadas de expedientes. Ver
// docs/reportes/modulo-reportes.md.

const (
	permisoReportes = "reportes.view"
	formatoFecha    = "2006-01-02"
	// Periodo máximo de una consulta (desde-hasta).
	maxAniosReporte    = 10
	alcanceInstitucion = "INSTITUCION"
	alcanceArea        = "AREA"
)

// Perú no tiene horario de verano: UTC-5 fijo (sin depender de tzdata del
// sistema operativo). La consulta SQL usa la misma zona (America/Lima).
var zonaReportes = time.FixedZone("America/Lima", -5*60*60)

// ahoraReporte se reemplaza en pruebas.
var ahoraReporte = time.Now

// Catálogos en el orden en que se presentan (incluyen los ceros).
var (
	ordenEstados     = []string{estados.EstadoPendiente, estados.EstadoEnProceso, estados.EstadoObservado, estados.EstadoAtendido}
	ordenPrioridades = []string{estados.PrioridadNormal, estados.PrioridadUrgente}
	ordenTipos       = []string{
		estados.TipoCertificado, estados.TipoConstancia, estados.TipoPermiso,
		estados.TipoJustificacionFalta, estados.TipoJustificacionTardanza,
		estados.TipoSolicitud, estados.TipoTramite, estados.TipoOficio, estados.TipoOtro,
	}
	ordenAreas = []string{
		estados.AreaSecretaria, estados.AreaDirector, estados.AreaSubdirector,
		estados.AreaDocente, estados.AreaAuxiliar, estados.AreaAdmin,
	}
)

// Rangos de antigüedad de los pendientes (días desde fecha_registro).
// Descriptivos: no representan plazos institucionales.
var rangosAntiguedad = []struct {
	clave string
	hasta int // inclusive; -1 = sin límite
}{
	{"0-3", 3}, {"4-7", 7}, {"8-15", 15}, {"16-30", 30}, {"31+", -1},
}

type ReporteExpedientesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReporteExpedientesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReporteExpedientesLogic {
	return &ReporteExpedientesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ReporteExpedientesLogic) ReporteExpedientes(in *expedientes.ReporteExpedientesRequest) (*expedientes.ReporteExpedientesResponse, error) {
	userID, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if err := authorization.RequirePermission(l.ctx, l.svcCtx.AuthClient, userID, permisoReportes); err != nil {
		return nil, err
	}
	// Defensa en profundidad: los reportes son del personal interno, aunque
	// por error se asignara el permiso a un rol externo.
	if !authorization.IsInternal(role) {
		return nil, status.Error(codes.PermissionDenied, "permission_denied: los reportes son exclusivos del personal interno")
	}

	ahora := ahoraReporte().In(zonaReportes)
	filtro, err := validarFiltroReporte(in, ahora)
	if err != nil {
		return nil, err
	}
	// Fase 2 (opcional): serie por día, semana o mes y registros por usuario.
	granularidad := ""
	if strings.TrimSpace(in.GetGranularidad()) != "" {
		if granularidad, err = reportes.Granularidad(in.GetGranularidad()); err != nil {
			return nil, invalido(err.Error())
		}
		filtro.ConDia = true
	}
	filtro.ConRegistrador = in.GetIncluirRegistros()
	// Mismo alcance que ListExpedientes: ADMIN/SECRETARIA ven toda la
	// institución; el resto, solo lo que está actualmente en su área.
	alcance := alcanceInstitucion
	if !authorization.CanViewAll(role) {
		filtro.AreaAlcance = authorization.AreaDelRol(role)
		alcance = alcanceArea
	}

	filas, err := l.svcCtx.ExpedienteRepository.Reporte(l.ctx, filtro)
	if err != nil {
		l.Errorf("reporte de expedientes: %v", err)
		return nil, status.Error(codes.Internal, "no se pudo generar el reporte")
	}
	resp := armarReporte(filas, filtro, ahora)
	if err := completarFase2(resp, filas, filtro, granularidad, ahora); err != nil {
		return nil, err
	}
	resp.Alcance = alcance
	resp.AreaAlcance = filtro.AreaAlcance
	resp.Generado = ahora.Format(time.RFC3339)
	return resp, nil
}

func invalido(msg string) error {
	return status.Error(codes.InvalidArgument, "validation_error: "+msg)
}

// validarFiltroReporte normaliza y valida los filtros. Los mensajes no
// repiten los valores recibidos.
func validarFiltroReporte(in *expedientes.ReporteExpedientesRequest, ahora time.Time) (repository.ReporteFilter, error) {
	f := repository.ReporteFilter{
		Desde:     strings.TrimSpace(in.GetDesde()),
		Hasta:     strings.TrimSpace(in.GetHasta()),
		Estado:    strings.ToUpper(strings.TrimSpace(in.GetEstado())),
		Tipo:      strings.ToUpper(strings.TrimSpace(in.GetTipo())),
		Prioridad: strings.ToUpper(strings.TrimSpace(in.GetPrioridad())),
		Area:      strings.ToUpper(strings.TrimSpace(in.GetArea())),
		Hoy:       ahora.Format(formatoFecha),
	}
	if f.Estado != "" && !estados.IsValidEstado(f.Estado) {
		return f, invalido("el estado indicado no es válido")
	}
	if f.Tipo != "" && !estados.IsValidTipo(f.Tipo) {
		return f, invalido("el tipo de trámite indicado no es válido")
	}
	if f.Prioridad != "" && !estados.IsValidPrioridad(f.Prioridad) {
		return f, invalido("la prioridad indicada no es válida")
	}
	if f.Area != "" && !estados.IsValidArea(f.Area) {
		return f, invalido("el área indicada no es válida")
	}
	var desde, hasta time.Time
	var err error
	if f.Desde != "" {
		if desde, err = time.ParseInLocation(formatoFecha, f.Desde, zonaReportes); err != nil {
			return f, invalido("la fecha desde debe tener el formato AAAA-MM-DD")
		}
	}
	if f.Hasta != "" {
		if hasta, err = time.ParseInLocation(formatoFecha, f.Hasta, zonaReportes); err != nil {
			return f, invalido("la fecha hasta debe tener el formato AAAA-MM-DD")
		}
	}
	minimo := time.Date(2000, 1, 1, 0, 0, 0, 0, zonaReportes)
	hoy := time.Date(ahora.Year(), ahora.Month(), ahora.Day(), 0, 0, 0, 0, zonaReportes)
	if f.Desde != "" && (desde.Before(minimo) || desde.After(hoy)) {
		return f, invalido("la fecha desde debe estar entre 2000-01-01 y hoy")
	}
	if f.Hasta != "" && hasta.Before(minimo) {
		return f, invalido("la fecha hasta no puede ser anterior a 2000-01-01")
	}
	if f.Desde != "" && f.Hasta != "" {
		if hasta.Before(desde) {
			return f, invalido("la fecha desde no puede ser posterior a la fecha hasta")
		}
		if hasta.After(desde.AddDate(maxAniosReporte, 0, 0)) {
			return f, invalido("el periodo no puede superar 10 años")
		}
	}
	return f, nil
}

// rangoAntiguedad devuelve la clave del rango para unos días de antigüedad.
func rangoAntiguedad(dias int) string {
	for _, r := range rangosAntiguedad {
		if r.hasta < 0 || dias <= r.hasta {
			return r.clave
		}
	}
	return rangosAntiguedad[len(rangosAntiguedad)-1].clave
}

// mesesContinuos: todos los meses AAAA-MM entre inicio y fin (inclusive).
func mesesContinuos(inicio, fin string) []string {
	a, errA := time.Parse("2006-01", inicio)
	b, errB := time.Parse("2006-01", fin)
	if errA != nil || errB != nil || b.Before(a) {
		return nil
	}
	out := []string{}
	for m := a; !m.After(b); m = m.AddDate(0, 1, 0) {
		out = append(out, m.Format("2006-01"))
	}
	return out
}

func conteos(orden []string, valores map[string]int) []*expedientes.ReporteConteo {
	out := make([]*expedientes.ReporteConteo, 0, len(orden))
	vistos := map[string]bool{}
	for _, clave := range orden {
		out = append(out, &expedientes.ReporteConteo{Clave: clave, Cantidad: int32(valores[clave])})
		vistos[clave] = true
	}
	// Valores fuera del catálogo (no debería ocurrir por los CHECK de la
	// tabla): se agregan al final para que los totales cuadren.
	extra := []string{}
	for clave := range valores {
		if !vistos[clave] {
			extra = append(extra, clave)
		}
	}
	sort.Strings(extra)
	for _, clave := range extra {
		out = append(out, &expedientes.ReporteConteo{Clave: clave, Cantidad: int32(valores[clave])})
	}
	return out
}

// armarReporte construye todas las agrupaciones a partir de los grupos de la
// consulta agregada.
func armarReporte(filas []repository.ReporteFila, f repository.ReporteFilter, ahora time.Time) *expedientes.ReporteExpedientesResponse {
	porEstado, porTipo, porPrioridad, porArea, porMes, antig := map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
	total, pendientes := 0, 0
	mesMin, mesMax := "", ""
	for _, fila := range filas {
		total += fila.Cantidad
		porEstado[fila.Estado] += fila.Cantidad
		porTipo[fila.Tipo] += fila.Cantidad
		porPrioridad[fila.Prioridad] += fila.Cantidad
		porArea[fila.Area] += fila.Cantidad
		porMes[fila.Mes] += fila.Cantidad
		if mesMin == "" || fila.Mes < mesMin {
			mesMin = fila.Mes
		}
		if fila.Mes > mesMax {
			mesMax = fila.Mes
		}
		if fila.DiasPendiente != nil {
			pendientes += fila.Cantidad
			antig[rangoAntiguedad(*fila.DiasPendiente)] += fila.Cantidad
		}
	}

	// Serie mensual continua: del inicio del periodo (o del primer dato) al
	// fin del periodo (o al mes actual).
	inicio, fin := mesMin, ahora.Format("2006-01")
	if f.Desde != "" {
		inicio = f.Desde[:7]
	}
	if f.Hasta != "" {
		fin = f.Hasta[:7]
	}
	serie := []*expedientes.ReporteConteo{}
	if inicio != "" {
		meses := mesesContinuos(inicio, fin)
		// Un dato fuera de la serie (no debería ocurrir) no se pierde.
		if mesMax > fin {
			meses = mesesContinuos(inicio, mesMax)
		}
		serie = conteos(meses, porMes)
	}

	claves := make([]string, len(rangosAntiguedad))
	for i, r := range rangosAntiguedad {
		claves[i] = r.clave
	}
	return &expedientes.ReporteExpedientesResponse{
		Total:                   int32(total),
		PorEstado:               conteos(ordenEstados, porEstado),
		PorTipo:                 conteos(ordenTipos, porTipo),
		PorPrioridad:            conteos(ordenPrioridades, porPrioridad),
		PorArea:                 conteos(ordenAreas, porArea),
		PorMes:                  serie,
		Pendientes:              int32(pendientes),
		PendientesPorAntiguedad: conteos(claves, antig),
	}
}

// completarFase2 agrega las tablas cruzadas (siempre), la serie por la
// granularidad pedida y los registros por usuario (si se pidieron). No
// modifica los campos de la Fase 1.
func completarFase2(resp *expedientes.ReporteExpedientesResponse, filas []repository.ReporteFila, f repository.ReporteFilter, granularidad string, ahora time.Time) error {
	areaEstado, tipoEstado := map[[2]string]int{}, map[[2]string]int{}
	porDia, porUsuario := map[string]int{}, map[string]int{}
	porDiaEstado := map[string]map[string]int{} // estado -> día -> cantidad
	for _, fila := range filas {
		areaEstado[[2]string{fila.Area, fila.Estado}] += fila.Cantidad
		tipoEstado[[2]string{fila.Tipo, fila.Estado}] += fila.Cantidad
		if fila.Dia != "" {
			porDia[fila.Dia] += fila.Cantidad
			if porDiaEstado[fila.Estado] == nil {
				porDiaEstado[fila.Estado] = map[string]int{}
			}
			porDiaEstado[fila.Estado][fila.Dia] += fila.Cantidad
		}
		if fila.Registrador != "" {
			porUsuario[fila.Registrador] += fila.Cantidad
		}
	}
	resp.AreaEstado = cruces(ordenAreas, areaEstado)
	resp.TipoEstado = cruces(ordenTipos, tipoEstado)
	if granularidad != "" {
		serie, err := reportes.Serie(porDia, granularidad, f.Desde, f.Hasta, ahora)
		if err != nil {
			return invalido(err.Error())
		}
		resp.Granularidad = granularidad
		resp.Serie = make([]*expedientes.ReporteConteo, 0, len(serie))
		for _, c := range serie {
			resp.Serie = append(resp.Serie, &expedientes.ReporteConteo{Clave: c.Clave, Cantidad: int32(c.Cantidad)})
		}
		// Evolución por estado ACTUAL: misma serie (mismas claves) separada por
		// estado; solo celdas con valor.
		porClave := map[[2]string]int{}
		for estado, dias := range porDiaEstado {
			for dia, n := range dias {
				if clave, ok := reportes.Clave(dia, granularidad); ok {
					porClave[[2]string{clave, estado}] += n
				}
			}
		}
		for _, c := range serie {
			for _, estado := range ordenEstados {
				if n := porClave[[2]string{c.Clave, estado}]; n > 0 {
					resp.SerieEstado = append(resp.SerieEstado, &expedientes.ReporteCruce{Fila: c.Clave, Columna: estado, Cantidad: int32(n)})
				}
			}
		}
	}
	if f.ConRegistrador {
		claves := make([]string, 0, len(porUsuario))
		for k := range porUsuario {
			claves = append(claves, k)
		}
		sort.Strings(claves)
		for _, k := range claves {
			resp.RegistrosPorUsuario = append(resp.RegistrosPorUsuario, &expedientes.ReporteConteo{Clave: k, Cantidad: int32(porUsuario[k])})
		}
	}
	return nil
}

// cruces: celdas con valor, en orden de filas (catálogo) y de estados.
func cruces(ordenFilas []string, valores map[[2]string]int) []*expedientes.ReporteCruce {
	out := []*expedientes.ReporteCruce{}
	filas := append([]string{}, ordenFilas...)
	vistas := map[string]bool{}
	for _, f := range filas {
		vistas[f] = true
	}
	extra := []string{}
	for k := range valores {
		if !vistas[k[0]] {
			vistas[k[0]] = true
			extra = append(extra, k[0])
		}
	}
	sort.Strings(extra)
	for _, fila := range append(filas, extra...) {
		for _, estado := range ordenEstados {
			if n := valores[[2]string{fila, estado}]; n > 0 {
				out = append(out, &expedientes.ReporteCruce{Fila: fila, Columna: estado, Cantidad: int32(n)})
			}
		}
	}
	return out
}
