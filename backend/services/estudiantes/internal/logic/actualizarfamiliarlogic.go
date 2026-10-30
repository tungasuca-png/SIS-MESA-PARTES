package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ActualizarFamiliarLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewActualizarFamiliarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ActualizarFamiliarLogic {
	return &ActualizarFamiliarLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ActualizarFamiliar (etapa 5): asigna o quita el rol de apoderado de una
// relación vigente ("estudiantes.familia.update"). Para cambiar de apoderado
// primero se quita al actual (un solo apoderado vigente). El parentesco no se
// edita: se desvincula y se vincula de nuevo, y el historial se conserva.
// Auditado como FAMILIA_UPDATE.
func (l *ActualizarFamiliarLogic) ActualizarFamiliar(in *estudiantes.ActualizarFamiliarRequest) (*estudiantes.ActualizarFamiliarResponse, error) {
	userID, err := autorizar(l.ctx, l.svcCtx, permisoFamiliaUpdate)
	if err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.ActualizarFamiliarRequest{}
	}
	estudianteID, err := validarID(in.EstudianteId)
	if err != nil {
		return nil, escrituraError(l.Logger, "actualizar familiar", err)
	}
	relacionID, err := validarID(in.RelacionId)
	if err != nil {
		return nil, escrituraError(l.Logger, "actualizar familiar", err)
	}
	f, err := l.svcCtx.EstudianteRepository.ActualizarFamiliar(l.ctx, estudianteID, relacionID, in.EsApoderado, repository.Auditoria{UsuarioID: userID})
	if err != nil {
		return nil, escrituraError(l.Logger, "actualizar familiar", err)
	}
	return &estudiantes.ActualizarFamiliarResponse{Familiar: familiarToProto(f)}, nil
}
