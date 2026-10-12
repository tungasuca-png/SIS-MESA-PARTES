package logic

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"psicologia/internal/authorization"
	"psicologia/internal/pdf"
	"psicologia/internal/repository"
	"psicologia/internal/svc"
	"psicologia/internal/validation"
	"psicologia/psicologia"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Atenciones psicológicas (Hoja de Atención) — reglas comunes:
//   - Permiso de ACCIÓN (psicologia.atenciones.*) vía Auth.HasPermission y,
//     además, ALCANCE del expediente (psicologia.expedientes.view / view_own):
//     una atención de un expediente fuera de alcance responde igual que una
//     inexistente (404) y queda auditada como DENIED.
//   - Los borradores solo existen para su autor; los descartados no existen.
//   - Toda escritura ocurre en una transacción con su evento de auditoría; las
//     lecturas se auditan ANTES de responder (fail-closed).
//   - La auditoría y los logs nunca llevan contenido de la hoja.
const (
	permAtencionesView    = "psicologia.atenciones.view"
	permAtencionesCreate  = "psicologia.atenciones.create"
	permAtencionesCorrect = "psicologia.atenciones.correct"
	permAtencionesAnnul   = "psicologia.atenciones.annul"
)

const (
	motivoBorradorAjeno     = "BORRADOR_AJENO"
	msgAtencionNoEncontrada = "atención psicológica no encontrada"
)

func requireAtencionPermission(ctx context.Context, sc *svc.ServiceContext, userID, permission, recursoID string) error {
	allowed, err := authorization.HasPermission(ctx, sc.AuthClient, userID, permission)
	if err != nil {
		return err
	}
	if !allowed {
		return denyAtencion(ctx, sc, userID, recursoID, "", motivoPermisoInsuficiente)
	}
	return nil
}

func auditAtencion(ctx context.Context, sc *svc.ServiceContext, e repository.AuditEntry) error {
	e.Recurso = repository.RecursoAtencion
	return sc.AuditoriaRepo.Registrar(ctx, e)
}

// denyAtencion registra DENIED (mejor esfuerzo) y devuelve PermissionDenied.
func denyAtencion(ctx context.Context, sc *svc.ServiceContext, userID, recursoID, estudianteID, motivo string) error {
	if err := auditAtencion(ctx, sc, repository.AuditEntry{UsuarioID: userID, Accion: repository.AccionDenied,
		RecursoID: recursoID, EstudianteID: estudianteID, Resultado: repository.ResultadoDenegado, Motivo: motivo}); err != nil {
		logx.WithContext(ctx).Errorf("no se pudo registrar el evento DENIED en la auditoría: %v", err)
	}
	return status.Error(codes.PermissionDenied, msgSinPermiso)
}

// hiddenAtencion: fuera de alcance o borrador ajeno -> 404 uniforme + DENIED.
func hiddenAtencion(ctx context.Context, sc *svc.ServiceContext, userID, recursoID, estudianteID, motivo string) error {
	if err := auditAtencion(ctx, sc, repository.AuditEntry{UsuarioID: userID, Accion: repository.AccionDenied,
		RecursoID: recursoID, EstudianteID: estudianteID, Resultado: repository.ResultadoDenegado, Motivo: motivo}); err != nil {
		logx.WithContext(ctx).Errorf("no se pudo registrar el evento DENIED en la auditoría: %v", err)
	}
	return status.Error(codes.NotFound, msgAtencionNoEncontrada)
}

// auditLectura: evento VIEW/EXPORT antes de entregar datos (fail-closed).
func auditLectura(ctx context.Context, sc *svc.ServiceContext, userID, accion, recursoID, estudianteID, motivo string) error {
	if err := auditAtencion(ctx, sc, repository.AuditEntry{UsuarioID: userID, Accion: accion,
		RecursoID: recursoID, EstudianteID: estudianteID, Resultado: repository.ResultadoPermitido, Motivo: motivo}); err != nil {
		logx.WithContext(ctx).Errorf("no se pudo registrar el evento %s en la auditoría: %v", accion, err)
		return status.Error(codes.Internal, "no se pudo registrar la auditoría de la consulta")
	}
	return nil
}

