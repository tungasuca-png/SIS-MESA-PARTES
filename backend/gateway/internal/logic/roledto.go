package logic

import (
	"auth/authclient"
	"gateway/internal/types"
)

func toRoleDTO(r *authclient.Role) types.RoleDTO {
	if r == nil {
		return types.RoleDTO{}
	}

	return types.RoleDTO{
		Id:          r.Id,
		Nombre:      r.Nombre,
		Descripcion: r.Descripcion,
		Estado:      r.Estado,
	}
}

func toRoleDTOs(items []*authclient.Role) []types.RoleDTO {
	result := make([]types.RoleDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toRoleDTO(item))
	}
	return result
}

func toPermissionDTO(p *authclient.Permission) types.PermissionDTO {
	if p == nil {
		return types.PermissionDTO{}
	}

	return types.PermissionDTO{
		Id:          p.Id,
		Codigo:      p.Codigo,
		Nombre:      p.Nombre,
		Descripcion: p.Descripcion,
		Modulo:      p.Modulo,
		Estado:      p.Estado,
	}
}

func toPermissionDTOs(items []*authclient.Permission) []types.PermissionDTO {
	result := make([]types.PermissionDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toPermissionDTO(item))
	}
	return result
}
