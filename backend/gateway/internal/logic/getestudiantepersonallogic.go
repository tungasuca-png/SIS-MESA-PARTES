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

type GetEstudiantePersonalLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetEstudiantePersonalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetEstudiantePersonalLogic {
	return &GetEstudiantePersonalLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GetEstudiantePersonal (FASE E-08): transporte puro. El permiso
// (estudiantes.personal.view) y la auditoría de la lectura están en
// Estudiantes Service.
func (l *GetEstudiantePersonalLogic) GetEstudiantePersonal(req *types.GetEstudiantePersonalRequest) (resp *types.GetEstudiantePersonalResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	out, err := l.svcCtx.EstudiantesClient.GetEstudiantePersonal(ctx, &estudiantesclient.GetEstudiantePersonalRequest{Id: req.Id, FechaReferencia: req.FechaReferencia})
	if err != nil {
		return nil, err
	}
	p := out.GetPersonal()
	return &types.GetEstudiantePersonalResponse{Personal: types.EstudiantePersonalDTO{
		Id:               p.GetId(),
		Version:          p.GetVersion(),
		FechaNacimiento:  p.GetFechaNacimiento(),
		Sexo:             p.GetSexo(),
		PaisNacimiento:   p.GetPaisNacimiento(),
		UbigeoNacimiento: p.GetUbigeoNacimiento(),
		LenguaMaterna:    p.GetLenguaMaterna(),
		SegundaLengua:    p.GetSegundaLengua(),
		// Modelo maestro: domicilio, contacto y edad calculada.
		DomicilioDireccion:    p.GetDomicilioDireccion(),
		DomicilioDepartamento: p.GetDomicilioDepartamento(),
		DomicilioProvincia:    p.GetDomicilioProvincia(),
		DomicilioDistrito:     p.GetDomicilioDistrito(),
		DomicilioUbigeo:       p.GetDomicilioUbigeo(),
		Correo:                p.GetCorreo(),
		Telefono:              p.GetTelefono(),
		Edad:                  p.Edad,
	}}, nil
}