// atencionEnAlcance resuelve el alcance del usuario sobre el expediente de la
// atención. Devuelve la atención, su expediente y el filtro de alcance que el
// repositorio vuelve a aplicar dentro de su transacción.
func atencionEnAlcance(ctx context.Context, sc *svc.ServiceContext, userID, id string) (*repository.Atencion, *repository.Expediente, string, error) {
	s, ok, err := resolveScope(ctx, sc, userID)
	if err != nil {
		return nil, nil, "", err
	}
	if !ok {
		return nil, nil, "", denyAtencion(ctx, sc, userID, id, "", motivoSinAlcance)
	}
	a, err := sc.AtencionRepo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, "", atencionRepoError(ctx, err, "no se pudo consultar la atención psicológica")
	}
	if a.Estado == repository.AtencionDescartada {
		return nil, nil, "", status.Error(codes.NotFound, msgAtencionNoEncontrada)
	}
	exp, err := sc.ExpedienteRepo.GetByID(ctx, a.ExpedienteID)
	if err != nil {
		return nil, nil, "", repoError(ctx, err, "no se pudo consultar el expediente psicológico")
	}
	if !s.allows(exp, userID) {
		return nil, nil, "", hiddenAtencion(ctx, sc, userID, a.ID, exp.EstudianteID, motivoFueraDeAlcance)
	}
	if a.Estado == repository.AtencionBorrador && a.AutorID != userID {
		return nil, nil, "", hiddenAtencion(ctx, sc, userID, a.ID, exp.EstudianteID, motivoBorradorAjeno)
	}
	return a, exp, s.responsableFilter(userID), nil
}

// atencionRepoError traduce errores del repositorio sin exponer texto de
// PostgreSQL ni contenido de la hoja.
func atencionRepoError(ctx context.Context, err error, internalMsg string) error {
	if st, ok := status.FromError(err); ok && st.Code() != codes.Unknown {
		return err // validación de dominio ya expresada como status gRPC
	}
	switch {
	case errors.Is(err, repository.ErrAtencionNotFound):
		return status.Error(codes.NotFound, msgAtencionNoEncontrada)
	case errors.Is(err, repository.ErrVersionNotFound):
		return status.Error(codes.NotFound, "la versión solicitada no existe")
	case errors.Is(err, repository.ErrExpedienteNotFound):
		return status.Error(codes.NotFound, msgExpedienteNoEncontrado)
	case errors.Is(err, repository.ErrAtencionEstado):
		return status.Error(codes.FailedPrecondition, "el estado de la atención no permite esta operación")
	case errors.Is(err, repository.ErrAtencionConflicto):
		return status.Error(codes.Aborted, "la atención fue modificada por otra operación; actualiza e inténtalo nuevamente")
	case errors.Is(err, repository.ErrCatalogoInvalido):
		return status.Error(codes.InvalidArgument, "un motivo o destino de derivación no es válido")
	case errors.Is(err, repository.ErrExpedienteCerrado):
		return status.Error(codes.FailedPrecondition, "el expediente psicológico está cerrado; reábrelo para registrar o modificar atenciones")
	}
	logx.WithContext(ctx).Errorf("%s: %v", internalMsg, err)
	return status.Error(codes.Internal, internalMsg)
}

// ---------------------------------------------------------------------------
// Contenido: validación de formato (sin reglas clínicas).
// ---------------------------------------------------------------------------

