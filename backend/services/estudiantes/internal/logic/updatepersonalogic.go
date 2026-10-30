package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"
	"estudiantes/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdatePersonaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdatePersonaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdatePersonaLogic {
	return &UpdatePersonaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpdatePersona (etapa 5): corrige los datos de identidad de una persona con
// control de versión ("estudiantes.familia.update"). El documento se valida
// sobre el resultado final (cambiar a SIN_DOCUMENTO exige numero_documento
// ""), y no puede repetir el de otra persona. Auditado como PERSONA_UPDATE
// con los nombres de los campos cambiados.
func (l *UpdatePersonaLogic) UpdatePersona(in *estudiantes.UpdatePersonaRequest) (*estudiantes.PersonaResponse, error) {
	userID, err := autorizar(l.ctx, l.svcCtx, permisoFamiliaUpdate)
	if err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.UpdatePersonaRequest{}
	}
	id, err := validarID(in.Id)
	if err != nil {
		return nil, escrituraError(l.Logger, "editar persona", err)
	}
	if err := validarVersion(in.ExpectedVersion); err != nil {
		return nil, escrituraError(l.Logger, "editar persona", err)
	}
	if in.TipoDocumento == nil && in.NumeroDocumento == nil && in.Nombres == nil && in.ApellidoPaterno == nil && in.ApellidoMaterno == nil {
		return nil, escrituraError(l.Logger, "editar persona", validation.Invalid("no se indicó ningún campo para modificar"))
	}

	p, err := l.svcCtx.EstudianteRepository.UpdatePersona(l.ctx, id, int(in.ExpectedVersion), repository.Auditoria{UsuarioID: userID},
		func(cur *repository.PersonaDatos) error {
			if in.TipoDocumento != nil {
				cur.TipoDocumento = validation.Upper(*in.TipoDocumento)
			}
			if in.NumeroDocumento != nil {
				cur.NumeroDocumento = optUpper(*in.NumeroDocumento)
			}
			if in.Nombres != nil {
				cur.Nombres = validation.CleanText(*in.Nombres)
			}
			if in.ApellidoPaterno != nil {
				cur.ApellidoPaterno = optClean(*in.ApellidoPaterno)
			}
			if in.ApellidoMaterno != nil {
				cur.ApellidoMaterno = optClean(*in.ApellidoMaterno)
			}
			return validarPersona(cur)
		})
	if err != nil {
		return nil, escrituraError(l.Logger, "editar persona", err)
	}
	return &estudiantes.PersonaResponse{Persona: personaToProto(p)}, nil
}
