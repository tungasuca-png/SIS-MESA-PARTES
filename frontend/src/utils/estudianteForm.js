import {
    BUSQUEDA_MAX,
    BUSQUEDA_MAX_PALABRAS,
    BUSQUEDA_MIN,
    MAX_APELLIDO,
    MAX_APELLIDOS,
    MAX_CODIGO,
    MAX_FOTO_REF,
    MAX_NOMBRES,
} from "../constants/estudiantes";

// Maestro de Estudiantes (FASE E-06): validación TEMPRANA de formularios y
// armado de solicitudes. Replica las reglas de Estudiantes Service (E-05)
// solo para avisar antes de enviar; el backend sigue siendo la autoridad y
// vuelve a validar todo.

// Mismo criterio que el backend: recorta y colapsa espacios, sin cambiar
// mayúsculas ("DE LA CRUZ" se conserva).
export const cleanText = (value) => (value ?? "").split(/\s+/).filter(Boolean).join(" ");
const upper = (value) => (value ?? "").trim().toUpperCase();

// ---- Búsqueda del listado ----
export function validarBusqueda(q) {
    const texto = (q ?? "").trim();
    if (!texto) return null;
    if ([...texto].length < BUSQUEDA_MIN) return `Escribe al menos ${BUSQUEDA_MIN} caracteres para buscar.`;
    if ([...texto].length > BUSQUEDA_MAX) return `La búsqueda admite como máximo ${BUSQUEDA_MAX} caracteres.`;
    if (texto.split(/\s+/).length > BUSQUEDA_MAX_PALABRAS) {
        return `La búsqueda admite como máximo ${BUSQUEDA_MAX_PALABRAS} palabras.`;
    }
    return null;
}

// ---- Formulario ----
export const EMPTY_ESTUDIANTE_FORM = {
    codigo: "",
    codigo_siagie: "",
    tipo_documento: "SIN_DOCUMENTO",
    numero_documento: "",
    nombres: "",
    apellido_paterno: "",
    apellido_materno: "",
    apellidos: "",
    fecha_nacimiento: "",
    sexo: "",
    pais_nacimiento: "",
    ubigeo_nacimiento: "",
    lengua_materna: "",
    segunda_lengua: "",
    foto_ref: "",
    estado: "ACTIVO",
};

// Formulario de edición a partir del registro BÁSICO que devuelve el
// backend. Solo contiene lo que el backend permite leer: los datos
// personales (fecha de nacimiento, sexo, lugar de nacimiento, lenguas, foto)
// no se exponen en ninguna lectura y por eso no se editan desde aquí.
export function formFromEstudiante(estudiante) {
    return {
        ...EMPTY_ESTUDIANTE_FORM,
        codigo: estudiante?.codigo ?? "",
        codigo_siagie: estudiante?.codigo_siagie ?? "",
        tipo_documento: estudiante?.tipo_documento || "SIN_DOCUMENTO",
        numero_documento: estudiante?.numero_documento ?? "",
        nombres: estudiante?.nombres ?? "",
        apellido_paterno: estudiante?.apellido_paterno ?? "",
        apellido_materno: estudiante?.apellido_materno ?? "",
        apellidos: estudiante?.apellidos ?? "",
        estado: estudiante?.estado || "ACTIVO",
    };
}

// Apellidos que se guardarán: con apellido paterno se derivan (paterno +
// materno); sin él, se usa el campo "apellidos" tal cual. Nunca se separan.
export function apellidosResultantes(form) {
    const paterno = cleanText(form.apellido_paterno);
    if (paterno) return cleanText(`${paterno} ${cleanText(form.apellido_materno)}`);
    return cleanText(form.apellidos);
}

const tieneControl = (value) => [...value].some((ch) => ch.charCodeAt(0) < 32 || ch.charCodeAt(0) === 127);