func contenidoDesdeProto(c *psicologia.AtencionContenido) (repository.ContenidoInput, error) {
	if c == nil {
		c = &psicologia.AtencionContenido{}
	}
	invalid := func(msg string) (repository.ContenidoInput, error) {
		return repository.ContenidoInput{}, status.Error(codes.InvalidArgument, msg)
	}
	out := repository.ContenidoInput{AtencionContenido: repository.AtencionContenido{
		FechaAtencion:         strings.TrimSpace(c.FechaAtencion),
		HoraAtencion:          strings.TrimSpace(c.HoraAtencion),
		RealizadaPorOtros:     strings.TrimSpace(c.RealizadaPorOtros),
		MotivoOtros:           strings.TrimSpace(c.MotivoOtros),
		DescripcionMotivo:     strings.TrimSpace(c.DescripcionMotivo),
		AnalisisIntegral:      strings.TrimSpace(c.AnalisisIntegral),
		Estrategias:           strings.TrimSpace(c.Estrategias),
		DerivacionOtro:        strings.TrimSpace(c.DerivacionOtro),
		SeguimientoResultados: strings.TrimSpace(c.SeguimientoResultados),
		Acuerdos:              strings.TrimSpace(c.Acuerdos),
		CompromisoEstudiante:  strings.TrimSpace(c.CompromisoEstudiante),
		CompromisoFamilia:     strings.TrimSpace(c.CompromisoFamilia),
	}}
	if out.FechaAtencion != "" && !validation.IsFecha(out.FechaAtencion) {
		return invalid("la fecha de atención no es válida")
	}
	if out.HoraAtencion != "" && !validation.IsHora(out.HoraAtencion) {
		return invalid("la hora de atención no es válida")
	}
	for _, t := range []string{out.RealizadaPorOtros, out.MotivoOtros, out.DerivacionOtro} {
		if !validation.TextoDentroDeLimite(t, validation.MaxTextoOtros) {
			return invalid("un campo «otros» es demasiado largo")
		}
	}
	for _, t := range []string{out.DescripcionMotivo, out.AnalisisIntegral, out.Estrategias, out.SeguimientoResultados,
		out.Acuerdos, out.CompromisoEstudiante, out.CompromisoFamilia} {
		if !validation.TextoDentroDeLimite(t, validation.MaxTextoAtencion) {
			return invalid("un campo de la hoja de atención es demasiado largo")
		}
	}
	realizada, ok := normalizarCodigos(c.RealizadaPor, validation.IsRealizadaPor)
	if !ok {
		return invalid("«atención realizada por» contiene un valor no válido")
	}
	out.RealizadaPor = realizada
	if out.MotivoCodigos, ok = normalizarCodigos(c.Motivos, validation.IsCodigoCatalogo); !ok {
		return invalid("un motivo no es válido")
	}
	if out.DerivacionCodigos, ok = normalizarCodigos(c.Derivaciones, validation.IsCodigoCatalogo); !ok {
		return invalid("un destino de derivación no es válido")
	}
	return out, nil
}

// normalizarCodigos: mayúsculas, sin vacíos ni duplicados, formato válido.
func normalizarCodigos(values []string, valido func(string) bool) ([]string, bool) {
	out := []string{}
	seen := map[string]bool{}
	for _, raw := range values {
		v := strings.ToUpper(strings.TrimSpace(raw))
		if v == "" || seen[v] {
			continue
		}
		if !valido(v) {
			return nil, false
		}
		seen[v] = true
		out = append(out, v)
	}
	return out, true
}

// validarOficial: requisitos mínimos para oficializar una versión (registrar o
// corregir). Son TÉCNICOS, no clínicos: la atención debe poder ubicarse en el
// tiempo y "Otro" en derivaciones debe decir cuál. Los demás campos
// obligatorios siguen pendientes de decisión institucional.
func validarOficial(c repository.AtencionContenido) error {
	if c.FechaAtencion == "" {
		return status.Error(codes.InvalidArgument, "la fecha de atención es obligatoria para registrar")
	}
	if c.HoraAtencion == "" {
		return status.Error(codes.InvalidArgument, "la hora de atención es obligatoria para registrar")
	}
	for _, d := range c.Derivaciones {
		if d.Codigo == "OTRO" && c.DerivacionOtro == "" {
			return status.Error(codes.InvalidArgument, "indica a qué otro destino se deriva")
		}
	}
	return nil
}

func validarId(id string, msg string) (string, error) {
	id = strings.TrimSpace(id)
	if !validation.IsValidUUID(id) {
		return "", status.Error(codes.InvalidArgument, msg)
	}
	return strings.ToLower(id), nil
}

// ---------------------------------------------------------------------------
// Conversión a proto
// ---------------------------------------------------------------------------

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatTime(*t)
}

func strPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func opcionesToProto(ops []repository.OpcionCatalogo) []*psicologia.OpcionCatalogo {
	out := make([]*psicologia.OpcionCatalogo, 0, len(ops))
	for _, o := range ops {
		out = append(out, &psicologia.OpcionCatalogo{Codigo: o.Codigo, Etiqueta: o.Etiqueta})
	}
	return out
}

func codigos(ops []repository.OpcionCatalogo) []string {
	out := make([]string, 0, len(ops))
	for _, o := range ops {
		out = append(out, o.Codigo)
	}
	return out
}

