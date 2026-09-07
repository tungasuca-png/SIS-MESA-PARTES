// MOCK de permisos — solo para maquetar el Dashboard (menús, botones, acciones).
// La autorización real vive en el API Gateway + Auth Service + microservicios;
// esta tabla NO debe tratarse como fuente de verdad de seguridad.
//
// "expedientes.update" y "expedientes.change_estado" se agregaron para la
// integración con Expedientes Service: reflejan (pero no reemplazan) la regla
// real de ese microservicio, donde solo el personal interno puede editar o
// cambiar el estado de un expediente (ver authorization.go de Expedientes
// Service). El backend sigue siendo quien realmente lo hace cumplir.
//
// "documentos.view/create/delete" se alinearon con Documentos Service: TODO
// el personal interno puede ver/subir/eliminar cualquier tipo de documento;
// SOLICITANTE puede ver y subir, pero solo lo suyo ("documentos.view_own",
// igual que "expedientes.view_own") — el backend ya acota el listado global
// a "lo que este usuario subió" cuando el rol no es interno (ver
// authorization.CanViewAll en Documentos Service). El backend restringe
// además su subida a tipo ADJUNTO — el frontend no distingue por tipo, solo
// el backend lo hace cumplir.
export const ROLE_PERMISSIONS = {
    ADMIN: [
        "dashboard.view",
        "expedientes.view",
        "expedientes.create",
        "expedientes.update",
        "expedientes.change_estado",
        "documentos.view",
        "documentos.create",
        "documentos.delete",
        "derivaciones.view",
        "seguimiento.view",
        "reportes.view",
        "usuarios.view",
        "configuracion.view",
    ],
    DIRECTOR: [
        "dashboard.view",
        "expedientes.view",
        "expedientes.update",
        "expedientes.change_estado",
        "documentos.view",
        "documentos.create",
        "documentos.delete",
        "derivaciones.view",
        "seguimiento.view",
        "reportes.view",
    ],
    SUBDIRECTOR: [
        "dashboard.view",
        "expedientes.view",
        "expedientes.update",
        "expedientes.change_estado",
        "documentos.view",
        "documentos.create",
        "documentos.delete",
        "derivaciones.view",
        "seguimiento.view",
        "reportes.view",
    ],
    SECRETARIA: [
        "dashboard.view",
        "expedientes.view",
        "expedientes.create",
        "expedientes.update",
        "expedientes.change_estado",
        "documentos.view",
        "documentos.create",
        "documentos.delete",
        "derivaciones.view",
        "seguimiento.view",
    ],
    DOCENTE: [
        "dashboard.view",
        "expedientes.view",
        "expedientes.update",
        "expedientes.change_estado",
        "documentos.view",
        "documentos.create",
        "documentos.delete",
        "seguimiento.view",
    ],
    AUXILIAR: [
        "dashboard.view",
        "expedientes.view",
        "expedientes.update",
        "expedientes.change_estado",
        "documentos.view",
        "documentos.create",
        "documentos.delete",
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
