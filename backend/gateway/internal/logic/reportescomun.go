package logic

import (
	"context"
	"encoding/base64"
	"strings"
	"time"

	"usuarios/usuariosclient"

	"gateway/internal/reportes"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Utilidades comunes de los reportes en el Gateway (Fase 1 y 2).

var zonaLima = time.FixedZone("America/Lima", -5*60*60)

// formatoReporte valida el formato de exportación (xlsx | csv | pdf).
func formatoReporte(formato string) (string, error) {
	f := strings.ToLower(strings.TrimSpace(formato))
	if reportes.MimeDe(f) == "" {
		return "", status.Error(codes.InvalidArgument, "validation_error: formato no admitido: use xlsx, csv o pdf")
	}
	return f, nil
}

// generadoLima: fecha de cálculo informada por el servicio (o ahora).
func generadoLima(rfc3339 string) time.Time {
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		t = time.Now()
	}
	return t.In(zonaLima)
}

func archivoReporte(logger logx.Logger, doc reportes.Documento, nombre, formato string) (*types.ArchivoReporteResponse, error) {
	contenido, err := reportes.GenerarDocumento(doc, formato)
	if err != nil {
		logger.Errorf("exportar reporte: %v", err)
		return nil, status.Error(codes.Internal, "no se pudo generar el archivo del reporte")
	}
	return &types.ArchivoReporteResponse{
		Nombre:          nombre,
		Mime:            reportes.MimeDe(formato),
		ContenidoBase64: base64.StdEncoding.EncodeToString(contenido),
	}, nil
}

// lotesUsuarios: máximo de IDs por llamada a Usuarios.GetUsuariosBasic.
const lotesUsuarios = 100

// rolesDeUsuarios resuelve el ROL (tipo_usuario) de cada ID. Solo conserva el
// rol: nombres y DNI que pudiera devolver Usuarios se descartan aquí mismo.
func rolesDeUsuarios(ctx context.Context, svcCtx *svc.ServiceContext, authorization string, ids []string) (map[string]string, error) {
	roles := make(map[string]string, len(ids))
	out := withAuthorization(ctx, authorization)
	for i := 0; i < len(ids); i += lotesUsuarios {
		fin := min(i+lotesUsuarios, len(ids))
		resp, err := svcCtx.UsuariosClient.GetUsuariosBasic(out, &usuariosclient.GetUsuariosBasicRequest{Ids: ids[i:fin]})
		if err != nil {
			return nil, err
		}
		for _, u := range resp.GetUsuarios() {
			roles[strings.ToLower(u.GetId())] = strings.ToUpper(u.GetTipoUsuario())
		}
	}
	return roles, nil
}
