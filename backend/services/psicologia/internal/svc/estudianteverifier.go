package svc

import (
	"context"
	"strings"

	"psicologia/internal/interceptor"

	"estudiantes/estudiantesclient"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// estudianteServiceVerifier es la implementación real de EstudianteVerifier y
// EstudianteDirectory sobre Estudiantes Service.
//
// Modo SERVICIO (tokens != nil, credenciales de client credentials
// configuradas): Psicología consulta como sistema consumidor con su propio
// JWT de servicio (scope estudiantes.basic.view) y la operación de datos
// básicos GetEstudiantesBasic. Nunca reenvía el JWT del usuario. Quién puede
// abrir un expediente o descargar una Hoja de Atención lo siguen decidiendo
// los permisos de Psicología, que se comprueban ANTES de llegar aquí; el
// usuario ya no necesita "estudiantes.view". Estudiantes registra el acceso
// del sistema; Psicología conserva al usuario en su propia auditoría.
//
// Modo ANTERIOR (tokens == nil): solo en desarrollo y con
// PSICOLOGIA_ALLOW_LEGACY_AUTH=true explícito (ver ResolverModoEstudiantes);
// GetEstudiante con el MISMO JWT del usuario (exige "estudiantes.view").
//
// En ambos modos es fail-closed: solo una respuesta exitosa sin el estudiante
// significa "no existe"; cualquier error se propaga y nunca se traduce en
// "estudiante inexistente".
type estudianteServiceVerifier struct {
	client estudiantesclient.Estudiantes
	tokens *ServiceTokenProvider
}

// NewEstudianteVerifier construye el verificador sobre un cliente ya creado
// (la conexión gRPC se arma una sola vez en NewServiceContext). tokens nil =
// modo anterior (solo desarrollo, habilitado explícitamente).
func NewEstudianteVerifier(client estudiantesclient.Estudiantes, tokens *ServiceTokenProvider) *estudianteServiceVerifier {
	return &estudianteServiceVerifier{client: client, tokens: tokens}
}

// Exists: solo un "no encontrado" explícito de Estudiantes es (false, nil).
func (v *estudianteServiceVerifier) Exists(ctx context.Context, estudianteID string) (bool, error) {
	if v.tokens == nil {
		_, err := v.client.GetEstudiante(v.outgoingUsuario(ctx), &estudiantesclient.GetEstudianteRequest{Id: estudianteID})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}
	_, found, err := v.basico(ctx, estudianteID)
	return found, err
}

// NombreCompleto (PDF de la Hoja de Atención): nombres y apellidos del
// estudiante. Cualquier error se propaga: el PDF no se genera sin identificar
// al estudiante (fail-closed).
func (v *estudianteServiceVerifier) NombreCompleto(ctx context.Context, estudianteID string) (string, error) {
	if v.tokens == nil {
		resp, err := v.client.GetEstudiante(v.outgoingUsuario(ctx), &estudiantesclient.GetEstudianteRequest{Id: estudianteID})
		if err != nil {
			return "", err
		}
		e := resp.GetEstudiante()
		return strings.TrimSpace(e.GetNombres() + " " + e.GetApellidos()), nil
	}
	e, found, err := v.basico(ctx, estudianteID)
	if err != nil {
		return "", err
	}
	if !found {
		return "", status.Error(codes.NotFound, "estudiante no encontrado")
	}
	return nombreDe(e), nil
}

// conServicio ejecuta una llamada a Estudiantes con el token de servicio. Si
// Estudiantes rechaza el token (Unauthenticated), se descarta, se emite uno
// nuevo y se reintenta UNA sola vez. El contexto saliente lleva SOLO ese
// token (NewOutgoingContext reemplaza toda la metadata: nunca el JWT humano).
func (v *estudianteServiceVerifier) conServicio(ctx context.Context, llamada func(out context.Context) error) error {
	token, err := v.tokens.Token(ctx)
	if err != nil {
		return err
	}
	err = llamada(salienteServicio(ctx, token))
	if status.Code(err) == codes.Unauthenticated {
		v.tokens.Invalidate(token)
		if token, err = v.tokens.Token(ctx); err != nil {
			return err
		}
		err = llamada(salienteServicio(ctx, token))
	}
	if err != nil {
		return errorDeServicio(ctx, err)
	}
	return nil
}

func salienteServicio(ctx context.Context, token string) context.Context {
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token))
}

