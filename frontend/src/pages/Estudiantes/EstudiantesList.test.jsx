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

const ana = estudianteBasico();
const luis = estudianteBasico({
    id: "22222222-2222-4222-8222-222222222222",
    codigo: "EST-002",
    nombres: "Luis",
    apellidos: "Quispe",
    apellido_paterno: "",
    apellido_materno: "",
    tipo_documento: "SIN_DOCUMENTO",
    numero_documento: "",
    estado: "INACTIVO",
    version: 2,
});

const pagina = (estudiantes, total = estudiantes.length) => ({ estudiantes, total });
const ultimaBusqueda = () => service.searchEstudiantes.mock.calls.at(-1)[0];

beforeEach(() => {
    vi.clearAllMocks();
    service.searchEstudiantes.mockResolvedValue(pagina([ana, luis]));
    // El detalle (al que navega el alta) carga también el historial de matrícula.
    service.getMatriculasEstudiante.mockResolvedValue({ matriculas: [] });
});
afterEach(cleanup);

describe("Directorio de estudiantes — listado", () => {
    it("carga el listado general una sola vez, con orden del backend", async () => {
        renderEstudiantes("/estudiantes");
        expect(await screen.findByText("EST-001")).toBeTruthy();
        expect(screen.getByText("EST-002")).toBeTruthy();
        expect(screen.getByText("DNI 12345678")).toBeTruthy();
        expect(screen.getByText("Sin documento")).toBeTruthy();
        expect(ultimaBusqueda()).toMatchObject({ q: "", page: 1, pageSize: 20, estado: "", nivel: "" });
        // El debounce inicial no dispara una segunda consulta idéntica.
        await new Promise((resolve) => setTimeout(resolve, 450));
        expect(service.searchEstudiantes).toHaveBeenCalledTimes(1);
    });

    it("busca con q en el backend y no consulta con 1 carácter", async () => {
        renderEstudiantes("/estudiantes");
        await screen.findByText("EST-001");
        const input = screen.getByLabelText("Buscar estudiantes");

        fireEvent.change(input, { target: { value: "p" } });
        expect(await screen.findByText(/al menos 2 caracteres/)).toBeTruthy();
        expect(service.searchEstudiantes).toHaveBeenCalledTimes(1);
        expect(screen.queryByText("EST-001")).toBeNull();

        service.searchEstudiantes.mockResolvedValue(pagina([ana]));
        fireEvent.change(input, { target: { value: "pérez ana" } });
        await waitFor(() => expect(ultimaBusqueda()).toMatchObject({ q: "pérez ana", page: 1 }));
        expect(await screen.findByText("EST-001")).toBeTruthy();
    });

    it("filtra por estado, nivel, grado, sección y turno (valores exactos, sin inventar catálogos)", async () => {
        renderEstudiantes("/estudiantes");
        await screen.findByText("EST-001");
        const estado = screen.getByLabelText("Filtrar por estado");
        expect([...estado.options].map((o) => o.value)).toEqual(["", "ACTIVO", "INACTIVO"]);

        fireEvent.change(estado, { target: { value: "INACTIVO" } });
        fireEvent.change(screen.getByLabelText("Filtrar por nivel"), { target: { value: "PRIMARIA" } });
        fireEvent.change(screen.getByLabelText("Filtrar por grado"), { target: { value: "3" } });
        fireEvent.change(screen.getByLabelText("Filtrar por sección"), { target: { value: "A" } });
        fireEvent.change(screen.getByLabelText("Filtrar por turno"), { target: { value: " MAÑANA " } });
        await waitFor(() =>
            expect(ultimaBusqueda()).toMatchObject({
                estado: "INACTIVO",
                nivel: "PRIMARIA",
                grado: "3",
                seccion: "A",
                turno: "MAÑANA",
                page: 1,
            })
        );
    });

    it("pagina con Anterior/Siguiente y vuelve a la página 1 al cambiar un filtro", async () => {
        service.searchEstudiantes.mockResolvedValue(pagina([ana, luis], 45));
        renderEstudiantes("/estudiantes");
        expect(await screen.findByText(/Página 1 de 3/)).toBeTruthy();
        expect(screen.getByRole("button", { name: "Anterior" }).disabled).toBe(true);

        fireEvent.click(screen.getByRole("button", { name: "Siguiente" }));
        await waitFor(() => expect(ultimaBusqueda()).toMatchObject({ page: 2 }));
        expect(await screen.findByText(/Página 2 de 3/)).toBeTruthy();

        fireEvent.change(screen.getByLabelText("Filtrar por estado"), { target: { value: "ACTIVO" } });
        await waitFor(() => expect(ultimaBusqueda()).toMatchObject({ page: 1, estado: "ACTIVO" }));
    });

    it("distingue directorio vacío de búsqueda sin resultados", async () => {
        service.searchEstudiantes.mockResolvedValue(pagina([]));
        renderEstudiantes("/estudiantes");
        expect(await screen.findByText("Todavía no hay estudiantes registrados.")).toBeTruthy();

        fireEvent.change(screen.getByLabelText("Buscar estudiantes"), { target: { value: "zzqq" } });
        expect(await screen.findByText("No se encontraron estudiantes con esos criterios.")).toBeTruthy();
    });

    it("muestra un error genérico sin detalles internos", async () => {
        service.searchEstudiantes.mockRejectedValue(httpError(500, "ERROR: algo (SQLSTATE 42P01)"));
        renderEstudiantes("/estudiantes");
        expect(await screen.findByText(/Ocurrió un error en el servidor/)).toBeTruthy();
        expect(screen.queryByText(/SQLSTATE/)).toBeNull();
    });

    it("403 del backend muestra acceso denegado", async () => {
        service.searchEstudiantes.mockRejectedValue(httpError(403, "permission_denied: x"));
        renderEstudiantes("/estudiantes");
        expect(await screen.findByText("Acceso denegado")).toBeTruthy();
    });
});

