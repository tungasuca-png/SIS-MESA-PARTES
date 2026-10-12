package logic

import (
	"context"

	"auth/authclient"
	"gateway/internal/svc"
	"gateway/internal/types"
	"usuarios/usuariosclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateUsuarioLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateUsuarioLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateUsuarioLogic {
	return &CreateUsuarioLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// CreateUsuario reenvía el JWT tal cual y delega la decisión completa en
// Auth Service (identidad/credenciales/rol) — el Gateway no valida
// "usuarios.change_role" por su cuenta, ni decide qué rol es válido, solo
// compone la ruta REST. Mismo principio que UpdateUserRole: Auth Service es
// la ÚNICA fuente de verdad; Usuarios Service solo guarda una copia de
// perfil (usada por GET /api/usuarios) que se sincroniza SOLO si Auth tuvo
// éxito, con el role_nombre que Auth ya confirmó (nunca uno que venga del
// frontend). Si Auth falla, no se sincroniza nada.
func (l *CreateUsuarioLogic) CreateUsuario(req *types.CreateUsuarioRequest) (resp *types.CreateUsuarioResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	created, err := l.svcCtx.AuthClient.CreateUser(ctx, &authclient.CreateUserRequest{
		Username: req.Username,
		Dni:      req.Dni,
		Email:    req.Email,
		Password: req.Password,
		RoleId:   req.RoleId,
	})
	if err != nil {
		return nil, err
	}

	// No hay transacción distribuida real entre Auth Service y Usuarios
	// Service (bases aisladas por microservicio): la identidad y el rol en
	// Auth YA son reales y válidos en este punto y no se revierten.
	// "usuarios_sync_ok" solo informa si el perfil de Usuarios Service
	// quedó al día — nunca se finge una sincronización que no ocurrió. A
	// diferencia de UpdateUsuarioRole, UpsertUsuario siempre crea la fila si
	// falta (no hay caso "NotFound = nada que sincronizar" acá).
	syncOk := true
	if _, syncErr := l.svcCtx.UsuariosClient.UpsertUsuario(ctx, &usuariosclient.UpsertUsuarioRequest{
		Id:          created.Id,
		Nombres:     req.Nombres,
		Apellidos:   req.Apellidos,
		Dni:         req.Dni,
		Telefono:    req.Telefono,
		Correo:      created.Email,
		Direccion:   req.Direccion,
		TipoUsuario: created.RoleNombre,
	}); syncErr != nil {
		l.Errorf("sincronizar perfil en Usuarios Service para el usuario %s: %v", created.Id, syncErr)
		syncOk = false
	}

	return &types.CreateUsuarioResponse{
		Id:             created.Id,
		Username:       created.Username,
		Email:          created.Email,
		RoleId:         created.RoleId,
		RoleNombre:     created.RoleNombre,
		UsuariosSyncOk: syncOk,
	}, nil
}
