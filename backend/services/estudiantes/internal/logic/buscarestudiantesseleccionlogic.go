package logic

import (
	"context"
	"regexp"
	"strings"
	"unicode/utf8"

	"estudiantes/estudiantes"
	"estudiantes/internal/interceptor"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"
	"estudiantes/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type BuscarEstudiantesSeleccionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBuscarEstudiantesSeleccionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BuscarEstudiantesSeleccionLogic {
	return &BuscarEstudiantesSeleccionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

const (
	seleccionMaxTerminos   = 5
	seleccionMaxPageSize   = 20
	seleccionMaxPagina     = 500
	seleccionPermisoBuscar = "estudiantes.search.basic"
)

var soloDigitos = regexp.MustCompile(`^[0-9]+$`)

// BuscarEstudiantesSeleccion: búsqueda MÍNIMA para seleccionar un estudiante
// (sistemas consumidores con estudiantes.search.basic; ningún rol humano lo
// tiene). Devuelve solo id, nombre completo, grado y sección. La consulta
// nunca se registra (puede contener un DNI): los errores solo resumen la
// causa técnica.
func (l *BuscarEstudiantesSeleccionLogic) BuscarEstudiantesSeleccion(in *estudiantes.BuscarEstudiantesSeleccionRequest) (*estudiantes.BuscarEstudiantesSeleccionResponse, error) {
	// Exclusiva de sistemas consumidores: un JWT humano no la sustituye,
	// aunque algún rol tuviera el permiso.
	if _, ok := interceptor.ServiceFromContext(l.ctx); !ok {
		return nil, status.Error(codes.PermissionDenied, "permission_denied: operación exclusiva de sistemas consumidores")
	}
	if _, err := autorizar(l.ctx, l.svcCtx, seleccionPermisoBuscar); err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.BuscarEstudiantesSeleccionRequest{}
	}
	filtro, sinResultados, err := parsearConsultaSeleccion(in.Q)
	if err != nil {
		return nil, err
	}
	vacio := &estudiantes.BuscarEstudiantesSeleccionResponse{Estudiantes: []*estudiantes.EstudianteSeleccion{}}
	if sinResultados {
		return vacio, nil
	}
	filtro.Page, filtro.PageSize = paginaSeleccion(in.Page, in.PageSize)

	items, total, err := l.svcCtx.EstudianteRepository.BuscarSeleccion(l.ctx, filtro)
	if err != nil {
		l.Errorf("búsqueda de selección de estudiantes: %s", repository.SafeErrorSummary(err))
		return nil, status.Error(codes.Internal, "no se pudo buscar estudiantes")
	}
	out := make([]*estudiantes.EstudianteSeleccion, 0, len(items))
	for _, it := range items {
		out = append(out, &estudiantes.EstudianteSeleccion{Id: it.ID, NombreCompleto: it.NombreCompleto, Grado: it.Grado, Seccion: it.Seccion})
	}
	return &estudiantes.BuscarEstudiantesSeleccionResponse{Estudiantes: out, Total: int32(total)}, nil
}

// parsearConsultaSeleccion separa la consulta en términos de nombre y DNI
// completos. sinResultados=true cuando un término numérico no es un DNI de 8
// dígitos (DNI parcial, código numérico): no se consulta la base.
func parsearConsultaSeleccion(q string) (repository.SeleccionFilter, bool, error) {
	var f repository.SeleccionFilter
	q = strings.TrimSpace(q)
	if n := utf8.RuneCountInString(q); n < validation.MinQueryLength || n > validation.MaxQueryLength {
		return f, false, status.Errorf(codes.InvalidArgument, "validation_error: la búsqueda debe tener entre %d y %d caracteres", validation.MinQueryLength, validation.MaxQueryLength)
	}
	terminos := strings.Fields(q)
	if len(terminos) > seleccionMaxTerminos {
		return f, false, status.Errorf(codes.InvalidArgument, "validation_error: use como máximo %d palabras", seleccionMaxTerminos)
	}
	for _, t := range terminos {
		if soloDigitos.MatchString(t) {
			if len(t) != 8 {
				return f, true, nil
			}
			f.DNIs = append(f.DNIs, t)
			continue
		}
		f.Nombres = append(f.Nombres, t)
	}
	return f, false, nil
}

func paginaSeleccion(page, pageSize int32) (int, int) {
	p, s := int(page), int(pageSize)
	if p <= 0 {
		p = 1
	}
	if p > seleccionMaxPagina {
		p = seleccionMaxPagina
	}
	if s <= 0 || s > seleccionMaxPageSize {
		s = seleccionMaxPageSize
	}
	return p, s
}
