package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"
	"estudiantes/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
)

type RetirarMatriculaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRetirarMatriculaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RetirarMatriculaLogic {
	return &RetirarMatriculaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RetirarMatricula (etapa 5): RETIRO (estado RETIRADO) o TRASLADO (estado
// TRASLADADO) de una matrícula MATRICULADO, con fecha y motivo obligatorio
// ("estudiantes.matricula.update"). El registro del estudiante no cambia: el
// retiro es de la matrícula del periodo, no de la persona.
func (l *RetirarMatriculaLogic) RetirarMatricula(in *estudiantes.RetirarMatriculaRequest) (*estudiantes.MatriculaResponse, error) {
	userID, err := autorizar(l.ctx, l.svcCtx, permisoMatriculaUpdate)
	if err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.RetirarMatriculaRequest{}
	}
	id, err := validarID(in.MatriculaId)
	if err != nil {
		return nil, matriculaError(l.Logger, "retirar matrícula", err)
	}
	if err := validarVersion(in.ExpectedVersion); err != nil {
		return nil, matriculaError(l.Logger, "retirar matrícula", err)
	}
	var traslado bool
	switch validation.Upper(in.Tipo) {
	case "RETIRO":
	case "TRASLADO":
		traslado = true
	default:
		return nil, matriculaError(l.Logger, "retirar matrícula", validation.Invalid("tipo inválido: use RETIRO o TRASLADO"))
	}
	fecha, err := validation.FechaMovimiento("fecha", in.Fecha, validation.HoyLima())
	if err != nil {
		return nil, matriculaError(l.Logger, "retirar matrícula", err)
	}
	motivo := validation.CleanText(in.Motivo)
	if err := validation.RequiredText("motivo", motivo, validation.MaxMotivo); err != nil {
		return nil, matriculaError(l.Logger, "retirar matrícula", err)
	}
	out, err := l.svcCtx.EstudianteRepository.Retirar(l.ctx, id, int(in.ExpectedVersion), traslado, fecha, motivo,
		repository.Auditoria{UsuarioID: userID})
	if err != nil {
		return nil, matriculaError(l.Logger, "retirar matrícula", err)
	}
	return &estudiantes.MatriculaResponse{Matricula: matriculaToProto(out)}, nil
}
