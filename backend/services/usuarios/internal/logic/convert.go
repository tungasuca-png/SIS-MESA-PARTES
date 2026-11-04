package logic

import (
	"time"

	"usuarios/internal/repository"
	"usuarios/usuarios"
)

func toProto(u *repository.Usuario) *usuarios.Usuario {
	if u == nil {
		return nil
	}

	return &usuarios.Usuario{
		Id:          u.ID,
		Nombres:     u.Nombres,
		Apellidos:   u.Apellidos,
		Dni:         u.DNI,
		Telefono:    u.Telefono,
		Correo:      u.Correo,
		Direccion:   u.Direccion,
		TipoUsuario: u.TipoUsuario,
		Activo:      u.Activo,
		CreatedAt:   u.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   u.UpdatedAt.Format(time.RFC3339),
		// DNI declarado sin verificar (solo para quien ya puede ver el perfil
		// completo: el propio solicitante o el personal autorizado).
		DniDeclarado:       u.DNIDeclarado,
		IdentidadPendiente: u.IdentidadPendiente(),
	}
}

// toProtoBasic entrega la proyección mínima: nunca teléfono, correo ni
// dirección. El DNI solo se incluye si includeDNI es true (quien consulta
// está autorizado a ver datos personales).
func toProtoBasic(u *repository.Usuario, includeDNI bool) *usuarios.UsuarioBasic {
	if u == nil {
		return nil
	}

	basic := &usuarios.UsuarioBasic{
		Id:             u.ID,
		Nombres:        u.Nombres,
		Apellidos:      u.Apellidos,
		NombreCompleto: u.NombreCompleto(),
		TipoUsuario:    u.TipoUsuario,
		// Solo la marca; el DNI declarado nunca va en la vista básica.
		IdentidadPendiente: u.IdentidadPendiente(),
	}
	if includeDNI {
		basic.Dni = u.DNI
	}

	return basic
}
