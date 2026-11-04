package logic

import (
	"context"

	"usuarios/internal/svc"
	"usuarios/usuarios"

	"github.com/zeromicro/go-zero/core/logx"
)

type PublicRegistrationReadyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPublicRegistrationReadyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublicRegistrationReadyLogic {
	return &PublicRegistrationReadyLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// PublicRegistrationReady: ready = Usuarios puede crear ahora mismo el perfil
// de un registro público (base disponible y migración 003 aplicada). Nunca
// devuelve el motivo: el detalle queda solo en el log del servicio.
func (l *PublicRegistrationReadyLogic) PublicRegistrationReady(_ *usuarios.PublicRegistrationReadyRequest) (*usuarios.PublicRegistrationReadyResponse, error) {
	if err := l.svcCtx.UsuarioRepository.RegistroPublicoDisponible(l.ctx); err != nil {
		l.Errorf("registro público: Usuarios no está preparado (%v)", err)
		return &usuarios.PublicRegistrationReadyResponse{Ready: false}, nil
	}
	return &usuarios.PublicRegistrationReadyResponse{Ready: true}, nil
}
