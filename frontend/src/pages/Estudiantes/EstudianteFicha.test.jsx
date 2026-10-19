import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import { estudianteBasico, httpError, renderEstudiantes } from "../../test/estudiantesTestUtils";
import * as service from "../../services/estudiantesService";
import * as psicologia from "../../services/psicologiaService";
import * as usuarios from "../../services/usuariosService";

// Ficha integral (etapa 6): pestañas con su estado real e integración con
// los endpoints existentes. Datos ficticios; los servicios HTTP se simulan.
vi.mock("../../services/estudiantesService", () => ({
    getEstudiante: vi.fn(),
    getEstudiantesBasic: vi.fn(),
    getEstudiantePersonal: vi.fn(),
    getMatriculasEstudiante: vi.fn(),
    getFamiliaresEstudiante: vi.fn(),
    getHistorialEstudiante: vi.fn(),
    vincularFamiliar: vi.fn(),
    desvincularFamiliar: vi.fn(),
    actualizarFamiliar: vi.fn(),
    cambiarParentescoFamiliar: vi.fn(),
    searchPersonas: vi.fn(),
    updatePersona: vi.fn(),
    listPeriodos: vi.fn(),
    listSecciones: vi.fn(),
    matricularEstudiante: vi.fn(),
    cambiarSeccionMatricula: vi.fn(),
    retirarMatricula: vi.fn(),
    reingresarMatricula: vi.fn(),
    concluirMatricula: vi.fn(),
}));
vi.mock("../../services/psicologiaService", () => ({ getExpedientePsicologicoByEstudiante: vi.fn() }));
vi.mock("../../services/usuariosService", () => ({ getUsuariosBasic: vi.fn() }));
vi.mock("../../components/portal/RoleLayout", () => ({
    default: ({ title, children }) => (
        <div>
            <h1>{title}</h1>
            {children}
        </div>
    ),
}));

const basico = estudianteBasico();
const ruta = `/estudiantes/${basico.id}`;
const P2026 = { id: "p-2026", anio: 2026, nombre: "Año escolar 2026", estado: "EN_CURSO" };
const P2025 = { id: "p-2025", anio: 2025, nombre: "Año escolar 2025", estado: "CERRADO" };
const SEC_A = { id: "s-a", periodo_id: "p-2026", nivel: "PRIMARIA", grado: "3", seccion: "A", turno: "MAÑANA", matriculados: 10, vacantes: 30 };
const SEC_B = { id: "s-b", periodo_id: "p-2026", nivel: "PRIMARIA", grado: "3", seccion: "B", turno: "MAÑANA", matriculados: 5 };

const matricula = (overrides = {}) => ({
    id: "m-2026",
    estudiante_id: basico.id,
    periodo: P2026,
    seccion: SEC_A,
    estado_matricula: "MATRICULADO",
    fecha_matricula: "2026-03-01",
    fecha_retiro: "",
    origen: "REGISTRO",
    version: 3,
    movimientos: [],
    ...overrides,
});

const pestana = async (nombre) => fireEvent.click(await screen.findByRole("tab", { name: new RegExp(nombre) }));

beforeEach(() => {
    vi.clearAllMocks();
    service.getEstudiante.mockResolvedValue({ estudiante: basico });
    service.getEstudiantesBasic.mockResolvedValue({ estudiantes: [basico] });
    service.getMatriculasEstudiante.mockResolvedValue({ matriculas: [] });
    service.getFamiliaresEstudiante.mockResolvedValue({ familiares: [] });
    service.getHistorialEstudiante.mockResolvedValue({ eventos: [], total: 0 });
    service.listPeriodos.mockResolvedValue({ periodos: [P2026, P2025] });
    service.listSecciones.mockResolvedValue({ secciones: [SEC_A, SEC_B] });
    usuarios.getUsuariosBasic.mockResolvedValue({ usuarios: [{ id: "u-1", nombre_completo: "Secretaría Uno" }] });
});
afterEach(cleanup);