describe("Directorio de estudiantes — permisos", () => {
    it("sin estudiantes.view no monta la página ni consulta", () => {
        renderEstudiantes("/estudiantes", { permissions: ["dashboard.view"] });
        expect(screen.getByRole("heading", { level: 2, name: "Acceso denegado" })).toBeTruthy();
        expect(service.searchEstudiantes).not.toHaveBeenCalled();
    });

    it("solo con estudiantes.view: ver sí; crear, editar y cambiar estado no", async () => {
        renderEstudiantes("/estudiantes");
        await screen.findByText("EST-001");
        expect(screen.getAllByText("Ver").length).toBe(2);
        expect(screen.queryByRole("button", { name: "Nuevo estudiante" })).toBeNull();
        expect(screen.queryByRole("button", { name: /Editar/ })).toBeNull();
        expect(screen.queryByRole("button", { name: /Cambiar estado/ })).toBeNull();
    });

    it("cada acción aparece solo con su permiso", async () => {
        renderEstudiantes("/estudiantes", { permissions: ["estudiantes.view", "estudiantes.update"] });
        await screen.findByText("EST-001");
        expect(screen.getAllByRole("button", { name: /Editar/ }).length).toBe(2);
        expect(screen.queryByRole("button", { name: "Nuevo estudiante" })).toBeNull();
        expect(screen.queryByRole("button", { name: /Cambiar estado/ })).toBeNull();
        cleanup();

        renderEstudiantes("/estudiantes", { permissions: ["estudiantes.view", "estudiantes.change_estado"] });
        await screen.findByText("EST-001");
        expect(screen.getAllByRole("button", { name: /Cambiar estado/ }).length).toBe(2);
        expect(screen.queryByRole("button", { name: /Editar/ })).toBeNull();
        cleanup();

        renderEstudiantes("/estudiantes", { permissions: ["estudiantes.view", "estudiantes.create"] });
        await screen.findByText("EST-001");
        expect(screen.getByRole("button", { name: "Nuevo estudiante" })).toBeTruthy();
    });
});

