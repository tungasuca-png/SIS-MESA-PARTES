package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"
	"estudiantes/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
)

type DesvincularFamiliarLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDesvincularFamiliarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DesvincularFamiliarLogic {
	return &DesvincularFamiliarLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// DesvincularFamiliar (modelo maestro, etapa 4): cierra una relación vigente
// (vigente=false, hasta) con "estudiantes.familia.update". No borra nada: el
// historial se conserva y la persona sigue registrada. Auditado como
// FAMILIA_UPDATE.
func (l *DesvincularFamiliarLogic) DesvincularFamiliar(in *estudiantes.DesvincularFamiliarRequest) (*estudiantes.DesvincularFamiliarResponse, error) {
	userID, err := autorizar(l.ctx, l.svcCtx, permisoFamiliaUpdate)
	if err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.DesvincularFamiliarRequest{}
	}
	estudianteID, err := validarID(in.EstudianteId)
	if err != nil {
		return nil, escrituraError(l.Logger, "desvincular familiar", err)
	}
	relacionID, err := validarID(in.RelacionId)
	if err != nil {
		return nil, escrituraError(l.Logger, "desvincular familiar", err)
	}
	hasta := validation.HoyLima().Format("2006-01-02")
	if f, err := fechaOpcional("hasta", in.Hasta); err != nil {
		return nil, escrituraError(l.Logger, "desvincular familiar", err)
	} else if f != nil {
		hasta = *f
	}

	f, err := l.svcCtx.EstudianteRepository.DesvincularFamiliar(l.ctx, estudianteID, relacionID, hasta, repository.Auditoria{UsuarioID: userID})
	if err != nil {
		return nil, escrituraError(l.Logger, "desvincular familiar", err)
	}
	return &estudiantes.DesvincularFamiliarResponse{Familiar: familiarToProto(f)}, nil
}