// validarEstudiante devuelve { campo: mensaje }. `modo` = "create" | "edit":
// en edición no se validan los campos que el formulario no muestra.
// opciones.personal (FASE E-08): el alta incluye datos personales.
export function validarEstudiante(form, modo = "create", opciones = {}) {
    const errors = {};
    const texto = (campo, valor, max, obligatorio, etiqueta) => {
        if (!valor) {
            if (obligatorio) errors[campo] = `${etiqueta} es obligatorio.`;
            return;
        }
        if ([...valor].length > max) errors[campo] = `${etiqueta} admite como máximo ${max} caracteres.`;
        else if (tieneControl(valor)) errors[campo] = `${etiqueta} contiene caracteres no permitidos.`;
    };

    if (modo === "create") texto("codigo", cleanText(form.codigo), MAX_CODIGO, true, "El código");
    texto("nombres", cleanText(form.nombres), MAX_NOMBRES, true, "Nombres");

    const paterno = cleanText(form.apellido_paterno);
    const materno = cleanText(form.apellido_materno);
    texto("apellido_paterno", paterno, MAX_APELLIDO, false, "Apellido paterno");
    texto("apellido_materno", materno, MAX_APELLIDO, false, "Apellido materno");
    if (materno && !paterno) errors.apellido_materno = "El apellido materno requiere el apellido paterno.";
    const apellidos = apellidosResultantes(form);
    if (!apellidos) {
        errors[paterno ? "apellido_paterno" : "apellidos"] = "Ingresa el apellido paterno o los apellidos.";
    } else if ([...apellidos].length > MAX_APELLIDOS) {
        errors.apellidos = `Los apellidos admiten como máximo ${MAX_APELLIDOS} caracteres.`;
    }

    const tipo = form.tipo_documento || "SIN_DOCUMENTO";
    const numero = upper(form.numero_documento);
    if (tipo === "SIN_DOCUMENTO") {
        if (numero) errors.numero_documento = "Sin documento no admite número.";
    } else if (tipo === "DNI") {
        if (!/^[0-9]{8}$/.test(numero)) errors.numero_documento = "El DNI debe tener 8 dígitos.";
    } else if (tipo === "CE" || tipo === "PASAPORTE") {
        if (!/^[A-Z0-9]{4,12}$/.test(numero)) errors.numero_documento = "Debe tener de 4 a 12 letras o dígitos.";
    } else {
        errors.tipo_documento = "Tipo de documento inválido.";
    }

    const siagie = (form.codigo_siagie ?? "").trim();
    if (siagie && !/^[0-9]{14}$/.test(siagie)) errors.codigo_siagie = "El código SIAGIE debe tener 14 dígitos.";

    if (modo === "create") {
        // FASE E-08: los datos personales se validan solo si el formulario
        // los incluye (usuario con estudiantes.personal.update).
        if (opciones.personal) Object.assign(errors, validarPersonal(form));
        const foto = (form.foto_ref ?? "").trim();
        if (foto && /^[a-zA-Z][a-zA-Z0-9+.-]*:\/\//.test(foto)) errors.foto_ref = "Debe ser una referencia interna, no una URL.";
        else if ([...foto].length > MAX_FOTO_REF) errors.foto_ref = `Admite como máximo ${MAX_FOTO_REF} caracteres.`;
        if (form.estado !== "ACTIVO" && form.estado !== "INACTIVO") errors.estado = "Estado inválido.";
    }
    return errors;
}

// ---- FASE E-08: datos personales (dominio separado) ----
// Solo estos seis campos; nunca foto_ref, identidad, estado ni académicos.
export const CAMPOS_PERSONALES = [
    "fecha_nacimiento",
    "sexo",
    "pais_nacimiento",
    "ubigeo_nacimiento",
    "lengua_materna",
    "segunda_lengua",
    // Modelo maestro: domicilio vigente y contacto (mismo dominio personal).
    "domicilio_direccion",
    "domicilio_departamento",
    "domicilio_provincia",
    "domicilio_distrito",
    "domicilio_ubigeo",
    "correo",
    "telefono",
];

// Normalización igual a la del backend: códigos en mayúsculas, recortados.
export function normalizarPersonal(form) {
    return {
        fecha_nacimiento: (form.fecha_nacimiento ?? "").trim(),
        sexo: upper(form.sexo),
        pais_nacimiento: upper(form.pais_nacimiento),
        ubigeo_nacimiento: (form.ubigeo_nacimiento ?? "").trim(),
        lengua_materna: upper(form.lengua_materna),
        segunda_lengua: upper(form.segunda_lengua),
        // Domicilio: texto con espacios colapsados (sin cambiar mayúsculas).
        domicilio_direccion: cleanText(form.domicilio_direccion),
        domicilio_departamento: cleanText(form.domicilio_departamento),
        domicilio_provincia: cleanText(form.domicilio_provincia),
        domicilio_distrito: cleanText(form.domicilio_distrito),
        domicilio_ubigeo: (form.domicilio_ubigeo ?? "").trim(),
        // Contacto: solo recortado; no se reescribe su contenido.
        correo: (form.correo ?? "").trim(),
        telefono: (form.telefono ?? "").trim(),
    };
}

export function formFromPersonal(personal) {
    return Object.fromEntries(CAMPOS_PERSONALES.map((campo) => [campo, personal?.[campo] ?? ""]));
}

// Mismas reglas que Estudiantes Service (E-03/E-05/E-08).
export function validarPersonal(form) {
    const errors = {};
    const p = normalizarPersonal(form);
    if (p.fecha_nacimiento) {
        const [y, m, d] = p.fecha_nacimiento.split("-").map(Number);
        const date = new Date(Date.UTC(y, (m || 0) - 1, d || 0));
        const valida =
            /^\d{4}-\d{2}-\d{2}$/.test(p.fecha_nacimiento) && date.getUTCMonth() === m - 1 && date.getUTCDate() === d;
        if (!valida) errors.fecha_nacimiento = "Fecha inválida (AAAA-MM-DD).";
        else if (y < 1900 || date > new Date()) errors.fecha_nacimiento = "La fecha de nacimiento está fuera de rango.";
    }
    if (p.sexo && p.sexo !== "M" && p.sexo !== "F") errors.sexo = "Selecciona M o F.";
    if (p.pais_nacimiento && !/^[A-Z]{2}$/.test(p.pais_nacimiento)) {
        errors.pais_nacimiento = "Usa el código de 2 letras (p. ej. PE).";
    }
    if (p.ubigeo_nacimiento) {
        if (!/^[0-9]{6}$/.test(p.ubigeo_nacimiento)) errors.ubigeo_nacimiento = "El ubigeo debe tener 6 dígitos.";
        else if (p.pais_nacimiento !== "PE") errors.ubigeo_nacimiento = "El ubigeo solo aplica si el país es PE.";
    }
    const lengua = /^[A-Z0-9_]{1,10}$/;
    if (p.lengua_materna && !lengua.test(p.lengua_materna)) errors.lengua_materna = "Código de lengua inválido (hasta 10 letras).";
    if (p.segunda_lengua && !lengua.test(p.segunda_lengua)) errors.segunda_lengua = "Código de lengua inválido (hasta 10 letras).";
    if (p.lengua_materna && p.segunda_lengua && p.lengua_materna === p.segunda_lengua) {
        errors.segunda_lengua = "Debe ser distinta de la lengua materna.";
    }
    // Domicilio y contacto (mismas reglas que Estudiantes Service).
    if (form.domicilio_direccion !== undefined) {
        if ([...p.domicilio_direccion].length > 200) errors.domicilio_direccion = "Admite como máximo 200 caracteres.";
        for (const campo of ["domicilio_departamento", "domicilio_provincia", "domicilio_distrito"]) {
            if ([...p[campo]].length > 60) errors[campo] = "Admite como máximo 60 caracteres.";
        }
        if (p.domicilio_ubigeo && !/^[0-9]{6}$/.test(p.domicilio_ubigeo)) {
            errors.domicilio_ubigeo = "El ubigeo debe tener 6 dígitos.";
        }
        if (p.correo && ([...p.correo].length > 254 || !/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(p.correo))) {
            errors.correo = "El correo no tiene un formato válido.";
        }
        if (p.telefono && !/^\+?[0-9][0-9 -]{5,18}$/.test(p.telefono)) {
            errors.telefono = "Usa de 7 a 20 dígitos (se admiten +, espacios y guiones).";
        }
    }
    return errors;
}

// Cuerpo de PATCH /api/estudiantes/:id/personal: SOLO los campos personales
// que cambiaron ("" = dejarlo vacío) + expected_version (siempre).
export function buildPersonalPatch(original, form, expectedVersion) {
    const nuevos = normalizarPersonal(form);
    const patch = {};
    for (const campo of CAMPOS_PERSONALES) {
        if (nuevos[campo] !== (original?.[campo] ?? "")) patch[campo] = nuevos[campo];
    }
    const changed = Object.keys(patch).length > 0;
    return { changed, body: { ...patch, expected_version: expectedVersion }, normalizados: nuevos };
}

// Cuerpo de POST /api/estudiantes: solo campos del maestro (nunca nivel,
// grado, sección, turno ni estado de matrícula); los vacíos se omiten.
// FASE E-08: los datos personales solo viajan si opciones.personal (el
// usuario tiene estudiantes.personal.update); si no, nunca se envían.
export function buildCreatePayload(form, opciones = {}) {
    const paterno = cleanText(form.apellido_paterno);
    const payload = {
        codigo: cleanText(form.codigo),
        codigo_siagie: (form.codigo_siagie ?? "").trim(),
        tipo_documento: form.tipo_documento || "SIN_DOCUMENTO",
        numero_documento: form.tipo_documento === "SIN_DOCUMENTO" ? "" : upper(form.numero_documento),
        nombres: cleanText(form.nombres),
        apellido_paterno: paterno,
        apellido_materno: paterno ? cleanText(form.apellido_materno) : "",
        apellidos: paterno ? "" : cleanText(form.apellidos),
        fecha_nacimiento: (form.fecha_nacimiento ?? "").trim(),
        sexo: upper(form.sexo),
        pais_nacimiento: upper(form.pais_nacimiento),
        ubigeo_nacimiento: (form.ubigeo_nacimiento ?? "").trim(),
        lengua_materna: upper(form.lengua_materna),
        segunda_lengua: upper(form.segunda_lengua),
        foto_ref: (form.foto_ref ?? "").trim(),
        estado: form.estado || "ACTIVO",
    };
    if (!opciones.personal) {
        for (const campo of CAMPOS_PERSONALES) delete payload[campo];
    }
    return Object.fromEntries(Object.entries(payload).filter(([, value]) => value !== ""));
}

// Cuerpo de PATCH /api/estudiantes/:id: SOLO los campos que cambiaron
// respecto del registro leído ("" = dejarlo vacío) + expected_version, que
// SIEMPRE viaja con la versión recibida del backend. No incluye código,
// estado (acción propia) ni datos académicos.
export function buildUpdatePatch(original, form) {
    const paterno = cleanText(form.apellido_paterno);
    const tipo = form.tipo_documento || "SIN_DOCUMENTO";
    const nuevos = {
        codigo_siagie: (form.codigo_siagie ?? "").trim(),
        tipo_documento: tipo,
        numero_documento: tipo === "SIN_DOCUMENTO" ? "" : upper(form.numero_documento),
        nombres: cleanText(form.nombres),
        apellido_paterno: paterno,
        apellido_materno: cleanText(form.apellido_materno),
    };
    const patch = {};
    for (const [campo, valor] of Object.entries(nuevos)) {
        if (valor !== (original?.[campo] ?? "")) patch[campo] = valor;
    }
    // Sin apellido paterno, "apellidos" se edita directamente.
    if (!paterno) {
        const apellidos = cleanText(form.apellidos);
        if (apellidos !== (original?.apellidos ?? "")) patch.apellidos = apellidos;
    }
    // Documento: se envían juntos si cambia cualquiera de los dos, porque el
    // backend valida la combinación final tipo + número.
    if ("tipo_documento" in patch || "numero_documento" in patch) {
        patch.tipo_documento = nuevos.tipo_documento;
        patch.numero_documento = nuevos.numero_documento;
    }
    const changed = Object.keys(patch).length > 0;
    return { changed, body: { ...patch, expected_version: original?.version } };
}