describe("Ficha integral — pestañas", () => {
    it("muestra las 9 pestañas con su estado real", async () => {
        renderEstudiantes(ruta);
        const tabs = await screen.findAllByRole("tab");
        expect(tabs.map((t) => t.textContent)).toEqual([
            "Información general",
            "Padres y apoderadosParcial",
            "Historial académico",
            "NotasPróximamente",
            "Atenciones psicológicasParcial",
            "Permisos y justificacionesPróximamente",
            "Documentos adjuntosPróximamente",
            "AsistenciaPróximamente",
            "Historial del estudiante",
        ]);
        expect(screen.getByRole("tab", { name: "Información general" }).getAttribute("aria-selected")).toBe("true");
    });

    it.each([
        ["Notas", /servicio de calificaciones/],
        ["Permisos y justificaciones", /módulo de permisos y justificaciones/],
        ["Documentos adjuntos", /no los asocia a un estudiante/],
        ["Asistencia", /servicio de asistencia/],
    ])("«%s» explica que no está disponible, sin datos ni botones", async (nombre, motivo) => {
        renderEstudiantes(ruta);
        await pestana(nombre);
        const panel = screen.getByRole("tabpanel");
        expect(within(panel).getByText(motivo)).toBeTruthy();
        expect(within(panel).getByText("Próximamente")).toBeTruthy();
        expect(within(panel).queryAllByRole("button")).toHaveLength(0);
        expect(within(panel).queryByRole("table")).toBeNull();
    });

    it("abre la pestaña indicada en la URL (?tab=)", async () => {
        renderEstudiantes(`${ruta}?tab=historial`);
        expect(await screen.findByText("Cambios en el registro")).toBeTruthy();
        expect(service.getHistorialEstudiante).toHaveBeenCalledWith(basico.id, { page: 1, pageSize: 20 });
    });

    it("«Ver historial» de la cabecera abre la pestaña de historial", async () => {
        renderEstudiantes(ruta);
        fireEvent.click(await screen.findByRole("button", { name: "Ver historial" }));
        expect(await screen.findByText("Cambios en el registro")).toBeTruthy();
    });
});

describe("Ficha integral — historial del estudiante", () => {
    it("muestra trayectoria académica y cambios del registro, solo con nombres de campos", async () => {
        service.getMatriculasEstudiante.mockResolvedValue({
            matriculas: [
                matricula({
                    movimientos: [
                        { tipo: "CAMBIO_SECCION", fecha: "2026-05-02", seccion_anterior_id: "s-b", seccion_nueva_id: "s-a" },
                        { tipo: "RETIRO", fecha: "2026-06-10", seccion_anterior_id: "s-a" },
                    ],
                }),
            ],
        });
        service.getHistorialEstudiante.mockResolvedValue({
            total: 2,
            eventos: [
                { id: "e2", accion: "CHANGE_ESTADO", fecha_hora: "2026-06-01T15:00:00Z", usuario_id: "u-1", campos: ["estado"], estado_anterior: "ACTIVO", estado_nuevo: "INACTIVO" },
                { id: "e1", accion: "UPDATE", fecha_hora: "2026-05-01T15:00:00Z", usuario_id: "u-1", campos: ["nombres", "numero_documento"] },
            ],
        });
        renderEstudiantes(`${ruta}?tab=historial`);

        const trayectoria = await screen.findByRole("list", { name: "Trayectoria académica" });
        expect(within(trayectoria).getByText("Retiro")).toBeTruthy();
        expect(within(trayectoria).getByText("Matrícula en Año escolar 2026")).toBeTruthy();
        await waitFor(() => expect(within(trayectoria).getByText(/de PRIMARIA · 3° · «B» · MAÑANA a PRIMARIA · 3° · «A» · MAÑANA/)).toBeTruthy());

        const cambios = await screen.findByRole("list", { name: "Cambios en el registro" });
        expect(within(cambios).getByText("Cambio de estado del registro")).toBeTruthy();
        expect(within(cambios).getByText("De activo a inactivo")).toBeTruthy();
        expect(within(cambios).getByText("Campos: nombres, número de documento")).toBeTruthy();
        expect(await within(cambios).findAllByText("Por Secretaría Uno")).toHaveLength(2);
    });

    it("pagina los cambios anteriores", async () => {
        const evento = (i) => ({ id: `e${i}`, accion: "UPDATE", fecha_hora: "2026-05-01T15:00:00Z", usuario_id: "u-1", campos: ["nombres"] });
        service.getHistorialEstudiante
            .mockResolvedValueOnce({ total: 21, eventos: Array.from({ length: 20 }, (_, i) => evento(i)) })
            .mockResolvedValueOnce({ total: 21, eventos: [evento(20)] });
        renderEstudiantes(`${ruta}?tab=historial`);
        fireEvent.click(await screen.findByRole("button", { name: "Ver cambios anteriores" }));
        await waitFor(() => expect(service.getHistorialEstudiante).toHaveBeenLastCalledWith(basico.id, { page: 2, pageSize: 20 }));
        await waitFor(() => expect(screen.queryByRole("button", { name: "Ver cambios anteriores" })).toBeNull());
        expect(screen.getAllByText("Campos: nombres")).toHaveLength(21);
    });
});

