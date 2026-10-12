import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import {
    TODOS_LOS_PERMISOS,
    estudianteBasico,
    httpError,
    renderEstudiantes,
} from "../../test/estudiantesTestUtils";
import * as service from "../../services/estudiantesService";

vi.mock("../../services/estudiantesService", () => ({
    searchEstudiantes: vi.fn(),
    getEstudiante: vi.fn(),
    getEstudiantesBasic: vi.fn(),
    createEstudiante: vi.fn(),
    updateEstudiante: vi.fn(),
    changeEstadoEstudiante: vi.fn(),
    getEstudiantePersonal: vi.fn(),
    updateEstudiantePersonal: vi.fn(),
    getMatriculasEstudiante: vi.fn(),
    getFamiliaresEstudiante: vi.fn(),
}));
vi.mock("../../components/portal/RoleLayout", () => ({
    default: ({ title, children }) => (
        <div>
            <h1>{title}</h1>
            {children}
        </div>
    ),
}));

const basico = estudianteBasico({ codigo_siagie: "12345678901234" });
// GET /api/estudiantes/:id devuelve SOLO estos campos (contrato real).
const detalle = {
    id: basico.id,
    codigo: basico.codigo,
    nombres: basico.nombres,
    apellidos: basico.apellidos,
    nivel: basico.nivel,
    grado: basico.grado,
    seccion: basico.seccion,
    turno: basico.turno,
    estado_matricula: basico.estado_matricula,
    version: basico.version,
};
const ruta = `/estudiantes/${basico.id}`;

const cargar = (overrides = {}) => {
    service.getEstudiante.mockResolvedValue({ estudiante: { ...detalle, ...overrides } });
    service.getEstudiantesBasic.mockResolvedValue({ estudiantes: [{ ...basico, ...overrides }] });
};

beforeEach(() => {
    vi.clearAllMocks();
    cargar();
    service.getMatriculasEstudiante.mockResolvedValue({ matriculas: [] });
    service.getFamiliaresEstudiante.mockResolvedValue({ familiares: [] });
});
afterEach(cleanup);

describe("Detalle del estudiante", () => {
    it("combina GET /:id y basic-batch y separa identidad de datos académicos", async () => {
        renderEstudiantes(ruta);
        expect(await screen.findByText("Identidad básica")).toBeTruthy();
        expect(service.getEstudiante).toHaveBeenCalledWith(basico.id);
        expect(service.getEstudiantesBasic).toHaveBeenCalledWith([basico.id]);
        expect(screen.getByText("12345678901234")).toBeTruthy();
        expect(screen.getByText("DNI 12345678")).toBeTruthy();
        expect(screen.getByText("Pérez / Soto")).toBeTruthy();
        expect(screen.getByText("Información académica actual")).toBeTruthy();
        expect(screen.getByText(/Datos heredados, solo de lectura/)).toBeTruthy();
        // FASE E-08: sin estudiantes.personal.view no hay sección personal
        // ni se consulta el endpoint personal.
        expect(screen.queryByText("Datos personales")).toBeNull();
        expect(screen.queryByText("Fecha de nacimiento")).toBeNull();
        expect(service.getEstudiantePersonal).not.toHaveBeenCalled();
    });

    it("estudiante inexistente", async () => {
        service.getEstudiante.mockRejectedValue(httpError(404, "estudiante no encontrado"));
        renderEstudiantes(ruta);
        expect(await screen.findByText("El estudiante no existe o ya no está disponible.")).toBeTruthy();
    });

    it("sin permisos de escritura no muestra Editar ni Inactivar", async () => {
        renderEstudiantes(ruta);
        await screen.findByText("Identidad básica");
        expect(screen.queryByRole("button", { name: "Editar" })).toBeNull();
        expect(screen.queryByRole("button", { name: "Inactivar" })).toBeNull();
    });
});

