package logic

import (
	"context"
	"strings"
	"unicode/utf8"

	"psicologia/internal/authorization"
	"psicologia/internal/svc"
	"psicologia/psicologia"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type BuscarEstudiantesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBuscarEstudiantesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BuscarEstudiantesLogic {
	return &BuscarEstudiantesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// permisosBusquedaEstudiantes: los que hoy habilitan "Nuevo expediente"
// (create) y "Buscar alumno" (view / view_own). Basta uno.
var permisosBusquedaEstudiantes = []string{permExpedientesCreate, permExpedientesView, permExpedientesViewOwn}

const (
	busquedaMinCaracteres = 2
	busquedaMaxCaracteres = 100
)

// BuscarEstudiantes: búsqueda de estudiantes para seleccionarlos, resuelta por
// Estudiantes con la credencial de servicio de Psicología (nunca con el JWT
// del usuario). Los permisos del USUARIO se comprueban antes; sin ninguno, no
// se consulta Estudiantes. La respuesta solo trae id, nombre completo, grado y
// sección. La consulta (puede ser un DNI) nunca se registra.
func (l *BuscarEstudiantesLogic) BuscarEstudiantes(in *psicologia.BuscarEstudiantesRequest) (*psicologia.BuscarEstudiantesResponse, error) {
	userID, err := authenticatedUser(l.ctx)
	if err != nil {
		return nil, err
	}
	permitido := false
	for _, permiso := range permisosBusquedaEstudiantes {
		ok, err := authorization.HasPermission(l.ctx, l.svcCtx.AuthClient, userID, permiso)
		if err != nil {
			return nil, err
		}
		if ok {
			permitido = true
			break
		}
	}
	if !permitido {
		return nil, denied(l.ctx, l.svcCtx, userID, "", "", motivoPermisoInsuficiente)
	}
	if in == nil {
		in = &psicologia.BuscarEstudiantesRequest{}
	}
	q := strings.TrimSpace(in.Q)
	if n := utf8.RuneCountInString(q); n < busquedaMinCaracteres || n > busquedaMaxCaracteres {
		return nil, status.Errorf(codes.InvalidArgument, "la búsqueda debe tener entre %d y %d caracteres", busquedaMinCaracteres, busquedaMaxCaracteres)
	}
	if l.svcCtx.EstudiantesBuscador == nil {
		return nil, status.Error(codes.Unavailable, "la búsqueda de estudiantes no está disponible")
	}
	resp, err := l.svcCtx.EstudiantesBuscador.Buscar(l.ctx, q, in.Page, in.PageSize)
	if err != nil {
		l.Errorf("búsqueda de estudiantes no disponible (%s)", status.Code(err))
		return nil, err
	}
	out := make([]*psicologia.EstudianteBusqueda, 0, len(resp.GetEstudiantes()))
	for _, e := range resp.GetEstudiantes() {
		out = append(out, &psicologia.EstudianteBusqueda{Id: e.GetId(), NombreCompleto: e.GetNombreCompleto(), Grado: e.GetGrado(), Seccion: e.GetSeccion()})
	}
	return &psicologia.BuscarEstudiantesResponse{Estudiantes: out, Total: resp.GetTotal()}, nil
}
