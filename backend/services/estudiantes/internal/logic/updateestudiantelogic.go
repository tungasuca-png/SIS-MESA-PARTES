package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"
	"estudiantes/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateEstudianteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateEstudianteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateEstudianteLogic {
	return &UpdateEstudianteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpdateEstudiante (FASE E-05): edición PARCIAL del maestro con
// "estudiantes.update" y control de concurrencia optimista (expected_version).
// Solo cambian los campos presentes en la solicitud; luego se valida el
// registro COMPLETO resultante (p. ej. tipo y número de documento juntos).
// No modifica el código institucional, el estado (ChangeEstadoEstudiante) ni
// las columnas académicas.
func (l *UpdateEstudianteLogic) UpdateEstudiante(in *estudiantes.UpdateEstudianteRequest) (*estudiantes.UpdateEstudianteResponse, error) {
	userID, err := autorizar(l.ctx, l.svcCtx, "estudiantes.update")
	if err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.UpdateEstudianteRequest{}
	}
	// FASE E-08: los datos personales tienen su propio RPC y permiso. Si la
	// solicitud trae CUALQUIERA (aunque sea vacío), se rechaza completa antes
	// de tocar la base: nada cambia, ni versión ni auditoría.
	if in.FechaNacimiento != nil || in.Sexo != nil || in.PaisNacimiento != nil ||
		in.UbigeoNacimiento != nil || in.LenguaMaterna != nil || in.SegundaLengua != nil {
		return nil, escrituraError(l.Logger, "editar", errPersonalFueraDeUpdate)
	}
	id, err := validarID(in.Id)
	if err != nil {
		return nil, escrituraError(l.Logger, "editar", err)
	}
	if err := validarVersion(in.ExpectedVersion); err != nil {
		return nil, escrituraError(l.Logger, "editar", err)
	}

	updated, err := l.svcCtx.EstudianteRepository.Update(l.ctx, id, int(in.ExpectedVersion),
		repository.Auditoria{UsuarioID: userID, Accion: repository.AccionUpdate},
		func(cur *repository.Maestro) ([]string, error) {
			antes := *cur
			if err := aplicarCambios(cur, in); err != nil {
				return nil, err
			}
			if err := validarMaestro(cur); err != nil {
				return nil, err
			}
			return camposModificados(&antes, cur), nil
		})
	if err != nil {
		return nil, escrituraError(l.Logger, "editar", err)
	}
	return &estudiantes.UpdateEstudianteResponse{Estudiante: respuestaBasica(updated)}, nil
}

// aplicarCambios copia al registro actual solo los campos presentes, ya
// normalizados. "" deja un opcional en NULL; en los obligatorios lo rechaza
// validarMaestro. Los apellidos se resuelven al final (ver resolverApellidos).
func aplicarCambios(cur *repository.Maestro, in *estudiantes.UpdateEstudianteRequest) error {
	set := func(dst **string, v *string, norm func(string) *string) {
		if v != nil {
			*dst = norm(*v)
		}
	}
	set(&cur.CodigoSiagie, in.CodigoSiagie, optTrim)
	set(&cur.NumeroDocumento, in.NumeroDocumento, optUpper)
	set(&cur.ApellidoPaterno, in.ApellidoPaterno, optClean)
	set(&cur.ApellidoMaterno, in.ApellidoMaterno, optClean)
	// Datos personales: nunca aquí (FASE E-08, ver UpdateEstudiantePersonal).
	set(&cur.FotoRef, in.FotoRef, optTrim)
	if in.TipoDocumento != nil {
		cur.TipoDocumento = validation.Upper(*in.TipoDocumento)
	}
	if in.Nombres != nil {
		cur.Nombres = validation.CleanText(*in.Nombres)
	}
	return resolverApellidos(cur, in.Apellidos)
}
