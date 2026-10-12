package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPosiblesDuplicadosLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListPosiblesDuplicadosLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPosiblesDuplicadosLogic {
	return &ListPosiblesDuplicadosLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListPosiblesDuplicados (modelo maestro): reporte para REVISIÓN HUMANA de
// estudiantes que podrían estar registrados dos veces. No fusiona ni
// modifica nada. Es una tarea de quien administra el maestro, por eso exige
// "estudiantes.update". Devuelve solo identidad básica (el criterio puede
// usar la fecha de nacimiento, pero no la expone).
func (l *ListPosiblesDuplicadosLogic) ListPosiblesDuplicados(in *estudiantes.ListPosiblesDuplicadosRequest) (*estudiantes.ListPosiblesDuplicadosResponse, error) {
	if _, err := autorizar(l.ctx, l.svcCtx, "estudiantes.update"); err != nil {
		return nil, err
	}
	page, pageSize := 1, defaultPageSize
	if in != nil {
		page, pageSize = int(in.Page), int(in.PageSize)
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > maxPageSize {
		pageSize = defaultPageSize
	}

	grupos, total, err := l.svcCtx.EstudianteRepository.ListPosiblesDuplicados(l.ctx, page, pageSize)
	if err != nil {
		return nil, errorSeguro(l.Logger, "detectar duplicados", "no se pudo generar el reporte de posibles duplicados", err)
	}
	ids := make([]string, 0)
	for _, g := range grupos {
		ids = append(ids, g.IDs...)
	}
	basicos, err := l.svcCtx.EstudianteRepository.GetBasicByIDs(l.ctx, ids)
	if err != nil {
		return nil, errorSeguro(l.Logger, "detectar duplicados", "no se pudo generar el reporte de posibles duplicados", err)
	}
	porID := make(map[string]*estudiantes.EstudianteBasico, len(basicos))
	for _, b := range basicos {
		porID[b.ID] = toProtoBasic(b)
	}

	out := make([]*estudiantes.GrupoPosibleDuplicado, 0, len(grupos))
	for _, g := range grupos {
		grupo := &estudiantes.GrupoPosibleDuplicado{Criterio: g.Criterio}
		for _, id := range g.IDs {
			if b, ok := porID[id]; ok {
				grupo.Estudiantes = append(grupo.Estudiantes, b)
			}
		}
		out = append(out, grupo)
	}
	return &estudiantes.ListPosiblesDuplicadosResponse{Grupos: out, Total: int32(total)}, nil
}