func versionToProto(v *repository.AtencionVersion) *psicologia.AtencionVersion {
	if v == nil {
		return nil
	}
	c := v.Contenido
	return &psicologia.AtencionVersion{
		NumeroVersion: int32(v.NumeroVersion),
		Contenido: &psicologia.AtencionContenido{
			FechaAtencion: c.FechaAtencion, HoraAtencion: c.HoraAtencion,
			RealizadaPor: c.RealizadaPor, RealizadaPorOtros: c.RealizadaPorOtros,
			Motivos: codigos(c.Motivos), MotivoOtros: c.MotivoOtros,
			DescripcionMotivo: c.DescripcionMotivo, AnalisisIntegral: c.AnalisisIntegral, Estrategias: c.Estrategias,
			Derivaciones: codigos(c.Derivaciones), DerivacionOtro: c.DerivacionOtro,
			SeguimientoResultados: c.SeguimientoResultados, Acuerdos: c.Acuerdos,
			CompromisoEstudiante: c.CompromisoEstudiante, CompromisoFamilia: c.CompromisoFamilia,
		},
		Motivos:          opcionesToProto(c.Motivos),
		Derivaciones:     opcionesToProto(c.Derivaciones),
		MotivoCorreccion: strPtr(v.MotivoCorreccion),
		CreadaPor:        v.CreadaPor,
		CreadaEn:         formatTime(v.CreadaEn),
	}
}

func atencionToProto(a *repository.Atencion, v *repository.AtencionVersion) *psicologia.AtencionPsicologica {
	numero := 0
	if a.Numero != nil {
		numero = *a.Numero
	}
	return &psicologia.AtencionPsicologica{
		Id: a.ID, ExpedienteId: a.ExpedienteID, Numero: int32(numero), Estado: a.Estado, AutorId: a.AutorID,
		RegistradaPor: strPtr(a.RegistradaPor), RegistradaEn: formatTimePtr(a.RegistradaEn),
		VersionVigente: int32(a.VersionVigente), Revision: int32(a.Revision),
		AnuladaPor: strPtr(a.AnuladaPor), AnuladaEn: formatTimePtr(a.AnuladaEn), MotivoAnulacion: strPtr(a.MotivoAnulacion),
		CreatedAt: formatTime(a.CreatedAt), UpdatedAt: formatTime(a.UpdatedAt),
		Version: versionToProto(v),
	}
}

// respuestaConVersion carga la versión vigente (o el borrador) de la atención.
func respuestaConVersion(ctx context.Context, sc *svc.ServiceContext, a *repository.Atencion) (*psicologia.AtencionResponse, error) {
	v, err := sc.AtencionRepo.GetVersion(ctx, a.ID, a.VersionVigente)
	if err != nil {
		return nil, atencionRepoError(ctx, err, "no se pudo consultar la atención psicológica")
	}
	return &psicologia.AtencionResponse{Atencion: atencionToProto(a, v)}, nil
}

// ---------------------------------------------------------------------------
// RPC
// ---------------------------------------------------------------------------

type AtencionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAtencionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AtencionLogic {
	return &AtencionLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// CreateAtencionDraft crea un borrador (versión 0) en un expediente del alcance.
func (l *AtencionLogic) CreateAtencionDraft(in *psicologia.CreateAtencionDraftRequest) (*psicologia.AtencionResponse, error) {
	userID, err := authenticatedUser(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := requireAtencionPermission(l.ctx, l.svcCtx, userID, permAtencionesCreate, ""); err != nil {
		return nil, err
	}
	expedienteID, err := validarId(in.GetExpedienteId(), "el expediente_id no es válido")
	if err != nil {
		return nil, err
	}
	contenido, err := contenidoDesdeProto(in.GetContenido())
	if err != nil {
		return nil, err
	}
	_, scopeFilter, err := loadInScope(l.ctx, l.svcCtx, userID, expedienteID)
	if err != nil {
		return nil, err
	}
	a, err := l.svcCtx.AtencionRepo.CreateDraft(l.ctx, repository.CreateDraftInput{
		ExpedienteID: expedienteID, ScopeResponsableID: scopeFilter, ActorID: userID, Contenido: contenido,
	})
	if err != nil {
		return nil, atencionRepoError(l.ctx, err, "no se pudo crear la atención psicológica")
	}
	return respuestaConVersion(l.ctx, l.svcCtx, a)
}

// UpdateAtencionDraft guarda el borrador propio (control por revision).
func (l *AtencionLogic) UpdateAtencionDraft(in *psicologia.UpdateAtencionDraftRequest) (*psicologia.AtencionResponse, error) {
	userID, err := authenticatedUser(l.ctx)
	if err != nil {
		return nil, err
	}
	id, err := validarId(in.GetId(), "el identificador no es válido")
	if err != nil {
		return nil, err
	}
	if err := requireAtencionPermission(l.ctx, l.svcCtx, userID, permAtencionesCreate, id); err != nil {
		return nil, err
	}
	contenido, err := contenidoDesdeProto(in.GetContenido())
	if err != nil {
		return nil, err
	}
	_, _, scopeFilter, err := atencionEnAlcance(l.ctx, l.svcCtx, userID, id)
	if err != nil {
		return nil, err
	}
	a, err := l.svcCtx.AtencionRepo.UpdateDraft(l.ctx, repository.UpdateDraftInput{
		ID: id, Revision: int(in.GetRevision()), ScopeResponsableID: scopeFilter, ActorID: userID, Contenido: contenido,
	})
	if err != nil {
		return nil, atencionRepoError(l.ctx, err, "no se pudo guardar el borrador de la atención")
	}
	return respuestaConVersion(l.ctx, l.svcCtx, a)
}

// RegisterAtencion: BORRADOR -> REGISTRADA (número + versión 1 inmutable).
func (l *AtencionLogic) RegisterAtencion(in *psicologia.RegisterAtencionRequest) (*psicologia.AtencionResponse, error) {
	userID, err := authenticatedUser(l.ctx)
	if err != nil {
		return nil, err
	}
	id, err := validarId(in.GetId(), "el identificador no es válido")
	if err != nil {
		return nil, err
	}
	if err := requireAtencionPermission(l.ctx, l.svcCtx, userID, permAtencionesCreate, id); err != nil {
		return nil, err
	}
	_, _, scopeFilter, err := atencionEnAlcance(l.ctx, l.svcCtx, userID, id)
	if err != nil {
		return nil, err
	}
	a, err := l.svcCtx.AtencionRepo.Register(l.ctx, repository.TransitionInput{
		ID: id, Revision: int(in.GetRevision()), ScopeResponsableID: scopeFilter, ActorID: userID, Validar: validarOficial,
	})
	if err != nil {
		return nil, atencionRepoError(l.ctx, err, "no se pudo registrar la atención psicológica")
	}
	return respuestaConVersion(l.ctx, l.svcCtx, a)
}

// DiscardAtencionDraft descarta el borrador propio (sin borrarlo físicamente).
func (l *AtencionLogic) DiscardAtencionDraft(in *psicologia.DiscardAtencionDraftRequest) (*psicologia.DiscardAtencionDraftResponse, error) {
	userID, err := authenticatedUser(l.ctx)
	if err != nil {
		return nil, err
	}
	id, err := validarId(in.GetId(), "el identificador no es válido")
	if err != nil {
		return nil, err
	}
	if err := requireAtencionPermission(l.ctx, l.svcCtx, userID, permAtencionesCreate, id); err != nil {
		return nil, err
	}
	_, _, scopeFilter, err := atencionEnAlcance(l.ctx, l.svcCtx, userID, id)
	if err != nil {
		return nil, err
	}
	if err := l.svcCtx.AtencionRepo.Discard(l.ctx, repository.TransitionInput{
		ID: id, Revision: int(in.GetRevision()), ScopeResponsableID: scopeFilter, ActorID: userID,
	}); err != nil {
		return nil, atencionRepoError(l.ctx, err, "no se pudo descartar el borrador de la atención")
	}
	return &psicologia.DiscardAtencionDraftResponse{}, nil
}

// GetAtencion devuelve la cabecera y la versión vigente (o el borrador propio).
func (l *AtencionLogic) GetAtencion(in *psicologia.GetAtencionRequest) (*psicologia.AtencionResponse, error) {
	userID, err := authenticatedUser(l.ctx)
	if err != nil {
		return nil, err
	}
	id, err := validarId(in.GetId(), "el identificador no es válido")
	if err != nil {
		return nil, err
	}
	if err := requireAtencionPermission(l.ctx, l.svcCtx, userID, permAtencionesView, id); err != nil {
		return nil, err
	}
	a, exp, _, err := atencionEnAlcance(l.ctx, l.svcCtx, userID, id)
	if err != nil {
		return nil, err
	}
	resp, err := respuestaConVersion(l.ctx, l.svcCtx, a)
	if err != nil {
		return nil, err
	}
	if err := auditLectura(l.ctx, l.svcCtx, userID, repository.AccionView, a.ID, exp.EstudianteID,
		fmt.Sprintf("VERSION_%d", a.VersionVigente)); err != nil {
		return nil, err
	}
	return resp, nil
}

// ListAtenciones: atenciones del expediente, SIN contenido clínico.
func (l *AtencionLogic) ListAtenciones(in *psicologia.ListAtencionesRequest) (*psicologia.ListAtencionesResponse, error) {
	userID, err := authenticatedUser(l.ctx)
	if err != nil {
		return nil, err
	}
	expedienteID, err := validarId(in.GetExpedienteId(), "el expediente_id no es válido")
	if err != nil {
		return nil, err
	}
	if err := requireAtencionPermission(l.ctx, l.svcCtx, userID, permAtencionesView, ""); err != nil {
		return nil, err
	}
	estado := strings.ToUpper(strings.TrimSpace(in.GetEstado()))
	if estado != "" && !validation.IsEstadoAtencionListable(estado) {
		return nil, status.Error(codes.InvalidArgument, "el estado no es válido")
	}
	page, pageSize := int(in.GetPage()), int(in.GetPageSize())
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > maxPageSize {
		pageSize = defaultPageSize
	}

	s, ok, err := resolveScope(l.ctx, l.svcCtx, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, denyAtencion(l.ctx, l.svcCtx, userID, "", "", motivoSinAlcance)
	}
	exp, err := l.svcCtx.ExpedienteRepo.GetByID(l.ctx, expedienteID)
	if err != nil {
		return nil, repoError(l.ctx, err, "no se pudo consultar el expediente psicológico")
	}
	if !s.allows(exp, userID) {
		return nil, notFoundHidden(l.ctx, l.svcCtx, userID, exp.ID, exp.EstudianteID)
	}

	items, total, err := l.svcCtx.AtencionRepo.ListByExpediente(l.ctx, repository.ListAtencionesFilter{
		ExpedienteID: expedienteID, ActorID: userID, Estado: estado, Page: page, PageSize: pageSize,
	})
	if err != nil {
		return nil, atencionRepoError(l.ctx, err, "no se pudo listar las atenciones psicológicas")
	}
	if err := auditLectura(l.ctx, l.svcCtx, userID, repository.AccionView, "", exp.EstudianteID, motivoListado); err != nil {
		return nil, err
	}
	out := make([]*psicologia.AtencionResumen, 0, len(items))
	for _, it := range items {
		numero := 0
		if it.Numero != nil {
			numero = *it.Numero
		}
		out = append(out, &psicologia.AtencionResumen{
			Id: it.ID, Numero: int32(numero), Estado: it.Estado,
			FechaAtencion: it.FechaAtencion, HoraAtencion: it.HoraAtencion, Motivos: opcionesToProto(it.Motivos),
			AutorId: it.AutorID, RegistradaPor: strPtr(it.RegistradaPor), RegistradaEn: formatTimePtr(it.RegistradaEn),
			VersionVigente: int32(it.VersionVigente),
		})
	}
	return &psicologia.ListAtencionesResponse{Atenciones: out, Total: int32(total)}, nil
}

// CorrectAtencion inserta una versión nueva (la anterior no se modifica).
func (l *AtencionLogic) CorrectAtencion(in *psicologia.CorrectAtencionRequest) (*psicologia.AtencionResponse, error) {
	userID, err := authenticatedUser(l.ctx)
	if err != nil {
		return nil, err
	}
	id, err := validarId(in.GetId(), "el identificador no es válido")
	if err != nil {
		return nil, err
	}
	if err := requireAtencionPermission(l.ctx, l.svcCtx, userID, permAtencionesCorrect, id); err != nil {
		return nil, err
	}
	motivo := strings.TrimSpace(in.GetMotivoCorreccion())
	if motivo == "" {
		return nil, status.Error(codes.InvalidArgument, "el motivo de la corrección es obligatorio")
	}
	if !validation.TextoDentroDeLimite(motivo, validation.MaxMotivo) {
		return nil, status.Error(codes.InvalidArgument, "el motivo de la corrección es demasiado largo")
	}
	contenido, err := contenidoDesdeProto(in.GetContenido())
	if err != nil {
		return nil, err
	}
	_, _, scopeFilter, err := atencionEnAlcance(l.ctx, l.svcCtx, userID, id)
	if err != nil {
		return nil, err
	}
	a, err := l.svcCtx.AtencionRepo.Correct(l.ctx, repository.CorrectInput{
		ID: id, VersionBase: int(in.GetVersionBase()), MotivoCorreccion: motivo, ScopeResponsableID: scopeFilter,
		ActorID: userID, Contenido: contenido, Validar: validarOficial,
	})
	if err != nil {
		return nil, atencionRepoError(l.ctx, err, "no se pudo corregir la atención psicológica")
	}
	return respuestaConVersion(l.ctx, l.svcCtx, a)
}

// ListAtencionVersions: versiones oficiales (sin el contenido).
func (l *AtencionLogic) ListAtencionVersions(in *psicologia.ListAtencionVersionsRequest) (*psicologia.ListAtencionVersionsResponse, error) {
	userID, err := authenticatedUser(l.ctx)
	if err != nil {
		return nil, err
	}
	id, err := validarId(in.GetId(), "el identificador no es válido")
	if err != nil {
		return nil, err
	}
	if err := requireAtencionPermission(l.ctx, l.svcCtx, userID, permAtencionesView, id); err != nil {
		return nil, err
	}
	a, exp, _, err := atencionEnAlcance(l.ctx, l.svcCtx, userID, id)
	if err != nil {
		return nil, err
	}
	versiones, err := l.svcCtx.AtencionRepo.ListVersions(l.ctx, a.ID)
	if err != nil {
		return nil, atencionRepoError(l.ctx, err, "no se pudo listar las versiones de la atención")
	}
	if err := auditLectura(l.ctx, l.svcCtx, userID, repository.AccionView, a.ID, exp.EstudianteID, "VERSIONES"); err != nil {
		return nil, err
	}
	out := make([]*psicologia.AtencionVersionResumen, 0, len(versiones))
	for _, v := range versiones {
		out = append(out, &psicologia.AtencionVersionResumen{
			NumeroVersion: int32(v.NumeroVersion), MotivoCorreccion: strPtr(v.MotivoCorreccion),
			CreadaPor: v.CreadaPor, CreadaEn: formatTime(v.CreadaEn),
		})
	}
	return &psicologia.ListAtencionVersionsResponse{Versiones: out}, nil
}

// GetAtencionVersion: una versión oficial concreta (1..vigente).
func (l *AtencionLogic) GetAtencionVersion(in *psicologia.GetAtencionVersionRequest) (*psicologia.AtencionResponse, error) {
	userID, err := authenticatedUser(l.ctx)
	if err != nil {
		return nil, err
	}
	id, err := validarId(in.GetId(), "el identificador no es válido")
	if err != nil {
		return nil, err
	}
	if err := requireAtencionPermission(l.ctx, l.svcCtx, userID, permAtencionesView, id); err != nil {
		return nil, err
	}
	a, exp, _, err := atencionEnAlcance(l.ctx, l.svcCtx, userID, id)
	if err != nil {
		return nil, err
	}
	n := int(in.GetNumeroVersion())
	if n < 1 || n > a.VersionVigente {
		return nil, status.Error(codes.NotFound, "la versión solicitada no existe")
	}
	v, err := l.svcCtx.AtencionRepo.GetVersion(l.ctx, a.ID, n)
	if err != nil {
		return nil, atencionRepoError(l.ctx, err, "no se pudo consultar la versión de la atención")
	}
	if err := auditLectura(l.ctx, l.svcCtx, userID, repository.AccionView, a.ID, exp.EstudianteID, fmt.Sprintf("VERSION_%d", n)); err != nil {
		return nil, err
	}
	return &psicologia.AtencionResponse{Atencion: atencionToProto(a, v)}, nil
}

// AnnulAtencion: REGISTRADA -> ANULADA con motivo (sin borrar nada).
func (l *AtencionLogic) AnnulAtencion(in *psicologia.AnnulAtencionRequest) (*psicologia.AtencionResponse, error) {
	userID, err := authenticatedUser(l.ctx)
	if err != nil {
		return nil, err
	}
	id, err := validarId(in.GetId(), "el identificador no es válido")
	if err != nil {
		return nil, err
	}
	if err := requireAtencionPermission(l.ctx, l.svcCtx, userID, permAtencionesAnnul, id); err != nil {
		return nil, err
	}
	motivo := strings.TrimSpace(in.GetMotivo())
	if motivo == "" {
		return nil, status.Error(codes.InvalidArgument, "el motivo de la anulación es obligatorio")
	}
	if !validation.TextoDentroDeLimite(motivo, validation.MaxMotivo) {
		return nil, status.Error(codes.InvalidArgument, "el motivo de la anulación es demasiado largo")
	}
	_, _, scopeFilter, err := atencionEnAlcance(l.ctx, l.svcCtx, userID, id)
	if err != nil {
		return nil, err
	}
	a, err := l.svcCtx.AtencionRepo.Annul(l.ctx, repository.AnnulInput{
		ID: id, Motivo: motivo, ScopeResponsableID: scopeFilter, ActorID: userID,
	})
	if err != nil {
		return nil, atencionRepoError(l.ctx, err, "no se pudo anular la atención psicológica")
	}
	return respuestaConVersion(l.ctx, l.svcCtx, a)
}

// ExportAtencionPdf genera el PDF de una versión oficial (vigente por
// defecto), en memoria, y audita EXPORT antes de entregarlo. Un borrador no
// es un documento oficial y no se exporta.
func (l *AtencionLogic) ExportAtencionPdf(in *psicologia.ExportAtencionPdfRequest) (*psicologia.ExportAtencionPdfResponse, error) {
	userID, err := authenticatedUser(l.ctx)
	if err != nil {
		return nil, err
	}
	id, err := validarId(in.GetId(), "el identificador no es válido")
	if err != nil {
		return nil, err
	}
	if err := requireAtencionPermission(l.ctx, l.svcCtx, userID, permAtencionesView, id); err != nil {
		return nil, err
	}
	a, exp, _, err := atencionEnAlcance(l.ctx, l.svcCtx, userID, id)
	if err != nil {
		return nil, err
	}
	if a.Estado == repository.AtencionBorrador || a.Numero == nil {
		return nil, status.Error(codes.FailedPrecondition, "un borrador no es un documento oficial: regístralo antes de descargarlo")
	}
	n := int(in.GetNumeroVersion())
	if n == 0 {
		n = a.VersionVigente
	}
	if n < 1 || n > a.VersionVigente {
		return nil, status.Error(codes.NotFound, "la versión solicitada no existe")
	}
	v, err := l.svcCtx.AtencionRepo.GetVersion(l.ctx, a.ID, n)
	if err != nil {
		return nil, atencionRepoError(l.ctx, err, "no se pudo consultar la versión de la atención")
	}
	if l.svcCtx.EstudiantesDir == nil {
		return nil, status.Error(codes.Unavailable, "no se pudo identificar al estudiante")
	}
	nombre, err := l.svcCtx.EstudiantesDir.NombreCompleto(l.ctx, exp.EstudianteID)
	if err != nil {
		l.Errorf("no se pudo obtener el estudiante para el PDF: %v", err)
		return nil, err
	}

	c := v.Contenido
	contenido, err := pdf.Build(pdf.HojaAtencion{
		EstudianteNombre: nombre, ExpedienteCodigo: exp.Codigo, Numero: *a.Numero, Version: n, Estado: a.Estado,
		FechaAtencion: fechaDMY(c.FechaAtencion), HoraAtencion: c.HoraAtencion,
		RealizadaPor: c.RealizadaPor, RealizadaPorOtros: c.RealizadaPorOtros,
		Motivos: opcionesPdf(c.Motivos), MotivoOtros: c.MotivoOtros,
		DescripcionMotivo: c.DescripcionMotivo, AnalisisIntegral: c.AnalisisIntegral, Estrategias: c.Estrategias,
		Derivaciones: opcionesPdf(c.Derivaciones), DerivacionOtro: c.DerivacionOtro,
		SeguimientoResultados: c.SeguimientoResultados, Acuerdos: c.Acuerdos,
		CompromisoEstudiante: c.CompromisoEstudiante, CompromisoFamilia: c.CompromisoFamilia,
	})
	if err != nil {
		l.Errorf("no se pudo generar el PDF de la atención: %v", err)
		return nil, status.Error(codes.Internal, "no se pudo generar el PDF de la atención")
	}
	if err := auditLectura(l.ctx, l.svcCtx, userID, repository.AccionExport, a.ID, exp.EstudianteID, fmt.Sprintf("VERSION_%d", n)); err != nil {
		return nil, err
	}
	return &psicologia.ExportAtencionPdfResponse{
		// Sin nombre ni DNI del estudiante en el nombre del archivo.
		Nombre:    fmt.Sprintf("%s-AT%02d-v%d.pdf", exp.Codigo, *a.Numero, n),
		Contenido: contenido,
	}, nil
}

func opcionesPdf(ops []repository.OpcionCatalogo) []pdf.Opcion {
	out := make([]pdf.Opcion, 0, len(ops))
	for _, o := range ops {
		out = append(out, pdf.Opcion{Codigo: o.Codigo, Etiqueta: o.Etiqueta})
	}
	return out
}

func fechaDMY(iso string) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	return t.Format("02/01/2006")
}
