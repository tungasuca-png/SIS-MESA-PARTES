// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"auth/authclient"
	"gateway/internal/svc"
	"gateway/internal/types"
	"usuarios/usuariosclient"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UpdateUserRoleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateUserRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateUserRoleLogic {
	return &UpdateUserRoleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// UpdateUserRole reenvía el JWT tal cual y delega la decisión completa en
// Auth Service — el Gateway no valida "usuarios.change_role" por su
// cuenta, ni decide qué usuario/rol es válido, solo compone la ruta REST.
//
// Auth Service es la ÚNICA fuente de verdad del rol. Usuarios Service solo
// guarda una copia denormalizada (tipo_usuario) que GET /api/usuarios lee
// para mostrarla — sin esta sincronización esa copia queda obsoleta tras
// un cambio de rol exitoso. Secuencia (nunca al revés): 1) Auth decide y
// persiste el rol real; 2) solo SI Auth tuvo éxito, se sincroniza Usuarios
// Service con el role_nombre que Auth ya confirmó (nunca con un nombre que
// venga directo del frontend); 3) si Auth falla, no se sincroniza nada.
func (l *UpdateUserRoleLogic) UpdateUserRole(req *types.UpdateUserRoleRequest) (resp *types.UpdateUserRoleResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	updated, err := l.svcCtx.AuthClient.UpdateUserRole(ctx, &authclient.UpdateUserRoleRequest{
		UserId: req.Id,
		RoleId: req.RoleId,
	})
	if err != nil {
		return nil, err
	}

	// No hay transacción distribuida real entre Auth Service y Usuarios
	// Service (bases de datos aisladas por microservicio): el rol de Auth
	// YA es real y válido en este punto y no se revierte. "usuarios_sync_ok"
	// solo informa si la copia denormalizada de Usuarios Service quedó al
	// día — nunca se finge una sincronización que no ocurrió. NotFound (el
	// usuario no tiene perfil en Usuarios Service) no es un fallo: no hay
	// nada que desincronizar, porque GET /api/usuarios nunca lo listaría.
	syncOk := true
	if _, syncErr := l.svcCtx.UsuariosClient.UpdateUsuarioRole(ctx, &usuariosclient.UpdateUsuarioRoleRequest{
		Id:          updated.UserId,
		TipoUsuario: updated.RoleNombre,
	}); syncErr != nil && status.Code(syncErr) != codes.NotFound {
		l.Errorf("sincronizar tipo_usuario en Usuarios Service para el usuario %s: %v", updated.UserId, syncErr)
		syncOk = false
	}

	return &types.UpdateUserRoleResponse{
		UserId:         updated.UserId,
		RoleId:         updated.RoleId,
		RoleNombre:     updated.RoleNombre,
		UsuariosSyncOk: syncOk,
	}, nil
}
