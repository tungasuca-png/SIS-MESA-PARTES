package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"
	"estudiantes/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
)

type ReingresarMatriculaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReingresarMatriculaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReingresarMatriculaLogic {
	return &ReingresarMatriculaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ReingresarMatricula (etapa 5): vuelve a MATRICULADO una matrícula RETIRADO
// o TRASLADADO del mismo periodo abierto; el retiro queda en el historial de
// movimientos ("estudiantes.matricula.update").
func (l *ReingresarMatriculaLogic) ReingresarMatricula(in *estudiantes.ReingresarMatriculaRequest) (*estudiantes.MatriculaResponse, error) {
	userID, err := autorizar(l.ctx, l.svcCtx, permisoMatriculaUpdate)
	if err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.ReingresarMatriculaRequest{}
	}
	id, err := validarID(in.MatriculaId)
	if err != nil {
		return nil, matriculaError(l.Logger, "reingresar matrícula", err)
	}
	if err := validarVersion(in.ExpectedVersion); err != nil {
		return nil, matriculaError(l.Logger, "reingresar matrícula", err)
	}
	fecha, err := validation.FechaMovimiento("fecha", in.Fecha, validation.HoyLima())
	if err != nil {
		return nil, matriculaError(l.Logger, "reingresar matrícula", err)
	}
	motivo, err := motivoOpcional(in.Motivo)
	if err != nil {
		return nil, matriculaError(l.Logger, "reingresar matrícula", err)
	}
	out, err := l.svcCtx.EstudianteRepository.Reingresar(l.ctx, id, int(in.ExpectedVersion), fecha, motivo,
		repository.Auditoria{UsuarioID: userID})
	if err != nil {
		return nil, matriculaError(l.Logger, "reingresar matrícula", err)
	}
	return &estudiantes.MatriculaResponse{Matricula: matriculaToProto(out)}, nil
}
