package logic

import (
	"gateway/internal/types"
	"usuarios/usuariosclient"
)

func toUsuarioDTO(u *usuariosclient.Usuario) types.UsuarioDTO {
	if u == nil {
		return types.UsuarioDTO{}
	}

	return types.UsuarioDTO{
		Id:          u.Id,
		Nombres:     u.Nombres,
		Apellidos:   u.Apellidos,
		Dni:         u.Dni,
		Telefono:    u.Telefono,
		Correo:      u.Correo,
		Direccion:   u.Direccion,
		TipoUsuario: u.TipoUsuario,
		Activo:      u.Activo,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}

func toUsuarioBasicDTO(u *usuariosclient.UsuarioBasic) types.UsuarioBasicDTO {
	if u == nil {
		return types.UsuarioBasicDTO{}
	}

	return types.UsuarioBasicDTO{
		Id:             u.Id,
		Nombres:        u.Nombres,
		Apellidos:      u.Apellidos,
		NombreCompleto: u.NombreCompleto,
		TipoUsuario:    u.TipoUsuario,
		Dni:            u.Dni,
	}
}
