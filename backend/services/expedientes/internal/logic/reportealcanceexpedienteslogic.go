package logic

import (
	"context"
	"strings"

	"expedientes/expedientes"
	"expedientes/internal/authorization"
	"expedientes/internal/estados"
	"expedientes/internal/interceptor"
	"expedientes/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ReporteAlcanceExpedientesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReporteAlcanceExpedientesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReporteAlcanceExpedientesLogic {
	return &ReporteAlcanceExpedientesLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// ReporteAlcanceExpedientes (Fase 2): expedientes ACTIVOS visibles para el
// usuario (id y tipo), con el MISMO permiso y alcance que ReporteExpedientes.
// Derivaciones y Documentos lo usan para excluir bajas lógicas y respetar el
// alcance en sus reportes.
func (l *ReporteAlcanceExpedientesLogic) ReporteAlcanceExpedientes(in *expedientes.ReporteAlcanceRequest) (*expedientes.ReporteAlcanceResponse, error) {
	userID, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if err := authorization.RequirePermission(l.ctx, l.svcCtx.AuthClient, userID, permisoReportes); err != nil {
		return nil, err
	}
	if !authorization.IsInternal(role) {
		return nil, status.Error(codes.PermissionDenied, "permission_denied: los reportes son exclusivos del personal interno")
	}
	tipo := strings.ToUpper(strings.TrimSpace(in.GetTipo()))
	if tipo != "" && !estados.IsValidTipo(tipo) {
		return nil, invalido("el tipo de trámite indicado no es válido")
	}
	area, alcance := "", alcanceInstitucion
	if !authorization.CanViewAll(role) {
		area, alcance = authorization.AreaDelRol(role), alcanceArea
	}
	refs, err := l.svcCtx.ExpedienteRepository.ReporteAlcance(l.ctx, tipo, area)
	if err != nil {
		l.Errorf("alcance de reportes: %v", err)
		return nil, status.Error(codes.Internal, "no se pudo generar el reporte")
	}
	out := make([]*expedientes.ReporteExpedienteRef, 0, len(refs))
	for _, r := range refs {
		out = append(out, &expedientes.ReporteExpedienteRef{Id: r.ID, Tipo: r.Tipo})
	}
	return &expedientes.ReporteAlcanceResponse{Expedientes: out, Alcance: alcance, AreaAlcance: area}, nil
}
