package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"
	"estudiantes/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChangeEstadoEstudianteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChangeEstadoEstudianteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangeEstadoEstudianteLogic {
	return &ChangeEstadoEstudianteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ChangeEstadoEstudiante (FASE E-05): activa o inactiva el REGISTRO del
// estudiante con "estudiantes.change_estado". Es la única "baja" del maestro
// (no hay borrado físico). Solo ACTIVO/INACTIVO: RETIRADO, TRASLADADO o
// CONCLUIDO pertenecen a la futura Matrícula. También exige expected_version,
// para no inactivar sobre una vista desactualizada.
func (l *ChangeEstadoEstudianteLogic) ChangeEstadoEstudiante(in *estudiantes.ChangeEstadoEstudianteRequest) (*estudiantes.ChangeEstadoEstudianteResponse, error) {
	userID, err := autorizar(l.ctx, l.svcCtx, "estudiantes.change_estado")
	if err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.ChangeEstadoEstudianteRequest{}
	}
	id, err := validarID(in.Id)
	if err != nil {
		return nil, escrituraError(l.Logger, "cambiar estado", err)
	}
	if err := validarVersion(in.ExpectedVersion); err != nil {
		return nil, escrituraError(l.Logger, "cambiar estado", err)
	}
	estado := validation.Upper(in.Estado)
	if !validation.IsValidEstadoRegistro(estado) {
		return nil, escrituraError(l.Logger, "cambiar estado", validation.EstadoInvalido())
	}

	updated, err := l.svcCtx.EstudianteRepository.ChangeEstado(l.ctx, id, int(in.ExpectedVersion), estado,
		repository.Auditoria{UsuarioID: userID, Accion: repository.AccionChangeEstado})
	if err != nil {
		return nil, escrituraError(l.Logger, "cambiar estado", err)
	}
	return &estudiantes.ChangeEstadoEstudianteResponse{Estudiante: respuestaBasica(updated)}, nil
}