// basico consulta GetEstudiantesBasic para UN estudiante con el token de
// servicio.
func (v *estudianteServiceVerifier) basico(ctx context.Context, estudianteID string) (*estudiantesclient.EstudianteBasico, bool, error) {
	var resp *estudiantesclient.GetEstudiantesBasicResponse
	err := v.conServicio(ctx, func(out context.Context) error {
		var err error
		resp, err = v.client.GetEstudiantesBasic(out, &estudiantesclient.GetEstudiantesBasicRequest{Ids: []string{estudianteID}})
		return err
	})
	if err != nil {
		return nil, false, err
	}
	for _, e := range resp.GetEstudiantes() {
		if strings.EqualFold(e.GetId(), estudianteID) {
			return e, true, nil
		}
	}
	return nil, false, nil
}

// maxLoteBasico: tope de IDs por llamada de Estudiantes.GetEstudiantesBasic.
const maxLoteBasico = 100

// Resumenes resuelve el nombre de varios estudiantes con UNA llamada por lote
// (IDs únicos, sin consultas individuales). Devuelve id (minúsculas) ->
// nombre; un id ausente del mapa no existe. Solo se conserva el nombre: el
// resto de EstudianteBasico (documento, datos académicos) se descarta aquí.
func (v *estudianteServiceVerifier) Resumenes(ctx context.Context, ids []string) (map[string]string, error) {
	unicos := make([]string, 0, len(ids))
	vistos := map[string]bool{}
	for _, id := range ids {
		id = strings.ToLower(strings.TrimSpace(id))
		if id != "" && !vistos[id] {
			vistos[id] = true
			unicos = append(unicos, id)
		}
	}
	out := make(map[string]string, len(unicos))
	for inicio := 0; inicio < len(unicos); inicio += maxLoteBasico {
		fin := inicio + maxLoteBasico
		if fin > len(unicos) {
			fin = len(unicos)
		}
		lote := unicos[inicio:fin]
		var resp *estudiantesclient.GetEstudiantesBasicResponse
		var err error
		if v.tokens == nil {
			resp, err = v.client.GetEstudiantesBasic(v.outgoingUsuario(ctx), &estudiantesclient.GetEstudiantesBasicRequest{Ids: lote})
		} else {
			err = v.conServicio(ctx, func(o context.Context) error {
				var e error
				resp, e = v.client.GetEstudiantesBasic(o, &estudiantesclient.GetEstudiantesBasicRequest{Ids: lote})
				return e
			})
		}
		if err != nil {
			return nil, err
		}
		for _, e := range resp.GetEstudiantes() {
			out[strings.ToLower(e.GetId())] = nombreDe(e)
		}
	}
	return out, nil
}

// Buscar consulta la búsqueda MÍNIMA de selección de Estudiantes con el token
// de servicio (exige estudiantes.search.basic). En el modo anterior (sin
// credenciales, solo desarrollo) no está disponible.
func (v *estudianteServiceVerifier) Buscar(ctx context.Context, q string, page, pageSize int32) (*estudiantesclient.BuscarEstudiantesSeleccionResponse, error) {
	if v.tokens == nil {
		return nil, status.Error(codes.Unavailable, "la búsqueda de estudiantes requiere las credenciales de servicio de Psicología")
	}
	var resp *estudiantesclient.BuscarEstudiantesSeleccionResponse
	err := v.conServicio(ctx, func(out context.Context) error {
		var err error
		resp, err = v.client.BuscarEstudiantesSeleccion(out, &estudiantesclient.BuscarEstudiantesSeleccionRequest{Q: q, Page: page, PageSize: pageSize})
		return err
	})
	return resp, err
}

// errorDeServicio: si Estudiantes sigue rechazando la credencial del
// SERVICIO (token o scope), el usuario recibe Unavailable, no un 401/403 que
// el frontend interpretaría como problema de su sesión o de su rol. El resto
// de errores se propaga tal cual.
func errorDeServicio(ctx context.Context, err error) error {
	switch status.Code(err) {
	case codes.Unauthenticated, codes.PermissionDenied:
		logx.WithContext(ctx).Errorf("Estudiantes rechazó la credencial de servicio de Psicología (%s)", status.Code(err))
		return status.Error(codes.Unavailable, "la integración con Estudiantes rechazó la credencial del servicio")
	}
	return err
}

func (v *estudianteServiceVerifier) outgoingUsuario(ctx context.Context) context.Context {
	if token, ok := interceptor.TokenFromContext(ctx); ok {
		return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
	}
	return ctx
}

// nombreDe: Estudiantes entrega a los sistemas consumidores solo
// nombre_completo (respuesta mínima); nombres/apellidos quedan como respaldo
// para versiones anteriores de Estudiantes que aún los envíen.
func nombreDe(e interface {
	GetNombreCompleto() string
	GetNombres() string
	GetApellidos() string
}) string {
	if n := strings.TrimSpace(e.GetNombreCompleto()); n != "" {
		return n
	}
	return strings.TrimSpace(e.GetNombres() + " " + e.GetApellidos())
}
