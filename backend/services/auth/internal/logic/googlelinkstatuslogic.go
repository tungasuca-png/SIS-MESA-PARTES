package logic

import (
	"context"
	"errors"

	"auth/auth"
	"auth/internal/interceptor"
	"auth/internal/repository"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GoogleLinkStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGoogleLinkStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GoogleLinkStatusLogic {
	return &GoogleLinkStatusLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// GoogleLinkStatus: estado de la vinculación de la cuenta autenticada (solo
// la propia: el usuario sale del JWT).
func (l *GoogleLinkStatusLogic) GoogleLinkStatus(_ *auth.GoogleLinkStatusRequest) (*auth.GoogleLinkStatusResponse, error) {
	usuarioID, ok := interceptor.UserIDFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	resp := &auth.GoogleLinkStatusResponse{Available: l.svcCtx.Google.Configurado()}
	identidad, err := l.svcCtx.GoogleRepository.FindIdentityByUser(l.ctx, usuarioID)
	switch {
	case err == nil && identidad.RegistroCompleto:
		resp.Linked, resp.Email = true, identidad.Email
	case err == nil, errors.Is(err, repository.ErrIdentidadNoEncontrada):
	default:
		return nil, status.Error(codes.Internal, "no se pudo consultar la vinculación")
	}
	return resp, nil
}
