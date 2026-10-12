package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateEstudiantePersonalLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateEstudiantePersonalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateEstudiantePersonalLogic {
	return &UpdateEstudiantePersonalLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpdateEstudiantePersonal (FASE E-08): edición PARCIAL de los seis datos
// personales con "estudiantes.personal.update" y expected_version. Solo
// cambian los campos presentes ("" = NULL); se valida el resultado combinado
// (p. ej. ubigeo con país PE). Conflicto de versión = version_conflict
// (ABORTED, 409). Auditoría PERSONAL_UPDATE con la versión nueva y los
// nombres de los campos cambiados. Responde el registro BÁSICO (sin datos
// personales: editar no otorga lectura).
func (l *UpdateEstudiantePersonalLogic) UpdateEstudiantePersonal(in *estudiantes.UpdateEstudiantePersonalRequest) (*estudiantes.UpdateEstudiantePersonalResponse, error) {
	userID, err := autorizar(l.ctx, l.svcCtx, permisoPersonalUpdate)
	if err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.UpdateEstudiantePersonalRequest{}
	}
	id, err := validarID(in.Id)
	if err != nil {
		return nil, escrituraError(l.Logger, "editar datos personales", err)
	}
	if err := validarVersion(in.ExpectedVersion); err != nil {
		return nil, escrituraError(l.Logger, "editar datos personales", err)
	}

	updated, err := l.svcCtx.EstudianteRepository.UpdatePersonal(l.ctx, id, int(in.ExpectedVersion),
		repository.Auditoria{UsuarioID: userID},
		func(p *repository.Personal) ([]string, error) {
			antes := *p
			set := func(dst **string, v *string, norm func(string) *string) {
				if v != nil {
					*dst = norm(*v)
				}
			}
			set(&p.FechaNacimiento, in.FechaNacimiento, optTrim)
			set(&p.Sexo, in.Sexo, optUpper)
			set(&p.PaisNacimiento, in.PaisNacimiento, optUpper)
			set(&p.UbigeoNacimiento, in.UbigeoNacimiento, optTrim)
			set(&p.LenguaMaterna, in.LenguaMaterna, optUpper)
			set(&p.SegundaLengua, in.SegundaLengua, optUpper)
			// Domicilio y contacto (etapa 2): texto recortado y con espacios
			// colapsados; correo y teléfono solo recortados (no se reescriben).
			set(&p.DomicilioDireccion, in.DomicilioDireccion, optClean)
			set(&p.DomicilioDepartamento, in.DomicilioDepartamento, optClean)
			set(&p.DomicilioProvincia, in.DomicilioProvincia, optClean)
			set(&p.DomicilioDistrito, in.DomicilioDistrito, optClean)
			set(&p.DomicilioUbigeo, in.DomicilioUbigeo, optTrim)
			set(&p.Correo, in.Correo, optTrim)
			set(&p.Telefono, in.Telefono, optTrim)
			if err := validarPersonal(p); err != nil {
				return nil, err
			}
			return camposPersonalesModificados(&antes, p), nil
		})
	if err != nil {
		return nil, escrituraError(l.Logger, "editar datos personales", err)
	}
	return &estudiantes.UpdateEstudiantePersonalResponse{Estudiante: respuestaBasica(updated)}, nil
}
