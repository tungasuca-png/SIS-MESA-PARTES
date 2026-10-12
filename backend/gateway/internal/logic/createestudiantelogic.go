// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"gateway/internal/svc"
	"gateway/internal/types"

	"estudiantes/estudiantesclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateEstudianteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateEstudianteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateEstudianteLogic {
	return &CreateEstudianteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// CreateEstudiante (FASE E-05): transporte puro; permisos, validación y
// auditoría en Estudiantes Service.
func (l *CreateEstudianteLogic) CreateEstudiante(req *types.CreateEstudianteRequest) (resp *types.EstudianteWriteResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	out, err := l.svcCtx.EstudiantesClient.CreateEstudiante(ctx, &estudiantesclient.CreateEstudianteRequest{
		Codigo:           req.Codigo,
		CodigoSiagie:     req.CodigoSiagie,
		TipoDocumento:    req.TipoDocumento,
		NumeroDocumento:  req.NumeroDocumento,
		Nombres:          req.Nombres,
		ApellidoPaterno:  req.ApellidoPaterno,
		ApellidoMaterno:  req.ApellidoMaterno,
		Apellidos:        req.Apellidos,
		FechaNacimiento:  req.FechaNacimiento,
		Sexo:             req.Sexo,
		PaisNacimiento:   req.PaisNacimiento,
		UbigeoNacimiento: req.UbigeoNacimiento,
		LenguaMaterna:    req.LenguaMaterna,
		SegundaLengua:    req.SegundaLengua,
		FotoRef:          req.FotoRef,
		Estado:           req.Estado,
	})
	if err != nil {
		return nil, err
	}
	return &types.EstudianteWriteResponse{Estudiante: toEstudianteBasicoDTO(out.Estudiante)}, nil
}