describe("Ficha integral — padres y apoderados", () => {
    const FAMILIA = ["estudiantes.view", "estudiantes.familia.view", "estudiantes.familia.update"];
    const padre = {
        relacion_id: "r-1",
        parentesco: "TIO_A",
        es_apoderado: false,
        vigente: true,
        desde: "2024-01-01",
        persona: { id: "per-1", version: 2, tipo_documento: "DNI", numero_documento: "40000001", nombres: "Pedro", apellido_paterno: "Ríos", apellido_materno: "" },
    };

    it("vincula una persona nueva con parentesco y rol de apoderado", async () => {
        service.vincularFamiliar.mockResolvedValue({ familiar: { ...padre, parentesco: "MADRE", persona: { ...padre.persona, nombres: "Rosa" } } });
        renderEstudiantes(ruta, { permissions: FAMILIA });
        await pestana("Padres y apoderados");
        fireEvent.click(await screen.findByRole("button", { name: "Vincular responsable" }));
        const dialog = screen.getByRole("dialog", { name: "Vincular responsable" });
        fireEvent.click(within(dialog).getByRole("radio", { name: "Registrar nueva persona" }));

        fireEvent.click(within(dialog).getByRole("button", { name: "Vincular" }));
        expect(within(dialog).getByText("Elige el parentesco.")).toBeTruthy();
        fireEvent.change(within(dialog).getByLabelText("Parentesco *"), { target: { value: "MADRE" } });
        fireEvent.change(within(dialog).getByLabelText("Número de documento"), { target: { value: "123" } });
        fireEvent.change(within(dialog).getByLabelText("Nombres *"), { target: { value: " Rosa " } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Vincular" }));
        expect(within(dialog).getByText("El DNI debe tener 8 dígitos.")).toBeTruthy();
        expect(service.vincularFamiliar).not.toHaveBeenCalled();

        fireEvent.change(within(dialog).getByLabelText("Número de documento"), { target: { value: "40000002" } });
        fireEvent.click(within(dialog).getByLabelText(/Es el apoderado/));
        fireEvent.click(within(dialog).getByRole("button", { name: "Vincular" }));
        await waitFor(() =>
            expect(service.vincularFamiliar).toHaveBeenCalledWith(basico.id, {
                personaId: undefined,
                persona: { tipo_documento: "DNI", numero_documento: "40000002", nombres: "Rosa", apellido_paterno: "", apellido_materno: "" },
                parentesco: "MADRE",
                esApoderado: true,
                desde: "",
            })
        );
        expect(await screen.findByText(/quedó vinculado\(a\) como madre/)).toBeTruthy();
        expect(service.getFamiliaresEstudiante).toHaveBeenCalledTimes(2);
    });

    it("busca y vincula una persona ya registrada (documento enmascarado)", async () => {
        service.searchPersonas.mockResolvedValue({ personas: [padre.persona], total: 1 });
        service.vincularFamiliar.mockResolvedValue({ familiar: padre });
        renderEstudiantes(ruta, { permissions: FAMILIA });
        await pestana("Padres y apoderados");
        fireEvent.click(await screen.findByRole("button", { name: "Vincular responsable" }));
        const dialog = screen.getByRole("dialog", { name: "Vincular responsable" });
        fireEvent.change(within(dialog).getByLabelText(/Buscar persona/), { target: { value: "ríos" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Buscar" }));
        expect(await within(dialog).findByText("Pedro Ríos")).toBeTruthy();
        expect(within(dialog).getByText(/••••0001/)).toBeTruthy();
        expect(service.searchPersonas).toHaveBeenCalledWith({ q: "ríos", pageSize: 10 });
        fireEvent.click(within(dialog).getByRole("radio", { name: /Pedro Ríos/ }));
        fireEvent.change(within(dialog).getByLabelText("Parentesco *"), { target: { value: "TIO_A" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Vincular" }));
        await waitFor(() => expect(service.vincularFamiliar).toHaveBeenCalledWith(basico.id, expect.objectContaining({ personaId: "per-1", persona: undefined })));
    });

    it("cambia el parentesco con el endpoint que conserva el historial", async () => {
        service.getFamiliaresEstudiante.mockResolvedValue({ familiares: [padre] });
        service.cambiarParentescoFamiliar.mockResolvedValue({ anterior: { ...padre, vigente: false }, familiar: { ...padre, relacion_id: "r-2", parentesco: "ABUELO_A" } });
        renderEstudiantes(ruta, { permissions: FAMILIA });
        await pestana("Padres y apoderados");
        fireEvent.click(await screen.findByRole("button", { name: "Cambiar parentesco" }));
        const dialog = screen.getByRole("dialog", { name: "Cambiar parentesco" });
        expect(within(dialog).queryByRole("option", { name: "Tío(a)" })).toBeNull();
        fireEvent.change(within(dialog).getByLabelText("Nuevo parentesco *"), { target: { value: "ABUELO_A" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Cambiar parentesco" }));
        await waitFor(() => expect(service.cambiarParentescoFamiliar).toHaveBeenCalledWith(basico.id, "r-1", { parentesco: "ABUELO_A", desde: "" }));
        expect(service.desvincularFamiliar).not.toHaveBeenCalled();
        expect(service.vincularFamiliar).not.toHaveBeenCalled();
        expect(await screen.findByText(/La relación anterior quedó en el historial/)).toBeTruthy();
    });

    it("designar apoderado: muestra la regla del backend si ya hay otro", async () => {
        service.getFamiliaresEstudiante.mockResolvedValue({ familiares: [padre] });
        service.actualizarFamiliar.mockRejectedValue(httpError(409, "duplicate_apoderado: ya tiene"));
        renderEstudiantes(ruta, { permissions: FAMILIA });
        await pestana("Padres y apoderados");
        fireEvent.click(await screen.findByRole("button", { name: "Designar apoderado" }));
        const dialog = screen.getByRole("dialog", { name: "Designar apoderado" });
        fireEvent.click(within(dialog).getByRole("button", { name: "Confirmar" }));
        expect(await within(dialog).findByText(/ya tiene un apoderado vigente/)).toBeTruthy();
        expect(service.actualizarFamiliar).toHaveBeenCalledWith(basico.id, "r-1", { esApoderado: true });
    });

    it("corrige los datos de la persona enviando solo lo cambiado con su versión", async () => {
        service.getFamiliaresEstudiante.mockResolvedValue({ familiares: [padre] });
        service.updatePersona.mockResolvedValue({ persona: { ...padre.persona, apellido_materno: "Soto", version: 3 } });
        renderEstudiantes(ruta, { permissions: FAMILIA });
        await pestana("Padres y apoderados");
        fireEvent.click(await screen.findByRole("button", { name: "Corregir datos" }));
        const dialog = screen.getByRole("dialog", { name: "Corregir datos de la persona" });
        fireEvent.change(within(dialog).getByLabelText("Apellido materno"), { target: { value: "Soto" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Guardar cambios" }));
        await waitFor(() => expect(service.updatePersona).toHaveBeenCalledWith("per-1", { apellido_materno: "Soto", expected_version: 2 }));
    });

    it("desvincula sin borrar (cierra la relación)", async () => {
        service.getFamiliaresEstudiante.mockResolvedValue({ familiares: [padre] });
        service.desvincularFamiliar.mockResolvedValue({ familiar: { ...padre, vigente: false, hasta: "2026-10-19" } });
        renderEstudiantes(ruta, { permissions: FAMILIA });
        await pestana("Padres y apoderados");
        fireEvent.click(await screen.findByRole("button", { name: "Desvincular" }));
        const dialog = screen.getByRole("dialog", { name: "Desvincular responsable" });
        expect(within(dialog).getByText(/No se borra/)).toBeTruthy();
        fireEvent.click(within(dialog).getByRole("button", { name: "Desvincular" }));
        await waitFor(() => expect(service.desvincularFamiliar).toHaveBeenCalledWith(basico.id, "r-1", { hasta: "" }));
    });
});

describe("Ficha integral — matrícula", () => {
    const MATRICULA = ["estudiantes.view", "estudiantes.matricula.update"];

    it("matricula en un periodo abierto en el que no está (secciones con ocupación)", async () => {
        service.getMatriculasEstudiante.mockResolvedValue({ matriculas: [matricula({ id: "m-25", periodo: P2025, estado_matricula: "CONCLUIDO" })] });
        const P2027 = { id: "p-2027", anio: 2027, nombre: "Año escolar 2027", estado: "PLANIFICADO" };
        service.listPeriodos.mockResolvedValue({ periodos: [P2027, P2026, P2025] });
        service.matricularEstudiante.mockResolvedValue({ matricula: matricula() });
        renderEstudiantes(ruta, { permissions: MATRICULA });
        await pestana("Historial académico");
        fireEvent.click(await screen.findByRole("button", { name: "Matricular" }));
        const dialog = screen.getByRole("dialog", { name: "Matricular en un periodo" });
        const periodo = within(dialog).getByLabelText("Periodo académico *");
        await waitFor(() => expect(periodo.options.length).toBe(3));
        // Sin el periodo cerrado.
        expect([...periodo.options].map((o) => o.value)).toEqual(["", "p-2027", "p-2026"]);
        fireEvent.change(periodo, { target: { value: "p-2026" } });
        const seccion = await within(dialog).findByLabelText("Sección");
        await waitFor(() => expect(seccion.options.length).toBe(3));
        expect(within(dialog).getByText(/10\/30 vacantes ocupadas/)).toBeTruthy();
        fireEvent.change(seccion, { target: { value: "s-a" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Matricular" }));
        await waitFor(() =>
            expect(service.matricularEstudiante).toHaveBeenCalledWith(basico.id, { periodoId: "p-2026", seccionId: "s-a", fechaMatricula: "" })
        );
        expect(await screen.findByText("Matrícula registrada en Año escolar 2026.")).toBeTruthy();
        expect(service.getMatriculasEstudiante).toHaveBeenCalledTimes(2);
    });

    it("retiro exige motivo y muestra el error de regla del backend", async () => {
        service.getMatriculasEstudiante.mockResolvedValue({ matriculas: [matricula()] });
        service.retirarMatricula.mockRejectedValue(httpError(400, "periodo_cerrado: cerrado"));
        renderEstudiantes(ruta, { permissions: MATRICULA });
        await pestana("Historial académico");
        fireEvent.click(await screen.findByRole("button", { name: "Retiro o traslado" }));
        const dialog = screen.getByRole("dialog", { name: "Registrar retiro o traslado" });
        fireEvent.click(within(dialog).getByRole("button", { name: "Registrar" }));
        expect(within(dialog).getByText("Indica el motivo.")).toBeTruthy();
        fireEvent.click(within(dialog).getByRole("radio", { name: "Traslado a otra institución" }));
        fireEvent.change(within(dialog).getByLabelText("Motivo *"), { target: { value: " Cambio de domicilio " } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Registrar" }));
        await waitFor(() =>
            expect(service.retirarMatricula).toHaveBeenCalledWith("m-2026", { expectedVersion: 3, fecha: "", tipo: "TRASLADO", motivo: "Cambio de domicilio" })
        );
        expect(await within(dialog).findByText("El periodo está cerrado y ya no admite cambios.")).toBeTruthy();
    });

    it("ofrece solo las operaciones válidas según estado y periodo", async () => {
        service.getMatriculasEstudiante.mockResolvedValue({
            matriculas: [
                matricula({ id: "m-ret", estado_matricula: "RETIRADO", fecha_retiro: "2026-06-01" }),
                matricula({ id: "m-cer", periodo: P2025, estado_matricula: "MATRICULADO" }),
            ],
        });
        renderEstudiantes(ruta, { permissions: MATRICULA });
        await pestana("Historial académico");
        expect(await screen.findByRole("button", { name: "Reingreso" })).toBeTruthy();
        // Periodo cerrado: solo concluir.
        expect(screen.getAllByRole("button", { name: "Concluir" })).toHaveLength(1);
        expect(screen.queryByRole("button", { name: "Cambiar sección" })).toBeNull();
        expect(screen.queryByRole("button", { name: "Retiro o traslado" })).toBeNull();
    });

    it("sin estudiantes.matricula.update es solo lectura", async () => {
        service.getMatriculasEstudiante.mockResolvedValue({ matriculas: [matricula()] });
        renderEstudiantes(ruta);
        await pestana("Historial académico");
        expect(await screen.findByText("Fecha de matrícula")).toBeTruthy();
        expect(screen.queryByRole("button", { name: "Matricular" })).toBeNull();
        expect(screen.queryByRole("button", { name: "Retiro o traslado" })).toBeNull();
    });
});

describe("Ficha integral — atenciones psicológicas", () => {
    it("sin permiso de Psicología no consulta ni muestra nada clínico", async () => {
        renderEstudiantes(ruta);
        await pestana("Atenciones psicológicas");
        expect(screen.getByText(/información es confidencial/)).toBeTruthy();
        expect(psicologia.getExpedientePsicologicoByEstudiante).not.toHaveBeenCalled();
    });

    it("con permiso muestra solo existencia y estado, nunca el resumen del caso", async () => {
        psicologia.getExpedientePsicologicoByEstudiante.mockResolvedValue({
            expediente: { id: "x-1", codigo: "PSI-2026-001", estado: "ABIERTO", responsable_id: "u-1", fecha_apertura: "2026-04-01T10:00:00Z", resumen_caso: "TEXTO CLINICO" },
        });
        renderEstudiantes(ruta, { permissions: ["estudiantes.view", "psicologia.expedientes.view"] });
        await pestana("Atenciones psicológicas");
        expect(await screen.findByText("PSI-2026-001")).toBeTruthy();
        expect(screen.getByText("Abierto")).toBeTruthy();
        expect(screen.queryByText(/TEXTO CLINICO/)).toBeNull();
        expect(screen.getByRole("link", { name: /módulo de Psicología/ }).getAttribute("href")).toBe("/psicologia/expedientes/x-1");
    });

    it("404 = sin expediente en el alcance del usuario", async () => {
        psicologia.getExpedientePsicologicoByEstudiante.mockRejectedValue(httpError(404, "no encontrado"));
        renderEstudiantes(ruta, { permissions: ["estudiantes.view", "psicologia.expedientes.view_own"] });
        await pestana("Atenciones psicológicas");
        expect(await screen.findByText(/No hay un expediente psicológico/)).toBeTruthy();
    });
});
