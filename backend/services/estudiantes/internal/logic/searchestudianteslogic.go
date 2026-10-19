package logic

import (
	"context"
	"strings"
	"unicode/utf8"

	"estudiantes/estudiantes"
	"estudiantes/internal/authorization"
	"estudiantes/internal/interceptor"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"
	"estudiantes/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

type SearchEstudiantesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSearchEstudiantesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchEstudiantesLogic {
	return &SearchEstudiantesLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// SearchEstudiantes (FASE 5B): autenticación + "estudiantes.view" (mismo
// criterio que GetEstudiante, ver su comentario).
//
// FASE E-04: listado del maestro con búsqueda y filtros.
//   - q vacío o solo espacios = listado general; si trae texto, mínimo 2
//     caracteres (se mantiene el InvalidArgument para 1 carácter).
//   - q se separa en palabras: todas deben coincidir, en cualquier orden y
//     sin distinguir mayúsculas ni tildes (ver repository.Search).
//   - Filtros opcionales por coincidencia exacta; estado solo ACTIVO/INACTIVO.
//   - Nunca se registra q ni los filtros en logs (pueden contener un DNI o un
//     nombre completo); solo el error técnico.
func (l *SearchEstudiantesLogic) SearchEstudiantes(in *estudiantes.SearchEstudiantesRequest) (*estudiantes.SearchEstudiantesResponse, error) {
	userID, _, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if err := authorization.RequirePermission(l.ctx, l.svcCtx.AuthClient, userID, "estudiantes.view"); err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.SearchEstudiantesRequest{}
	}

	filter := repository.SearchFilter{
		Page:     int(in.Page),
		PageSize: int(in.PageSize),
		Estado:   strings.ToUpper(strings.TrimSpace(in.Estado)),
		Nivel:    strings.TrimSpace(in.Nivel),
		Grado:    strings.TrimSpace(in.Grado),
		Seccion:  strings.TrimSpace(in.Seccion),
		Turno:    strings.TrimSpace(in.Turno),
		Orden:    strings.ToLower(strings.TrimSpace(in.Orden)),
	}
	// Etapa 6: solo los órdenes de la lista blanca del repositorio.
	if _, ok := repository.OrdenesBusqueda[filter.Orden]; !ok {
		return nil, status.Error(codes.InvalidArgument, "validation_error: orden inválido: use apellidos, codigo o recientes")
	}

	q := strings.TrimSpace(in.Q)
	if q != "" {
		if !validation.QueryLenOK(q) {
			return nil, status.Error(codes.InvalidArgument, "la búsqueda debe tener al menos 2 caracteres")
		}
		if utf8.RuneCountInString(q) > validation.MaxQueryLength {
			return nil, status.Error(codes.InvalidArgument, "la búsqueda es demasiado larga")
		}
		filter.Terms = validation.QueryTerms(q)
		if len(filter.Terms) > validation.MaxQueryTerms {
			return nil, status.Error(codes.InvalidArgument, "la búsqueda tiene demasiadas palabras")
		}
	}

	if filter.Estado != "" && !validation.IsValidEstadoRegistro(filter.Estado) {
		return nil, status.Error(codes.InvalidArgument, "estado inválido: use ACTIVO o INACTIVO")
	}
	for _, v := range []string{filter.Nivel, filter.Grado, filter.Seccion, filter.Turno} {
		if !validation.FilterLenOK(v) {
			return nil, status.Error(codes.InvalidArgument, "valor de filtro demasiado largo")
		}
	}

	// Mismo criterio ya usado por Usuarios/Expedientes/Psicología: fuera de
	// rango se recorta a un valor seguro en vez de rechazar la solicitud.
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 || filter.PageSize > maxPageSize {
		filter.PageSize = defaultPageSize
	}

	items, total, err := l.svcCtx.EstudianteRepository.Search(l.ctx, filter)
	if err != nil {
		l.Errorf("buscar estudiantes: %s", repository.SafeErrorSummary(err))
		return nil, status.Error(codes.Internal, "no se pudo buscar estudiantes")
	}

	out := make([]*estudiantes.EstudianteBasico, 0, len(items))
	for _, item := range items {
		out = append(out, toProtoBasic(item))
	}
	return &estudiantes.SearchEstudiantesResponse{Estudiantes: out, Total: int32(total)}, nil
}