describe("Crear estudiante", () => {
    const abrir = async () => {
        renderEstudiantes("/estudiantes", { permissions: TODOS_LOS_PERMISOS });
        await screen.findByText("EST-001");
        fireEvent.click(screen.getByRole("button", { name: "Nuevo estudiante" }));
        return screen.getByRole("dialog", { name: "Nuevo estudiante" });
    };

    it("muestra el formulario solo con campos del maestro (sin datos académicos ni personales sin permiso)", async () => {
        const dialog = await abrir();
        for (const label of ["Código *", "Código SIAGIE", "Tipo de documento", "Nombres *", "Apellido paterno", "Referencia de foto", "Estado del registro"]) {
            expect(within(dialog).getByLabelText(label)).toBeTruthy();
        }
        // FASE E-08: sin estudiantes.personal.update no hay datos personales.
        for (const label of ["Fecha de nacimiento", "Sexo", /País de nacimiento/, /Ubigeo/, /Lengua materna/, /Segunda lengua/]) {
            expect(within(dialog).queryByLabelText(label)).toBeNull();
        }
        for (const label of [/^Nivel/, /^Grado/, /^Sección/, /^Turno/, /matrícula/i]) {
            expect(within(dialog).queryByLabelText(label)).toBeNull();
        }
        const estado = within(dialog).getByLabelText("Estado del registro");
        expect([...estado.options].map((o) => o.value)).toEqual(["ACTIVO", "INACTIVO"]);
    });

    it("valida antes de enviar", async () => {
        const dialog = await abrir();
        fireEvent.click(within(dialog).getByRole("button", { name: "Registrar estudiante" }));
        expect(within(dialog).getByText("El código es obligatorio.")).toBeTruthy();
        expect(within(dialog).getByText("Nombres es obligatorio.")).toBeTruthy();
        expect(service.createEstudiante).not.toHaveBeenCalled();

        fireEvent.change(within(dialog).getByLabelText("Tipo de documento"), { target: { value: "DNI" } });
        fireEvent.change(within(dialog).getByLabelText("Número de documento"), { target: { value: "123" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Registrar estudiante" }));
        expect(within(dialog).getByText("El DNI debe tener 8 dígitos.")).toBeTruthy();
        expect(service.createEstudiante).not.toHaveBeenCalled();
    });

    it("registra y abre el detalle del estudiante creado", async () => {
        const creado = estudianteBasico({ id: "33333333-3333-4333-8333-333333333333", codigo: "NEW-1", version: 1 });
        service.createEstudiante.mockResolvedValue({ estudiante: creado });
        service.getEstudiante.mockResolvedValue({ estudiante: creado });
        service.getEstudiantesBasic.mockResolvedValue({ estudiantes: [creado] });
        const dialog = await abrir();
        fireEvent.change(within(dialog).getByLabelText("Código *"), { target: { value: " NEW-1 " } });
        fireEvent.change(within(dialog).getByLabelText("Nombres *"), { target: { value: "Ana  Lucía" } });
        fireEvent.change(within(dialog).getByLabelText("Apellido paterno"), { target: { value: "Pérez" } });
        fireEvent.change(within(dialog).getByLabelText("Apellido materno"), { target: { value: "Soto" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Registrar estudiante" }));

        await waitFor(() =>
            expect(service.createEstudiante).toHaveBeenCalledWith({
                codigo: "NEW-1",
                nombres: "Ana Lucía",
                apellido_paterno: "Pérez",
                apellido_materno: "Soto",
                tipo_documento: "SIN_DOCUMENTO",
                estado: "ACTIVO",
            })
        );
        expect(await screen.findByText("Estudiante registrado correctamente.")).toBeTruthy();
        expect(service.getEstudiante).toHaveBeenCalledWith(creado.id);
    });

    it.each([
        [httpError(409, "duplicate_codigo: ya existe"), "Ya existe un estudiante con ese código."],
        [httpError(409, "duplicate_document: ya existe"), "Ya existe un estudiante con ese tipo y número de documento."],
        [httpError(403, "permission_denied: x"), "No tienes permiso para realizar esta acción."],
        [httpError(400, "invalid_document: el DNI debe tener 8 dígitos"), "Documento inválido: el DNI debe tener 8 dígitos."],
    ])("muestra el error del backend sin cerrar el formulario (%#)", async (error, mensaje) => {
        service.createEstudiante.mockRejectedValue(error);
        const dialog = await abrir();
        fireEvent.change(within(dialog).getByLabelText("Código *"), { target: { value: "EST-001" } });
        fireEvent.change(within(dialog).getByLabelText("Nombres *"), { target: { value: "Ana" } });
        fireEvent.change(within(dialog).getByLabelText(/^Apellidos \*/), { target: { value: "Soto" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Registrar estudiante" }));
        expect(await within(dialog).findByText(mensaje)).toBeTruthy();
        expect(screen.getByRole("dialog", { name: "Nuevo estudiante" })).toBeTruthy();
    });
});

describe("Crear estudiante con datos personales (FASE E-08)", () => {
    const abrirCon = async (permissions) => {
        renderEstudiantes("/estudiantes", { permissions });
        await screen.findByText("EST-001");
        fireEvent.click(screen.getByRole("button", { name: "Nuevo estudiante" }));
        const dialog = screen.getByRole("dialog", { name: "Nuevo estudiante" });
        fireEvent.change(within(dialog).getByLabelText("Código *"), { target: { value: "NEW-P" } });
        fireEvent.change(within(dialog).getByLabelText("Nombres *"), { target: { value: "Ana" } });
        fireEvent.change(within(dialog).getByLabelText(/^Apellidos \*/), { target: { value: "Soto" } });
        return dialog;
    };

    beforeEach(() => {
        const creado = estudianteBasico({ id: "44444444-4444-4444-8444-444444444444", codigo: "NEW-P", version: 1 });
        service.createEstudiante.mockResolvedValue({ estudiante: creado });
        service.getEstudiante.mockResolvedValue({ estudiante: creado });
        service.getEstudiantesBasic.mockResolvedValue({ estudiantes: [creado] });
    });

    it("con estudiantes.personal.update muestra y envía los datos personales", async () => {
        const dialog = await abrirCon([...TODOS_LOS_PERMISOS, "estudiantes.personal.update"]);
        fireEvent.change(within(dialog).getByLabelText("Fecha de nacimiento"), { target: { value: "2014-05-06" } });
        fireEvent.change(within(dialog).getByLabelText("Sexo"), { target: { value: "F" } });
        fireEvent.change(within(dialog).getByLabelText(/Lengua materna/), { target: { value: "quechua" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Registrar estudiante" }));
        await waitFor(() =>
            expect(service.createEstudiante).toHaveBeenCalledWith({
                codigo: "NEW-P",
                nombres: "Ana",
                apellidos: "Soto",
                tipo_documento: "SIN_DOCUMENTO",
                fecha_nacimiento: "2014-05-06",
                sexo: "F",
                lengua_materna: "QUECHUA",
                estado: "ACTIVO",
            })
        );
    });

    it("sin el permiso personal el alta no muestra ni envía datos personales", async () => {
        const dialog = await abrirCon(TODOS_LOS_PERMISOS);
        expect(within(dialog).queryByLabelText("Fecha de nacimiento")).toBeNull();
        fireEvent.click(within(dialog).getByRole("button", { name: "Registrar estudiante" }));
        await waitFor(() => expect(service.createEstudiante).toHaveBeenCalled());
        const payload = service.createEstudiante.mock.calls[0][0];
        for (const campo of ["fecha_nacimiento", "sexo", "pais_nacimiento", "ubigeo_nacimiento", "lengua_materna", "segunda_lengua"]) {
            expect(payload).not.toHaveProperty(campo);
        }
    });

    it("403 del backend si igualmente falta el permiso personal", async () => {
        service.createEstudiante.mockRejectedValue(httpError(403, "permission_denied: los datos personales requieren el permiso"));
        const dialog = await abrirCon([...TODOS_LOS_PERMISOS, "estudiantes.personal.update"]);
        fireEvent.change(within(dialog).getByLabelText("Sexo"), { target: { value: "F" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Registrar estudiante" }));
        expect(await within(dialog).findByText("No tienes permiso para realizar esta acción.")).toBeTruthy();
    });
});

describe("Cambiar estado desde el listado", () => {
    it("pide confirmación, envía expected_version y actualiza la fila", async () => {
        service.changeEstadoEstudiante.mockResolvedValue({ estudiante: { ...ana, estado: "INACTIVO", version: 5 } });
        renderEstudiantes("/estudiantes", { permissions: TODOS_LOS_PERMISOS });
        await screen.findByText("EST-001");
        fireEvent.click(screen.getByRole("button", { name: "Cambiar estado EST-001" }));
        const dialog = screen.getByRole("dialog", { name: "Cambiar estado" });
        expect(within(dialog).getByText(/El registro no se borra/)).toBeTruthy();
        fireEvent.click(within(dialog).getByRole("button", { name: "Cambiar a Inactivo" }));

        await waitFor(() =>
            expect(service.changeEstadoEstudiante).toHaveBeenCalledWith(ana.id, { estado: "INACTIVO", expectedVersion: 4 })
        );
        expect(await screen.findByText("Estado actualizado correctamente.")).toBeTruthy();
        const fila = screen.getByText("EST-001").closest("tr");
        expect(within(fila).getByText("Inactivo")).toBeTruthy();
        expect(within(fila).getByRole("button", { name: "Cambiar estado EST-001" }).textContent).toBe("Activar");
    });
});