describe("Editar estudiante", () => {
    const abrir = async () => {
        renderEstudiantes(ruta, { permissions: TODOS_LOS_PERMISOS });
        await screen.findByText("Identidad básica");
        fireEvent.click(screen.getByRole("button", { name: "Editar" }));
        return screen.getByRole("dialog", { name: "Editar estudiante" });
    };

    it("carga los datos actuales y no permite editar código, estado ni datos académicos", async () => {
        const dialog = await abrir();
        expect(within(dialog).getByLabelText("Nombres *").value).toBe("Ana Lucía");
        expect(within(dialog).getByLabelText("Apellido paterno").value).toBe("Pérez");
        expect(within(dialog).getByLabelText("Número de documento").value).toBe("12345678");
        expect(within(dialog).queryByLabelText("Código *")).toBeNull();
        expect(within(dialog).getByText(/no se modifica/)).toBeTruthy();
        for (const label of ["Estado del registro", /^Nivel/, /^Grado/, /^Sección/, /^Turno/]) {
            expect(within(dialog).queryByLabelText(label)).toBeNull();
        }
    });

    it("envía solo los cambios con expected_version y refresca la versión", async () => {
        service.updateEstudiante.mockResolvedValue({
            estudiante: { ...basico, nombres: "Ana María", version: 5 },
        });
        const dialog = await abrir();
        fireEvent.change(within(dialog).getByLabelText("Nombres *"), { target: { value: "Ana  María " } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Guardar cambios" }));

        await waitFor(() =>
            expect(service.updateEstudiante).toHaveBeenCalledWith(basico.id, { nombres: "Ana María", expected_version: 4 })
        );
        expect(await screen.findByText("Cambios guardados correctamente.")).toBeTruthy();
        expect(screen.getByText("Ana María")).toBeTruthy();
        expect(screen.getByText("5")).toBeTruthy();
    });

    it("sin cambios no envía nada", async () => {
        const dialog = await abrir();
        fireEvent.click(within(dialog).getByRole("button", { name: "Guardar cambios" }));
        expect(within(dialog).getByText("No hay cambios para guardar.")).toBeTruthy();
        expect(service.updateEstudiante).not.toHaveBeenCalled();
    });

    it("version_conflict: avisa, no reintenta y permite recargar", async () => {
        service.updateEstudiante.mockRejectedValue(httpError(409, "version_conflict: el estudiante fue modificado"));
        const dialog = await abrir();
        fireEvent.change(within(dialog).getByLabelText("Nombres *"), { target: { value: "Otra" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Guardar cambios" }));

        expect(await within(dialog).findByText(/Tus cambios NO se guardaron/)).toBeTruthy();
        expect(service.updateEstudiante).toHaveBeenCalledTimes(1);

        cargar({ nombres: "Cambio ajeno", version: 6 });
        fireEvent.click(within(dialog).getByRole("button", { name: "Recargar datos" }));
        await waitFor(() => expect(service.getEstudiante).toHaveBeenCalledTimes(2));
        expect(await screen.findByText("6")).toBeTruthy();
        expect(screen.queryByRole("dialog", { name: "Editar estudiante" })).toBeNull();
        expect(service.updateEstudiante).toHaveBeenCalledTimes(1);
    });

    it("403 al guardar", async () => {
        service.updateEstudiante.mockRejectedValue(httpError(403, "permission_denied: x"));
        const dialog = await abrir();
        fireEvent.change(within(dialog).getByLabelText("Nombres *"), { target: { value: "Otra" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Guardar cambios" }));
        expect(await within(dialog).findByText("No tienes permiso para realizar esta acción.")).toBeTruthy();
    });
});

describe("Cambiar estado desde el detalle", () => {
    const abrir = async (estado) => {
        cargar({ estado });
        renderEstudiantes(ruta, { permissions: TODOS_LOS_PERMISOS });
        await screen.findByText("Identidad básica");
        fireEvent.click(screen.getByRole("button", { name: estado === "ACTIVO" ? "Inactivar" : "Activar" }));
        return screen.getByRole("dialog", { name: "Cambiar estado" });
    };

    it("ACTIVO -> INACTIVO", async () => {
        service.changeEstadoEstudiante.mockResolvedValue({ estudiante: { ...basico, estado: "INACTIVO", version: 5 } });
        const dialog = await abrir("ACTIVO");
        fireEvent.click(within(dialog).getByRole("button", { name: "Cambiar a Inactivo" }));
        await waitFor(() =>
            expect(service.changeEstadoEstudiante).toHaveBeenCalledWith(basico.id, { estado: "INACTIVO", expectedVersion: 4 })
        );
        expect(await screen.findByText("Estado actualizado correctamente.")).toBeTruthy();
        expect(screen.getByRole("button", { name: "Activar" })).toBeTruthy();
    });

    it("INACTIVO -> ACTIVO", async () => {
        service.changeEstadoEstudiante.mockResolvedValue({ estudiante: { ...basico, estado: "ACTIVO", version: 5 } });
        const dialog = await abrir("INACTIVO");
        fireEvent.click(within(dialog).getByRole("button", { name: "Cambiar a Activo" }));
        await waitFor(() =>
            expect(service.changeEstadoEstudiante).toHaveBeenCalledWith(basico.id, { estado: "ACTIVO", expectedVersion: 4 })
        );
        expect(await screen.findByRole("button", { name: "Inactivar" })).toBeTruthy();
    });

    it.each([
        [httpError(409, "version_conflict: x"), /Tus cambios NO se guardaron/],
        [httpError(400, "invalid_state: el estudiante ya tiene ese estado"), /^Estado inválido/],
        [httpError(403, "permission_denied: x"), /No tienes permiso/],
    ])("muestra el error y no cambia el estado (%#)", async (error, mensaje) => {
        service.changeEstadoEstudiante.mockRejectedValue(error);
        const dialog = await abrir("ACTIVO");
        fireEvent.click(within(dialog).getByRole("button", { name: "Cambiar a Inactivo" }));
        expect(await within(dialog).findByText(mensaje)).toBeTruthy();
        expect(screen.getAllByText("Activo").length).toBeGreaterThan(0);
    });
});

describe("Datos personales (FASE E-08)", () => {
    const personal = {
        id: basico.id,
        version: 4,
        fecha_nacimiento: "2013-07-21",
        sexo: "F",
        pais_nacimiento: "PE",
        ubigeo_nacimiento: "080101",
        lengua_materna: "AWAJUN",
        segunda_lengua: "",
    };
    const VER = ["estudiantes.view", "estudiantes.personal.view"];
    const VER_Y_EDITAR = [...VER, "estudiantes.personal.update"];

    beforeEach(() => {
        service.getEstudiantePersonal.mockResolvedValue({ personal });
    });

    it("con estudiantes.personal.view muestra la sección desde su propio endpoint", async () => {
        renderEstudiantes(ruta, { permissions: VER });
        expect(await screen.findByText("080101")).toBeTruthy();
        expect(service.getEstudiantePersonal).toHaveBeenCalledWith(basico.id);
        expect(service.getEstudiantePersonal).toHaveBeenCalledTimes(1);
        expect(screen.getByText("Femenino")).toBeTruthy();
        expect(screen.getByText("AWAJUN")).toBeTruthy();
        expect(screen.getAllByText("No registrado").length).toBeGreaterThan(0);
        // Solo lectura: sin estudiantes.personal.update no hay edición.
        expect(screen.queryByRole("button", { name: "Editar datos personales" })).toBeNull();
    });

    it("403 al leer: mensaje de permiso, sin datos", async () => {
        service.getEstudiantePersonal.mockRejectedValue(httpError(403, "permission_denied: x"));
        renderEstudiantes(ruta, { permissions: VER });
        expect(await screen.findByText("No tienes permiso para ver los datos personales.")).toBeTruthy();
        expect(screen.queryByText("Fecha de nacimiento")).toBeNull();
    });

    const abrir = async () => {
        renderEstudiantes(ruta, { permissions: VER_Y_EDITAR });
        await screen.findByText("080101");
        fireEvent.click(screen.getByRole("button", { name: "Editar datos personales" }));
        return screen.getByRole("dialog", { name: "Editar datos personales" });
    };

    it("edita solo lo cambiado por su propio PATCH con expected_version", async () => {
        service.updateEstudiantePersonal.mockResolvedValue({ estudiante: { ...basico, version: 5 } });
        const dialog = await abrir();
        expect(within(dialog).getByLabelText("Fecha de nacimiento").value).toBe("2013-07-21");
        fireEvent.change(within(dialog).getByLabelText("Sexo"), { target: { value: "M" } });
        fireEvent.change(within(dialog).getByLabelText(/Segunda lengua/), { target: { value: "quechua" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Guardar datos personales" }));

        await waitFor(() =>
            expect(service.updateEstudiantePersonal).toHaveBeenCalledWith(basico.id, {
                sexo: "M",
                segunda_lengua: "QUECHUA",
                expected_version: 4,
            })
        );
        expect(service.updateEstudiante).not.toHaveBeenCalled();
        expect(await screen.findByText("Datos personales guardados correctamente.")).toBeTruthy();
        expect(screen.getByText("Masculino")).toBeTruthy();
        expect(screen.getByText("QUECHUA")).toBeTruthy();
        expect(screen.getByText("5")).toBeTruthy();
    });

    it("usa la versión más reciente conocida (lectura personal posterior al detalle)", async () => {
        service.getEstudiantePersonal.mockResolvedValue({ personal: { ...personal, version: 6 } });
        service.updateEstudiantePersonal.mockResolvedValue({ estudiante: { ...basico, version: 7 } });
        const dialog = await abrir();
        fireEvent.change(within(dialog).getByLabelText("Sexo"), { target: { value: "M" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Guardar datos personales" }));
        await waitFor(() =>
            expect(service.updateEstudiantePersonal).toHaveBeenCalledWith(basico.id, { sexo: "M", expected_version: 6 })
        );
    });

    it("valida antes de enviar", async () => {
        const dialog = await abrir();
        fireEvent.change(within(dialog).getByLabelText(/País de nacimiento/), { target: { value: "CL" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Guardar datos personales" }));
        expect(within(dialog).getByText("El ubigeo solo aplica si el país es PE.")).toBeTruthy();
        expect(service.updateEstudiantePersonal).not.toHaveBeenCalled();
    });

    it("version_conflict: avisa, no reintenta y recarga detalle y datos personales", async () => {
        service.updateEstudiantePersonal.mockRejectedValue(httpError(409, "version_conflict: x"));
        const dialog = await abrir();
        fireEvent.change(within(dialog).getByLabelText("Sexo"), { target: { value: "M" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Guardar datos personales" }));
        expect(await within(dialog).findByText(/Tus cambios NO se guardaron/)).toBeTruthy();
        fireEvent.click(within(dialog).getByRole("button", { name: "Recargar datos" }));
        await waitFor(() => expect(service.getEstudiante).toHaveBeenCalledTimes(2));
        await waitFor(() => expect(service.getEstudiantePersonal).toHaveBeenCalledTimes(2));
        expect(service.updateEstudiantePersonal).toHaveBeenCalledTimes(1);
    });

    it("403 al guardar", async () => {
        service.updateEstudiantePersonal.mockRejectedValue(httpError(403, "permission_denied: x"));
        const dialog = await abrir();
        fireEvent.change(within(dialog).getByLabelText("Sexo"), { target: { value: "M" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Guardar datos personales" }));
        expect(await within(dialog).findByText("No tienes permiso para realizar esta acción.")).toBeTruthy();
    });
});

describe("Modelo maestro: domicilio, contacto y edad", () => {
    const personal = {
        id: basico.id,
        version: 4,
        fecha_nacimiento: "2013-07-21",
        sexo: "F",
        pais_nacimiento: "PE",
        ubigeo_nacimiento: "",
        lengua_materna: "",
        segunda_lengua: "",
        domicilio_direccion: "Jr. Tupac Amaru 777",
        domicilio_departamento: "Cusco",
        domicilio_provincia: "Canchis",
        domicilio_distrito: "Tinta",
        domicilio_ubigeo: "",
        correo: "familia@correo.pe",
        telefono: "984777111",
        edad: 13,
    };
    const PERMISOS = ["estudiantes.view", "estudiantes.personal.view", "estudiantes.personal.update"];

    it("muestra domicilio, contacto y la edad calculada por el backend", async () => {
        service.getEstudiantePersonal.mockResolvedValue({ personal });
        renderEstudiantes(ruta, { permissions: PERMISOS });
        expect(await screen.findByText("Jr. Tupac Amaru 777")).toBeTruthy();
        expect(screen.getByText("Cusco / Canchis / Tinta")).toBeTruthy();
        expect(screen.getByText("familia@correo.pe")).toBeTruthy();
        expect(screen.getByText("13 años")).toBeTruthy();
    });

    it("edita el domicilio por el PATCH personal (solo lo cambiado) y valida el contacto", async () => {
        service.getEstudiantePersonal.mockResolvedValue({ personal });
        service.updateEstudiantePersonal.mockResolvedValue({ estudiante: { ...basico, version: 5 } });
        renderEstudiantes(ruta, { permissions: PERMISOS });
        await screen.findByText("Jr. Tupac Amaru 777");
        fireEvent.click(screen.getByRole("button", { name: "Editar datos personales" }));
        const dialog = screen.getByRole("dialog", { name: "Editar datos personales" });

        fireEvent.change(within(dialog).getByLabelText("Correo electrónico"), { target: { value: "sin-arroba" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Guardar datos personales" }));
        expect(within(dialog).getByText("El correo no tiene un formato válido.")).toBeTruthy();
        expect(service.updateEstudiantePersonal).not.toHaveBeenCalled();

        fireEvent.change(within(dialog).getByLabelText("Correo electrónico"), { target: { value: "familia@correo.pe" } });
        fireEvent.change(within(dialog).getByLabelText("Dirección"), { target: { value: "  Av.  Nueva 10 " } });
        fireEvent.change(within(dialog).getByLabelText("Teléfono"), { target: { value: "" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Guardar datos personales" }));
        await waitFor(() =>
            expect(service.updateEstudiantePersonal).toHaveBeenCalledWith(basico.id, {
                domicilio_direccion: "Av. Nueva 10",
                telefono: "",
                expected_version: 4,
            })
        );
        expect(await screen.findByText("Av. Nueva 10")).toBeTruthy();
    });
});

describe("Modelo maestro: historial de matrícula", () => {
    it("muestra una matrícula por periodo con sus movimientos", async () => {
        service.getMatriculasEstudiante.mockResolvedValue({
            matriculas: [
                {
                    id: "m2",
                    periodo: { anio: 2026, nombre: "2026", estado: "EN_CURSO" },
                    seccion: { nivel: "PRIMARIA", grado: "4", seccion: "B", turno: "MAÑANA", aula: "Aula 3" },
                    estado_matricula: "MATRICULADO",
                    movimientos: [{ tipo: "CAMBIO_SECCION", fecha: "2026-06-01" }],
                },
                { id: "m1", periodo: { anio: 2025 }, seccion: null, estado_matricula: "CONCLUIDO", movimientos: [] },
            ],
        });
        renderEstudiantes(ruta);
        expect(await screen.findByText("Historial de matrícula")).toBeTruthy();
        expect(await screen.findByText("PRIMARIA · 4 · B")).toBeTruthy();
        expect(screen.getByText("Cambio de sección (2026-06-01)")).toBeTruthy();
        expect(screen.getByText("Sin sección asignada")).toBeTruthy();
        expect(screen.getByText("Concluido")).toBeTruthy();
        expect(service.getMatriculasEstudiante).toHaveBeenCalledWith(basico.id);
    });

    it("sin matrículas registradas", async () => {
        renderEstudiantes(ruta);
        expect(await screen.findByText("Sin matrículas registradas.")).toBeTruthy();
    });
});

describe("Modelo maestro: familiares", () => {
    const familiares = [
        {
            relacion_id: "r1",
            parentesco: "PADRE",
            es_apoderado: true,
            vigente: true,
            persona: { tipo_documento: "DNI", numero_documento: "41234567", nombres: "Juan", apellido_paterno: "Pérez" },
        },
        {
            relacion_id: "r2",
            parentesco: "MADRE",
            es_apoderado: false,
            vigente: true,
            persona: { tipo_documento: "SIN_DOCUMENTO", numero_documento: "", nombres: "María" },
        },
    ];

    it("sin estudiantes.familia.view no hay sección ni consulta", async () => {
        renderEstudiantes(ruta);
        await screen.findByText("Identidad básica");
        expect(screen.queryByText("Familiares")).toBeNull();
        expect(service.getFamiliaresEstudiante).not.toHaveBeenCalled();
    });

    it("con estudiantes.familia.view lista las relaciones y permite ver el historial", async () => {
        service.getFamiliaresEstudiante.mockResolvedValue({ familiares });
        renderEstudiantes(ruta, { permissions: ["estudiantes.view", "estudiantes.familia.view"] });
        expect(await screen.findByText("Padre · Apoderado")).toBeTruthy();
        expect(screen.getByText("Juan Pérez — DNI 41234567")).toBeTruthy();
        expect(screen.getByText("María — Sin documento")).toBeTruthy();
        expect(service.getFamiliaresEstudiante).toHaveBeenCalledWith(basico.id, { historial: false });

        fireEvent.click(screen.getByLabelText(/Incluir/));
        await waitFor(() => expect(service.getFamiliaresEstudiante).toHaveBeenCalledWith(basico.id, { historial: true }));
    });

    it("403 al leer familiares", async () => {
        service.getFamiliaresEstudiante.mockRejectedValue(httpError(403, "permission_denied: x"));
        renderEstudiantes(ruta, { permissions: ["estudiantes.view", "estudiantes.familia.view"] });
        expect(await screen.findByText("No tienes permiso para ver los familiares.")).toBeTruthy();
    });
});
