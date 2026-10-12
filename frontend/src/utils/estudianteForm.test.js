import { describe, expect, it } from "vitest";
import {
    EMPTY_ESTUDIANTE_FORM,
    apellidosResultantes,
    buildCreatePayload,
    buildPersonalPatch,
    validarPersonal,
    buildUpdatePatch,
    formFromEstudiante,
    validarBusqueda,
    validarEstudiante,
} from "./estudianteForm";

const base = { ...EMPTY_ESTUDIANTE_FORM, codigo: "C-1", nombres: "Ana", apellidos: "Soto" };

describe("validarBusqueda", () => {
    it("acepta vacío (listado general) y rechaza 1 carácter, >100 caracteres y >6 palabras", () => {
        expect(validarBusqueda("")).toBeNull();
        expect(validarBusqueda("   ")).toBeNull();
        expect(validarBusqueda("ab")).toBeNull();
        expect(validarBusqueda("a")).toMatch(/al menos 2/);
        expect(validarBusqueda("x".repeat(101))).toMatch(/100/);
        expect(validarBusqueda("a b c d e f g")).toMatch(/6 palabras/);
        expect(validarBusqueda("a b c d e f")).toBeNull();
    });
});

describe("validarEstudiante (alta)", () => {
    it("exige código, nombres y apellidos (o apellido paterno)", () => {
        const errors = validarEstudiante({ ...EMPTY_ESTUDIANTE_FORM });
        expect(errors.codigo).toBeTruthy();
        expect(errors.nombres).toBeTruthy();
        expect(errors.apellidos).toBeTruthy();
        expect(validarEstudiante(base)).toEqual({});
        expect(validarEstudiante({ ...base, apellidos: "", apellido_paterno: "DE LA CRUZ" })).toEqual({});
    });

    it("aplica las reglas de documento de E-03", () => {
        expect(validarEstudiante({ ...base, tipo_documento: "DNI", numero_documento: "1234567" }).numero_documento).toBeTruthy();
        expect(validarEstudiante({ ...base, tipo_documento: "DNI", numero_documento: "" }).numero_documento).toBeTruthy();
        expect(validarEstudiante({ ...base, tipo_documento: "DNI", numero_documento: "12345678" })).toEqual({});
        expect(validarEstudiante({ ...base, tipo_documento: "CE", numero_documento: "ab12" })).toEqual({});
        expect(validarEstudiante({ ...base, tipo_documento: "PASAPORTE", numero_documento: "A1" }).numero_documento).toBeTruthy();
        expect(validarEstudiante({ ...base, tipo_documento: "SIN_DOCUMENTO", numero_documento: "123" }).numero_documento).toBeTruthy();
    });

    it("valida SIAGIE, fecha, país/ubigeo, lenguas, foto y apellido materno sin paterno", () => {
        const personal = { personal: true };
        expect(validarEstudiante({ ...base, codigo_siagie: "123" }).codigo_siagie).toBeTruthy();
        expect(validarEstudiante({ ...base, fecha_nacimiento: "2015-02-30" }, "create", personal).fecha_nacimiento).toBeTruthy();
        expect(validarEstudiante({ ...base, fecha_nacimiento: "2999-01-01" }, "create", personal).fecha_nacimiento).toBeTruthy();
        expect(validarEstudiante({ ...base, ubigeo_nacimiento: "150101", pais_nacimiento: "CL" }, "create", personal).ubigeo_nacimiento).toBeTruthy();
        expect(validarEstudiante({ ...base, ubigeo_nacimiento: "150101", pais_nacimiento: "pe" }, "create", personal)).toEqual({});
        expect(validarEstudiante({ ...base, lengua_materna: "quechua", segunda_lengua: "QUECHUA" }, "create", personal).segunda_lengua).toBeTruthy();
        // FASE E-08: sin el permiso personal esos campos no forman parte del alta.
        expect(validarEstudiante({ ...base, fecha_nacimiento: "2999-01-01" })).toEqual({});
        expect(validarEstudiante({ ...base, foto_ref: "https://x.com/a.jpg" }).foto_ref).toBeTruthy();
        expect(validarEstudiante({ ...base, apellido_materno: "Soto" }).apellido_materno).toBeTruthy();
        expect(validarEstudiante({ ...base, estado: "RETIRADO" }).estado).toBeTruthy();
    });
});

describe("apellidos", () => {
    it("se derivan del paterno y materno sin separar apellidos compuestos", () => {
        expect(apellidosResultantes({ apellido_paterno: " DE  LA CRUZ ", apellido_materno: "Quispe", apellidos: "x" })).toBe(
            "DE LA CRUZ Quispe"
        );
        expect(apellidosResultantes({ apellido_paterno: "", apellido_materno: "", apellidos: " Del  Castillo " })).toBe("Del Castillo");
    });
});

