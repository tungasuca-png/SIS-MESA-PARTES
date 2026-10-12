package svc

import (
	"context"
	"strings"

	"psicologia/internal/interceptor"

	"estudiantes/estudiantesclient"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// estudianteServiceVerifier es la implementación real de EstudianteVerifier
// (FASE 5C) sobre Estudiantes Service. Reenvía el MISMO JWT que autenticó la
// llamada a Psicología — nunca inventa una identidad ni bypasea
// Estudiantes.RequirePermission — exactamente el mismo patrón que
// authorization.HasPermission ya usa para llamar a Auth y que
// validateResponsable ya usa para llamar a Usuarios (ver internal/logic/common.go).
//
// Consecuencia explícita de este diseño: el usuario que abre un expediente en
// Psicología debe tener también "estudiantes.view" en Auth. No se le asigna
// ese permiso a ningún rol automáticamente aquí — es una decisión
// administrativa aparte, igual que cualquier otro permiso (ver PASO 11 de
// FASE 5C). Mientras no lo tenga, Estudiantes responde PermissionDenied y esa
// respuesta se propaga tal cual (fail-closed): nunca se traduce en "estudiante
// inexistente".
type estudianteServiceVerifier struct {
	client estudiantesclient.Estudiantes
}

// NewEstudianteVerifier construye el verificador real sobre un cliente ya
// creado (mismo patrón que authclient.NewAuth/usuariosclient.NewUsuarios: la
// conexión gRPC se arma una sola vez en NewServiceContext).
func NewEstudianteVerifier(client estudiantesclient.Estudiantes) *estudianteServiceVerifier {
	return &estudianteServiceVerifier{client: client}
}

// Exists consulta Estudiantes.GetEstudiante. Solo un NotFound explícito de
// Estudiantes Service (estudiante_id con formato válido pero inexistente) se
// traduce en (false, nil); cualquier otro resultado — inaccesible, permiso
// denegado, JWT inválido, error interno — se propaga como error para que el
// llamador (OpenExpediente) nunca lo confunda con "no existe".
func (v *estudianteServiceVerifier) Exists(ctx context.Context, estudianteID string) (bool, error) {
	_, err := v.client.GetEstudiante(v.outgoing(ctx), &estudiantesclient.GetEstudianteRequest{Id: estudianteID})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// NombreCompleto (PDF de la Hoja de Atención): nombres y apellidos del
// estudiante, consultados con el MISMO JWT del usuario (exige
// "estudiantes.view"). Cualquier error se propaga: el PDF no se genera sin
// identificar al estudiante (fail-closed).
func (v *estudianteServiceVerifier) NombreCompleto(ctx context.Context, estudianteID string) (string, error) {
	resp, err := v.client.GetEstudiante(v.outgoing(ctx), &estudiantesclient.GetEstudianteRequest{Id: estudianteID})
	if err != nil {
		return "", err
	}
	e := resp.GetEstudiante()
	return strings.TrimSpace(e.GetNombres() + " " + e.GetApellidos()), nil
}

func (v *estudianteServiceVerifier) outgoing(ctx context.Context) context.Context {
	if token, ok := interceptor.TokenFromContext(ctx); ok {
		return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
	}
	return ctx
}
