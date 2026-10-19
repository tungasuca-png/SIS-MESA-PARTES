package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"
	"estudiantes/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
)

type CambiarParentescoFamiliarLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCambiarParentescoFamiliarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CambiarParentescoFamiliarLogic {
	return &CambiarParentescoFamiliarLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CambiarParentescoFamiliar (etapa 6): corrige el parentesco de una relación
// vigente conservando el historial (cierra la actual y crea otra en la misma
// transacción) — "estudiantes.familia.update". Auditado como FAMILIA_UPDATE.
func (l *CambiarParentescoFamiliarLogic) CambiarParentescoFamiliar(in *estudiantes.CambiarParentescoFamiliarRequest) (*estudiantes.CambiarParentescoFamiliarResponse, error) {
	userID, err := autorizar(l.ctx, l.svcCtx, permisoFamiliaUpdate)
	if err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.CambiarParentescoFamiliarRequest{}
	}
	estudianteID, err := validarID(in.EstudianteId)
	if err != nil {
		return nil, escrituraError(l.Logger, "cambiar parentesco", err)
	}
	relacionID, err := validarID(in.RelacionId)
	if err != nil {
		return nil, escrituraError(l.Logger, "cambiar parentesco", err)
	}
	parentesco := validation.Upper(in.Parentesco)
	if err := validation.Parentesco(parentesco); err != nil {
		return nil, escrituraError(l.Logger, "cambiar parentesco", err)
	}
	desde, err := validation.FechaMovimiento("desde", in.Desde, validation.HoyLima())
	if err != nil {
		return nil, escrituraError(l.Logger, "cambiar parentesco", err)
	}
	anterior, nueva, err := l.svcCtx.EstudianteRepository.CambiarParentesco(l.ctx, estudianteID, relacionID, parentesco, desde,
		repository.Auditoria{UsuarioID: userID})
	if err != nil {
		return nil, escrituraError(l.Logger, "cambiar parentesco", err)
	}
	return &estudiantes.CambiarParentescoFamiliarResponse{Anterior: familiarToProto(anterior), Familiar: familiarToProto(nueva)}, nil
}
