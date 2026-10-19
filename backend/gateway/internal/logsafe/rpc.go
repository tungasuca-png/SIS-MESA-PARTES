package logsafe

import (
	"auth/auth"
	"estudiantes/estudiantes"
	"psicologia/psicologia"

	"github.com/zeromicro/go-zero/zrpc"
)

// sensitiveRPCMethods son los métodos gRPC cuya SOLICITUD o RESPUESTA no debe
// registrarse nunca. Los nombres salen de las constantes generadas por protoc
// (no son cadenas inventadas).
//
// go-zero registra, en el cliente gRPC, "fail - <método> - <solicitud> - <error>"
// ante cualquier fallo y, en llamadas lentas, también la respuesta. Con
// DontLogClientContentForMethod se conserva la línea de error/duración
// ("fail - <método> - <error>") pero SIN el contenido. No se usa
// `Middlewares: Duration: false` porque eliminaría también esos errores.
var sensitiveRPCMethods = []string{
	// Auth: contraseñas, DNI + clave, JWT y refresh tokens (solicitud y/o respuesta).
	auth.Auth_Login_FullMethodName,
	auth.Auth_Register_FullMethodName,
	auth.Auth_RefreshToken_FullMethodName,
	auth.Auth_Logout_FullMethodName,
	auth.Auth_ValidateToken_FullMethodName,
	auth.Auth_CreateUser_FullMethodName,

	// Psicología: identificadores de estudiantes y texto clínico (resumen_caso,
	// motivo de cierre) en solicitudes y respuestas.
	psicologia.Psicologia_OpenExpediente_FullMethodName,
	psicologia.Psicologia_GetExpediente_FullMethodName,
	psicologia.Psicologia_GetExpedienteByEstudiante_FullMethodName,
	psicologia.Psicologia_ListExpedientes_FullMethodName,
	psicologia.Psicologia_UpdateExpediente_FullMethodName,
	psicologia.Psicologia_CloseExpediente_FullMethodName,
	psicologia.Psicologia_ReopenExpediente_FullMethodName,
	// Atenciones (Hoja de Atención): contenido clínico, motivos de corrección
	// y anulación, y el PDF generado.
	psicologia.Psicologia_CreateAtencionDraft_FullMethodName,
	psicologia.Psicologia_UpdateAtencionDraft_FullMethodName,
	psicologia.Psicologia_RegisterAtencion_FullMethodName,
	psicologia.Psicologia_DiscardAtencionDraft_FullMethodName,
	psicologia.Psicologia_GetAtencion_FullMethodName,
	psicologia.Psicologia_ListAtenciones_FullMethodName,
	psicologia.Psicologia_CorrectAtencion_FullMethodName,
	psicologia.Psicologia_ListAtencionVersions_FullMethodName,
	psicologia.Psicologia_GetAtencionVersion_FullMethodName,
	psicologia.Psicologia_AnnulAtencion_FullMethodName,
	psicologia.Psicologia_ExportAtencionPdf_FullMethodName,

	// Estudiantes: nombres y datos de matrícula de estudiantes (solicitud y/o
	// respuesta) — datos personales de menores de edad.
	estudiantes.Estudiantes_GetEstudiante_FullMethodName,
	estudiantes.Estudiantes_SearchEstudiantes_FullMethodName,
	estudiantes.Estudiantes_GetEstudiantesBasic_FullMethodName,
	// FASE E-05: la escritura lleva documento, nombres y fecha de nacimiento.
	estudiantes.Estudiantes_CreateEstudiante_FullMethodName,
	estudiantes.Estudiantes_UpdateEstudiante_FullMethodName,
	estudiantes.Estudiantes_ChangeEstadoEstudiante_FullMethodName,
	// FASE E-08: datos personales (fecha de nacimiento, sexo, lugar, lenguas).
	estudiantes.Estudiantes_GetEstudiantePersonal_FullMethodName,
	estudiantes.Estudiantes_UpdateEstudiantePersonal_FullMethodName,
	// Modelo maestro: resolución (documento), duplicados, matrícula y familia.
	estudiantes.Estudiantes_ResolveEstudiante_FullMethodName,
	estudiantes.Estudiantes_ListPosiblesDuplicados_FullMethodName,
	estudiantes.Estudiantes_ListMatriculasEstudiante_FullMethodName,
	estudiantes.Estudiantes_ListFamiliaresEstudiante_FullMethodName,
	estudiantes.Estudiantes_VincularFamiliar_FullMethodName,
	estudiantes.Estudiantes_DesvincularFamiliar_FullMethodName,
	// Etapa 5: personas (documento, nombres), apoderado, periodos, secciones
	// y matrículas (estudiante, sección, motivos de retiro o traslado).
	estudiantes.Estudiantes_ActualizarFamiliar_FullMethodName,
	estudiantes.Estudiantes_CreatePersona_FullMethodName,
	estudiantes.Estudiantes_GetPersona_FullMethodName,
	estudiantes.Estudiantes_SearchPersonas_FullMethodName,
	estudiantes.Estudiantes_UpdatePersona_FullMethodName,
	estudiantes.Estudiantes_CreatePeriodo_FullMethodName,
	estudiantes.Estudiantes_ListPeriodos_FullMethodName,
	estudiantes.Estudiantes_ChangeEstadoPeriodo_FullMethodName,
	estudiantes.Estudiantes_CreateSeccion_FullMethodName,
	estudiantes.Estudiantes_ListSecciones_FullMethodName,
	estudiantes.Estudiantes_MatricularEstudiante_FullMethodName,
	estudiantes.Estudiantes_CambiarSeccionMatricula_FullMethodName,
	estudiantes.Estudiantes_RetirarMatricula_FullMethodName,
	estudiantes.Estudiantes_ReingresarMatricula_FullMethodName,
	estudiantes.Estudiantes_ConcluirMatricula_FullMethodName,
	estudiantes.Estudiantes_ListMatriculasPeriodo_FullMethodName,
}

// ProtectSensitiveRPCLogs debe llamarse una vez al arrancar, antes de crear los
// clientes gRPC. El registro es global del proceso y se indexa por método.
func ProtectSensitiveRPCLogs() {
	for _, method := range sensitiveRPCMethods {
		zrpc.DontLogClientContentForMethod(method)
	}
}