describe("buildCreatePayload", () => {
    it("normaliza, omite vacíos y nunca envía campos académicos", () => {
        const payload = buildCreatePayload(
            {
            ...base,
            codigo: "  C-9 ",
            nombres: "  josé   Luis ",
            apellidos: "ignorado",
            apellido_paterno: "PEREZ",
            apellido_materno: "",
            tipo_documento: "CE",
            numero_documento: " ab1234 ",
            sexo: "m",
            pais_nacimiento: "pe",
            },
            { personal: true }
        );
        expect(payload).toEqual({
            codigo: "C-9",
            nombres: "josé Luis",
            apellido_paterno: "PEREZ",
            tipo_documento: "CE",
            numero_documento: "AB1234",
            sexo: "M",
            pais_nacimiento: "PE",
            estado: "ACTIVO",
        });
        for (const campo of ["nivel", "grado", "seccion", "turno", "estado_matricula", "apellidos"]) {
            expect(payload).not.toHaveProperty(campo);
        }
    });

    it("sin el permiso personal nunca envía datos personales (foto_ref sí: no es dato personal)", () => {
        const payload = buildCreatePayload({
            ...base,
            fecha_nacimiento: "2015-01-01",
            sexo: "F",
            pais_nacimiento: "PE",
            ubigeo_nacimiento: "150101",
            lengua_materna: "QUECHUA",
            segunda_lengua: "AIMARA",
            foto_ref: "fotos/a.jpg",
        });
        for (const campo of ["fecha_nacimiento", "sexo", "pais_nacimiento", "ubigeo_nacimiento", "lengua_materna", "segunda_lengua"]) {
            expect(payload).not.toHaveProperty(campo);
        }
        expect(payload.foto_ref).toBe("fotos/a.jpg");
    });
});

describe("datos personales (FASE E-08)", () => {
    const original = {
        fecha_nacimiento: "2013-07-21",
        sexo: "F",
        pais_nacimiento: "PE",
        ubigeo_nacimiento: "080101",
        lengua_materna: "AWAJUN",
        segunda_lengua: "",
        version: 3,
    };

    it("valida con las reglas del backend", () => {
        expect(validarPersonal(original)).toEqual({});
        expect(validarPersonal({ ...original, pais_nacimiento: "CL" }).ubigeo_nacimiento).toBeTruthy();
        expect(validarPersonal({ ...original, sexo: "X" }).sexo).toBeTruthy();
        expect(validarPersonal({ ...original, segunda_lengua: "awajun" }).segunda_lengua).toBeTruthy();
    });

    it("el PATCH lleva solo lo cambiado, normalizado, y siempre expected_version", () => {
        const { changed, body } = buildPersonalPatch(original, { ...original, sexo: "m", segunda_lengua: " quechua " }, 5);
        expect(changed).toBe(true);
        expect(body).toEqual({ sexo: "M", segunda_lengua: "QUECHUA", expected_version: 5 });
        expect(buildPersonalPatch(original, { ...original, ubigeo_nacimiento: "" }, 3).body).toEqual({
            ubigeo_nacimiento: "",
            expected_version: 3,
        });
        expect(buildPersonalPatch(original, { ...original }, 3).changed).toBe(false);
    });
});

describe("buildUpdatePatch", () => {
    const original = {
        id: "x",
        codigo: "C-1",
        nombres: "Ana",
        apellidos: "PEREZ SOTO",
        apellido_paterno: "PEREZ",
        apellido_materno: "SOTO",
        codigo_siagie: "",
        tipo_documento: "SIN_DOCUMENTO",
        numero_documento: "",
        estado: "ACTIVO",
        version: 7,
    };

    it("envía solo lo que cambió y SIEMPRE expected_version con la versión leída", () => {
        const form = { ...formFromEstudiante(original), nombres: "Ana María" };
        expect(buildUpdatePatch(original, form)).toEqual({ changed: true, body: { nombres: "Ana María", expected_version: 7 } });
    });

    it("sin cambios no hay nada que guardar", () => {
        expect(buildUpdatePatch(original, formFromEstudiante(original)).changed).toBe(false);
    });

    it("envía tipo y número de documento juntos", () => {
        const form = { ...formFromEstudiante(original), tipo_documento: "DNI", numero_documento: "87654321" };
        expect(buildUpdatePatch(original, form).body).toEqual({
            tipo_documento: "DNI",
            numero_documento: "87654321",
            expected_version: 7,
        });
    });

    it("no incluye código, estado ni campos académicos", () => {
        const form = { ...formFromEstudiante(original), codigo: "OTRO", estado: "INACTIVO", nombres: "Ana María" };
        const { body } = buildUpdatePatch(original, form);
        for (const campo of ["codigo", "estado", "nivel", "grado", "seccion", "turno", "estado_matricula"]) {
            expect(body).not.toHaveProperty(campo);
        }
    });
});

describe("modelo maestro: domicilio y contacto", () => {
    const base = { fecha_nacimiento: "", sexo: "", pais_nacimiento: "", ubigeo_nacimiento: "", lengua_materna: "", segunda_lengua: "",
        domicilio_direccion: "", domicilio_departamento: "", domicilio_provincia: "", domicilio_distrito: "", domicilio_ubigeo: "",
        correo: "", telefono: "" };

    it("valida ubigeo, correo y teléfono sin reescribirlos", () => {
        expect(validarPersonal(base)).toEqual({});
        expect(validarPersonal({ ...base, domicilio_ubigeo: "0808" }).domicilio_ubigeo).toBeTruthy();
        expect(validarPersonal({ ...base, correo: "a@b" }).correo).toBeTruthy();
        expect(validarPersonal({ ...base, correo: "familia@correo.pe" })).toEqual({});
        expect(validarPersonal({ ...base, telefono: "12ab" }).telefono).toBeTruthy();
        expect(validarPersonal({ ...base, telefono: "+51 984 123 456" })).toEqual({});
    });

    it("el PATCH incluye solo el domicilio y contacto cambiados", () => {
        const { body } = buildPersonalPatch(base, { ...base, domicilio_distrito: "  Tinta ", correo: " a@b.pe " }, 2);
        expect(body).toEqual({ domicilio_distrito: "Tinta", correo: "a@b.pe", expected_version: 2 });
    });
});
