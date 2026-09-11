// MOCK de permisos — solo para maquetar el Dashboard (menús, botones, acciones).
// La autorización real vive en el API Gateway + Auth Service + microservicios;
// esta tabla NO debe tratarse como fuente de verdad de seguridad.
//
// "expedientes.update", "expedientes.change_estado" y "expedientes.delete" se
// agregaron para la integración con Expedientes Service: reflejan (pero no
// reemplazan) la regla real de ese microservicio, donde solo el personal
// interno puede editar, cambiar el estado o eliminar (baja lógica) un
// expediente (ver authorization.go de Expedientes Service). El backend sigue
// siendo quien realmente lo hace cumplir.
//
// "documentos.view/create/delete" se alinearon con Documentos Service: TODO
// el personal interno puede ver/subir/eliminar cualquier tipo de documento;
// SOLICITANTE puede ver y subir, pero solo lo suyo ("documentos.view_own",
// igual que "expedientes.view_own") — el backend ya acota el listado global
// a "lo que este usuario subió" cuando el rol no es interno (ver
// authorization.CanViewAll en Documentos Service). El backend restringe
// además su subida a tipo ADJUNTO — el frontend no distingue por tipo, solo
// el backend lo hace cumplir.
//
// "derivaciones.view/create" se alinearon con Derivaciones Service: TODO el
// personal interno puede registrar y ver derivaciones (mismo criterio que
// CanCreate/CanViewAll de ese microservicio — es una acción de ruteo interno
// del expediente, no algo que decida un SOLICITANTE). Un SOLICITANTE puede
// ver el historial de derivaciones de SU propio expediente puntual (por eso
// no tiene "derivaciones.view" acá: ese permiso es para el listado global;
// el panel del expediente no depende de este permiso, solo de estar
// autenticado — el backend ya lo acota a un expediente_id obligatorio para
// quien no sea personal interno).
export const ROLE_PERMISSIONS = {
    ADMIN: [
        "dashboard.view",
        "expedientes.view",
        "expedientes.create",
        "expedientes.update",
        "expedientes.change_estado",
        "expedientes.delete",
        "documentos.view",
        "documentos.create",
        "documentos.delete",
        "derivaciones.view",
        "derivaciones.create",
        "seguimiento.view",
        "reportes.view",
        "usuarios.view",
        "roles.view",
        "configuracion.view",
    ],
    DIRECTOR: [
        "dashboard.view",
        "expedientes.view",
        "expedientes.update",
        "expedientes.change_estado",
        "expedientes.delete",
        "documentos.view",
        "documentos.create",
        "documentos.delete",
        "derivaciones.view",
        "derivaciones.create",
        "seguimiento.view",
        "reportes.view",
    ],
    SUBDIRECTOR: [
        "dashboard.view",
        "expedientes.view",
        "expedientes.update",
        "expedientes.change_estado",
        "expedientes.delete",
        "documentos.view",
        "documentos.create",
        "documentos.delete",
        "derivaciones.view",
        "derivaciones.create",
        "seguimiento.view",
        "reportes.view",
    ],
    SECRETARIA: [
        "dashboard.view",
        "expedientes.view",
        "expedientes.create",
        "expedientes.update",
        "expedientes.change_estado",
        "expedientes.delete",
        "documentos.view",
        "documentos.create",
        "documentos.delete",
        "derivaciones.view",
        "derivaciones.create",
        "seguimiento.view",
    ],
    DOCENTE: [
        "dashboard.view",
        "expedientes.view",
        "expedientes.update",
        "expedientes.change_estado",
        "expedientes.delete",
        "documentos.view",
        "documentos.create",
        "documentos.delete",
        "derivaciones.view",
        "derivaciones.create",
        "seguimiento.view",
    ],
    AUXILIAR: [
        "dashboard.view",
        "expedientes.view",
        "expedientes.update",
        "expedientes.change_estado",
        "expedientes.delete",
        "documentos.view",
        "documentos.create",
        "documentos.delete",
        "derivaciones.view",
        "derivaciones.create",
        "seguimiento.view",
    ],
    SOLICITANTE: [
        "dashboard.view",
        "solicitudes.create",
        "expedientes.view_own",
        "documentos.view_own",
        "documentos.create",
        "seguimiento.view_own",
    ],
};

export const getPermissionsForRole = (role) => ROLE_PERMISSIONS[role] ?? [];
